// 用 jsdom 真渲染一遍前端 bundle，确认不是白屏、登录页能出来。
// 用法：node scripts/smoke.mjs [baseURL]
import { JSDOM } from 'jsdom'
import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath, pathToFileURL } from 'node:url'
import path from 'node:path'

const here = path.dirname(fileURLToPath(import.meta.url))
const dist = path.resolve(here, '../../internal/web/dist')
const argv = process.argv.slice(2)
const login = argv.includes('--login')
const base = argv.find((a) => a.startsWith('http')) || 'http://127.0.0.1:8019'

const html = readFileSync(path.join(dist, 'index.html'), 'utf8')
const assetsDir = path.join(dist, 'assets')
const jsFile = readdirSync(assetsDir).find((f) => f.endsWith('.js'))
if (!jsFile) throw new Error('dist/assets 里没有 js，先 npm run build')

const dom = new JSDOM(html, {
  url: base + '/',
  pretendToBeVisual: true,
  runScripts: 'outside-only',
})
const { window } = dom

// 把 jsdom 的浏览器环境塞进 Node 全局
for (const key of [
  'window', 'document', 'navigator', 'location', 'history', 'localStorage', 'sessionStorage',
  'HTMLElement', 'SVGElement', 'Element', 'Node', 'Event', 'CustomEvent', 'MouseEvent',
  'MutationObserver', 'DOMParser', 'getComputedStyle', 'requestAnimationFrame',
  'cancelAnimationFrame', 'CSS', 'Image',
]) {
  if (window[key] !== undefined) globalThis[key] = window[key]
}
globalThis.window = window

window.matchMedia ||= (query) => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener() {},
  removeEventListener() {},
  addListener() {},
  removeListener() {},
  dispatchEvent() {
    return false
  },
})
globalThis.matchMedia = window.matchMedia

class RO {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver = window.ResizeObserver = RO
globalThis.IntersectionObserver = window.IntersectionObserver = RO

// fetch：相对路径补全 base，顺便打印请求，方便看链路
const realFetch = globalThis.fetch

// 传 --login 就先登录，把 cookie 带上，验证"登录后的首页"
let cookie = ''
if (login) {
  const res = await realFetch(base + '/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: 'admin', password: 'admin123' }),
  })
  const setCookie = res.headers.getSetCookie?.() || []
  cookie = (setCookie[0] || '').split(';')[0]
  console.log('  登录:', res.status, cookie ? '拿到 cookie' : '没拿到 cookie')
}

globalThis.fetch = window.fetch = (input, init = {}) => {
  const url = typeof input === 'string' ? new URL(input, base).toString() : input
  console.log('  →', typeof input === 'string' ? input : input.url)
  if (cookie) {
    init.headers = { ...(init.headers || {}), Cookie: cookie }
  }
  return realFetch(url, init)
}

const errors = []
window.addEventListener('error', (e) => errors.push(String(e.error || e.message)))
const origError = console.error
console.error = (...args) => {
  errors.push(args.map(String).join(' '))
  origError(...args)
}

const code = readFileSync(path.join(assetsDir, jsFile), 'utf8')
try {
  await import(pathToFileURL(path.join(assetsDir, jsFile)).href)
} catch (e) {
  // 有些产物带 import.meta 只能在浏览器跑，退化成 eval
  console.log('  (import 失败，改用 eval:', e.message, ')')
  new Function(code)()
}

// 等 Vue 挂载 + 首次路由跳转完成
await new Promise((r) => setTimeout(r, 1500))

// 「只显示图标」布局下卡片名只在 title 属性里，所以标题也要算进来
const titles = [...window.document.querySelectorAll('[title]')].map((e) => e.getAttribute('title')).join(' ')
const body = (window.document.body.textContent + ' ' + titles).replace(/\s+/g, ' ').trim()
console.log('\n--- 渲染结果 ---')
console.log(body.slice(0, 500) || '(空)')
console.log('--- 错误 ---')
console.log(errors.length ? errors.slice(0, 5).join('\n') : '(无)')

