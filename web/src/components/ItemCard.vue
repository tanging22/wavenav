<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { TItem } from '../types'
import { state, toast } from '../store'
import { iconKind, pickURL } from '../utils'

const props = defineProps<{ item: TItem; layout: string; readonly: boolean }>()
const emit = defineEmits<{ (e: 'edit', id: number): void }>()

const kind = computed(() => iconKind(props.item.icon))
// 外链图标 404 / 防盗链时不能留个裂图，退回地球
const broken = ref(false)
watch(
  () => props.item.icon,
  () => {
    broken.value = false
  },
)
const target = computed(() => pickURL(props.item, state.lanAccess))
const isIcon = computed(() => props.layout === 'icon')

function open() {
  if (!props.readonly && state.editing) {
    emit('edit', props.item.id)
    return
  }
  if (cardType.value !== 'link') return
  const u = target.value
  if (!u?.url) {
    toast('这张卡片还没填地址呢', 'err')
    return
  }
  if (props.item.openMode === 'self') window.location.href = u.url
  else window.open(u.url, '_blank', 'noopener')
}

// 右上角可达性小点：绿/黄/红，探过才有
const lat = computed<number | null | undefined>(() => state.latency[props.item.id])
const latClass = computed(() => {
  const v = lat.value
  if (v === undefined) return ''
  if (v === null) return 'lat-bad'
  if (v < 150) return 'lat-ok'
  if (v < 600) return 'lat-mid'
  return 'lat-bad'
})
// 容器 / 主机状态卡：从 stats 轮询里取数
const cardType = computed(() =>
  props.item.type === 'container' ? 'container' : props.item.type === 'host' ? 'host' : 'link',
)
const stat = computed(() => {
  if (cardType.value === 'host') {
    const h = state.dockerStats?.host
    if (!h) return null
    return { state: 'running', cpu: h.cpuPercent, mem: h.memUsed, limit: h.memTotal }
  }
  if (cardType.value === 'container') {
    const c = (state.dockerStats?.containers ?? []).find((x) => x.name === props.item.name)
    if (!c) return null
    return { state: c.state, cpu: c.cpuPercent, mem: c.memUsed, limit: c.memLimit }
  }
  return null
})
const fmtMem = (v: number) =>
  v >= 1073741824 ? (v / 1073741824).toFixed(1) + 'GB' : v >= 1048576 ? Math.round(v / 1048576) + 'MB' : Math.round(v / 1024) + 'KB'
const statDesc = computed(() => {
  if (cardType.value === 'link') return ''
  const st = stat.value
  if (!st) return '等待状态数据…'
  const mode = props.item.stat === 'cpu' || props.item.stat === 'mem' ? props.item.stat : 'both'
  if (mode === 'cpu') return `CPU ${st.cpu}%`
  if (mode === 'mem') return `内存 ${fmtMem(st.mem)}${st.limit ? ' / ' + fmtMem(st.limit) : ''}`
  return `CPU ${st.cpu}% · 内存 ${fmtMem(st.mem)}${st.limit ? ' / ' + fmtMem(st.limit) : ''}`
})
// 状态卡的角点按容器状态着色，链接卡沿用探测延迟
const memPct = computed(() => {
  const st = stat.value
  if (!st || !st.limit) return 0
  return (st.mem / st.limit) * 100
})
const stateDotClass = computed(() => {
  if (cardType.value === 'link') return latClass.value
  const st = stat.value
  if (!st) return ''
  // 容器没在跑直接红；角点分级跟随卡片选择的显示内容：
  // 只要内存 → 按内存占比（≤33 绿 / ≤66 黄 / 其余红），其余 → 按 CPU（≤33 / ≤66）
  if (cardType.value === 'container' && st.state !== 'running') return 'lat-bad'
  if ((props.item.stat || 'both') === 'mem')
    return memPct.value <= 33 ? 'lat-ok' : memPct.value <= 66 ? 'lat-mid' : 'lat-bad'
  return st.cpu <= 33 ? 'lat-ok' : st.cpu <= 66 ? 'lat-mid' : 'lat-bad'
})
const dotTitle2 = computed(() => {
  if (cardType.value === 'link') return latText.value
  const st = stat.value
  if (!st) return ''
  if (cardType.value === 'container' && st.state !== 'running') return st.state
  const mode = props.item.stat || 'both'
  if (mode === 'mem') return `内存 ${memPct.value.toFixed(0)}%`
  if (mode === 'cpu') return `CPU ${st.cpu}%`
  return `CPU ${st.cpu}% · 内存 ${memPct.value.toFixed(0)}%`
})
const fallbackEmoji = computed(() =>
  cardType.value === 'container' ? '🐳' : cardType.value === 'host' ? '🏠' : '🌐',
)

// 小点位置（设置里能选四角 + 自定义离边缘的距离）
const dotPos = computed(() => {
  const c = state.settings?.card
  const o = `${Math.min(40, Math.max(0, c?.dotOffset ?? 8))}px`
  switch (c?.dotPos) {
    case 'tl':
      return { left: o, top: o }
    case 'bl':
      return { left: o, bottom: o }
    case 'br':
      return { right: o, bottom: o }
    default:
      return { right: o, top: o }
  }
})

