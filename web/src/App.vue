<script setup lang="ts">
import { state } from './store'
</script>

<template>
  <RouterView v-slot="{ Component }">
    <Transition name="fade" mode="out-in">
      <component :is="Component" />
    </Transition>
  </RouterView>

  <!-- 轻提示 -->
  <div class="pointer-events-none fixed inset-x-0 top-4 z-[100] flex flex-col items-center gap-2">
    <TransitionGroup name="fade">
      <div
        v-for="t in state.toasts"
        :key="t.id"
        class="pointer-events-auto max-w-[90vw] rounded-xl border px-4 py-2 text-sm shadow-card"
        :class="
          t.type === 'err'
            ? 'border border-rose-400/30 bg-rose-500/20 text-rose-100'
            : 'border border-emerald-400/30 bg-emerald-500/20 text-emerald-50'
        "
      >
        {{ t.text }}
      </div>
    </TransitionGroup>
  </div>
</template>
