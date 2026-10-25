<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { state } from '../store'

const wp = computed(
  () => state.settings?.wallpaper ?? { type: 'none', src: '', blur: 0, mask: 0.25, brightness: 1, smooth: true },
)
const isVideo = computed(() => wp.value.type === 'video' && !!wp.value.src)
const isImage = computed(() => wp.value.type === 'image' && !!wp.value.src)

// 只有「勾了流畅模式 + 机器真的弱」才停首帧。手机默认照常播放
// （iOS 的 Safari 没有 deviceMemory，undefined 时按能播处理）
const freeze = computed(() => {
  if (!isVideo.value) return false
  if (!(wp.value.smooth ?? true)) return false
  if (typeof window === 'undefined') return false
  const dm = (navigator as unknown as { deviceMemory?: number }).deviceMemory
  return typeof dm === 'number' && dm <= 2
})

// #t=0.1 让浏览器只解码一帧，比暂停在 0 秒还稳定（0 秒常常是黑屏）
const videoSrc = computed(() => (freeze.value ? `${wp.value.src}#t=0.1` : wp.value.src))

const mediaStyle = computed(() => {
  const blur = wp.value.blur || 0
  const bright = wp.value.brightness ?? 1
  const filters: string[] = []
  if (blur > 0) filters.push(`blur(${blur}px)`)
  if (bright !== 1) filters.push(`brightness(${bright})`)
  return {
    // blur(0px) 也会让浏览器开一层合成，视频壁纸下这就是掉帧的元凶
    filter: filters.length ? filters.join(' ') : 'none',
    transform: blur > 0 ? 'scale(1.06)' : 'none',
  }
})

const maskStyle = computed(() => {
  const m = Math.min(Math.max(wp.value.mask ?? 0, 0), 1)
  return { background: `rgba(4, 8, 20, ${m})` }
})

const videoEl = ref<HTMLVideoElement | null>(null)

/** 切到后台就停，切回来接着放——比一直空转省电得多 */
function syncPlay() {
  const v = videoEl.value
  if (!v || freeze.value) return
  if (document.hidden) {
    v.pause()
    return
  }
  void v.play().catch(() => {})
}

watch(freeze, () => {
  if (!freeze.value) window.requestAnimationFrame(syncPlay)
})

onMounted(() => {
  document.addEventListener('visibilitychange', syncPlay)
  syncPlay()
})

onBeforeUnmount(() => document.removeEventListener('visibilitychange', syncPlay))
</script>

<template>
  <div class="pointer-events-none fixed inset-0 -z-10 overflow-hidden bg-[#0b1020]">
    <video
      v-if="isVideo"
      ref="videoEl"
      :src="videoSrc"
      class="h-full w-full object-cover"
      :style="mediaStyle"
      :autoplay="!freeze"
      muted
      loop
      playsinline
      preload="auto"
      disable-picture-in-picture
    />
    <img v-else-if="isImage" :src="wp.src" class="h-full w-full object-cover" :style="mediaStyle" alt="" />
    <template v-else>
      <!-- 无壁纸时给一层淡淡的光晕，避免纯平 -->
      <div
        class="absolute inset-0 bg-[radial-gradient(circle_at_20%_10%,rgba(56,132,255,.28),transparent_55%),radial-gradient(circle_at_85%_80%,rgba(129,96,255,.24),transparent_55%)]"
      />
    </template>
    <div v-if="isVideo || isImage" class="absolute inset-0" :style="maskStyle" />
  </div>
</template>
