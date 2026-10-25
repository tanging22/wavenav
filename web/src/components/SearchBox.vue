<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { persistSettingsSoon, state } from '../store'

const q = ref('')
const openEngines = ref(false)
const iconFailed = ref(false)

const engines = computed(() => state.settings?.search.engines ?? [])
const current = computed(() => {
  const key = state.settings?.search.engine
  return engines.value.find((e) => e.key === key) ?? engines.value[0]
})
const initial = computed(() => (current.value?.name ?? '?').trim().slice(0, 1))

watch(
  () => current.value?.key,
  () => {
    iconFailed.value = false
  },
)

function buildURL(text: string) {
  const engine = current.value
  if (!engine) return ''
  const encoded = encodeURIComponent(text)
  if (engine.url.includes('%s')) return engine.url.replace(/%s/g, encoded)
  return engine.url + encoded
}

function go() {
  const text = q.value.trim()
  if (!text) return
  const url = buildURL(text)
  if (url) window.open(url, '_blank', 'noopener')
}

function pickEngine(key: string) {
  if (!state.settings) return
  state.settings.search.engine = key
  // 换引擎是"下次还想用这个"，得记住 —— 防抖写回后端
  persistSettingsSoon()
  openEngines.value = false
}

function onBlur() {
  // 延迟关闭，让点击事件先命中
  window.setTimeout(() => (openEngines.value = false), 120)
}
// 一言直接顶掉占位符：框大小一点不动，空着的时候它就在那儿
const hint = computed(() => {
  const h = state.hitokoto
  if (!state.settings?.hitokoto.enabled || !h.text) return '搜点什么…'
  return h.from ? `${h.text} — ${h.from}` : h.text
})

const searchCfg = computed(() => {
  const s = state.settings?.search
  return { height: s?.height ?? 52, radius: s?.radius ?? 16, fontSize: s?.fontSize ?? 14, maxWidth: s?.maxWidth ?? 640 }
})

const barStyle = computed(() => ({
  height: `${searchCfg.value.height}px`,
  borderRadius: `${searchCfg.value.radius}px`,
  maxWidth: `${searchCfg.value.maxWidth}px`,
}))

// 泛光颜色跟随主机 CPU 占用：≤33 绿 / ≤66 黄 / 其余红（与卡片状态点同一套阈值）
const glowLevel = computed(() => {
  const cpu = state.dockerStats?.host?.cpuPercent
  if (cpu == null) return 'idle' // 没有主机数据时回落到默认蓝色
  if (cpu <= 33) return 'ok'
  if (cpu <= 66) return 'mid'
  return 'bad'
})

// 泛光对应的颜色值：无数据用默认蓝，否则绿/黄/红
const glowColor = computed(() => {
  switch (glowLevel.value) {
    case 'ok':
      return 'rgba(52, 211, 153, X)' // #34d399
    case 'mid':
      return 'rgba(251, 191, 36, X)' // #fbbf24
    case 'bad':
      return 'rgba(251, 113, 133, X)' // #fb7185
    default:
      return 'rgba(56, 189, 248, X)' // 默认 sky 蓝
  }
})

const glowStyle = computed(() => {
  if (!state.settings?.search.glow) return {}
  const c = glowColor.value.replace('X', '0.5')
  return {
    borderColor: c.replace('0.5', '0.55'),
    boxShadow: `0 0 18px ${c.replace('0.5', '0.3')}, inset 0 0 10px ${c.replace('0.5', '0.07')}`,
  }
})
</script>

<template>
  <!-- z-30：整条搜索栏抬到卡片之上，下拉才不会被盖住 -->
  <div
    :class="['search-bar', 'relative z-30 mx-auto w-full rounded-2xl border border-[var(--card-border)] bg-[var(--search-bg)] px-2 shadow-card sm:px-3', state.settings?.search.glow ? 'search-glow' : '']"
    :style="[barStyle, glowStyle]"
  >
    <div class="flex h-full items-center gap-1.5 sm:gap-2">
    <!-- 左：当前搜索引擎的图标，点一下换引擎 -->
    <div class="relative shrink-0">
      <button
        type="button"
        class="grid h-9 w-9 place-items-center rounded-xl transition hover:bg-black/5 dark:hover:bg-white/10"
        :title="current?.name ?? '搜索引擎'"
        @click="openEngines = !openEngines"
      >
        <img
          v-if="current?.icon && !iconFailed"
          :key="current.key"
          :src="current.icon"
          class="h-[18px] w-[18px] object-contain"
          alt=""
          referrerpolicy="no-referrer"
          @error="iconFailed = true"
        />
        <span v-else class="text-[13px] font-semibold opacity-70 text-[#e8eeff]">{{ initial }}</span>
      </button>

      <Transition name="fade">
        <div
          v-if="openEngines"
          class="engine-menu absolute left-0 top-full z-50 mt-2 w-44 overflow-hidden rounded-xl border border-[var(--card-border)] p-1 shadow-card"
          @mouseleave="openEngines = false"
        >
          <button
            v-for="e in engines"
            :key="e.key"
            type="button"
            class="engine-opt flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-left text-sm hover:bg-black/5 dark:hover:bg-white/10"
            :class="e.key === current?.key ? 'on' : ''"
            @click="pickEngine(e.key)"
          >
            <img
              v-if="e.icon"
              :src="e.icon"
              class="h-4 w-4 shrink-0 object-contain"
              alt=""
              referrerpolicy="no-referrer"
              @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"
            />
            <span v-else class="grid h-4 w-4 shrink-0 place-items-center text-[10px] opacity-60">{{ e.name.slice(0, 1) }}</span>
            <span class="truncate">{{ e.name }}</span>
            <span v-if="e.key === current?.key" class="ml-auto text-[11px]">✓</span>
          </button>
        </div>
      </Transition>
    </div>

    <input
      v-model="q"
      class="min-w-0 flex-1 bg-transparent text-[#e8eeff] outline-none"
      :style="{ fontSize: `${searchCfg.fontSize}px` }"
      :placeholder="hint"
      @keydown.enter="go"
      @blur="onBlur"
    />

    <!-- 右：线条放大镜，就是搜索按钮（不再显示"搜索"两个字） -->
    <button
      type="button"
      class="grid h-9 w-9 shrink-0 place-items-center rounded-xl opacity-65 transition hover:bg-black/5 hover:opacity-100 dark:hover:bg-white/10"
      title="搜索"
      aria-label="搜索"
      @click="go"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" class="h-[18px] w-[18px] text-[#e8eeff]">
        <circle cx="11" cy="11" r="7.2" />
        <path d="m20.2 20.2-3.6-3.6" />
      </svg>
    </button>
    </div>
  </div>
</template>

<style scoped>
/* 引擎菜单：不透明面板，不用磨砂，避免和卡片糊在一起。字色跟面板不跟壁纸 */
.engine-menu {
  background-color: var(--panel-bg);
}
.engine-opt { color: #e8eeff; }
html:not(.dark) .engine-opt { color: #1e293b; }
html:not(.dark) .engine-opt:hover { background: rgba(15, 23, 42, 0.06); }
.engine-opt.on { color: #7dd3fc; }
html:not(.dark) .engine-opt.on { color: #0369a1; }
/* 引擎名首字占位（无图标时）跟整条一样用面板字色 */
</style>
