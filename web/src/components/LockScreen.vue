<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { state } from '../store'
import { greeting } from '../utils'
import type { LockAnimation } from '../types'

const emit = defineEmits<{ (e: 'done'): void }>()

const lock = computed(() => state.settings?.lock)
const animation = computed<LockAnimation>(() => lock.value?.animation ?? 'slide')
const trigger = computed(() => lock.value?.trigger ?? 'both')

const now = ref(new Date())
const leaving = ref(false)
let timer: number | null = null
let leavingTimer: number | null = null

const hh = computed(() => String(now.value.getHours()).padStart(2, '0'))
const mm = computed(() => String(now.value.getMinutes()).padStart(2, '0'))
const dateText = computed(() => {
  const d = now.value
  const week = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][d.getDay()]
  return `${d.getMonth() + 1}月${d.getDate()}日 ${week}`
})
const hello = computed(() => greeting(lock.value?.greet))

// 锁屏只显示到「分」，没有秒 —— 每 1 秒刷一次是白费；对齐到下一分钟边界才刷新。
// 秒针整点后最多 1 秒误差，但省掉每分钟 59 次无意义的重渲染。
function alignToNextMinute() {
  const ms = new Date()
  const delay = 60000 - (ms.getSeconds() * 1000 + ms.getMilliseconds()) + 50
  timer = window.setTimeout(() => {
    now.value = new Date()
    alignToNextMinute()
  }, delay)
}

onMounted(() => alignToNextMinute())

onBeforeUnmount(() => {
  if (timer !== null) clearTimeout(timer)
  if (leavingTimer !== null) clearTimeout(leavingTimer)
})

// ---------- 解锁手势 ----------
let startY = 0
let startX = 0
let dragging = false
let moved = false

function onPointerDown(e: PointerEvent) {
  if (trigger.value === 'click') return
  dragging = true
  moved = false
  startY = e.clientY
  startX = e.clientX
}

function onPointerMove(e: PointerEvent) {
  if (!dragging) return
  if (Math.abs(e.clientY - startY) > 10 || Math.abs(e.clientX - startX) > 10) moved = true
  // 上滑时露出一点视差，让手感更实
  if (e.clientY - startY < -8) {
    const shift = Math.min(0, e.clientY - startY)
    if (rootEl.value) rootEl.value.style.transform = `translateY(${shift * 0.35}px)`
  }
}

function onPointerUp(e: PointerEvent) {
  if (!dragging) return
  dragging = false
  if (rootEl.value) rootEl.value.style.transform = ''
  const dy = e.clientY - startY
  if (trigger.value !== 'click' && dy < -70) {
    unlock()
    return
  }
  if (trigger.value !== 'swipe' && !moved) unlock()
}

const rootEl = ref<HTMLElement>()

function unlock() {
  if (leaving.value) return
  leaving.value = true
  const ms = animation.value === 'blur' ? 620 : 560
  leavingTimer = window.setTimeout(() => emit('done'), ms)
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Enter' || e.key === ' ') unlock()
}

onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
const lockTimeStyle = computed(() => {
  const size = state.settings?.lock.fontSize ?? 0
  return size > 0 ? { fontSize: `${size}px`, lineHeight: 1.05 } : {}
})
</script>

