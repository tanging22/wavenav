<script setup lang="ts">
import MediaBrowser from './MediaBrowser.vue'

withDefaults(
  defineProps<{
    kind?: 'image' | 'video' | 'all'
    purpose?: 'pick' | 'wallpaper'
    title?: string
  }>(),
  { kind: 'all', purpose: 'pick', title: '素材库' },
)
const emit = defineEmits<{
  (e: 'pick', url: string): void
  (e: 'wallpaper', url: string, isVideo: boolean): void
  (e: 'close'): void
}>()
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-[70] flex items-center justify-center bg-black/65 p-2 sm:p-4" @click.self="emit('close')">
      <div class="panel-solid flex h-[86vh] w-full max-w-4xl flex-col overflow-hidden rounded-2xl shadow-2xl sm:h-[80vh]">
        <div class="flex items-center gap-2 border-b border-[var(--card-border)] px-3 py-2.5 sm:px-4">
          <h3 class="mr-auto text-sm font-semibold">{{ title }}</h3>
          <p class="hidden text-[11px] opacity-50 sm:block">点一下就用</p>
          <button
            class="grid h-8 w-8 place-items-center rounded-lg opacity-60 hover:bg-black/5 hover:opacity-100 dark:hover:bg-white/10"
            @click="emit('close')"
          >
            ✕
          </button>
        </div>
        <div class="flex min-h-0 flex-1 flex-col px-2.5 pb-3 pt-2 sm:px-4">
          <MediaBrowser
            :kind="kind"
            :purpose="purpose"
            @pick="(u) => emit('pick', u)"
            @wallpaper="(u, v) => emit('wallpaper', u, v)"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>
