<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import type { DockerContainer, LockAnimation, LockTrigger, SearchEngine, Settings } from '../types'
import { api } from '../api'
import { fail, saveSettings, state, toast } from '../store'
import { applyTheme, probeLatency } from '../utils'
import MediaBrowser from './MediaBrowser.vue'
import MediaPicker from './MediaPicker.vue'
import Picker from './Picker.vue'

const router = useRouter()

const draft = ref<Settings>(JSON.parse(JSON.stringify(state.settings)))

// 老数据兜底：新加的字段可能不存在
function fillDefaults(s: Settings) {
  s.text ??= { dark: '#f1f5f9', light: '#1e293b' }
  if (s.theme !== 'dark' && s.theme !== 'light' && s.theme !== 'auto') s.theme = 'dark'
  s.card ??= { opacity: 8, blur: 0, radius: 18, iconSize: 56, iconSpacing: 18, dotPos: 'tr', iconBg: '#ffffff', iconBgAlpha: 0, dotOffset: 8 }
  s.card.iconSize ||= 56
  s.card.iconSpacing ||= 18
  s.card.iconBg ||= '#ffffff'
  s.card.iconBgAlpha ??= 0
  s.card.dotPos ||= 'tr'
  s.card.dotOffset ??= 8
  s.page ??= { padTop: 28, padX: 24, padBottom: 40, maxWidth: 1152 }
  s.bar ??= { fixed: true, padX: 18, padTop: 14, iconSize: 34, titleSize: 17, maxWidth: 0 }
  s.clock ??= { show: true, seconds: false, format: '15:04:05', fontSize: 0, align: 'center', showDate: true }
  s.clock.format ||= '15:04:05'
  s.clock.fontSize ??= 0
  s.search.glow ??= true
  s.search.publicShow ??= true
  s.lock.fontSize ??= 0
  s.clock.align ||= 'center'
  s.clock.showDate ??= true
  s.search ??= {} as Settings['search']
  s.search.height ||= 52
  s.search.radius ??= 16
  s.search.fontSize ||= 14
  s.search.maxWidth ||= 640
  s.footerCode ??= ''
  s.tabTitle ??= s.title ?? ''
  s.lock ??= {
    enabled: true,
    animation: 'slide',
    trigger: 'both',
    showDate: true,
    greet: { dawn: '', morning: '', noon: '', afternoon: '', evening: '', night: '' },
  }
  s.lock.greet ??= { dawn: '', morning: '', noon: '', afternoon: '', evening: '', night: '' }
  s.hitokoto ??= { enabled: true, source: 'hitokoto', category: '' }
  s.sun ??= { lat: 39.9, lng: 116.4 }
  s.custom ??= { css: '', js: '' }
  s.docker ??= { enabled: false, socket: '/var/run/docker.sock' }
  // 壁纸这几个值被冲成 0 过：brightness=0 会让视频整屏全黑，看着就像壁纸失效
  s.wallpaper ??= { type: 'none', src: '', blur: 0, mask: 0.25, brightness: 1, smooth: true }
  s.wallpaper.type ||= 'none'
  s.wallpaper.brightness ||= 1
  s.wallpaper.mask ??= 0.25
  s.wallpaper.blur ??= 0
  s.probe ??= { enabled: true, lanProbe: '', showLatency: true, interval: 60, timeout: 3000 }
  s.probe.interval ||= 60
  s.probe.timeout ||= 3000
  return s
}
fillDefaults(draft.value)

const probing = ref(false)
const probeResult = ref('')

async function testLanProbe() {
  const url = draft.value.probe.lanProbe?.trim()
  if (!url) {
    probeResult.value = '先填个地址'
    return
  }
  probing.value = true
  probeResult.value = ''
  const ms = await probeLatency(url, draft.value.probe.timeout || 3000)
  probing.value = false
  probeResult.value = ms === null ? '不通，检查地址或端口' : `通了，${ms} ms`
}

const timePresets = ['15:04:05', '15:04', '15:04:05.000', '03:04:05', '15时04分', '15 点 04 分']

const tab = ref<'look' | 'search' | 'lock' | 'page' | 'media' | 'probe' | 'docker' | 'account'>('look')
const saving = ref(false)
const wpFile = ref<HTMLInputElement>()
const uploadingWp = ref(false)
const mediaOpen = ref(false)
const favOpen = ref(false)
const autoSaved = ref(false)
let autoTimer: number | null = null

const tabs = [
  { key: 'look', label: '外观' },
  { key: 'search', label: '搜索' },
  { key: 'lock', label: '锁屏' },
  { key: 'page', label: '页面' },
  { key: 'media', label: '素材库' },
  { key: 'probe', label: '探测' },
  { key: 'docker', label: 'Docker' },
  { key: 'account', label: '账号' },
] as const

const layoutOptions = [
  { v: 'grid', t: '网格' },
  { v: 'icon', t: '只图标' },
]

const themeOptions = [
  { v: 'dark', t: '深色' },
  { v: 'light', t: '浅色' },
  { v: 'auto', t: '跟随系统' },
]

const animationOptions: { v: LockAnimation; t: string }[] = [
  { v: 'slide', t: '上滑' },
  { v: 'fade', t: '淡出' },
  { v: 'zoom', t: '放大' },
  { v: 'blur', t: '模糊' },
  { v: 'curtain', t: '拉帘' },
  { v: 'flip', t: '翻转' },
]

const triggerOptions: { v: LockTrigger; t: string }[] = [
  { v: 'both', t: '点击 + 上滑' },
  { v: 'click', t: '仅点击' },
  { v: 'swipe', t: '仅上滑' },
]

const lanRulesText = ref((draft.value.lanRules ?? []).join('\n'))
const isAdmin = computed(() => state.user?.role === 'admin')

function close() {
  state.settingsOpen = false
}

