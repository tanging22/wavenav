<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

export interface PickerOption {
  v: string
  t: string
  sub?: string
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: PickerOption[]
    placeholder?: string
    disabled?: boolean
    loading?: boolean
  }>(),
  { placeholder: '选一个', disabled: false, loading: false },
)

const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()

const open = ref(false)
const root = ref<HTMLElement>()

const current = computed(() => {
  const hit = props.options.find((o) => o.v === props.modelValue)
  if (hit) return hit.t
  return props.modelValue || ''
})

function toggle() {
  if (props.disabled || props.loading) return
  open.value = !open.value
}

function pick(o: PickerOption) {
  emit('update:modelValue', o.v)
  open.value = false
}

function onDocClick(e: MouseEvent) {
  if (!open.value) return
  if (root.value && !root.value.contains(e.target as Node)) open.value = false
}

onMounted(() => document.addEventListener('mousedown', onDocClick))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocClick))
</script>

<template>
  <div ref="root" class="relative">
    <button
      type="button"
      class="picker-btn field flex items-center justify-between gap-2 text-left"
      :class="{ 'opacity-60': disabled || loading }"
      :disabled="disabled || loading"
      @click="toggle"
    >
      <span class="truncate">{{ loading ? '加载中…' : current || placeholder }}</span>
      <svg
        viewBox="0 0 24 24"
        class="h-3.5 w-3.5 shrink-0 transition-transform"
        :class="open ? 'rotate-180' : ''"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <path d="m6 9 6 6 6-6" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </button>

    <div
      v-if="open"
      class="picker-pop absolute left-0 right-0 top-[calc(100%+6px)] z-50 max-h-60 overflow-y-auto rounded-xl border border-white/10 p-1 shadow-card"
    >
      <div v-if="!options.length" class="px-3 py-2 text-[12px] opacity-60">{{ placeholder }}</div>
      <button
        v-for="o in options"
        :key="o.v"
        type="button"
        class="picker-opt flex w-full items-center justify-between gap-2 rounded-lg px-3 py-2 text-left text-[13px] transition"
        :class="o.v === modelValue ? 'on' : ''"
        @click="pick(o)"
      >
        <span class="min-w-0 truncate">{{ o.t }}</span>
        <span v-if="o.sub" class="shrink-0 text-[11px] opacity-55">{{ o.sub }}</span>
        <svg
          v-if="o.v === modelValue"
          viewBox="0 0 24 24"
          class="picker-opt on h-3.5 w-3.5 shrink-0"
          fill="none"
          stroke="currentColor"
          stroke-width="2.4"
        >
          <path d="m5 13 4 4L19 7" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </button>
    </div>
  </div>
</template>

<style>
/* 下拉弹层是不透明深底/白底，字色必须跟弹层走，不能继承页面上的 --text-main（那跟壁纸） */
.picker-pop { background: #141b2f; }
html:not(.dark) .picker-pop { background: #ffffff; border-color: rgba(15, 23, 42, 0.12); }
.picker-opt { color: #e8eeff; }
html:not(.dark) .picker-opt { color: #1e293b; }
html:not(.dark) .picker-opt:hover { background: rgba(15, 23, 42, 0.06); }
.picker-opt.on { color: #7dd3fc; }
html:not(.dark) .picker-opt.on { color: #0369a1; }
/* 关闭态按钮在白底面板里也要深字（field 本身已跟 --text-main，但 panel-solid 里会 inherit 到面板深字，这里兜底） */
html:not(.dark) .panel-solid .picker-btn { color: #1e293b; }
</style>
