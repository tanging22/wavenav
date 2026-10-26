import { reactive } from 'vue'
import { api } from './api'
import type { DockerStatsResp, Settings, TGroup, User } from './types'
import { applyTheme, detectLan, isLanAccess, faviconFor, pickURL, probeLatency } from './utils'

export interface Toast {
  id: number
  text: string
  type: 'ok' | 'err'
}

export const state = reactive({
  settings: null as Settings | null,
  groups: [] as TGroup[],
  user: null as User | null,
  readonly: true,
  loading: false,
  editing: false,
  settingsOpen: false,
  itemEditorOpen: false,
  editingItem: null as number | null, // null = 新建
  editingItemGroupId: 0,
  toasts: [] as Toast[],
  lanAccess: false,
  // 卡片延迟（ms，null = 探测失败，undefined = 还没探）
  latency: {} as Record<number, number | null | undefined>,
  // 一言（锁屏右上角）
  hitokoto: { text: '', from: '' },
  lockShowing: false,
  // 容器/主机状态卡的数据
  dockerStats: null as DockerStatsResp | null,
})
export type { DockerStatsResp }

let toastSeq = 0

export function toast(text: string, type: 'ok' | 'err' = 'ok') {
  const id = ++toastSeq
  state.toasts.push({ id, text, type })
  setTimeout(() => {
    const i = state.toasts.findIndex((t) => t.id === id)
    if (i >= 0) state.toasts.splice(i, 1)
  }, 2600)
}

export function fail(e: unknown) {
  toast(e instanceof Error ? e.message : String(e), 'err')
}

let hitokotoTimer: number | null = null

/** 拉一条一言；失败静默，不让首页报错 */
export async function refreshHitokoto() {
  if (!state.settings?.hitokoto.enabled) return
  try {
    const res = await api.hitokoto(state.settings.hitokoto.category || '')
    state.hitokoto = { text: res.text, from: res.from || '' }
  } catch {
    if (!state.hitokoto.text) state.hitokoto = { text: '海那么大，装得下你所有的情绪。', from: '' }
  }
}

/** 一言每 5 分钟换一条，顺手在切回页面时刷新 */
export function startHitokotoLoop() {
  stopHitokotoLoop()
  void refreshHitokoto()
  hitokotoTimer = window.setInterval(() => void refreshHitokoto(), 5 * 60 * 1000)
}

export function stopHitokotoLoop() {
  if (hitokotoTimer !== null) {
    clearInterval(hitokotoTimer)
    hitokotoTimer = null
  }
}

/** 一次性拉首页数据；未登录会抛 401 */
export async function loadPanel() {
  state.loading = true
  try {
    const data = await api.panel()
    state.settings = data.settings
    state.groups = data.groups
    state.user = data.user
    state.readonly = data.readonly
    // 老配置里的引擎可能没存图标，用域名 favicon 兜一下
    for (const e of data.settings.search.engines) {
      if (!e.icon) e.icon = faviconFor(e.url)
    }
    // 优先用「内网探测地址」实测，没填就按访问域名猜。
    // 探测是网络请求（最长 timeout 毫秒），绝不能挡在首屏渲染前面 ——
    // 先用域名同步猜一个，后台测完再悄悄修正
    state.lanAccess = isLanAccess(location.hostname, data.settings.lanRules ?? [])
    void detectLan(data.settings)
      .then((v) => { state.lanAccess = v })
      .catch(() => {})
    applyTheme(data.settings)
  } finally {
    state.loading = false
  }
}

export function sortGroups(groups: TGroup[]) {
  groups.sort((a, b) => a.sort - b.sort || a.id - b.id)
  for (const g of groups) {
    g.items.sort((a, b) => a.sort - b.sort || a.id - b.id)
  }
  return groups
}

/** 保存设置：先本地生效再落库，失败回滚并提示 */
export async function saveSettings(patch: Partial<Settings>, quiet = false) {
  if (!state.settings) return
  const next = { ...state.settings, ...patch } as Settings
  const prev = state.settings
  state.settings = next
  applyTheme(next)
  try {
    const res = await api.saveSettings(next)
    state.settings = res.settings
    applyTheme(res.settings)
    if (!quiet) toast('已保存')
  } catch (e) {
    state.settings = prev
    applyTheme(prev)
    fail(e)
    throw e
  }
}

let persistTimer: number | null = null

/** 只改过本地状态（比如换了个搜索引擎）时，防抖只把「改动的那一个字段」写回后端。
 *  后端 updateSettings 已改为字段级合并：只传 {search}，不会冲掉别处（如 docker.socket）刚保存的值。 */
export function persistSettingsSoon() {
  if (!state.settings) return
  applyTheme(state.settings)
  if (persistTimer !== null) clearTimeout(persistTimer)
  persistTimer = window.setTimeout(() => {
    persistTimer = null
    if (!state.settings) return
    // 只提交 search 这一个字段，避免把内存里可能过期的其它字段（docker.socket 等）覆盖回旧值
    void api.saveSettings({ search: state.settings.search } as Settings).catch((e) => fail(e))
  }, 500)
}