// 改外观时立刻预览（不保存也能看到效果）：合成一份临时 settings 交给 applyTheme，不动 store
watch(
  () => [draft.value.theme, draft.value.card, draft.value.bar] as const,
  () => {
    if (!state.settings) return
    applyTheme(
      { ...state.settings, theme: draft.value.theme, card: draft.value.card, bar: draft.value.bar },
      true,
    )
  },
  { deep: true },
)

function collect() {
  draft.value.lanRules = lanRulesText.value
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)
}

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    collect()
    await saveSettings(draft.value)
    close()
  } catch {
    /* saveSettings 已经提示过 */
  } finally {
    saving.value = false
  }
}

// 改完就自动存，别让人忘了点保存；保存按钮留着当"立刻存+关面板"
watch(
  () => JSON.stringify(draft.value),
  () => {
    if (saving.value) return
    if (autoTimer !== null) clearTimeout(autoTimer)
    autoTimer = window.setTimeout(async () => {
      autoTimer = null
      collect()
      try {
        await saveSettings(JSON.parse(JSON.stringify(draft.value)), true)
        autoSaved.value = true
        window.setTimeout(() => (autoSaved.value = false), 1600)
      } catch {
        /* fail 已经提示 */
      }
    }, 900)
  },
)

async function uploadWallpaper(evt: Event) {
  const input = evt.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  uploadingWp.value = true
  try {
    const url = await api.upload(file, 'wallpaper')
    draft.value.wallpaper.src = url
    draft.value.wallpaper.type = file.type.startsWith('video/') ? 'video' : 'image'
    toast('壁纸已上传')
  } catch (e) {
    fail(e)
  } finally {
    uploadingWp.value = false
    input.value = ''
  }
}

/** 从素材库挑一张当壁纸 */
function setWallpaper(url: string, isVideo: boolean) {
  draft.value.wallpaper.src = url
  draft.value.wallpaper.type = isVideo ? 'video' : 'image'
  mediaOpen.value = false
  toast('已设为壁纸，记得点保存')
}

const greetFields = [
  { k: 'dawn', t: '凌晨', d: '还没睡呀' },
  { k: 'morning', t: '早上', d: '早上好' },
  { k: 'noon', t: '中午', d: '中午好' },
  { k: 'afternoon', t: '下午', d: '下午好' },
  { k: 'evening', t: '晚上', d: '晚上好' },
  { k: 'night', t: '深夜', d: '夜深了' },
] as const

const grabbing = ref('')

/** 从引擎首页抓 favicon 回填 */
async function grabIcon(e: SearchEngine) {
  if (!e.url.trim()) return toast('先把引擎地址填上')
  grabbing.value = e.key
  try {
    const origin = new URL(e.url.trim()).origin
    const meta = await api.fetchSite(origin)
    if (!meta.icon) {
      toast('这个站没抛 favicon，手动填一个吧')
      return
    }
    e.icon = new URL(meta.icon, origin).toString()
    toast('图标抓到啦')
  } catch {
    fail('抓不到，可能是站点拦了或没图标')
  } finally {
    grabbing.value = ''
  }
}

function addEngine() {
  draft.value.search.engines.push({
    key: 'e' + Date.now().toString(36),
    name: '新引擎',
    url: 'https://www.example.com/search?q=%s',
  })
}

function removeEngine(i: number) {
  draft.value.search.engines.splice(i, 1)
  if (!draft.value.search.engines.some((e) => e.key === draft.value.search.engine)) {
    draft.value.search.engine = draft.value.search.engines[0]?.key ?? ''
  }
}

// ---------- Docker ----------
const dockerLoading = ref(false)
const dockerData = ref<{ available: boolean; msg?: string; containers: DockerContainer[] }>({
  available: false,
  containers: [],
})
const dockerBusy = ref('')

async function loadDocker() {
  if (!draft.value.docker.enabled) {
    dockerData.value = { available: false, msg: '还没打开 Docker 监控', containers: [] }
    return
  }
  dockerLoading.value = true
  try {
    const r = await api.docker()
    dockerData.value = { available: r.available, msg: r.msg, containers: r.containers ?? [] }
  } catch (e) {
    dockerData.value = { available: false, msg: (e as Error).message, containers: [] }
  } finally {
    dockerLoading.value = false
  }
}

watch(tab, (v) => {
  if (v === 'docker') void loadDocker()
})
onMounted(() => {
  if (tab.value === 'docker') void loadDocker()
})

// 关面板前把还没到点的自动保存立刻补上
onBeforeUnmount(() => {
  if (autoTimer === null) return
  clearTimeout(autoTimer)
  autoTimer = null
  collect()
  void saveSettings(JSON.parse(JSON.stringify(draft.value)), true).catch(() => {})
})

async function dockerOp(c: DockerContainer, action: 'start' | 'stop' | 'restart') {
  dockerBusy.value = c.id + action
  try {
    await api.dockerAction(c.id, action)
    toast(`${c.name} 已${action === 'start' ? '启动' : action === 'stop' ? '停止' : '重启'}`)
    await loadDocker()
  } catch (e) {
    fail(e)
  } finally {
    dockerBusy.value = ''
  }
}

function stateColor(s: string) {
  if (s === 'running') return 'text-emerald-400'
  if (s === 'paused') return 'text-amber-400'
  if (s === 'restarting') return 'text-sky-400'
  return 'text-slate-400'
}

// ---------- 账号 ----------
const oldPw = ref('')
const newPw = ref('')
const changing = ref(false)
const newName = ref(state.user?.username ?? '')
const namePw = ref('')
const renaming = ref(false)

async function changeUsername() {
  if (renaming.value) return
  const name = newName.value.trim()
  if (name.length < 2) {
    toast('账号至少 2 个字符', 'err')
    return
  }
  if (!namePw.value) {
    toast('要输当前密码确认', 'err')
    return
  }
  renaming.value = true
  try {
    await api.changeUsername(name, namePw.value)
    if (state.user) state.user.username = name
    namePw.value = ''
    toast('账号已改，下次就用新账号登录')
  } catch (e) {
    fail(e)
  } finally {
    renaming.value = false
  }
}