// 未登录时按钮全变图标了，不靠文案判断，靠 icon-btn 数量
const must = login ? ['常用', 'fnOS 管理', 'GitHub'] : []
const missing = must.filter((m) => !body.includes(m))
if (missing.length) {
  console.log('❌ 缺少关键文案:', missing.join(', '))
  process.exit(1)
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

// 登录态应该先看到锁屏
if (login) {
  const lock = window.document.querySelector('.lock-root')
  if (!lock) {
    // 锁屏是用户自己的开关，关着就跳过这组断言（不然每次调样式都误报）
    console.log('· 锁屏当前是关着的，跳过解锁相关断言')
  } else {
  const lockText = lock.textContent.replace(/\s+/g, ' ').trim()
  console.log('  锁屏文案:', lockText.slice(0, 120))
  if (lockText.includes('上滑 或 点一下')) {
    console.log('❌ 旧解锁提示语还在')
    process.exit(1)
  }
  if (!/还没睡呀|早上好|中午好|下午好|晚上好|夜深了/.test(lockText)) {
    console.log('❌ 锁屏底部没有问候语')
    process.exit(1)
  }

  // Enter 解锁 → 等出场动画走完
  window.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Enter' }))
  await sleep(900)
  if (window.document.querySelector('.lock-root')) {
    console.log('❌ 解锁后锁屏没消失')
    process.exit(1)
  }
  }

  // 设置抽屉：之前漏渲染，点开没反应，这里必须真开出来
  const gear = [...window.document.querySelectorAll('.icon-btn')].find((b) => b.title === '设置')
  if (!gear) {
    console.log('❌ 找不到设置按钮')
    process.exit(1)
  }
  gear.click()
  await sleep(400)
  const drawerText = window.document.body.textContent
  if (!drawerText.includes('外观') || !drawerText.includes('锁屏') || !drawerText.includes('账号')) {
    console.log('❌ 设置抽屉没打开')
    process.exit(1)
  }
  console.log('  ✅ 设置抽屉能打开')

  // 素材库 tab：能看到上传过的文件
  const mediaTab = [...window.document.querySelectorAll('button')].find((b) => b.textContent.trim() === '素材库')
  if (!mediaTab) {
    console.log('❌ 设置里没有素材库 tab')
    process.exit(1)
  }
  mediaTab.click()
  await sleep(700)
  const mediaText = window.document.body.textContent
  if (!/共 \d+ 个文件/.test(mediaText)) {
    console.log('❌ 素材库没加载出文件数')
    process.exit(1)
  }
  console.log('  ✅ 素材库能列出文件:', mediaText.match(/共 \d+ 个文件/)?.[0])

  // 探测 tab
  const ptab = [...window.document.querySelectorAll('button')].find((b) => (b.textContent || '').trim() === '探测')
  if (!ptab) {
    console.log('❌ 设置里没有「探测」页签')
    process.exit(1)
  }
  ptab.click()
  await sleep(300)
  const probeText = window.document.body.textContent || ''
  if (!/内网自动切换/.test(probeText)) {
    console.log('❌ 探测面板没渲染出来')
    process.exit(1)
  }
  console.log('  ✅ 探测设置面板: 有')

  // 卡片外观变量有没有真落到 :root 上（改了看不到的第一嫌疑）
  const cv = (k) => window.document.documentElement.style.getPropertyValue(k)
  const cardEl = window.document.querySelector('.card-hover')
  const bg = cardEl ? window.getComputedStyle(cardEl).backgroundColor : '(没有卡片)'
  console.log('  卡片变量: --card-bg=' + cv('--card-bg'), '| --card-blur=' + cv('--card-blur'), '| --card-radius=' + cv('--card-radius'))
  console.log('  卡片实际背景色:', bg)

  // 图标库：关设置 → 进编辑模式 → 点一张卡片 → 图标库
  const closeDrawer = [...window.document.querySelectorAll('button')].find((b) => (b.textContent || '').trim() === '关闭')
  closeDrawer?.click()
  await sleep(300)
  // 进编辑模式（已经在了就不动，别点成退出）
  const curTitle = window.document.querySelector('.icon-btn')?.getAttribute('title')
  if (curTitle !== '完成编辑') {
    const editBtn2 = [...window.document.querySelectorAll('.icon-btn')].find(
      (b) => b.getAttribute('title') === '编辑卡片',
    )
    editBtn2?.click()
  }
  await sleep(600)
  const addBtn = [...window.document.querySelectorAll('button')].find((b) => (b.textContent || '').includes('添加'))
  if (!addBtn) {
    console.log('❌ 找不到「+ 添加」按钮')
    process.exit(1)
  }
  addBtn.click()
  await sleep(500)
  const editorText = window.document.body.textContent || ''
  if (!/打开方式/.test(editorText)) {
    console.log('❌ 卡片编辑器没打开')
    process.exit(1)
  }
  console.log('  ✅ 卡片编辑器能打开（新建卡片）')

  const quit = [...window.document.querySelectorAll('.icon-btn')].find((b) => b.title === '退出登录')
  if (!quit) {
    console.log('❌ 没有退出登录按钮')
    process.exit(1)
  }
  // 一言应该在右上角那块（存在即可）
  console.log('  图标按钮:', [...window.document.querySelectorAll('.icon-btn')].map((b) => b.title).join(', '))
  // 页脚：一个框里的 <style> 拆出来挂 head，<script> 应该真的跑过
  const hd = window.document.querySelector('header')
  console.log('  顶栏:', hd ? hd.className.replace(/\s+/g, ' ') + ' | padding=' + hd.style.paddingLeft + '/' + hd.style.paddingTop : '(无)')
  const ib = window.document.querySelector('.icon-box-img')
  console.log('  图标容器:', ib ? ib.className.replace(/\s+/g, ' ').slice(0, 96) : '(无)')
  console.log('  图标底色变量:', window.getComputedStyle(window.document.documentElement).getPropertyValue('--icon-bg') || '(空)')
  console.log('  页面标题:', JSON.stringify(window.document.querySelector('header h1')?.textContent?.trim() || '(无)'), '| 浏览器标题:', JSON.stringify(window.document.title))
  console.log('  只图标布局:', window.document.querySelectorAll('.icon-row').length ? '开' : '关')
  const si = window.document.querySelector('.search-bar input')
  const vid = window.document.querySelector('video')
  console.log('  视频壁纸:', vid ? (JSON.stringify(vid.getAttribute('src')).slice(0, 46) + ' | autoplay=' + vid.getAttribute('autoplay') + ' | 停在首帧=' + (/[#]t=/.test(vid.getAttribute('src') || '') ? '是' : '否')) : '(无)')
  console.log('  搜索框占位符:', si ? JSON.stringify(si.getAttribute('placeholder')) : '(没有搜索框)')
  // 探测结果：等一会儿再看小点（jsdom 里外部 HEAD 基本都会失败，红点也算渲染出来了）
  await new Promise((r) => setTimeout(r, 3200))
  const dots = [...window.document.querySelectorAll('.lat-dot')]
  const dotKinds = dots.reduce((m, d) => { const k = ['lat-ok', 'lat-mid', 'lat-bad'].find((c) => d.classList.contains(c)) || '?'; m[k] = (m[k] || 0) + 1; return m }, {})
  console.log('  延迟小点:', dots.length ? `${dots.length} 个 ${JSON.stringify(dotKinds)}` : '无')

  const footEl = window.document.querySelector('.wavenav-foot')
  const footCss = window.document.getElementById('wavenav-footer-css')
  console.log('  页脚:', footEl ? '有' : '无', '| 页脚样式:', footCss ? '已注入' : '无', '| 页脚 JS:', window.__waveFoot ? '已执行' : '未执行')
} else {
  const btns = window.document.querySelectorAll('.icon-btn')
  const lock = window.document.querySelector('.lock-root')
  const card = window.document.querySelector('.login-card')
  console.log('  未登录：图标按钮', btns.length, '锁屏', lock ? '有（公开模式也锁）' : '无（非公开模式）', '登录浮层', card ? '有' : '无')
  if (card) {
    const t = card.textContent
    if (!t.includes('账号') || !t.includes('密码') || !t.includes('登录')) {
      console.log('❌ 登录浮层内容不对:', t.replace(/\s+/g, ' ').trim().slice(0, 80))
      process.exit(1)
    }
    console.log('  ✅ 登录是叠在主页上的浮层（背后 '.concat(window.document.querySelector('.content-in') ? '有主页' : '看不到主页', '）'))
  }
  if (btns.length > 1) {
    console.log('❌ 未登录只该有 1 个登录图标，实际', btns.length)
    process.exit(1)
  }
}
console.log('✅ 冒烟通过')
process.exit(0)