/** 后端 401 时前端拿来兜底的默认外观（只为让登录浮层背后有背景） */
export function fallbackSettings(): Settings {
  return {
    title: '鲸鲸导航',
    tabTitle: '鲸鲸导航',
    logo: '',
    theme: 'dark',
    wallpaper: { type: 'none', src: '', blur: 0, mask: 0.4, brightness: 1, smooth: true },
    clock: { show: true, seconds: false, format: '15:04:05', fontSize: 0, align: 'center', showDate: true },
    search: {
      show: true,
      publicShow: true,
      engine: 'baidu',
      height: 52,
      radius: 16,
      fontSize: 14,
      maxWidth: 640,
      engines: [
        { key: 'baidu', name: '百度', url: 'https://www.baidu.com/s?wd=%s', icon: 'https://www.baidu.com/favicon.ico' },
        { key: 'google', name: 'Google', url: 'https://www.google.com/search?q=%s', icon: 'https://www.google.com/favicon.ico' },
        { key: 'bing', name: '必应', url: 'https://cn.bing.com/search?q=%s', icon: 'https://cn.bing.com/favicon.ico' },
        { key: 'ddg', name: 'DuckDuckGo', url: 'https://duckduckgo.com/?q=%s', icon: 'https://duckduckgo.com/favicon.ico' },
      ],
    },
    card: { opacity: 8, blur: 0, radius: 18, iconSize: 56, iconSpacing: 18, dotPos: 'tr', iconBg: '#ffffff', iconBgAlpha: 0, dotOffset: 8 },
    bar: { fixed: true, padX: 18, padTop: 14, iconSize: 34, titleSize: 17, maxWidth: 0 },
    page: { padTop: 28, padX: 24, padBottom: 40, maxWidth: 1152 },
    text: { dark: '#f1f5f9', light: '#1e293b' },
    lock: {
      enabled: false,
      animation: 'slide',
      trigger: 'both',
      showDate: true,
      greet: { dawn: '', morning: '', noon: '', afternoon: '', evening: '', night: '' },
    },
    hitokoto: { enabled: true, source: 'hitokoto', category: '' },
    sun: { lat: 0, lng: 0 },
    custom: { css: '', js: '' },
    docker: { enabled: false, socket: '/var/run/docker.sock' },
    probe: { enabled: true, lanProbe: '', showLatency: true, interval: 60, timeout: 3000 },
    footerCode: '',
    lanRules: ['192.168.0.0/16', '10.0.0.0/8', '172.16.0.0/12', '127.0.0.0/8', '100.64.0.0/10'],
    layout: 'grid',
    publicMode: false,
    lang: 'zh-CN',
    favicon: '',
  }
}

// ---------------- 前台可达性探测 ----------------

let probeTimer: number | undefined

/** 把当前所有卡片按目标地址探一遍，结果写进 state.latency（并发 4，别一下打几十个） */
export async function probeAll() {
  const p = state.settings?.probe
  if (!p?.enabled) return
  const items = state.groups.flatMap((g) => g.items)
  const queue = [...items]
  const timeout = p.timeout || 3000
  const workers = Array.from({ length: Math.min(4, queue.length) }, async () => {
    for (;;) {
      const it = queue.shift()
      if (!it) return
      if (it.type && it.type !== 'link') {
        // 容器/主机状态卡没有可探测地址，状态由 stats 循环提供
        state.latency[it.id] = undefined
        continue
      }
      const u = pickURL(it, state.lanAccess)
      state.latency[it.id] = u?.url ? await probeLatency(u.url, timeout) : null
    }
  })
  await Promise.all(workers)
}

let statsTimer: ReturnType<typeof setInterval> | undefined

export async function refreshDockerStats() {
  // 只要有「状态卡」或者「搜索框泛光」开着，就要拉 host 数据：
  // host 来自 /proc，跟 Docker 通不通无关；搜索框泛光颜色也依赖 host.cpuPercent
  const needCard = state.groups
    .flatMap((g) => g.items)
    .some((it) => it.type === 'container' || it.type === 'host')
  const needGlow = !!state.settings?.search.glow
  if (!needCard && !needGlow) {
    state.dockerStats = null
    return
  }
  try {
    const d = await api.dockerStats()
    // host 数据来自 /proc，跟 Docker 通不通无关：哪怕 available=false（代理没连上），
    // 主机的 CPU/内存也能显示，不能因为容器拉不到就整个丢掉
    state.dockerStats = d
  } catch {
    state.dockerStats = null
  }
}

export function startStatsLoop() {
  stopStatsLoop()
  void refreshDockerStats()
  statsTimer = setInterval(() => {
    if (!document.hidden) void refreshDockerStats()
  }, 8000)
}

export function stopStatsLoop() {
  if (statsTimer) clearInterval(statsTimer)
  statsTimer = undefined
}

export function startProbeLoop() {
  stopProbeLoop()
  const p = state.settings?.probe
  if (!p?.enabled || !p.showLatency) return
  // 首轮推迟 1.5 秒：等壁纸和首屏都站稳了再开跑，不然 30 张卡的 HEAD
  // 请求会跟视频壁纸抢带宽，主页显得卡
  probeTimer = window.setTimeout(() => {
    void probeAll()
    probeTimer = window.setInterval(() => {
      if (!document.hidden) void probeAll()
    }, Math.max(10, p.interval || 60) * 1000)
  }, 1500)
}

export function stopProbeLoop() {
  if (probeTimer) {
    window.clearTimeout(probeTimer)
    window.clearInterval(probeTimer)
  }
  probeTimer = undefined
}
