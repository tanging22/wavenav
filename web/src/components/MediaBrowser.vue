<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api'
import { fail, toast } from '../store'
import type { GalleryIcon } from '../types'

/** 素材浏览器：设置里的「素材库」内嵌用，也被 MediaPicker 弹窗包着用 */
const props = withDefaults(
  defineProps<{
    /** 只看图 / 只看视频 / 全都要 */
    kind?: 'image' | 'video' | 'all'
    /** pick：点一下选中并回填；wallpaper：点一下直接设为壁纸 */
    purpose?: 'pick' | 'wallpaper'
  }>(),
  { kind: 'all', purpose: 'pick' },
)
const emit = defineEmits<{
  (e: 'pick', url: string): void
  (e: 'wallpaper', url: string, isVideo: boolean): void
}>()

type Filter = 'all' | 'image' | 'video'
const filter = ref<Filter>(props.kind)
const list = ref<GalleryIcon[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const uploading = ref(false)
const deleting = ref(0)
const fileInput = ref<HTMLInputElement>()

const size = 24
const pages = computed(() => Math.max(1, Math.ceil(total.value / size)))
const filters: { v: Filter; t: string }[] = [
  { v: 'all', t: '全部' },
  { v: 'image', t: '图片' },
  { v: 'video', t: '视频' },
]
const accept = computed(() =>
  filter.value === 'image' ? 'image/*' : filter.value === 'video' ? 'video/mp4,video/webm' : 'image/*,video/mp4,video/webm',
)

async function load() {
  loading.value = true
  try {
    const res = await api.gallery({
      type: filter.value === 'all' ? undefined : filter.value,
      page: page.value,
      size,
    })
    list.value = res.list
    total.value = res.total
  } catch (e) {
    fail(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)

function switchFilter(f: Filter) {
  if (filter.value === f) return
  filter.value = f
  page.value = 1
  void load()
}

function turn(p: number) {
  if (p < 1 || p > pages.value) return
  page.value = p
  void load()
}

async function onUpload(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files?.length) return
  uploading.value = true
  try {
    const file = files[0]
    const isVideo = file.type.startsWith('video/')
    const url = await api.upload(file, isVideo ? 'wallpaper' : 'icon')
    toast('上传好啦')
    page.value = 1
    await load()
    if (!url) return
    // 刚传的直接就用上，省得再找
    if (props.purpose === 'wallpaper') emit('wallpaper', url, isVideo)
    else emit('pick', url)
  } catch (err) {
    fail(err)
  } finally {
    uploading.value = false
    if (fileInput.value) fileInput.value.value = ''
  }
}

async function remove(g: GalleryIcon) {
  if (!window.confirm('删掉这个文件？正在被卡片或壁纸引用的文件会被拒绝删除。')) return
  deleting.value = g.id
  try {
    await api.deleteGallery(g.id)
    toast('删掉了')
    if (list.value.length === 1 && page.value > 1) page.value -= 1
    await load()
  } catch (e) {
    fail(e)
  } finally {
    deleting.value = 0
  }
}

function choose(g: GalleryIcon) {
  if (props.purpose === 'wallpaper') emit('wallpaper', g.path, isVideo(g))
  else emit('pick', g.path)
}

function kb(n: number) {
  if (n >= 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + 'MB'
  return Math.max(1, Math.round(n / 1024)) + 'KB'
}

function isVideo(g: GalleryIcon) {
  return g.mime.startsWith('video/')
}

/** 复制绝对路径，粘到别处直接能用 */
async function copyPath(g: GalleryIcon) {
  const abs = new URL(g.path, location.origin).toString()
  try {
    await navigator.clipboard.writeText(abs)
    toast('路径已复制')
  } catch {
    toast('浏览器不让写剪贴板：' + abs, 'err')
  }
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <!-- 工具栏 -->
    <div class="flex flex-wrap items-center gap-2 border-b border-[var(--card-border)] px-1 pb-2">
      <div class="seg shrink-0">
        <button
          v-for="f in filters"
          :key="f.v"
          :class="filter === f.v ? 'on' : 'opacity-70 hover:opacity-100'"
          @click="switchFilter(f.v)"
        >
          {{ f.t }}
        </button>
      </div>
      <span class="text-[11px] opacity-55">共 {{ total }} 个文件</span>
      <button class="btn-ghost ml-auto h-8 px-2.5 text-xs" :disabled="uploading" @click="fileInput?.click()">
        {{ uploading ? '上传中…' : '＋ 上传' }}
      </button>
      <input ref="fileInput" type="file" :accept="accept" class="hidden" @change="onUpload" />
    </div>

    <!-- 网格 -->
    <div class="min-h-0 flex-1 overflow-y-auto py-2">
      <div v-if="loading" class="grid h-40 place-items-center text-xs opacity-60">加载中…</div>
      <div v-else-if="list.length === 0" class="grid h-40 place-items-center text-center">
        <div>
          <p class="text-sm opacity-60">这里还空着</p>
          <button class="btn-primary mt-3 h-8 text-xs" @click="fileInput?.click()">上传第一个</button>
        </div>
      </div>
      <div v-else class="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-5">
        <div
          v-for="g in list"
          :key="g.id"
          class="group relative aspect-square cursor-pointer overflow-hidden rounded-xl border border-[var(--card-border)] bg-black/20 transition hover:border-sky-400/70 hover:ring-2 hover:ring-sky-400/30"
          :title="purpose === 'wallpaper' ? '点击设为壁纸' : g.path"
          @click="choose(g)"
        >
          <video
            v-if="isVideo(g)"
            :src="g.path"
            class="h-full w-full object-cover"
            muted
            playsinline
            preload="metadata"
            @loadeddata="(e) => ((e.target as HTMLVideoElement).currentTime = 0.1)"
          />
          <img v-else :src="g.path" class="h-full w-full object-cover" alt="" loading="lazy" />
          <span v-if="isVideo(g)" class="pointer-events-none absolute inset-0 grid place-items-center text-white/85 drop-shadow">
            <svg viewBox="0 0 24 24" fill="currentColor" class="h-8 w-8"><path d="M8 5.5v13l11-6.5z" /></svg>
          </span>
          <div class="pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/75 to-transparent px-1.5 pb-1 pt-5">
            <div class="flex items-center gap-1">
              <p class="min-w-0 flex-1 truncate text-[10px] text-white/80" :title="g.path">
                {{ g.path }}
              </p>
              <button
                class="pointer-events-auto shrink-0 rounded px-1 py-0.5 text-[10px] text-white/85 transition hover:bg-white/25"
                title="复制这个路径"
                @click.stop="copyPath(g)"
              >
                复制
              </button>
            </div>
            <p class="truncate text-[9px] text-white/50">{{ kb(g.size) }} · {{ g.mime }}</p>
          </div>
          <button
            class="absolute right-1 top-1 grid h-6 w-6 place-items-center rounded-md bg-black/55 text-[11px] text-white/85 opacity-0 transition hover:bg-rose-600 group-hover:opacity-100"
            :disabled="deleting === g.id"
            title="删除"
            @click.stop="remove(g)"
          >
            ✕
          </button>
        </div>
      </div>
    </div>

    <!-- 分页 -->
    <div v-if="pages > 1" class="flex items-center justify-end gap-1 border-t border-[var(--card-border)] px-1 pt-2 text-xs">
      <button class="btn-ghost h-7 px-2" :disabled="page <= 1" @click="turn(page - 1)">上一页</button>
      <span class="px-1 tabular-nums opacity-60">{{ page }} / {{ pages }}</span>
      <button class="btn-ghost h-7 px-2" :disabled="page >= pages" @click="turn(page + 1)">下一页</button>
    </div>
  </div>
</template>
