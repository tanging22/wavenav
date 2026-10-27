<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { state } from '../store'
import { clockText } from '../utils'

const now = ref(new Date())
let timer: number | undefined

const showSeconds = computed(() => state.settings?.clock.seconds ?? true)

function tick() {
  now.value = new Date()
}

// 对齐到整秒边界再走 1s 节拍：避免从「第 0.73 秒」开始，每次显示秒都会跳。
// 关掉秒显示时退化成对齐到下一分钟（主页时钟只到分，1 秒刷一次是白费）。
function schedule() {
  const d = new Date()
  const stepMs = showSeconds.value ? 1000 : 60000
  const delay = stepMs - (d.getMilliseconds() + (showSeconds.value ? 0 : d.getSeconds() * 1000)) + 30
  timer = window.setTimeout(() => {
    tick()
    schedule()
  }, Math.max(30, delay))
}

onMounted(() => {
  tick()
  schedule()
})
onBeforeUnmount(() => {
  if (timer) window.clearTimeout(timer)
})

const week = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

const clock = computed(() => clockText(now.value, state.settings?.clock.format || '15:04:05', showSeconds.value))

// 把时间里的「:」单独拎出来，比数字更小更淡，缩放时跟着整体字号走
const clockParts = computed(() => clock.value.split(':').flatMap((seg, i, arr) => {
  const parts = [{ t: seg, sep: false }]
  if (i < arr.length - 1) parts.push({ t: ':', sep: true })
  return parts
}))

const alignClass = computed(() => {
  const a = state.settings?.clock.align ?? 'center'
  return a === 'left' ? 'text-left' : a === 'right' ? 'text-right' : 'text-center'
})

const showDate = computed(() => state.settings?.clock.showDate ?? true)

const clockStyle = computed(() => {
  const size = state.settings?.clock.fontSize ?? 0
  return size > 0 ? { fontSize: `${size}px`, lineHeight: 1.1 } : {}
})

// 日期跟时间一起缩放：日期字号 = 时间字号 * 0.29（自适应时用固定 text-sm）
const dateStyle = computed(() => {
  const size = state.settings?.clock.fontSize ?? 0
  return size > 0 ? { fontSize: `${Math.max(10, Math.round(size * 0.29))}px` } : {}
})

const dateText = computed(() => {
  const d = now.value
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日 ${week[d.getDay()]}`
})
</script>

<style scoped>
.clock-colon {
  font-size: 0.55em;
  opacity: 0.45;
  vertical-align: 0.08em;
  margin: 0 0.04em;
}
</style>

<template>
  <div class="w-full select-none" :class="alignClass">
    <div class="font-mono text-5xl font-light tracking-tight tabular-nums text-[var(--text-main,#f1f5f9)] drop-shadow-[0_2px_12px_rgba(0,0,0,0.35)] sm:text-6xl" :style="clockStyle">
      <template v-for="(p, i) in clockParts" :key="i">
        <span v-if="p.sep" class="clock-colon">{{ p.t }}</span><span v-else>{{ p.t }}</span>
      </template>
    </div>
    <div v-if="showDate" class="mt-2 text-sm text-[var(--text-muted,rgba(241,245,249,0.72))]" :style="dateStyle">{{ dateText }}</div>
  </div>
</template>
