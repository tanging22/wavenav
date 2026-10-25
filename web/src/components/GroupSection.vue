<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import Sortable from 'sortablejs'
import type { TGroup } from '../types'
import { api } from '../api'
import { fail, loadPanel, state, toast } from '../store'
import ItemCard from './ItemCard.vue'

const props = defineProps<{ group: TGroup }>()
const emit = defineEmits<{ (e: 'edit', id: number): void }>()

const listEl = ref<HTMLElement>()
let sortable: Sortable | null = null

// 本组显示方式：display 优先（card/icon），空 = 跟随全局布局
const effLayout = computed(() => {
  const d = props.group.display
  if (d === 'card' || d === 'icon') return d
  return state.settings?.layout === 'icon' ? 'icon' : 'grid'
})

const gridClass = () => {
  switch (effLayout.value) {
    case 'icon':
      // auto-fill 网格：每一行都顶满左右两条边，列与列上下对齐，手机上不再一边宽一边窄
      return 'icon-row grid'
    default:
      return 'grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5'
  }
}

// 只图标模式的网格参数：列宽跟着图标走，行内自动均分
const iconGridStyle = computed(() => {
  if (effLayout.value !== 'icon') return {}
  return {
    gridTemplateColumns: 'repeat(auto-fill, minmax(min(var(--icon-size), 21vw), 1fr))',
    gap: 'min(var(--icon-spacing), 4.5vw)',
    justifyItems: 'center',
  }
})

async function initSortable() {
  await nextTick()
  if (!listEl.value || sortable) return
  sortable = Sortable.create(listEl.value, {
    animation: 150,
    ghostClass: 'opacity-40',
    draggable: '[data-item-id]',
    onEnd: async (evt) => {
      const from = evt.oldIndex ?? -1
      const to = evt.newIndex ?? -1
      if (from < 0 || to < 0 || from === to) return
      const items = [...props.group.items]
      const [moved] = items.splice(from, 1)
      items.splice(to, 0, moved)
      props.group.items.splice(0, props.group.items.length, ...items)
      const payload = items.map((it, i) => ({ id: it.id, groupId: props.group.id, sort: i }))
      try {
        await api.reorder({ items: payload })
        toast('排序已保存')
      } catch (e) {
        fail(e)
        await loadPanel().catch(() => {})
      }
    },
  })
}

function destroySortable() {
  sortable?.destroy()
  sortable = null
}

watch(
  () => state.editing,
  (editing) => {
    if (editing && !state.readonly) initSortable()
    else destroySortable()
  },
  { immediate: true },
)

onBeforeUnmount(destroySortable)

function addItem() {
  state.editingItem = null
  state.editingItemGroupId = props.group.id
  state.itemEditorOpen = true
}

async function renameGroup() {
  const name = window.prompt('分组名', props.group.name)
  if (!name || name === props.group.name) return
  try {
    const res = await api.updateGroup(props.group.id, { name })
    props.group.name = res.group.name
    toast('已改名')
  } catch (e) {
    fail(e)
  }
}

async function togglePublic() {
  const next = !props.group.hiddenPublic
  try {
    const res = await api.updateGroup(props.group.id, { hiddenPublic: next })
    props.group.hiddenPublic = res.group.hiddenPublic
    toast(next ? '这组已对访客隐藏' : '这组现在公开可见')
  } catch (e) {
    fail(e)
  }
}

async function setDisplay(v: '' | 'card' | 'icon') {
  try {
    const res = await api.updateGroup(props.group.id, { display: v })
    props.group.display = res.group.display
    toast(v === '' ? '本组跟随全局布局' : v === 'card' ? '本组用卡片显示' : '本组用图标显示')
  } catch (e) {
    fail(e)
  }
}

async function removeGroup() {
  if (!window.confirm(`删除分组「${props.group.name}」和里面所有卡片？`)) return
  try {
    await api.deleteGroup(props.group.id)
    const i = state.groups.findIndex((g) => g.id === props.group.id)
    if (i >= 0) state.groups.splice(i, 1)
    toast('已删除分组')
  } catch (e) {
    fail(e)
  }
}
</script>

