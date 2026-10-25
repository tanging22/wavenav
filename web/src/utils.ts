import type { Greetings, ItemURL, Settings, TItem } from './types'

/** IPv4 字符串转 32 位整数，非法返回 null */
export function hexToRgba(hex: string, alpha: number): string {
  const m = /^#?([0-9a-f]{6})$/i.exec((hex || '').trim())
  const a = Math.min(1, Math.max(0, alpha))
  if (!m) return `rgba(255, 255, 255, ${a})`
  const n = parseInt(m[1], 16)
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${a})`
}

export function ipv4ToInt(ip: string): number | null {
  const parts = ip.split('.')
  if (parts.length !== 4) return null
  let out = 0
  for (const p of parts) {
    if (!/^\d{1,3}$/.test(p)) return null
    const n = Number(p)
    if (n > 255) return null
    out = (out << 8) | n
  }
  return out >>> 0
}

/** ip 是否落在 cidr（形如 192.168.0.0/16）里 */
export function inCidr(ip: string, cidr: string): boolean {
  const [net, bitsRaw] = cidr.split('/')
  const bits = Number(bitsRaw ?? 32)
  const ipInt = ipv4ToInt(ip)
  const netInt = ipv4ToInt(net)
  if (ipInt === null || netInt === null || Number.isNaN(bits) || bits < 0 || bits > 32) return false
  if (bits === 0) return true
  const mask = (0xffffffff << (32 - bits)) >>> 0
  return (ipInt & mask) === (netInt & mask)
}

/**
 * 当前访问是不是"内网访问"：私有 IP / localhost / 无点主机名 / 命中 lanRules。
 * 用来决定卡片优先打开内网地址还是外网地址。
 */
export function isLanAccess(hostname: string, lanRules: string[] = []): boolean {
  const host = hostname.toLowerCase()
  if (!host) return false
  if (host === 'localhost' || host.endsWith('.local') || host.endsWith('.lan')) return true
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(host)) {
    const ipInt = ipv4ToInt(host)
    if (ipInt === null) return false
    const [a, b] = host.split('.').map(Number)
    const isPrivate = a === 10 || a === 127 || (a === 192 && b === 168) || (a === 172 && b >= 16 && b <= 31) || (a === 169 && b === 254)
    if (isPrivate) return true
    return lanRules.some((rule) => inCidr(host, rule))
  }
  return false
}

/** 按内网/外网挑选卡片该打开的地址 */
export function pickURL(item: TItem, lanAccess: boolean): ItemURL | null {
  const urls = item.urls || []
  if (urls.length === 0) return null
  // 就两条规则：探测到内网 → 优先内网地址；其他情况 → 通用地址。
  // 老数据里的 wan/custom 都当通用兜底，不会点不开。
  const rank = (u: ItemURL) => {
    if (lanAccess) return u.kind === 'lan' ? 0 : 1
    return u.kind === 'default' ? 0 : u.kind === 'lan' ? 2 : 1
  }
  return [...urls].sort((a, b) => rank(a) - rank(b) || (a.weight ?? 0) - (b.weight ?? 0))[0]
}

export type IconKind = 'image' | 'text' | 'empty'

/** 图标字段可以是 URL、/uploads 路径，也可以直接是一段文字/emoji */
export function iconKind(icon: string): IconKind {
  if (!icon) return 'empty'
  if (/^(https?:)?\/\//.test(icon) || icon.startsWith('/uploads/') || icon.startsWith('data:')) return 'image'
  return 'text'
}

// ---------- 日出日落 ----------
// 简化版 SunCalc：够用来判断"现在算白天还是晚上"，不用引第三方库。

const RAD = Math.PI / 180
const DAY_MS = 86400000
const J1970 = 2440588
const J2000 = 2451545

const toJulian = (date: Date) => date.valueOf() / DAY_MS - 0.5 + J1970
const fromJulian = (j: number) => new Date((j + 0.5 - J1970) * DAY_MS)
const toDays = (date: Date) => toJulian(date) - J2000

const solarMeanAnomaly = (d: number) => RAD * (357.5291 + 0.98560028 * d)

function eclipticLongitude(M: number) {
  const C = RAD * (1.9148 * Math.sin(M) + 0.02 * Math.sin(2 * M) + 0.0003 * Math.sin(3 * M))
  const P = RAD * 102.9372 // 近日点
  return M + C + P + Math.PI
}

const declination = (L: number) => Math.asin(Math.sin(RAD * 23.4397) * Math.sin(L))
const julianCycle = (d: number, lw: number) => Math.round(d - 0.0009 - lw / (2 * Math.PI))
const approxTransit = (Ht: number, lw: number, n: number) => 0.0009 + (Ht + lw) / (2 * Math.PI) + n
const solarTransitJ = (ds: number, M: number, L: number) => J2000 + ds + 0.0053 * Math.sin(M) - 0.0069 * Math.sin(2 * L)

export interface SunTimes {
  sunrise: Date
  sunset: Date
}

/** 给定经纬度算今天的日出日落；极昼/极夜返回 null */
export function sunTimes(lat: number, lng: number, date = new Date()): SunTimes | null {
  if (!lat && !lng) return null
  const lw = RAD * -lng
  const phi = RAD * lat
  const d = toDays(date)
  const n = julianCycle(d, lw)
  const ds = approxTransit(0, lw, n)
  const M = solarMeanAnomaly(ds)
  const L = eclipticLongitude(M)
  const dec = declination(L)
  const Jnoon = solarTransitJ(ds, M, L)

  const h0 = -0.833 * RAD // 太阳中心在地平线下 0.833°
  const cosH = (Math.sin(h0) - Math.sin(phi) * Math.sin(dec)) / (Math.cos(phi) * Math.cos(dec))
  if (cosH > 1 || cosH < -1) return null

  const H = Math.acos(cosH)
  const Jset = solarTransitJ(approxTransit(H, lw, n), M, L)
  const Jrise = Jnoon - (Jset - Jnoon)
  return { sunrise: fromJulian(Jrise), sunset: fromJulian(Jset) }
}

/** theme=sun 时：日落后算黑，日出前也算黑 */
export function isDarkNow(settings: Settings | null): boolean {
  // 深色 / 浅色 / 跟随系统 三种；没设置过就当深色
  const theme = settings?.theme ?? 'dark'
  if (theme === 'light') return false
  if (theme === 'auto') return window.matchMedia('(prefers-color-scheme: dark)').matches
  return true
}

// ---------- 外观应用 ----------

let lastCSS = ''
let lastJS = ''

/** 自定义 CSS 直接覆盖 style 内容；自定义 JS 只在内容变化时执行一次 */
export function applyCustom(custom?: { css: string; js: string } | null) {
  const css = custom?.css ?? ''
  if (css !== lastCSS) {
    let el = document.getElementById('wavenav-custom-css') as HTMLStyleElement | null
    if (!el) {
      el = document.createElement('style')
      el.id = 'wavenav-custom-css'
      document.head.appendChild(el)
    }
    el.textContent = css
    lastCSS = css
  }
  const js = custom?.js ?? ''
  if (js !== lastJS) {
    lastJS = js
    if (js.trim()) {
      try {
        // 站长自己的后台，信任级别等同于服务器代码
        new Function(js)()
      } catch (e) {
        console.warn('[wavenav] 自定义 JS 出错：', e)
      }
    }
  }
}

/** 粗判壁纸图片亮不亮：加载小图取平均亮度，>60% 算亮。失败按"暗"算（深底白字最稳）。 */
const brightCache = new Map<string, boolean>()
function wallpaperIsBright(src: string): boolean {
  if (!src) return false
  const cached = brightCache.get(src)
  if (cached !== undefined) return cached
  const img = new Image()
  img.referrerPolicy = 'no-referrer'
  img.onload = () => {
    try {
      const c = document.createElement('canvas')
      const scale = 32 / Math.max(img.width, img.height)
      c.width = Math.max(1, Math.round(img.width * scale))
      c.height = Math.max(1, Math.round(img.height * scale))
      const ctx = c.getContext('2d')
      if (!ctx) return
      ctx.drawImage(img, 0, 0, c.width, c.height)
      const d = ctx.getImageData(0, 0, c.width, c.height).data
      let sum = 0
      for (let i = 0; i < d.length; i += 4) sum += 0.2126 * d[i] + 0.7152 * d[i + 1] + 0.0722 * d[i + 2]
      const avg = sum / (d.length / 4) / 255
      brightCache.set(src, avg > 0.6)
      // 算完再刷一次主题，字色跟上
      if (lastSettings) applyTheme(lastSettings)
    } catch {
      /* 画布被污染（跨域）就保持暗底白字 */
    }
  }
  img.src = src
  return false
}

let lastSettings: Settings | null = null
/** 主题 + 卡片外观 + favicon + 自定义代码，一次全部落地。preview=true 时不执行自定义 JS（设置里拖滑块用） */
export function applyTheme(settings: Settings | null, preview = false) {
  lastSettings = settings
  const root = document.documentElement
  const dark = isDarkNow(settings)
  root.classList.toggle('dark', dark)
  root.style.colorScheme = dark ? 'dark' : 'light'

  const card = settings?.card ?? { opacity: 8, blur: 0, radius: 18, iconSize: 56, iconSpacing: 18, iconBg: '#ffffff', iconBgAlpha: 0, dotOffset: 8 }
  root.style.setProperty('--card-radius', `${card.radius ?? 18}px`)
  root.style.setProperty('--card-blur', `${card.blur ?? 0}px`)
  // 0 就是不加滤镜：blur(0) 也会让浏览器开一层合成，视频壁纸下特别掉帧
  root.style.setProperty('--card-backdrop', (card.blur ?? 0) > 0 ? `blur(${card.blur}px)` : 'none')
  // 「只显示图标」模式的尺寸与间距
  root.style.setProperty('--icon-size', `${card.iconSize ?? 56}px`)
  root.style.setProperty('--icon-spacing', `${card.iconSpacing ?? 18}px`)
  // 只图标模式的图标底色：透明 PNG 垫上这层才看得见，滑块 0 就是全透明
  root.style.setProperty('--icon-bg', hexToRgba(card.iconBg || '#ffffff', (card.iconBgAlpha ?? 0) / 100))
  // 顶栏是贴浏览器窗口的，尺寸单独一套
  const bar = settings?.bar
  if (bar) {
    root.style.setProperty('--bar-icon', `${bar.iconSize ?? 34}px`)
    root.style.setProperty('--bar-title', `${bar.titleSize ?? 17}px`)
  }
  // 滑块所见即所得：0 就是全透明，100 就是纯白，不再偷偷给小屏加成
  // 透明度滑块 0–100：0 全透明，100 接近纯白底。
  // 35 以上算"浅底"，底色会加速变实、字同时压深 —— 不然 44% 白底配白字就是一片糊。
  const raw = Math.min(1, Math.max(0, (card.opacity ?? 8) / 100))
  // 浅色主题的卡片天然是浅底（白卡 + 深字）；
  // 深色主题下透明度拉高（>=30%）卡片变白，也要切深字，不然白底白字一片糊
  const alpha = raw
  root.style.setProperty('--card-bg', `rgba(255, 255, 255, ${alpha.toFixed(3)})`)
  root.style.setProperty(
    '--card-border',
    `rgba(255, 255, 255, ${Math.min(0.28, raw + 0.06).toFixed(3)})`,
  )

  // 文字颜色每个模式独立可调；默认深色 #f1f5f9（白字）、浅色 #1e293b（深字）
  // —— 但主题只决定「面板」的底色，页面文字底下其实是壁纸。
  // 壁纸是深色的（绝大多数人），浅色主题也必须用白字，不然字糊进壁纸里。
  // 规则：壁纸是亮色图片 → 用浅色那套深字；其余（深色主题 / 视频壁纸 / 无壁纸的深底）→ 白字。
  const text = settings?.text ?? { dark: '#f1f5f9', light: '#1e293b' }
  const brightWallpaper =
    settings?.wallpaper?.type === 'image' && wallpaperIsBright(settings.wallpaper.src)
  const main = dark || !brightWallpaper ? text.dark || '#f1f5f9' : text.light || '#1e293b'
  root.style.setProperty('--text-main', main)
  root.style.setProperty('--text-muted', hexToRgba(main, 0.72))

  // 卡片文字单独一套：卡片是半透明白底叠在壁纸上，得按「卡片底色实际明暗」切。
  // 透明度拉到 30%+ 卡片变白 → 深字；否则白字。
  const lightCard = raw >= 0.3
  root.style.setProperty('--card-fg', lightCard ? '#1e293b' : '#ffffff')
  root.style.setProperty('--card-fg-muted', lightCard ? 'rgba(30,41,59,0.62)' : 'rgba(255,255,255,0.62)')

  // 浏览器标签可以用单独的标题，没设就退回站点名
  const tab = settings?.tabTitle?.trim() || settings?.title
  if (tab) document.title = tab
  const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  const fav = settings?.favicon?.trim()
  if (link) link.href = fav || '/favicon.svg'
  else if (fav) {
    const el = document.createElement('link')
    el.rel = 'icon'
    el.href = fav
    document.head.appendChild(el)
  }

  // 预览时不碰自定义代码，保留页面上已经在生效的那一份
  if (!preview) {
    applyCustom(settings?.custom)
    applyFooterCode(settings)
  }
}

/** 引擎没填图标时，用它自己域名的 favicon 顶上 */
export function faviconFor(url: string): string {
  try {
    return new URL(url).origin + '/favicon.ico'
  } catch {
    return ''
  }
}

const DEFAULT_GREET: Greetings = {
  dawn: '还没睡呀',
  morning: '早上好',
  noon: '中午好',
  afternoon: '下午好',
  evening: '晚上好',
  night: '夜深了',
}

/** 锁屏底部的问候，按时段取；站长自定义过就用他的，留空回默认 */
export function greeting(greet?: Partial<Greetings> | null): string {
  const pick = (k: keyof Greetings) => greet?.[k]?.trim() || DEFAULT_GREET[k]
  const h = new Date().getHours()
  if (h < 5) return pick('dawn')
  if (h < 11) return pick('morning')
  if (h < 14) return pick('noon')
  if (h < 18) return pick('afternoon')
  if (h < 23) return pick('evening')
  return pick('night')
}

// ---------- 页脚自定义 ----------

let lastFooterScript = ''

/** 一个框里的东西按 <style> / <script> 切开，剩下的当 HTML */
export function splitFooterCode(code: string) {
  const styles: string[] = []
  const scripts: string[] = []
  let html = code || ''
  html = html.replace(/<style[^>]*>([\s\S]*?)<\/style>/gi, (_m, css: string) => {
    styles.push(css)
    return ''
  })
  html = html.replace(/<script[^>]*>([\s\S]*?)<\/script>/gi, (_m, js: string) => {
    scripts.push(js)
    return ''
  })
  return { html: html.trim(), css: styles.join('\n'), js: scripts.join('\n') }
}

/** 页脚里的 CSS 挂 head，JS 内容变了才执行一次 */
export function applyFooterCode(settings: Settings | null) {
  const { css, js } = splitFooterCode(settings?.footerCode ?? '')

  let el = document.getElementById('wavenav-footer-css') as HTMLStyleElement | null
  if (css) {
    if (!el) {
      el = document.createElement('style')
      el.id = 'wavenav-footer-css'
      document.head.appendChild(el)
    }
    if (el.textContent !== css) el.textContent = css
  } else if (el) {
    el.remove()
  }

  if (!js) {
    lastFooterScript = ''
    return
  }
  if (js === lastFooterScript) return
  lastFooterScript = js
  try {
    new Function('nav', js)({
      title: settings?.title ?? '',
      year: new Date().getFullYear(),
      settings,
    })
  } catch (e) {
    console.warn('[页脚 JS 出错]', e)
  }
}

/** 页脚文案里的 {year} {month} {day} {date} {time} {title} 会被替换掉 */
export function renderFooterVars(html: string, settings: Settings | null): string {
  if (!html) return ''
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return html
    .replace(/\{year\}/g, String(now.getFullYear()))
    .replace(/\{month\}/g, pad(now.getMonth() + 1))
    .replace(/\{day\}/g, pad(now.getDate()))
    .replace(/\{date\}/g, `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`)
    .replace(/\{time\}/g, `${pad(now.getHours())}:${pad(now.getMinutes())}`)
    .replace(/\{title\}/g, settings?.title ?? '')
}

/** 真正塞进 v-html 的那段（已经去掉 style/script） */
export function footerMarkup(settings: Settings | null): string {
  return renderFooterVars(splitFooterCode(settings?.footerCode ?? '').html, settings)
}

// ---------- 时钟格式 ----------

/** 把 Go 那套 "15:04:05" 翻成前端能显示的，秒关掉就把 :05 一起拿掉 */
export function clockText(date: Date, format: string, seconds: boolean): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  let fmt = format || '15:04:05'
  if (!seconds) fmt = fmt.replace(/:?05/g, '')
  return fmt
    .replace(/15/g, pad(date.getHours()))
    .replace(/04/g, pad(date.getMinutes()))
    .replace(/05/g, pad(date.getSeconds()))
}

/**
 * 前台探一下某个地址能不能通、大概多快。
 * 用 no-cors + HEAD：拿不到响应内容（跨域本来也读不到），但能知道「通不通」和耗时。
 * 返回毫秒；超时 / 被浏览器拦 / 真连不上都返回 null。
 */
export async function probeLatency(url: string, timeout = 3000): Promise<number | null> {
  if (!url) return null
  const ctrl = new AbortController()
  const timer = window.setTimeout(() => ctrl.abort(), Math.max(300, timeout))
  const t0 = performance.now()
  try {
    await fetch(url, { method: 'HEAD', mode: 'no-cors', cache: 'no-store', redirect: 'follow', signal: ctrl.signal })
    return Math.round(performance.now() - t0)
  } catch {
    return null
  } finally {
    window.clearTimeout(timer)
  }
}

/**
 * 判断当前设备在内网还是外网。
 * 设置里填了「内网探测地址」就以它为准（能通=内网）；没填就退回按访问域名判断。
 */
export async function detectLan(settings: Settings | null): Promise<boolean> {
  const p = settings?.probe
  const probeURL = p?.lanProbe?.trim()
  if (p?.enabled && probeURL) {
    const ms = await probeLatency(probeURL, p.timeout || 3000)
    return ms !== null
  }
  return isLanAccess(location.hostname, settings?.lanRules ?? [])
}

/** 延迟分档：绿 / 黄 / 红 */
export function latencyLevel(ms: number | null | undefined): 'none' | 'fast' | 'ok' | 'bad' {
  if (ms === undefined) return 'none'
  if (ms === null) return 'bad'
  if (ms < 150) return 'fast'
  if (ms < 600) return 'ok'
  return 'bad'
}
