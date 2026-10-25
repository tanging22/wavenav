<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { api } from '../api'
import { fail, loadPanel, sortGroups, state, toast } from '../store'
import { iconKind } from '../utils'
import MediaPicker from './MediaPicker.vue'
import Picker from './Picker.vue'

const form = reactive({
  groupId: state.editingItemGroupId,
  name: '',
  desc: '',
  icon: '',
  type: 'link' as string,
  stat: 'both' as string,
  openMode: 'new' as string,
})

// 容器状态卡：从 Docker 拉容器列表做下拉
const ctList = ref<{ name: string; state: string }[]>([])
const ctOptions = computed(() =>
  ctList.value.map((ct) => ({ v: ct.name, t: ct.name, sub: ct.state })),
)
const groupOptions = computed(() => state.groups.map((g) => ({ v: String(g.id), t: g.name })))
const ctLoading = ref(false)
async function loadContainers() {
  if (ctList.value.length) return
  ctLoading.value = true
  try {
    const d = await api.docker()
    ctList.value = (d.containers ?? []).map((x) => ({ name: x.name, state: x.state }))
  } catch {
    ctList.value = []
  } finally {
    ctLoading.value = false
  }
}
watch(
  () => form.type,
  (t) => {
    if (t === 'container') {
      void loadContainers()
      if (!form.icon) form.icon = '🐳'
    } else if (t === 'host' && !form.icon) {
      form.icon = '🏠'
    }
  },
)
function pickContainer(name: string) {
  form.name = name
}

const saving = ref(false)
const uploading = ref(false)
const fetching = ref(false)
const iconFile = ref<HTMLInputElement>()
const mediaOpen = ref(false)

function onPickIcon(url: string) {
  form.icon = url
  mediaOpen.value = false
}

const isEdit = computed(() => state.editingItem !== null)
const kind = computed(() => iconKind(form.icon))

const emojis = ['🌐', '🏠', '📁', '🎬', '🎵', '☁️', '🖥️', '🐳', '📊', '🔧', '🎮', '📚', '🔒', '⚡', '🧪', '💾']

// 地址就两格：通用 + 内网。内网那格留空就永远走通用；
// 填了的话，探测到「现在是内网」就优先打开内网地址，其他情况走通用。
const urlDefault = ref('')
const urlLan = ref('')

function loadInitial() {
  if (state.editingItem === null) {
    form.name = ''
    form.desc = ''
    form.icon = ''
    form.type = 'link'
    form.stat = 'both'
    form.openMode = 'new'
    urlDefault.value = ''
    urlLan.value = ''
    form.groupId = state.editingItemGroupId
    return
  }
  for (const g of state.groups) {
    const found = g.items.find((it) => it.id === state.editingItem)
    if (found) {
      form.groupId = g.id
      form.name = found.name
      form.desc = found.desc
      form.icon = found.icon
      form.type = found.type || 'link'
      form.stat = found.stat || 'both'
      form.openMode = found.openMode
      // 老数据可能存了 default/lan/wan/custom 好几条：内网单拎出来，
      // 其余（default/wan/custom）合并进「通用」，不丢链接。
      urlLan.value = found.urls.find((u) => u.kind === 'lan')?.url ?? ''
      const gen = found.urls.find((u) => u.kind !== 'lan')
      urlDefault.value = gen?.url ?? ''
      return
    }
  }
}
loadInitial()

function pickEmoji(e: string) {
  form.icon = e
}

async function pickFile(evt: Event) {
  const input = evt.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  uploading.value = true
  try {
    form.icon = await api.upload(file, 'icon')
    toast('图标已上传')
  } catch (e) {
    fail(e)
  } finally {
    uploading.value = false
    input.value = ''
  }
}

/** 抓取直接用地址栏里那一行的链接，不用再单独粘一次 */
async function autoFill(which: 'default' | 'lan' = 'default') {
  const raw = (which === 'lan' ? urlLan.value : urlDefault.value).trim()
  if (!raw) {
    toast('先把这一行的地址填上', 'err')
    return
  }
  fetching.value = true
  try {
    const res = await api.fetchSite(raw)
    if ((!form.name || !form.name.trim()) && res.title) form.name = res.title
    if (res.icon) form.icon = res.icon
    toast(res.icon ? '标题和图标都抓到了' : '抓到标题了，图标挑一个吧')
  } catch (e) {
    fail(e)
  } finally {
    fetching.value = false
  }
}

