// Remix Icon 会带出 .ttf/.eot/.svg 三份老格式字体（合计 ~4MB），现代浏览器只用 woff2/woff。
// 构建完顺手删掉，别让它们跟着 go:embed 进二进制。
import fs from 'node:fs'
import path from 'node:path'

const dir = path.resolve('../internal/web/dist/assets')
let saved = 0
for (const f of fs.readdirSync(dir)) {
  if (!/^remixicon-.*\.(svg|ttf|eot)$/.test(f)) continue
  const p = path.join(dir, f)
  saved += fs.statSync(p).size
  fs.unlinkSync(p)
}
console.log(`prune-fonts: 删掉 ${(saved / 1048576).toFixed(1)}MB 多余字体格式`)