async function changePassword() {
  if (changing.value) return
  if (newPw.value.length < 8) {
    toast('新密码至少 8 位', 'err')
    return
  }
  changing.value = true
  try {
    await api.changePassword(oldPw.value, newPw.value)
    oldPw.value = ''
    newPw.value = ''
    toast('密码已修改')
  } catch (e) {
    fail(e)
  } finally {
    changing.value = false
  }
}

async function logout() {
  try {
    await api.logout()
  } catch {
    /* 忽略 */
  }
  state.user = null
  state.readonly = true
  router.replace('/login')
}

function exportData() {
  window.open('/api/v1/export', '_blank')
}

/** 站点图标从素材库里挑一个 */
function onPickSiteIcon(url: string) {
  draft.value.favicon = url
  favOpen.value = false
}
</script>

<template>
  <div class="fixed inset-0 z-40 flex justify-end bg-black/50" @click.self="close">
    <aside class="panel-solid flex h-full w-full max-w-md flex-col border-l shadow-card">
      <header class="flex items-center justify-between border-b border-[var(--card-border)] px-5 py-4">
        <div class="flex items-center gap-2">
          <h3 class="text-base font-semibold">设置</h3>
          <Transition name="fade">
            <span v-if="autoSaved" class="rounded-md bg-emerald-500/15 px-1.5 py-0.5 text-[10px] text-emerald-400">已自动保存</span>
          </Transition>
        </div>
        <button class="grid h-8 w-8 place-items-center rounded-lg opacity-60 hover:bg-black/5 hover:opacity-100 dark:hover:bg-white/10" @click="close">✕</button>
      </header>

      <nav class="flex flex-wrap gap-1 border-b border-[var(--card-border)] px-3 py-2">
        <button
          v-for="t in tabs"
          :key="t.key"
          class="rounded-lg px-2.5 py-1.5 text-[13px] transition"
          :class="tab === t.key ? 'bg-sky-500/15 text-sky-500 dark:text-sky-200' : 'opacity-60 hover:opacity-100'"
          @click="tab = t.key as any"
        >
          {{ t.label }}
        </button>
      </nav>

      <div class="flex-1 space-y-5 overflow-y-auto px-5 py-5">
        <!-- ============ 外观 ============ -->
        <template v-if="tab === 'look'">
        <div class="rounded-xl border border-[var(--card-border)] p-3">
          <label class="label">访问权限</label>
          <div class="flex gap-2">
            <button
              class="seg-btn flex-1 rounded-xl border px-3 py-2 text-xs transition"
              :class="!draft.publicMode ? 'on' : ''"
              type="button"
              @click="draft.publicMode = false"
            >
              私有（仅登录可见）
            </button>
            <button
              class="seg-btn flex-1 rounded-xl border px-3 py-2 text-xs transition"
              :class="draft.publicMode ? 'on' : ''"
              type="button"
              @click="draft.publicMode = true"
            >
              公开（游客可看）
            </button>
          </div>
          <p class="hint">公开模式下访客能看到主页和卡片（隐藏组除外），但不能改任何东西。</p>
        </div>
          <div>
            <label class="label">站点标题（主页上显示的名字）</label>
            <input v-model="draft.title" class="field" maxlength="64" placeholder="鲸鱼导航" />
          </div>
          <div>
            <label class="label">浏览器标题（标签页上那个，留空就跟站点标题一样）</label>
            <input v-model="draft.tabTitle" class="field" maxlength="64" placeholder="不填就跟随站点标题" />
            <p class="hint">两个可以分开写：主页叫「小站」，标签页写「小站 | 个人导航」也可以。</p>
          </div>
          <div>
            <label class="label">站点图标（浏览器标签上那个小图标）</label>
            <div class="flex items-center gap-2">
              <span class="grid h-9 w-9 shrink-0 place-items-center overflow-hidden rounded-lg border border-[var(--card-border)] bg-black/10">
                <img v-if="draft.favicon" :src="draft.favicon" class="h-full w-full object-contain" alt="" />
                <span v-else class="text-[10px] opacity-50">空</span>
              </span>
              <input v-model="draft.favicon" class="field min-w-0 flex-1" placeholder="/uploads/… 或 https://…" />
              <button class="btn-ghost h-8 shrink-0 px-2 text-[11px]" @click="favOpen = true">素材库</button>
            </div>
          </div>

          <div>
            <label class="label">主题</label>
            <div class="seg">
              <button
                v-for="o in themeOptions"
                :key="o.v"
                :class="draft.theme === o.v ? 'on' : 'opacity-70 hover:opacity-100'"
                @click="draft.theme = o.v as any"
              >
                {{ o.t }}
              </button>
            </div>
            <p class="hint mt-1">浅色模式适合亮色壁纸或不铺壁纸；暗色壁纸建议配深色，不然卡片上的字会看不清。</p>
          </div>

          <div>
            <label class="label">文字颜色</label>
            <div class="grid grid-cols-1 gap-2">
              <div class="flex items-center gap-2">
                <input v-model="draft.text.dark" type="color" class="h-8 w-10 cursor-pointer rounded border border-white/10 bg-transparent" />
                <div class="min-w-0 flex-1">
                  <div class="text-xs">深色模式下的文字</div>
                  <div class="text-[10px] opacity-60">时钟、标题、卡片名字和备注都用它</div>
                </div>
                <button class="btn-ghost px-2 py-1 text-[11px]" @click="draft.text.dark = '#f1f5f9'">默认</button>
              </div>
              <div class="flex items-center gap-2">
                <input v-model="draft.text.light" type="color" class="h-8 w-10 cursor-pointer rounded border border-white/10 bg-transparent" />
                <div class="min-w-0 flex-1">
                  <div class="text-xs">浅色模式下的文字</div>
                  <div class="text-[10px] opacity-60">默认深色；浅色页面配深字才看得清</div>
                </div>
                <button class="btn-ghost px-2 py-1 text-[11px]" @click="draft.text.light = '#1e293b'">默认</button>
              </div>
            </div>
          </div>

          <div>
            <label class="label">布局</label>
            <div class="seg">
              <button
                v-for="o in layoutOptions"
                :key="o.v"
                :class="draft.layout === o.v ? 'on' : 'opacity-70 hover:opacity-100'"
                @click="draft.layout = o.v as any"
              >
                {{ o.t }}
              </button>
            </div>
          </div>

          <!-- 「只显示图标」模式的尺寸 -->
          <div v-if="draft.layout === 'icon'" class="space-y-3 rounded-xl border border-[var(--card-border)] p-3">
            <div class="text-sm font-medium">只显示图标</div>
            <div>
              <label class="label">图标大小 {{ draft.card.iconSize }}px</label>
              <input v-model.number="draft.card.iconSize" type="range" min="16" max="120" class="w-full" />
            </div>
            <div>
              <label class="label">图标之间距离 {{ draft.card.iconSpacing }}px</label>
              <input v-model.number="draft.card.iconSpacing" type="range" min="0" max="80" class="w-full" />
            </div>
            <div>
              <label class="label">图标底色 {{ draft.card.iconBgAlpha }}%</label>
              <div class="flex items-center gap-2">
                <input v-model="draft.card.iconBg" type="color" class="h-8 w-12 shrink-0 cursor-pointer rounded-md border border-[var(--card-border)] bg-transparent" />
                <input v-model.number="draft.card.iconBgAlpha" type="range" min="0" max="100" class="w-full" />
                <button class="shrink-0 text-[11px] text-sky-500 hover:underline dark:text-sky-300" @click="draft.card.iconBg = '#ffffff'; draft.card.iconBgAlpha = 100">
                  白底
                </button>
              </div>
              <p class="hint">图标本身是透明 PNG 的话，把不透明度拉高就会垫上一层底色，不会糊在壁纸上。0% 就是完全透明。</p>
            </div>
            <p class="hint">这个模式下卡片不显示名字，鼠标停上去才提示。</p>
          </div>


          <!-- 卡片外观 -->
          <div class="space-y-3 rounded-xl border border-[var(--card-border)] p-3">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">卡片外观</span>
              <button
                class="text-[11px] text-sky-500 hover:underline dark:text-sky-300"
                @click="draft.card = { opacity: 8, blur: 0, radius: 18, iconSize: 56, iconSpacing: 18, dotPos: 'tr', iconBg: '#ffffff', iconBgAlpha: 0, dotOffset: 8 }"
              >
                复位
              </button>
            </div>
            <div>
              <label class="label">透明度 {{ draft.card.opacity }}%</label>
              <input v-model.number="draft.card.opacity" type="range" min="0" max="100" class="w-full" />
            </div>
            <div>
              <label class="label">磨砂 {{ draft.card.blur }}px<span class="ml-1 opacity-50">（0 = 不用磨砂）</span></label>
              <input v-model.number="draft.card.blur" type="range" min="0" max="40" class="w-full" />
            </div>
            <div>
              <label class="label">圆角 {{ draft.card.radius }}px</label>
              <input v-model.number="draft.card.radius" type="range" min="0" max="40" class="w-full" />
            </div>
            <div>
              <label class="label">延迟小点位置</label>
              <div class="seg">
                <button
                  v-for="o in [
                    { v: 'tl', t: '左上' },
                    { v: 'tr', t: '右上' },
                    { v: 'bl', t: '左下' },
                    { v: 'br', t: '右下' },
                  ]"
                  :key="o.v"
                  :class="draft.card.dotPos === o.v ? 'on' : 'opacity-70 hover:opacity-100'"
                  @click="draft.card.dotPos = o.v as any"
                >
                  {{ o.t }}
                </button>
              </div>
            </div>
            <div>
              <label class="label">小点离边缘 {{ draft.card.dotOffset }}px</label>
              <input v-model.number="draft.card.dotOffset" type="range" min="0" max="40" class="w-full" />
            </div>
          </div>

          <!-- 壁纸 -->
          <div class="space-y-2 rounded-xl border border-[var(--card-border)] p-3">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">壁纸</span>
              <div class="seg">
                <button
                  v-for="o in [
                    { v: 'none', t: '无' },
                    { v: 'image', t: '图片' },
                    { v: 'video', t: '视频' },
                  ]"
                  :key="o.v"
                  :class="draft.wallpaper.type === o.v ? 'on' : 'opacity-70 hover:opacity-100'"
                  @click="draft.wallpaper.type = o.v as any"
                >
                  {{ o.t }}
                </button>
              </div>
            </div>
            <input v-model="draft.wallpaper.src" class="field" placeholder="壁纸地址 / 视频地址" />
            <div class="flex items-center gap-2">
              <button class="btn-ghost h-7 px-2 text-[11px]" @click="mediaOpen = true">素材库</button>
              <button class="btn-ghost h-7 px-2 text-[11px]" :disabled="uploadingWp" @click="wpFile?.click()">
                {{ uploadingWp ? '上传中…' : '上传壁纸' }}
              </button>
              <input ref="wpFile" type="file" accept="image/*,video/mp4,video/webm" class="hidden" @change="uploadWallpaper" />
            </div>
            <div>
              <label class="label">模糊 {{ draft.wallpaper.blur }}px</label>
              <input v-model.number="draft.wallpaper.blur" type="range" min="0" max="24" class="w-full" />
            </div>
            <div>
              <label class="label">遮罩 {{ Math.round((draft.wallpaper.mask ?? 0) * 100) }}%</label>
              <input v-model.number="draft.wallpaper.mask" type="range" min="0" max="1" step="0.05" class="w-full" />
            </div>
            <div>
              <label class="label">亮度 {{ draft.wallpaper.brightness ?? 1 }}</label>
              <input v-model.number="draft.wallpaper.brightness" type="range" min="0.2" max="1.4" step="0.05" class="w-full" />
            </div>
            <label class="flex items-center gap-2 text-xs">
              <input v-model="draft.wallpaper.smooth" type="checkbox" class="h-4 w-4 accent-sky-500" />
              流畅模式（手机上视频壁纸只停在首帧，不播放；不勾就一直放）
            </label>
          </div>

          <!-- 时钟 -->
          <div class="space-y-3 rounded-xl border border-[var(--card-border)] p-3">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">时钟</span>
              <button
                class="text-[11px] text-sky-500 hover:underline dark:text-sky-300"
                @click="draft.clock = { ...draft.clock, show: true, seconds: false, format: '15:04:05', fontSize: 0, align: 'center', showDate: true }"
              >
                复位
              </button>
            </div>

            <div class="grid grid-cols-2 gap-2">
              <label class="flex items-center gap-2 text-xs">
                <input v-model="draft.clock.show" type="checkbox" class="h-4 w-4 accent-sky-500" />
                显示时钟
              </label>
              <label class="flex items-center gap-2 text-xs">
                <input v-model="draft.clock.showDate" type="checkbox" class="h-4 w-4 accent-sky-500" />
                显示日期
              </label>
              <label class="flex items-center gap-2 text-xs">
                <input v-model="draft.clock.seconds" type="checkbox" class="h-4 w-4 accent-sky-500" />
                显示秒
              </label>
            </div>

            <div>
              <label class="label">时间格式（Go 那套：15=时 04=分 05=秒）</label>
              <div class="flex gap-2">
                <input v-model="draft.clock.format" class="field font-mono" placeholder="15:04:05" maxlength="32" />
                <Picker
                  class="w-32 shrink-0"
                  :model-value="draft.clock.format"
                  :options="timePresets.map((f) => ({ v: f, t: f }))"
                  @update:model-value="draft.clock.format = $event"
                />
              </div>
              <p class="hint">例：<code class="field-code">15:04</code> 只有时分，<code class="field-code">15 时 04 分</code> 也行。</p>
            </div>

            <div>
              <label class="label">整体大小 {{ draft.clock.fontSize || '自适应' }}{{ draft.clock.fontSize ? 'px' : '' }}</label>
              <input v-model.number="draft.clock.fontSize" type="range" min="0" max="160" step="1" class="w-full" />
              <p class="hint">时间和日期一起缩放；数字越小字越小，0 按屏幕自适应。</p>
            </div>

            <div>
              <label class="label">对齐</label>
              <div class="seg">
                <button :class="{ on: draft.clock.align === 'left' }" @click="draft.clock.align = 'left'">居左</button>
                <button :class="{ on: draft.clock.align === 'center' }" @click="draft.clock.align = 'center'">居中</button>
                <button :class="{ on: draft.clock.align === 'right' }" @click="draft.clock.align = 'right'">居右</button>
              </div>
            </div>
          </div>

        </template>

        <!-- ============ 搜索 ============ -->
        <template v-else-if="tab === 'search'">
          <label class="flex items-center gap-2 text-sm">
            <input v-model="draft.search.show" type="checkbox" class="h-4 w-4 accent-sky-500" />
            显示搜索框
          </label>
          <label class="flex items-center gap-2 text-xs text-[var(--text-muted,rgba(241,245,249,0.72))]">
            <input v-model="draft.search.publicShow" type="checkbox" class="h-4 w-4 accent-sky-500" />
            公开（未登录）时也显示搜索框
          </label>
          <div class="space-y-3 rounded-xl border border-[var(--card-border)] p-3">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">搜索框外观</span>
              <button class="text-[11px] text-sky-500 hover:underline dark:text-sky-300" @click="draft.search.height = 52; draft.search.radius = 16; draft.search.fontSize = 14; draft.search.maxWidth = 640">
                复位
              </button>
            </div>
            <div>
              <label class="label">高度 {{ draft.search.height }}px</label>
              <input v-model.number="draft.search.height" type="range" min="24" max="120" class="w-full" />
            </div>
            <div>
              <label class="label">圆角 {{ draft.search.radius }}px</label>
              <input v-model.number="draft.search.radius" type="range" min="0" max="48" class="w-full" />
            </div>
            <div>
              <label class="label">字号 {{ draft.search.fontSize }}px</label>
              <input v-model.number="draft.search.fontSize" type="range" min="10" max="28" class="w-full" />
            </div>
            <div>
              <label class="label">最大宽度 {{ draft.search.maxWidth }}px</label>
              <input v-model.number="draft.search.maxWidth" type="range" min="280" max="1400" step="10" class="w-full" />

              <label class="mt-3 flex items-center gap-2 text-xs text-[var(--text-muted,rgba(241,245,249,0.72))]">
                <input v-model="draft.search.glow" type="checkbox" class="h-4 w-4 accent-sky-500" />
                搜索框边框淡淡泛光
              </label>
            </div>
          </div>

          <div class="space-y-2">
            <label class="label">搜索引擎（%s 会被替换成关键词）</label>
            <div v-for="(e, i) in draft.search.engines" :key="e.key" class="rounded-xl border border-[var(--card-border)] p-2">
              <div class="mb-1.5 flex gap-2">
                <input v-model="e.name" class="field h-8 text-xs" placeholder="名称" />
                <button
                  class="grid h-8 w-8 shrink-0 place-items-center rounded-lg opacity-50 hover:bg-rose-500/10 hover:text-rose-400 hover:opacity-100"
                  @click="removeEngine(i)"
                >
                  ✕
                </button>
              </div>
              <input v-model="e.url" class="field h-8 text-xs" placeholder="https://…?q=%s" />
              <div class="mt-1.5 flex gap-1.5">
                <div class="grid h-8 w-8 shrink-0 place-items-center overflow-hidden rounded-lg border border-[var(--card-border)]">
                  <img
                    v-if="e.icon"
                    :src="e.icon"
                    class="h-4 w-4 object-contain"
                    alt=""
                    referrerpolicy="no-referrer"
                    @error="e.icon = ''"
                  />
                  <span v-else class="text-[10px] opacity-40">{{ e.name.slice(0, 1) || '?' }}</span>
                </div>
                <input v-model="e.icon" class="field h-8 flex-1 text-[11px]" placeholder="图标地址（可留空）" />
                <button
                  class="h-8 shrink-0 rounded-lg border border-[var(--card-border)] px-2 text-[11px] opacity-70 hover:opacity-100"
                  :disabled="grabbing === e.key"
                  @click="grabIcon(e)"
                >
                  {{ grabbing === e.key ? '抓取中…' : '抓图标' }}
                </button>
              </div>
              <label class="mt-1.5 flex items-center gap-1.5 text-[11px] opacity-70">
                <input v-model="draft.search.engine" type="radio" :value="e.key" class="accent-sky-500" />
                默认用这个
              </label>
            </div>
            <button class="text-xs text-sky-500 hover:underline dark:text-sky-300" @click="addEngine">+ 加一个引擎</button>
          </div>
        </template>

        <!-- ============ 素材库 ============ -->
        <template v-else-if="tab === 'media'">
          <p class="hint">上传过的图片和视频都在这儿，点一下直接设成主页壁纸；还被卡片或壁纸引用的文件删不掉。</p>
          <div class="flex h-[56vh] flex-col">
            <MediaBrowser kind="all" purpose="wallpaper" @wallpaper="setWallpaper" />
          </div>
        </template>

        <!-- ============ 锁屏 ============ -->
        <template v-else-if="tab === 'lock'">
          <label class="flex items-center justify-between gap-2 text-sm">
            <span>进主页先锁屏（解锁后才看卡片）</span>
            <input v-model="draft.lock.enabled" type="checkbox" class="h-4 w-4 accent-sky-500" />
          </label>
          <p class="hint">公开模式下访客进来也会先看到锁屏，解锁后才看卡片。</p>

          <div class="space-y-2 rounded-xl border border-[var(--card-border)] p-3">
            <label class="label">解锁动画</label>
            <div class="seg">
              <button
                v-for="o in animationOptions"
                :key="o.v"
                :class="draft.lock.animation === o.v ? 'on' : 'opacity-70 hover:opacity-100'"
                @click="draft.lock.animation = o.v"
              >
                {{ o.t }}
              </button>
            </div>
            <p class="hint">保存后回主页就能看到效果。</p>
          </div>

          <div>
            <label class="label">触发方式</label>
            <div class="seg">
              <button
                v-for="o in triggerOptions"
                :key="o.v"
                :class="draft.lock.trigger === o.v ? 'on' : 'opacity-70 hover:opacity-100'"
                @click="draft.lock.trigger = o.v"
              >
                {{ o.t }}
              </button>
            </div>
          </div>

          <label class="flex items-center gap-2 text-sm">
            <input v-model="draft.lock.showDate" type="checkbox" class="h-4 w-4 accent-sky-500" />
            时间下面显示日期
          </label>

          <label class="label mt-1">锁屏时间字号 {{ draft.lock.fontSize || '自适应' }}{{ draft.lock.fontSize ? 'px' : '' }}</label>
          <input v-model.number="draft.lock.fontSize" type="range" min="0" max="160" step="1" class="w-full" />
          <p class="hint">数字越小字越小，0 跟随屏幕自适应。</p>

          <div class="space-y-2 rounded-xl border border-[var(--card-border)] p-3">
            <label class="label">底部问候（按时段取，留空用默认）</label>
            <div class="space-y-1.5">
              <div v-for="g in greetFields" :key="g.k" class="flex items-center gap-2">
                <span class="w-14 shrink-0 text-[11px] opacity-60">{{ g.t }}</span>
                <input
                  v-model="draft.lock.greet[g.k]"
                  class="field h-8 text-xs"
                  maxlength="20"
                  :placeholder="g.d"
                />
              </div>
            </div>
          </div>

          <!-- 一言 -->
          <div class="space-y-2 rounded-xl border border-[var(--card-border)] p-3">
            <label class="flex items-center justify-between gap-2 text-sm">
              <span>右上角显示一言</span>
              <input v-model="draft.hitokoto.enabled" type="checkbox" class="h-4 w-4 accent-sky-500" />
            </label>
            <div>
              <label class="label">类别（留空 = 随机）</label>
              <input v-model="draft.hitokoto.category" class="field h-8 text-xs" placeholder="a 动画 / b 漫画 / d 诗词 / i 网易云 / k 哲学" />
            </div>
            <p class="hint">接的是 hitokoto.cn。每 5 分钟、切回窗口时刷新一次。</p>
          </div>
        </template>

        <!-- ============ 页面 ============ -->
        <template v-else-if="tab === 'page'">
          <!-- 顶栏：贴浏览器窗口边缘，和主页留白分开管 -->
          <div class="space-y-3 rounded-xl border border-[var(--card-border)] p-3">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">顶栏（标题 + 按钮）</span>
              <label class="flex cursor-pointer items-center gap-1.5 text-[11px] opacity-80">
                <input v-model="draft.bar.fixed" type="checkbox" />
                贴在窗口边上
              </label>
            </div>
            <div>
              <label class="label">左右留白 {{ draft.bar.padX }}px</label>
              <input v-model.number="draft.bar.padX" type="range" min="0" max="120" class="w-full" />
            </div>
            <div>
              <label class="label">上留白 {{ draft.bar.padTop }}px</label>
              <input v-model.number="draft.bar.padTop" type="range" min="0" max="80" class="w-full" />
            </div>
            <div>
              <label class="label">按钮大小 {{ draft.bar.iconSize }}px</label>
              <input v-model.number="draft.bar.iconSize" type="range" min="18" max="52" class="w-full" />
            </div>
            <div>
              <label class="label">标题字号 {{ draft.bar.titleSize }}px</label>
              <input v-model.number="draft.bar.titleSize" type="range" min="10" max="30" class="w-full" />
            </div>
            <div>
              <label class="label">最大宽度 {{ draft.bar.maxWidth || '铺满' }}{{ draft.bar.maxWidth ? 'px' : '' }}</label>
              <input v-model.number="draft.bar.maxWidth" type="range" min="0" max="2400" step="16" class="w-full" />
              <p class="hint">0 = 顶栏铺满整个窗口；填个数值就限宽并居中，跟主页留白的「内容最大宽度」互不影响。</p>
            </div>
            <p class="hint">这几个值只管顶栏，不受下面的「主页留白」影响；手机上也会自动贴住屏幕边。</p>
          </div>

          <div class="space-y-3 rounded-xl border border-[var(--card-border)] p-3">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">主页留白</span>
              <button class="text-[11px] text-sky-500 hover:underline dark:text-sky-300" @click="draft.page = { padTop: 28, padX: 24, padBottom: 40, maxWidth: 1152 }">
                复位
              </button>
            </div>
            <div>
              <label class="label">上边距 {{ draft.page.padTop }}px</label>
              <input v-model.number="draft.page.padTop" type="range" min="0" max="160" class="w-full" />
            </div>
            <div>
              <label class="label">左右边距 {{ draft.page.padX }}px</label>
              <input v-model.number="draft.page.padX" type="range" min="0" max="160" class="w-full" />
            </div>
            <div>
              <label class="label">下边距 {{ draft.page.padBottom }}px</label>
              <input v-model.number="draft.page.padBottom" type="range" min="0" max="240" class="w-full" />
            </div>
            <div>
              <label class="label">内容最大宽度 {{ draft.page.maxWidth }}px</label>
              <input v-model.number="draft.page.maxWidth" type="range" min="320" max="2400" step="16" class="w-full" />
            </div>
          </div>

          <div>
            <label class="label">页脚（HTML / CSS / JS 都写这一个框）</label>
            <textarea
              v-model="draft.footerCode"
              rows="9"
              class="field font-mono text-xs"
              spellcheck="false"
              placeholder="<p>© {year} 鲸鱼导航</p>