async function save() {
  if (saving.value) return
  if (!form.name.trim()) {
    toast('名称不能为空', 'err')
    return
  }
  let urls: { kind: string; url: string; weight: number }[] = []
  if (form.type === 'link') {
    const dft = urlDefault.value.trim()
    const lan = urlLan.value.trim()
    if (!dft && !lan) {
      toast('至少填一个地址', 'err')
      return
    }
    urls = [
      { kind: 'default', url: dft, weight: 0 },
      { kind: 'lan', url: lan, weight: 1 },
    ].filter((u) => u.url)
  }
  saving.value = true
  try {
    const payload = {
      groupId: Number(form.groupId) || form.groupId,
      name: form.name.trim(),
      desc: form.desc.trim(),
      icon: form.icon.trim(),
      type: form.type,
      stat: form.stat,
      openMode: form.openMode,
      urls,
    }
    if (state.editingItem !== null) {
      await api.updateItem(state.editingItem, payload)
      toast('已保存')
    } else {
      await api.createItem(payload)
      toast('已添加')
    }
    state.itemEditorOpen = false
    await refresh()
  } catch (e) {
    fail(e)
  } finally {
    saving.value = false
  }
}

async function remove() {
  if (state.editingItem === null) return
  if (!window.confirm(`删除卡片「${form.name}」？`)) return
  try {
    await api.deleteItem(state.editingItem)
    state.itemEditorOpen = false
    toast('已删除')
    await refresh()
  } catch (e) {
    fail(e)
  }
}

async function refresh() {
  try {
    const data = await api.panel()
    state.groups = sortGroups(data.groups)
  } catch (e) {
    fail(e)
  }
}

