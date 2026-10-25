export interface ItemURL {
  id?: number
  itemId?: number
  kind: 'default' | 'lan' | 'wan' | 'custom' | string
  url: string
  weight: number
}

export interface TItem {
  id: number
  groupId?: number
  name: string
  desc: string
  icon: string
  sort: number
  /** link 普通链接 | container 容器状态 | host 主机状态 */
  type?: 'link' | 'container' | 'host' | string
  /** 状态卡显示内容：both 全部 | cpu 只要CPU | mem 只要内存 */
  stat?: 'both' | 'cpu' | 'mem' | string
  openMode: 'new' | 'self' | string
  layoutMode: string
  urls: ItemURL[]
}

export interface DockerStatEntry {
  id: string
  name: string
  cpuPercent: number
  memUsed: number
  memLimit: number
  state: string
}

export interface DockerStatsResp {
  ok: boolean
  available: boolean
  msg?: string
  host: { cpuPercent: number; memUsed: number; memTotal: number }
  containers?: DockerStatEntry[]
}

export interface TGroup {
  id: number
  name: string
  slug: string
  sort: number
  style: string
  /** 本组显示方式：空=跟随全局，card=卡片，icon=只图标 */
  display?: '' | 'card' | 'icon' | string
  hiddenPublic: boolean
  items: TItem[]
}

export interface SearchEngine {
  key: string
  name: string
  url: string
  icon?: string
}

export type ThemeMode = 'dark' | 'light' | 'auto'
export type LockAnimation = 'fade' | 'slide' | 'zoom' | 'blur' | 'curtain' | 'flip'
export type LockTrigger = 'click' | 'swipe' | 'both'

export interface CardStyle {
  opacity: number
  blur: number
  radius: number
  /** 「只显示图标」模式的图标边长 */
  iconSize: number
  /** 「只显示图标」模式的图标间距 */
  iconSpacing: number
  /** 「只显示图标」模式的图标底色（#rrggbb），透明 PNG 垫这层才看得清 */
  iconBg: string
  /** 底色不透明度 0-100，0 = 完全透明 */
  iconBgAlpha: number
  /** 延迟小点的位置 */
  dotPos: 'tl' | 'tr' | 'bl' | 'br'
  /** 延迟小点离卡片边缘的距离 px */
  dotOffset: number
}

/** 主页内容区留白 */
export interface ProbeConfig {
  enabled: boolean
  /** 只有内网能通的地址，填了就用它判断当前在哪边 */
  lanProbe: string
  showLatency: boolean
  interval: number
  timeout: number
}

export interface BarStyle {
  fixed: boolean
  padX: number
  padTop: number
  iconSize: number
  titleSize: number
  maxWidth: number
}

export interface TextConfig {
  dark: string
  light: string
}

export interface PageStyle {
  padTop: number
  padX: number
  padBottom: number
  maxWidth: number
}

/** 搜索框外观 */
export interface SearchStyle {
  height: number
  radius: number
  fontSize: number
  maxWidth: number
  /** 搜索框边框淡淡泛光 */
  glow?: boolean
  /** 公开（未登录）时是否也显示搜索框 */
  publicShow?: boolean
}

/** 底部问候语，按时段自定义，留空用默认 */
export interface Greetings {
  dawn: string
  morning: string
  noon: string
  afternoon: string
  evening: string
  night: string
}

export interface LockConfig {
  enabled: boolean
  animation: LockAnimation
  trigger: LockTrigger
  showDate: boolean
  greet: Greetings
  /** 锁屏时间字号，0=自适应 */
  fontSize?: number
}

export interface HitokotoConfig {
  enabled: boolean
  source: string
  category: string
}

export interface SunConfig {
  lat: number
  lng: number
}

export interface CustomCode {
  css: string
  js: string
}

export interface DockerConfig {
  enabled: boolean
  socket: string
}

export interface Settings {
  title: string
  /** 浏览器标签标题，留空跟 title 一样 */
  tabTitle: string
  logo: string
  theme: ThemeMode
  wallpaper: { type: 'none' | 'image' | 'video'; src: string; blur: number; mask: number; brightness: number; smooth?: boolean }
  clock: {
    show: boolean
    seconds: boolean
    format: string
    fontSize: number
    align: 'left' | 'center' | 'right'
    showDate: boolean
  }
  search: { show: boolean; engine: string; engines: SearchEngine[] } & SearchStyle
  card: CardStyle
  page: PageStyle
  text: TextConfig
  bar: BarStyle
  probe: ProbeConfig
  lock: LockConfig
  hitokoto: HitokotoConfig
  sun: SunConfig
  custom: CustomCode
  docker: DockerConfig
  /** 页脚一个框写到底，HTML 里可夹 <style> / <script> */
  footerCode: string
  lanRules: string[]
  layout: 'grid' | 'icon'
  publicMode: boolean
  lang: string
  favicon: string
}

export interface DockerContainer {
  id: string
  name: string
  image: string
  state: string
  status: string
  ports: string[]
  project: string
}

export interface User {
  id: number
  username: string
  nickname: string
  avatar: string
  role: 'admin' | 'user' | string
}

export interface GalleryIcon {
  id: number
  kind: string
  path: string
  thumb?: string
  size: number
  mime: string
  createdAt: string
}
