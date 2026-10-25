<script setup lang="ts">
import Sortable from 'sortablejs'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '../api'
import {
  fail,
  fallbackSettings,
  loadPanel,
  refreshHitokoto,
  sortGroups,
  startHitokotoLoop,
  probeAll,
  startProbeLoop,
  startStatsLoop,
  state,
  stopHitokotoLoop,
  stopProbeLoop,
  stopStatsLoop,
  toast,
} from '../store'
import { applyTheme, footerMarkup } from '../utils'
import Background from '../components/Background.vue'
import Clock from '../components/Clock.vue'
import SearchBox from '../components/SearchBox.vue'
import GroupSection from '../components/GroupSection.vue'
import ItemEditor from '../components/ItemEditor.vue'
import SettingsDrawer from '../components/SettingsDrawer.vue'
import LockScreen from '../components/LockScreen.vue'
import LoginModal from '../components/LoginModal.vue'

const ready = ref(false)
const booting = ref(true)
const unlocked = ref(false)
const lockVisible = ref(false)
const loginOpen = ref(false)
let schemeQuery: MediaQueryList | null = null

const footerHtml = computed(() => footerMarkup(state.settings))

// 主页内容区四边留白都可调，锁屏不受影响
// 顶栏自己贴窗口边，跟「主页留白」各管各的
const barCfg = computed(
  () => state.settings?.bar ?? { fixed: true, padX: 18, padTop: 14, iconSize: 34, titleSize: 17, maxWidth: 0 },
)
// 搜索框：公开（未登录）时默认也显示，除非在设置里关掉「公开显示」
const showSearch = computed(() => {
  const s = state.settings?.search
  if (!s?.show) return false
  if (state.readonly) return s.publicShow !== false
  return true
})
const barStyle = computed(() => {
  const style: Record<string, string> = {
    // 顺手让开手机的刘海/圆角，不然贴在边上会被吃掉一截
    paddingLeft: `calc(${barCfg.value.padX}px + env(safe-area-inset-left, 0px))`,
    paddingRight: `calc(${barCfg.value.padX}px + env(safe-area-inset-right, 0px))`,
    paddingTop: `calc(${barCfg.value.padTop}px + env(safe-area-inset-top, 0px))`,
  }
  // 顶栏宽度独立：用 bar.maxWidth（0 = 铺满），绝不跟主页留白的 maxWidth 混在一起
  if (barCfg.value.maxWidth > 0) {
    style.maxWidth = `${barCfg.value.maxWidth}px`
    style.marginInline = 'auto'
  }
  return style
})
// 固定顶栏之后，内容区得往下让出它的高度
// 补的高度就是顶栏内容本身那一行，主页自己的 paddingTop 仍然生效
const barSpacer = computed(() =>
  barCfg.value.fixed ? { height: `${barCfg.value.padTop + barCfg.value.iconSize + 6}px` } : {},
)

const pageStyle = computed(() => {
  const p = state.settings?.page
  if (!p) return {}
  return {
    paddingLeft: `${p.padX}px`,
    paddingRight: `${p.padX}px`,
    paddingTop: `${p.padTop}px`,
    paddingBottom: `${p.padBottom}px`,
    maxWidth: `${p.maxWidth}px`,
  }
})

let bootRetries = 0

async function boot() {
  booting.value = true
  let pendingRetry = false
  try {
    await loadPanel()
    bootRetries = 0
    ready.value = true
    // 锁屏对所有人都生效：公开模式下访客也得先解锁
    if (state.settings?.lock.enabled) {
      unlocked.value = false
      lockVisible.value = true
    } else {
      unlocked.value = true
    }
    if (state.settings?.hitokoto.enabled) startHitokotoLoop()
    startProbeLoop()
    startStatsLoop()
    void initGroupsSortable()
  } catch (e) {
    // 后端刚好在重启（网络失败）时别把页面钉死在默认壳上：3s 后自动重拉，最多 3 次
    const msg = String((e as Error)?.message || e)
    if (!msg.includes('401') && bootRetries < 3) {
      bootRetries++
      pendingRetry = true
      window.setTimeout(() => void boot(), 3000)
    } else {
      bootRetries = 0
      // 没登录又关了公开模式：用默认外观铺底，弹一个登录浮层，背后照样是主页
      state.settings = fallbackSettings()
      state.groups = []
      state.user = null
      state.readonly = true
      state.editing = false
      applyTheme(state.settings)
      ready.value = true
      unlocked.value = true
      lockVisible.value = false
      loginOpen.value = true
    }
  } finally {
    booting.value = pendingRetry
  }
}
const groupsEl = ref<HTMLElement>()
let groupsSortable: Sortable | null = null