const latText = computed(() => {
  const v = lat.value
  if (v === undefined) return ''
  return v === null ? '连不上' : `${v} ms`
})

// 卡片底色 / 边框 / 圆角 / 模糊全部走 CSS 变量，设置里能实时调
const itemClass = computed(() => [
  'card-hover group relative min-w-0 cursor-pointer border border-[var(--card-border)] bg-[var(--card-bg)]',
  'rounded-[var(--card-radius)]',
  // 只图标模式：卡片本身不能有底和边框，不然图标外面会多出一圈"相框"
  isIcon.value ? 'icon-tile grid place-items-center p-0' : 'flex items-center gap-2.5 py-2 pl-2 pr-3',
  isIcon.value ? '' : 'min-h-[62px]',
  !props.readonly && state.editing ? 'ring-1 ring-sky-400/40' : '',
])

// 只显示图标模式：边长走 --icon-size，图标本身几乎撑满
// 手机上按视口宽度收一下，一行至少能放四个，不会被 --icon-size 顶出去
const iconStyle = computed(() =>
  isIcon.value ? { width: 'min(var(--icon-size), 21vw)', height: 'min(var(--icon-size), 21vw)' } : {},
)

const iconBoxClass = computed(() => [
  'icon-box grid shrink-0 place-items-center',
  // 图片图标不套任何底/边，只用 --icon-bg 决定透明还是白底；emoji 才给一点点衬底
  kind.value === 'image' && !broken.value ? 'icon-box-img' : 'bg-black/5 dark:bg-white/10',
  isIcon.value ? 'h-full w-full' : 'h-10 w-10',
])
</script>

<template>
  <div :class="itemClass" :style="iconStyle" :title="isIcon ? item.name : undefined" @click="open">
    <!-- 图标 -->
    <div :class="iconBoxClass">
      <img
        v-if="kind === 'image' && !broken"
        :src="item.icon"
        :class="['icon-img h-full w-full', isIcon ? 'object-cover' : 'object-contain']"
        alt=""
        loading="lazy"
        referrerpolicy="no-referrer"
        @error="broken = true"
      />
      <span
        v-else-if="kind === 'text'"
        class="icon-emoji leading-none"
        :class="isIcon ? 'text-[26px]' : 'text-[22px]'"
      >
        {{ item.icon }}
      </span>
      <span v-else class="icon-emoji leading-none opacity-60" :class="'text-[19px]'">{{ fallbackEmoji }}</span>
    </div>

    <!-- 可达性小点：绿/黄/红，挂在卡片右上角，跟 Sun-Panel 那样 -->
    <span
      v-if="stateDotClass && !(state.editing && !readonly)"
      class="lat-dot"
      :class="stateDotClass"
      :style="dotPos"
      :title="dotTitle2"
    ></span>

    <div v-if="!isIcon" class="min-w-0 flex-1 text-left">
      <div class="truncate text-[13.5px] font-medium leading-tight text-[var(--card-fg)]" :title="item.name">
        {{ item.name }}
      </div>
      <div
        v-if="cardType === 'link' ? !!item.desc : true"
        class="mt-1 truncate text-[11px] leading-tight text-[var(--card-fg-muted)]"
        :title="cardType === 'link' ? item.desc : statDesc"
      >
        {{ cardType === 'link' ? item.desc : statDesc }}
      </div>
    </div>

    <!-- 编辑模式下的编辑角标 -->
    <span
      v-if="!readonly && state.editing"
      class="absolute right-1.5 top-1.5 rounded-md bg-sky-500/20 px-1.5 py-0.5 text-[10px] text-sky-500 dark:text-sky-200"
    >
      改
    </span>
  </div>
</template>

<style scoped>
/* 图片图标：不要边框、不要内衬，透明底或白底由设置决定 */
.icon-box {
  position: relative;
}

.lat-dot {
  position: absolute;
  width: 9px;
  height: 9px;
  border-radius: 9999px;
  box-shadow: 0 0 0 2px rgba(0, 0, 0, 0.45);
  pointer-events: none;
  z-index: 2;
}
.lat-ok {
  background: #34d399;
}
.lat-mid {
  background: #fbbf24;
}
.lat-bad {
  background: #fb7185;
}

.icon-box-img {
  background: transparent;
  /* 图2那种圆角方块，按自身边长的 22% 走，大小变了圆角也跟得上 */
  border-radius: 22%;
  overflow: hidden;
}
/* 撑满容器，object-contain 保证完整不裁切；放大一点点吃掉 ico 自带的白边 */
.icon-img {
  /* 只在图标格子里做 fill+crop，网格卡片的图标仍然完整显示 */
  display: block;
}
/* 只图标模式下卡片不能有底/边框，否则图标外面会多出一圈相框 */
/* 只图标模式才垫底色：透明 PNG 叠一层白底才看得清 */
.icon-tile .icon-box-img {
  background: var(--icon-bg, transparent);
}

.icon-tile {
  background: transparent !important;
  border-color: transparent !important;
  box-shadow: none !important;
}
.icon-tile:hover {
  background: rgba(255, 255, 255, 0.07) !important;
}
.icon-emoji {
  font-family: 'Apple Color Emoji', 'Segoe UI Emoji', 'Noto Color Emoji', sans-serif;
  line-height: 1;
}
</style>