<template>
  <div
    ref="rootEl"
    class="lock-root fixed inset-0 z-50 select-none"
    :class="[`lock-anim-${animation}`, leaving ? `lock-out-${animation}` : '']"
    @pointerdown="onPointerDown"
    @pointermove="onPointerMove"
    @pointerup="onPointerUp"
    @pointercancel="dragging = false"
  >
    <!-- 左上角：时间 -->
    <div class="absolute left-5 top-5 sm:left-10 sm:top-9">
      <div class="flex items-end gap-1 font-light tabular-nums text-white/95 drop-shadow-[0_2px_12px_rgba(0,0,0,0.45)]">
        <span class="text-[3.25rem] leading-none sm:text-7xl" :style="lockTimeStyle">{{ hh }}</span>
        <span class="mb-1 text-xl leading-none opacity-70 sm:text-3xl">:</span>
        <span class="text-[3.25rem] leading-none sm:text-7xl" :style="lockTimeStyle">{{ mm }}</span>
      </div>
      <div v-if="lock?.showDate" class="mt-1.5 text-[11px] tracking-[0.22em] text-white/70 sm:mt-2 sm:text-sm">
        {{ dateText }}
      </div>
    </div>

    <!-- 右上角：一言（位置固定，不随其他元素移动） -->
    <div
      v-if="state.settings?.hitokoto.enabled"
      class="absolute right-5 top-5 max-w-[46vw] text-right sm:right-10 sm:top-9 sm:max-w-xs"
    >
      <p class="text-[12px] leading-relaxed text-white/85 drop-shadow-[0_2px_10px_rgba(0,0,0,0.5)] sm:text-sm">
        {{ state.hitokoto.text || '……' }}
      </p>
      <p v-if="state.hitokoto.from" class="mt-1 text-[10px] text-white/45 sm:text-[11px]">— {{ state.hitokoto.from }}</p>
    </div>

    <!-- 底部：问候 + 站点名 -->
    <div class="absolute inset-x-0 bottom-8 flex flex-col items-center gap-2 px-6 text-center sm:bottom-12">
      <p class="hello text-2xl font-light tracking-[0.18em] text-white/80 drop-shadow-[0_2px_14px_rgba(0,0,0,0.5)] sm:text-3xl">
        {{ hello }}
      </p>
      <p v-if="state.settings?.title" class="text-[10px] tracking-[0.32em] text-white/35 sm:text-[11px]">
        {{ state.settings.title }}
      </p>
    </div>
  </div>
</template>

<style scoped>
.lock-root {
  /* 壁纸在底层，这里只加一层很薄的压暗，保证文字读得清 */
  background: radial-gradient(120% 120% at 50% 0%, rgba(0, 0, 0, 0.12), rgba(0, 0, 0, 0.38));
  touch-action: none;
  animation: lockIn 0.5s ease both;
}
html:not(.dark) .lock-root {
  background: radial-gradient(120% 120% at 50% 0%, rgba(255, 255, 255, 0.06), rgba(0, 0, 0, 0.22));
}

@keyframes lockIn {
  from {
    opacity: 0;
  }
}

/* 问候语轻轻起伏一下，提示"这里可以动" */
.hello {
  animation: helloBreath 3.6s ease-in-out infinite;
}
@keyframes helloBreath {
  0%,
  100% {
    opacity: 0.78;
  }
  50% {
    opacity: 1;
  }
}

/* ---- 出场动画：一次只挂一个 class ---- */
.lock-out-fade {
  animation: lockFade 0.5s ease forwards;
}
@keyframes lockFade {
  to {
    opacity: 0;
  }
}

.lock-out-slide {
  animation: lockSlide 0.56s cubic-bezier(0.4, 0, 0.2, 1) forwards;
}
@keyframes lockSlide {
  to {
    transform: translateY(-100%);
    opacity: 0.2;
  }
}

.lock-out-zoom {
  animation: lockZoom 0.56s ease forwards;
}
@keyframes lockZoom {
  to {
    transform: scale(1.28);
    opacity: 0;
  }
}

.lock-out-blur {
  animation: lockBlur 0.62s ease forwards;
}
@keyframes lockBlur {
  to {
    filter: blur(22px);
    transform: scale(1.05);
    opacity: 0;
  }
}

.lock-out-curtain {
  animation: lockCurtain 0.6s cubic-bezier(0.65, 0, 0.35, 1) forwards;
}
@keyframes lockCurtain {
  to {
    clip-path: inset(0 0 100% 0);
    opacity: 0.6;
  }
}

.lock-out-flip {
  transform-origin: center top;
  animation: lockFlip 0.6s ease forwards;
}
@keyframes lockFlip {
  to {
    transform: perspective(1400px) rotateX(-88deg);
    opacity: 0;
  }
}
</style>