// boot 里的 nextTick 可能跑在 <main>（v-if="booting" 结束后）首次渲染之前，
// 导致 groupsEl 还是空、Sortable 没建起来 —— 盯住 ref，元素一出现就补挂
watch(groupsEl, (el: HTMLElement | undefined) => {
  if (el && !groupsSortable) void initGroupsSortable()
})

async function initGroupsSortable() {
  await nextTick()
  if (!groupsEl.value || groupsSortable) return
  groupsSortable = Sortable.create(groupsEl.value, {
    handle: '.group-drag',
    animation: 180,
    draggable: '.group-section',
    ghostClass: 'opacity-40',
    // 原生 HTML5 拖拽在部分环境不稳定（draggable 属性时机/嵌套列表），统一走 JS 回退，真实鼠标与模拟鼠标都能拖
    forceFallback: true,
    fallbackOnBody: true,
    fallbackClass: 'group-dragging',
    onEnd: async () => {
      const order = [...(groupsEl.value?.querySelectorAll('.group-section') ?? [])].map(
        (el) => Number((el as HTMLElement).dataset.groupId),
      )
      const map = new Map(state.groups.map((g) => [g.id, g]))
      const next = order.map((id) => map.get(id)).filter(Boolean) as typeof state.groups
      if (next.length !== state.groups.length) return
      state.groups.splice(0, state.groups.length, ...next)
      try {
        await api.reorder({ groups: next.map((g, i) => ({ id: g.id, sort: i + 1 })) })
      } catch {
        /* 顺序本地已生效，失败不打断 */
      }
    },
  })
}

onMounted(() => {
  void boot()
  // 跟随系统：系统切了明暗就立刻重算一遍，不用刷新
  schemeQuery = window.matchMedia('(prefers-color-scheme: dark)')
  schemeQuery.addEventListener('change', onSchemeChange)
  document.addEventListener('visibilitychange', onVisible)
})

onBeforeUnmount(() => {
  if (groupsSortable) {
    groupsSortable.destroy()
    groupsSortable = null
  }
  if (schemeQuery) schemeQuery.removeEventListener('change', onSchemeChange)
  stopHitokotoLoop()
  stopProbeLoop()
  stopStatsLoop()
  document.removeEventListener('visibilitychange', onVisible)
})

/** 系统明暗变化时，auto 模式跟着走 */
function onSchemeChange() {
  if (state.settings?.theme === 'auto') applyTheme(state.settings)
}

function onVisible() {
  if (document.visibilityState === 'visible') {
    void refreshHitokoto()
    void probeAll()
  }
}

function onUnlocked() {
  lockVisible.value = false
  unlocked.value = true
}

function toggleEdit() {
  state.editing = !state.editing
  if (!state.editing) toast('编辑模式已关闭')
}

async function addGroup() {
  const name = window.prompt('新分组叫什么？')
  if (!name?.trim()) return
  try {
    const res = await api.createGroup({ name: name.trim() })
    state.groups.push({ ...res.group, items: [] })
    state.groups = sortGroups(state.groups)
    toast('分组已创建')
  } catch (e) {
    fail(e)
  }
}

function openItemEditor(id: number) {
  state.editingItem = id
  state.itemEditorOpen = true
}

function login() {
  loginOpen.value = true
}

async function onLoggedIn() {
  loginOpen.value = false
  try {
    await loadPanel()
  } catch (e) {
    fail(e)
    return
  }
  state.editing = false
  const lock = !!state.settings?.lock.enabled
  unlocked.value = !lock
  lockVisible.value = lock
  if (state.settings?.hitokoto.enabled) startHitokotoLoop()
  startProbeLoop()
  toast('欢迎回来')
}