function close() {
  state.itemEditorOpen = false
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/55 p-4 py-10" @click.self="close">
    <div class="panel-solid w-full max-w-lg rounded-3xl border p-6 shadow-card">
      <div class="mb-5 flex items-center justify-between">
        <h3 class="text-base font-semibold">{{ isEdit ? '编辑卡片' : '添加卡片' }}</h3>
        <button class="grid h-8 w-8 place-items-center rounded-lg opacity-60 hover:bg-black/5 hover:opacity-100 dark:hover:bg-white/10" @click="close">✕</button>
      </div>

      <div class="space-y-4">
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="label">名称 *</label>
            <input v-model="form.name" class="field" placeholder="我的 NAS" />
          </div>
          <div>
            <label class="label">备注（显示在标题下面的小字）</label>
            <input v-model="form.desc" class="field" placeholder="可选" />
          </div>
        </div>

        <!-- 图标 -->
        <div>
          <label class="label">图标</label>
          <div class="flex items-start gap-3">
            <div class="grid h-14 w-14 shrink-0 place-items-center overflow-hidden rounded-xl border border-white/10 bg-white/5">
              <img v-if="kind === 'image'" :src="form.icon" class="h-full w-full object-contain p-1" alt="" />
              <span v-else-if="kind === 'text'" class="text-2xl">{{ form.icon }}</span>
              <span v-else class="text-lg text-slate-500 dark:text-slate-400">🌐</span>
            </div>
            <div class="min-w-0 flex-1 space-y-2">
              <input v-model="form.icon" class="field" placeholder="图片地址 / emoji / /uploads/…" />
              <div class="flex flex-wrap items-center gap-1.5">
                <button class="btn-ghost h-7 px-2 text-[11px]" @click="mediaOpen = true">素材库</button>
                <button class="btn-ghost h-7 px-2 text-[11px]" :disabled="uploading" @click="iconFile?.click()">
                  {{ uploading ? '上传中…' : '上传图片' }}
                </button>
                <button class="btn-ghost h-7 px-2 text-[11px]" @click="form.icon = ''">清空</button>
                <input ref="iconFile" type="file" accept="image/*,.svg,.ico" class="hidden" @change="pickFile" />
                <div class="flex flex-wrap gap-0.5">
                  <button
                    v-for="e in emojis"
                    :key="e"
                    class="h-7 w-7 rounded-md text-base leading-none hover:bg-white/10"
                    @click="pickEmoji(e)"
                  >
                    {{ e }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 卡片类型 -->
        <div>
          <label class="label">卡片类型</label>
          <div class="flex gap-2">
            <button
              v-for="t in [
                { v: 'link', t: '链接' },
                { v: 'container', t: '容器状态' },
                { v: 'host', t: '主机状态' },
              ]"
              :key="t.v"
              class="seg-btn flex-1 rounded-xl border px-3 py-2 text-xs transition"
              :class="form.type === t.v ? 'on' : ''"
              type="button"
              @click="form.type = t.v"
            >
              {{ t.t }}
            </button>
          </div>
          <div class="mt-2 flex gap-2">
            <button
              v-for="sm in [
                { v: 'both', t: 'CPU+内存' },
                { v: 'cpu', t: '只要CPU' },
                { v: 'mem', t: '只要内存' },
              ]"
              :key="sm.v"
              class="seg-btn flex-1 rounded-xl border px-3 py-2 text-xs transition"
              :class="form.stat === sm.v ? 'on' : ''"
              type="button"
              @click="form.stat = sm.v"
            >
              {{ sm.t }}
            </button>
          </div>
          <p v-if="form.type === 'container'" class="hint">标题显示容器名，备注按上面的选择实时显示占用。</p>
          <p v-else-if="form.type === 'host'" class="hint">备注按上面的选择实时显示这台机器的占用。</p>
        </div>

        <!-- 分组：卡片可以搬去别的分组 -->
        <div>
          <label class="label">所属分组</label>
          <Picker
            :model-value="String(form.groupId)"
            :options="groupOptions"
            placeholder="选一个分组"
            @update:model-value="form.groupId = Number($event)"
          />
        </div>

        <!-- 容器选择：类型是容器状态时出现 -->
        <div v-if="form.type === 'container'">
          <label class="label">选择容器</label>
          <Picker
            :model-value="form.name"
            :options="ctOptions"
            :loading="ctLoading"
            :placeholder="ctLoading ? '加载中…' : ctOptions.length ? '选一个容器' : '拿不到容器列表（检查 Docker 监控设置）'"
            @update:model-value="pickContainer($event)"
          />
        </div>

        <!-- 地址：通用 + 内网 两格（只有链接卡需要） -->
        <div v-if="form.type === 'link'">
          <label class="label">地址</label>
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <span class="field-label w-10 shrink-0">通用</span>
              <input v-model="urlDefault" class="field" placeholder="https://example.com（任何时候都走的地址）" />
              <button
                class="grid h-8 w-8 shrink-0 place-items-center rounded-lg opacity-60 transition hover:bg-sky-500/15 hover:text-sky-600 hover:opacity-100 dark:hover:text-sky-300"
                :disabled="fetching"
                :title="fetching ? '抓取中…' : '用这一行的地址抓标题和图标'"
                @click="autoFill('default')"
              >
                <svg v-if="!fetching" viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.6">
                  <path d="M12 3v3M12 18v3M3 12h3M18 12h3M6 6l2 2M16 16l2 2M6 18l2-2M16 8l2-2" stroke-linecap="round" />
                  <circle cx="12" cy="12" r="3.4" />
                </svg>
                <span v-else class="text-[10px]">…</span>
              </button>
            </div>
            <div class="flex items-center gap-2">
              <span class="field-label w-10 shrink-0">内网</span>
              <input v-model="urlLan" class="field" placeholder="http://192.168.1.2:5666（可选，探测到内网时优先）" />
              <button
                class="grid h-8 w-8 shrink-0 place-items-center rounded-lg opacity-60 transition hover:bg-sky-500/15 hover:text-sky-600 hover:opacity-100 dark:hover:text-sky-300"
                :disabled="fetching"
                :title="fetching ? '抓取中…' : '用这一行的地址抓标题和图标'"
                @click="autoFill('lan')"
              >
                <svg v-if="!fetching" viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.6">
                  <path d="M12 3v3M12 18v3M3 12h3M18 12h3M6 6l2 2M16 16l2 2M6 18l2-2M16 8l2-2" stroke-linecap="round" />
                  <circle cx="12" cy="12" r="3.4" />
                </svg>
                <span v-else class="text-[10px]">…</span>
              </button>
            </div>
          </div>
          <p class="hint">内网那格留空就永远走通用；填了的话，探测到当前是内网就优先打开它。</p>
        </div>

        <!-- 打开方式 -->
        <div>
          <label class="label">打开方式</label>
          <div class="flex gap-2">
            <button
              v-for="o in [
                { v: 'new', t: '新标签页' },
                { v: 'self', t: '当前页' },
              ]"
              :key="o.v"
              class="btn-ghost h-8 px-3 text-xs"
              :class="form.openMode === o.v ? '!border-sky-400/60 !text-sky-600 dark:!text-sky-200' : ''"
              @click="form.openMode = o.v"
            >
              {{ o.t }}
            </button>
          </div>
        </div>
      </div>

      <div class="mt-6 flex items-center justify-between gap-2">
        <button v-if="isEdit" class="btn h-9 px-3 text-xs text-rose-500 dark:text-rose-300 hover:bg-rose-500/10" @click="remove">删除卡片</button>
        <span v-else />
        <div class="flex gap-2">
          <button class="btn-ghost h-9" @click="close">取消</button>
          <button class="btn-primary h-9" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </div>

      <MediaPicker
        v-if="mediaOpen"
        kind="image"
        title="选一张图标"
        @pick="onPickIcon"
        @close="mediaOpen = false"
      />

    </div>
  </div>
</template>

<style scoped>
.seg-btn {
  color: var(--text-main, rgba(232, 238, 255, 0.75));
  border-color: var(--card-border, rgba(255, 255, 255, 0.12));
  background: var(--card-bg, rgba(255, 255, 255, 0.06));
}
/* seg-btn 在 panel-solid 白底面板里：跟面板走，不跟壁纸（--text-main 跟壁纸会在浅色系统+深壁纸下翻车） */
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