<style>
.wavenav-foot a { color: #7dd3fc }
/* seg-btn 没有全局定义，样式一直只在 ItemEditor 的 scoped 里 —— 抽屉里的按钮全靠默认黑字白底
   浅色下 on 态给深色可读配色，非 on 给次要灰 */
.seg-btn { color: var(--text-main, #e8eeff); }
html:not(.dark) .seg-btn { color: #1e293b; }
.seg-btn.on {
  color: #7dd3fc;
  border-color: rgba(56, 189, 248, 0.55);
  background: rgba(56, 189, 248, 0.12);
}
html:not(.dark) .seg-btn.on {
  color: #0369a1;
  border-color: rgba(3, 105, 161, 0.55);
  background: rgba(56, 189, 248, 0.15);
}
</style>
<script>document.querySelectorAll('.wavenav-foot a').forEach(a => a.target = '_blank')</script>"
            />
          </div>
          <p class="hint">
            把 <code class="field-code">&lt;style&gt;</code> 和 <code class="field-code">&lt;script&gt;</code> 直接写在里面就行，会自动拆出来挂到页面上。
            HTML 里可用：<code class="field-code">{year}</code> <code class="field-code">{month}</code>
            <code class="field-code">{day}</code> <code class="field-code">{date}</code>
            <code class="field-code">{time}</code> <code class="field-code">{title}</code>。
          </p>

          <!-- 自定义代码 -->
          <div class="space-y-2 rounded-xl border border-[var(--card-border)] p-3">
            <div class="text-sm font-medium">自定义代码</div>
            <div>
              <label class="label">主页 CSS</label>
              <textarea v-model="draft.custom.css" class="field-code" spellcheck="false" placeholder="/* .card-hover { } */" />
            </div>
            <div>
              <label class="label">主页 JS</label>
              <textarea v-model="draft.custom.js" class="field-code" spellcheck="false" placeholder="// 加载完成后执行一次" />
            </div>
            <p class="hint">保存后立刻生效。JS 只在内容变化时执行一次，别写死循环。</p>
          </div>

        </template>

        <!-- ============ 探测 ============ -->
        <template v-else-if="tab === 'probe'">
          <div class="space-y-3 rounded-xl border border-[var(--card-border)] p-3">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">前台可达性探测</span>
              <label class="flex cursor-pointer items-center gap-1.5 text-[11px] opacity-80">
                <input v-model="draft.probe.enabled" type="checkbox" />
                启用
              </label>
            </div>
            <p class="hint">在浏览器里探一遍每张卡片的地址，右上角的小点是结果</p>
            <label class="flex cursor-pointer items-center gap-1.5 text-[11px] opacity-80">
              <input v-model="draft.probe.showLatency" type="checkbox" />
              在图标右上角显示延迟（绿 &lt;150ms / 黄 &lt;600ms / 红 不通）
            </label>
          </div>

          <div class="space-y-3 rounded-xl border border-[var(--card-border)] p-3">
            <span class="text-sm font-medium">内网自动切换</span>
            <p class="hint">填一个只有内网能通的地址（比如你的 NAS：http://192.168.1.2:5666）。能连上就当成在家，卡片自动走内网地址。</p>
            <input
              v-model="draft.probe.lanProbe"
              class="field"
              placeholder="http://192.168.1.2:5666"
            />
            <div class="flex items-center gap-2">
              <button class="btn-ghost text-[12px]" :disabled="probing" @click="testLanProbe">
                {{ probing ? '正在测…' : '测一下' }}
              </button>
              <span v-if="probeResult" class="text-[11px]" :class="probeResult.startsWith('通') ? 'text-emerald-400' : 'text-rose-400'">
                {{ probeResult }}
              </span>
            </div>
            <p class="hint">留空就按访问地址猜（私有网段算内网）。</p>
          </div>

          <div>
            <label class="label">内网网段（一行一个 CIDR，用来判断该开内网还是外网地址）</label>
            <textarea v-model="lanRulesText" rows="5" class="field font-mono text-xs" placeholder="192.168.0.0/16" />
          </div>

          <div class="space-y-3 rounded-xl border border-[var(--card-border)] p-3">
            <span class="text-sm font-medium">节奏</span>
            <div>
              <label class="label">探测间隔 {{ draft.probe.interval }} 秒</label>
              <input v-model.number="draft.probe.interval" type="range" min="10" max="600" step="10" class="w-full" />
            </div>
            <div>
              <label class="label">单次超时 {{ draft.probe.timeout }} ms</label>
              <input v-model.number="draft.probe.timeout" type="range" min="500" max="15000" step="500" class="w-full" />
            </div>
            <p class="hint">只在前台且页面可见时探测，切到后台会自动停。</p>
          </div>
        </template>

        <template v-else-if="tab === 'docker'">
          <label class="flex items-center justify-between gap-2 text-sm">
            <span>监控 Docker 容器</span>
            <input v-model="draft.docker.enabled" type="checkbox" class="h-4 w-4 accent-sky-500" @change="loadDocker" />
          </label>
          <div>
            <label class="label">Docker Socket</label>
            <input v-model="draft.docker.socket" class="field font-mono text-xs" placeholder="/var/run/docker.sock 或 tcp://10.11.12.13:2375" />
            <div class="mt-1.5 flex flex-wrap gap-1.5">
              <button class="btn-ghost h-6 px-2 text-[11px]" @click="draft.docker.socket = '/var/run/docker.sock'">本机 socket</button>
              <button class="btn-ghost h-6 px-2 text-[11px]" @click="draft.docker.socket = 'tcp://10.11.12.13:2375'">走代理（推荐）</button>
            </div>
            <p class="hint">容器里没挂 socket 时，填代理地址走只读转发：<code class="field-code">tcp://主机IP:2375</code>。</p>
          </div>

          <div class="flex items-center justify-between">
            <span class="text-sm font-medium">容器</span>
            <button class="btn-ghost h-7 px-2 text-[11px]" :disabled="dockerLoading" @click="loadDocker">
              {{ dockerLoading ? '读取中…' : '刷新' }}
            </button>
          </div>

          <p v-if="dockerData.msg" class="rounded-xl border border-amber-400/30 bg-amber-400/10 px-3 py-2 text-[11px] leading-relaxed text-amber-600 dark:text-amber-300">
            {{ dockerData.msg }}
          </p>

          <div v-if="dockerData.containers.length" class="space-y-2">
            <div
              v-for="c in dockerData.containers"
              :key="c.id"
              class="rounded-xl border border-[var(--card-border)] p-2.5"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <div class="truncate text-sm font-medium">{{ c.name }}</div>
                  <div class="truncate text-[11px] opacity-50">{{ c.image }}</div>
                </div>
                <span class="shrink-0 text-[11px]" :class="stateColor(c.state)">● {{ c.state }}</span>
              </div>
              <div v-if="c.project" class="mt-1 text-[10px] opacity-40">compose: {{ c.project }}</div>
              <div class="mt-2 flex gap-1.5">
                <button
                  v-if="c.state !== 'running'"
                  class="btn-ghost h-6 px-2 text-[11px]"
                  :disabled="dockerBusy === c.id + 'start'"
                  @click="dockerOp(c, 'start')"
                >
                  启动
                </button>
                <button
                  v-else
                  class="btn-ghost h-6 px-2 text-[11px]"
                  :disabled="dockerBusy === c.id + 'stop'"
                  @click="dockerOp(c, 'stop')"
                >
                  停止
                </button>
                <button
                  class="btn-ghost h-6 px-2 text-[11px]"
                  :disabled="dockerBusy === c.id + 'restart'"
                  @click="dockerOp(c, 'restart')"
                >
                  重启
                </button>
              </div>
            </div>
          </div>
          <p v-else-if="!dockerLoading && draft.docker.enabled && !dockerData.msg" class="text-xs opacity-50">
            一个容器都没有。
          </p>
        </template>

        <!-- ============ 账号 ============ -->
        <template v-else>
          <div class="rounded-xl border border-[var(--card-border)] p-3">
            <div class="text-sm font-medium">{{ state.user?.nickname || state.user?.username }}</div>
            <div class="mt-0.5 text-xs opacity-50">@{{ state.user?.username }} · {{ state.user?.role }}</div>
          </div>

          <div class="space-y-2">
            <label class="label">修改账号</label>
            <input v-model="newName" class="field" placeholder="新账号（字母 / 数字 / 中文，2-32 位）" autocomplete="username" />
            <input v-model="namePw" type="password" class="field" placeholder="当前密码（确认）" autocomplete="current-password" />
            <button class="btn-ghost h-8 text-xs" :disabled="renaming" @click="changeUsername">
              {{ renaming ? '提交中…' : '改账号' }}
            </button>
          </div>

          <div class="space-y-2 pt-2">
            <label class="label">修改密码</label>
            <input v-model="oldPw" type="password" class="field" placeholder="当前密码" autocomplete="current-password" />
            <input v-model="newPw" type="password" class="field" placeholder="新密码（至少 8 位）" autocomplete="new-password" />
            <button class="btn-ghost h-8 text-xs" :disabled="changing" @click="changePassword">
              {{ changing ? '提交中…' : '改密码' }}
            </button>
          </div>

          <div class="space-y-2 pt-2">
            <button class="btn-ghost h-8 w-full text-xs" @click="exportData">导出数据（JSON）</button>
            <button class="btn h-8 w-full text-xs text-rose-400 hover:bg-rose-500/10" @click="logout">退出登录</button>
          </div>
        </template>
      </div>

      <footer class="flex items-center justify-end gap-2 border-t border-[var(--card-border)] px-5 py-3">
        <button class="btn-ghost h-9" @click="close">关闭</button>
        <button class="btn-primary h-9" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
      </footer>
      <MediaPicker
        v-if="mediaOpen"
        kind="all"
        purpose="wallpaper"
        title="选壁纸"
        @wallpaper="setWallpaper"
        @close="mediaOpen = false"
      />
      <MediaPicker v-if="favOpen" kind="image" title="选站点图标" @pick="onPickSiteIcon" @close="favOpen = false" />
    </aside>
  </div>
</template>