/** 退出登录：公开模式退回访客视图（并重新上锁），否则回登录页 */
async function logout() {
  try {
    await api.logout()
  } catch {
    /* 已经没会话也算退出成功 */
  }
  state.user = null
  state.readonly = true
  state.editing = false
  state.settingsOpen = false
  state.itemEditorOpen = false
  try {
    await loadPanel()
    unlocked.value = false
    lockVisible.value = !!state.settings?.lock.enabled
    toast('已退出登录')
  } catch {
    // 退到访客状态，重新弹登录层
    state.settings = fallbackSettings()
    state.groups = []
    state.readonly = true
    applyTheme(state.settings)
    unlocked.value = true
    lockVisible.value = false
    loginOpen.value = true
  }
}

</script>

<template>
  <div class="relative min-h-full">
    <Background />

    <div v-if="booting" class="grid min-h-screen place-items-center">
      <div class="animate-pulse text-sm text-slate-400">加载中…</div>
    </div>

    <template v-else-if="ready">
      <!-- 主界面：解锁后才出现 -->
      <div
        v-show="unlocked"
        class="relative z-10 flex min-h-screen w-full flex-col"
        :class="unlocked ? 'content-in' : ''"
      >
        <!-- 顶栏固定后脱离文档流，这里补一个等高的空位 -->
        <div v-if="barCfg.fixed" :style="barSpacer" class="shrink-0" aria-hidden="true" />
        <!-- 顶栏：贴在浏览器窗口边缘，不受主页留白影响 -->
        <header
          class="flex items-start justify-between gap-3"
          :class="barCfg.fixed ? 'fixed inset-x-0 top-0 z-40' : 'mx-auto w-full'"
          :style="barStyle"
        >
          <div class="flex min-w-0 items-center gap-2.5">
            <img
              v-if="state.settings?.favicon"
              :src="state.settings.favicon"
              class="rounded-lg object-contain" style="width: calc(var(--bar-icon, 34px) * 0.82); height: calc(var(--bar-icon, 34px) * 0.82)"
              alt=""
            />
            <div v-else class="grid place-items-center rounded-lg bg-sky-500/20" style="width: calc(var(--bar-icon, 34px) * 0.82); height: calc(var(--bar-icon, 34px) * 0.82)">🐋</div>
            <h1 class="truncate font-semibold tracking-tight text-[var(--text-main,#f1f5f9)] drop-shadow-[0_1px_6px_rgba(0,0,0,0.35)]" style="font-size: var(--bar-title, 17px)">{{ state.settings?.title || '鲸鲸导航' }}</h1>
          </div>

          <!-- 右上角：图标化操作 -->
          <div class="flex shrink-0 items-center gap-1.5">
            <template v-if="!state.readonly">
              <button
                class="icon-btn"
                :class="state.editing ? 'is-on' : ''"
                :title="state.editing ? '完成编辑' : '编辑卡片'"
                :aria-label="state.editing ? '完成编辑' : '编辑卡片'"
                @click="toggleEdit"
              >
                <svg v-if="!state.editing" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
                  <path d="M12 20h9" stroke-linecap="round" />
                  <path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
                <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="m4 12.5 5 5L20 6" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              </button>
              <button class="icon-btn" title="设置" aria-label="设置" @click="state.settingsOpen = true">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
                  <circle cx="12" cy="12" r="3.2" />
                  <path
                    d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1 1.55V21a2 2 0 1 1-4 0v-.11a1.7 1.7 0 0 0-1.11-1.55 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.7 1.7 0 0 0 .34-1.87 1.7 1.7 0 0 0-1.55-1H3a2 2 0 1 1 0-4h.11A1.7 1.7 0 0 0 4.66 8.6a1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.7 1.7 0 0 0 1.87.34H9a1.7 1.7 0 0 0 1-1.55V3a2 2 0 1 1 4 0v.11a1.7 1.7 0 0 0 1 1.55 1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.7 1.7 0 0 0-.34 1.87V9a1.7 1.7 0 0 0 1.55 1H21a2 2 0 1 1 0 4h-.11a1.7 1.7 0 0 0-1.49 1Z"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  />
                </svg>
              </button>
              <button class="icon-btn" title="退出登录" aria-label="退出登录" @click="logout">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
                  <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" stroke-linecap="round" stroke-linejoin="round" />
                  <path d="M16 17l5-5-5-5M21 12H9" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              </button>
            </template>

            <!-- 未登录：只留一个登录图标 -->
            <button v-else class="icon-btn" title="登录" aria-label="登录" @click="login">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
                <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4" stroke-linecap="round" stroke-linejoin="round" />
                <path d="M10 17l5-5-5-5M15 12H3" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </button>
          </div>
        </header>

        <!-- 时钟/卡片/页脚：主页留白（pageStyle）只管这一块，跟顶栏的边距互不干扰 -->
        <div class="mx-auto flex w-full flex-1 flex-col" :style="pageStyle">
        <!-- 时钟 / 搜索 -->
        <div class="mt-6 flex flex-col items-center gap-5 sm:mt-12 sm:gap-6">
          <Clock v-if="state.settings?.clock.show" />
          <SearchBox v-if="showSearch" class="w-full max-w-xl" />
        </div>

        <!-- 分组 -->
        <main ref="groupsEl" class="mt-8 flex-1 space-y-6 sm:mt-10 sm:space-y-8">
          <GroupSection v-for="g in state.groups" :key="g.id" :group="g" @edit="openItemEditor" />

          <div v-if="state.groups.length === 0" class="rounded-2xl border border-dashed border-white/15 py-14 text-center">
            <p class="text-sm text-slate-400">还没有任何分组</p>
            <button v-if="!state.readonly" class="btn-primary mt-3 h-8 text-xs" @click="addGroup">建第一个分组</button>
          </div>

          <button
            v-if="!state.readonly && state.editing"
            class="w-full rounded-2xl border border-dashed border-white/15 py-3 text-xs text-slate-400 transition hover:border-sky-400/50 hover:text-sky-300"
            @click="addGroup"
          >
            + 新建分组
          </button>
        </main>

        <!-- 页脚：{year} 这类参数会被替换，另外可以单独写 CSS / JS -->
        <footer v-if="footerHtml" class="wavenav-foot mt-12 text-center text-[11px] leading-relaxed text-[var(--text-muted,rgba(241,245,249,0.72))]">
          <div class="footer-html" v-html="footerHtml" />
        </footer>
        </div>
      </div>

      <!-- 设置抽屉 / 卡片编辑器 -->
      <SettingsDrawer v-if="state.settingsOpen" />
      <ItemEditor v-if="state.itemEditorOpen" />

      <!-- 锁屏：盖在最上层 -->
      <LockScreen v-if="lockVisible" @done="onUnlocked" />

      <!-- 登录：浮层，其余地方透明 -->
      <LoginModal v-if="loginOpen" @done="onLoggedIn" @close="loginOpen = false" />
    </template>
  </div>
</template>

<style scoped>
.footer-html :deep(a) {
  color: inherit;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.icon-btn {
  display: grid;
  place-items: center;
  width: var(--bar-icon, 34px);
  height: var(--bar-icon, 34px);
  border-radius: 9999px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.06);
  color: rgb(226 232 240);
  backdrop-filter: none;
  transition: all 0.18s ease;
}
.icon-btn :deep(svg) {
  width: calc(var(--bar-icon, 34px) * 0.5);
  height: calc(var(--bar-icon, 34px) * 0.5);
}
.icon-btn:hover {
  background: rgba(255, 255, 255, 0.16);
  color: #fff;
  transform: translateY(-1px);
}
.icon-btn.is-on {
  border-color: rgba(56, 189, 248, 0.6);
  background: rgba(56, 189, 248, 0.18);
  color: rgb(186 230 253);
}
html:not(.dark) .icon-btn {
  border-color: rgba(15, 23, 42, 0.1);
  background: rgba(255, 255, 255, 0.6);
  color: rgb(30 41 59);
}
html:not(.dark) .icon-btn:hover {
  background: rgba(255, 255, 255, 0.92);
}

.content-in {
  animation: contentIn 0.55s ease both;
}
@keyframes contentIn {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
}
</style>