<template>
  <section class="group-section" :data-group-id="group.id">
    <div class="mb-3 flex items-center gap-2">
      <h2 class="text-sm font-semibold tracking-wide text-[var(--text-main,#f1f5f9)]">{{ group.name }}</h2>
      <span class="text-[11px] text-[var(--text-muted,rgba(241,245,249,0.72))]">{{ group.items.length }}</span>
      <span v-if="group.hiddenPublic && !state.editing" class="text-[11px] text-amber-300/80">仅自己</span>

      <template v-if="!state.readonly && state.editing">
        <span
          class="group-drag -ml-1 inline-flex cursor-grab select-none items-center rounded-md px-1.5 py-1 text-slate-400 hover:bg-white/10 hover:text-sky-300 active:cursor-grabbing"
          title="按住拖动，整组排序"
        >
          <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor" aria-hidden="true">
            <circle cx="5" cy="3" r="1.5" /><circle cx="11" cy="3" r="1.5" />
            <circle cx="5" cy="8" r="1.5" /><circle cx="11" cy="8" r="1.5" />
            <circle cx="5" cy="13" r="1.5" /><circle cx="11" cy="13" r="1.5" />
          </svg>
        </span>
        <button class="rounded-md px-1.5 py-0.5 text-[11px] text-[var(--text-muted,rgba(241,245,249,0.72))] hover:bg-white/10 hover:text-[var(--text-main,#f1f5f9)]" @click="renameGroup">
          改名
        </button>
        <button
          class="rounded-md px-1.5 py-0.5 text-[11px] hover:bg-white/10"
          :class="group.hiddenPublic ? 'text-amber-300' : 'text-emerald-300/90'"
          :title="group.hiddenPublic ? '访客看不到这组，登录后才显示' : '访客也能看到这组'"
          @click="togglePublic"
        >
          {{ group.hiddenPublic ? '仅自己' : '公开' }}
        </button>
        <button
          class="rounded-md px-1.5 py-0.5 text-[11px] hover:bg-white/10"
          :class="group.display === 'icon' ? 'text-sky-300' : group.display === 'card' ? 'text-emerald-300/90' : 'text-[var(--text-muted,rgba(241,245,249,0.72))]'"
          :title="`显示方式：${group.display === 'icon' ? '只图标' : group.display === 'card' ? '卡片' : '跟随全局'}（点击切换）`"
          @click="setDisplay(group.display === '' || !group.display ? 'card' : group.display === 'card' ? 'icon' : '')"
        >
          {{ group.display === 'icon' ? '图标' : group.display === 'card' ? '卡片' : '跟随' }}
        </button>
        <button class="rounded-md px-1.5 py-0.5 text-[11px] text-rose-300/80 hover:bg-rose-500/10" @click="removeGroup">
          删除
        </button>
        <button class="rounded-md px-1.5 py-0.5 text-[11px] text-sky-300 hover:bg-sky-500/10" @click="addItem">+ 卡片</button>
      </template>
    </div>

    <div ref="listEl" :class="gridClass()" :style="effLayout === 'icon' ? iconGridStyle : {}">
      <div v-for="it in group.items" :key="it.id" :data-item-id="it.id">
        <ItemCard :item="it" :layout="effLayout" :readonly="state.readonly" @edit="emit('edit', $event)" />
      </div>

      <button
        v-if="!state.readonly && state.editing"
        class="grid min-h-[72px] place-items-center rounded-2xl border border-dashed border-white/15 text-xs text-slate-400 transition hover:border-sky-400/50 hover:text-sky-300"
        :style="effLayout === 'icon' ? { width: 'min(var(--icon-size), 21vw)', height: 'min(var(--icon-size), 21vw)', minHeight: '0' } : {}"
        @click="addItem"
      >
        + 添加
      </button>
    </div>

    <p v-if="group.items.length === 0 && !state.editing" class="rounded-xl border border-dashed border-white/10 py-6 text-center text-xs text-[var(--text-muted,rgba(241,245,249,0.72))]">
      这组还是空的
    </p>
  </section>
</template>
