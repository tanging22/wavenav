# 🐋 WaveNav · 鲸鱼导航

一个**可完全替代 Sun-Panel** 的自托管个人导航页：Go + Vue3 单二进制、SQLite、后台可视化编辑、深浅主题、视频壁纸、内外网双地址自动切换。

> 为 NAS / 家庭服务器 / 个人云设计：`docker compose up -d` 就能跑，数据全在一个目录里。

---

## 为什么会有这个项目

调研了 GitHub 上所有主流导航页（详见 [docs/方案调研.md](docs/方案调研.md)），结论是：

| 类型 | 代表 | 能否完全替代 Sun-Panel |
| --- | --- | --- |
| 配置文件型 | homepage、Dashy、Homer、glance | ❌ 改导航要手动写 YAML |
| **后台 GUI + 数据库型** | **Sun-Panel**、homarr v1、Flame、Heimdall | ✅ 但 Sun-Panel 已闭源 |
| 书签管理器型 | linkwarden、linkding | ❌ 不是导航页场景 |
| 极简起始页型 | Bonjourr、nightTab | ❌ 不能存数据、无后台 |

**Sun-Panel 是本类里唯一"完全对味"的，但它从 v1.3.0 之后进入闭源状态**，PRO 功能收费。所以这个项目从零重写，功能对齐 Sun-Panel v1.8.1，且保持 MIT 开源。

---

## 功能

### 已实现（M1）

- **后台可视化编辑**：登录后点「编辑」，拖拽排序、增删分组与卡片，全部即时保存 —— 不用改一行配置
- **卡片双地址**：每张卡片就两个输入框 —— **通用**（在外时走它）和**内网**（在家时走它），地址栏里填好链接点一下就抓标题和图标；检测到在家就自动切换到内网地址
- **图标**：地址栏里填好链接后点一下就抓（标题 + favicon，内网地址同样能抓）、从素材库挑、上传本地图片、或直接用 emoji；外链图标挂了自动退回地球
- **壁纸**：图片 / 视频背景（mp4、webm、mov），支持模糊、遮罩透明度、亮度；**流畅模式**下手机只用视频首帧、切到后台自动暂停（20MB 级视频壁纸不再掉帧）
- **主题**：深色 / 浅色 / 跟随系统。浅色是真正的浅色（白色抽屉与输入框、深色文字），暗色壁纸想用浅色也行——卡片字色会跟着底色明暗自动切；两种布局（**网格**：图标在左、名字与备注在右，一行 2–5 张；**只显示图标**：只有图标，大小与间距都能调）
- **锁屏式首页**：进站先看到「左上时间 + 右上一言 + 底部问候」，点击或上滑像手机一样解锁才展开卡片。6 种解锁动画（上滑 / 淡出 / 放大 / 模糊 / 拉帘 / 翻转）任选，触发方式与日期显示可配
- **自定义问候**：底部六段问候（凌晨 / 早上 / 中午 / 下午 / 晚上 / 深夜）按时段取，留空用默认句，每句最长 20 字
- **锁屏对所有人生效**：公开模式下访客也要先解锁；登录后右上角多一个「退出登录」，退出即回到访客视图并重新上锁
- **一言**：接 [hitokoto.cn](https://hitokoto.cn)，锁屏时在右上角；解锁后直接顶替搜索框里那句「搜点什么…」当占位符（框的大小和输入完全不受影响）；支持类别筛选，3 分钟分桶缓存 + 断网兜底句子
- **搜索框图标化**：左边是当前引擎的 favicon（点开换引擎），右边是线条放大镜按钮；引擎图标可在设置里「抓图标」从首页自动取回，没填的按域名 `/favicon.ico` 兜底
- **标题可分开**：主页上显示的名字（站点标题）和浏览器标签上的标题各写各的，标签标题留空就跟着站点名走
- **顶栏独立定位**：标题 + 按钮贴在浏览器窗口边缘，左右/上留白、按钮大小、标题字号都能单独调，不受「主页留白」影响，手机上自动让开刘海
- **只图标模式图标底色**：底色颜色 + 不透明度可调（0% 即全透明）；透明 PNG 图标在暗壁纸上看不清时垫一层白底就清晰了，网格模式不受影响
- **卡片外观可调**：透明度、磨砂（默认 0，即不磨砂）、圆角三根滑块实时预览；手机窄屏会自动把底色调实一点，配视频壁纸也看得清
- **前台可达性探测**：在浏览器里把你的卡片挨个 HEAD 一遍，图标右上角点出绿（<150ms）/ 黄（<600ms）/ 红（不通），悬停看具体毫秒；间隔、超时都能调，只在页面可见时探
- **内网自动切换**：设置里填一个「只有内网能通」的地址（比如 `http://192.168.1.2:5666`），能连上就当作在家，卡片自动走内网地址；留空则按访问域名判断（私有网段算内网）
- **素材库**：上传过的图片/视频全在「设置 → 素材库」里，可分「全部 / 图片 / 视频」筛选、翻页、删除、一键设为壁纸；还被卡片或壁纸引用的文件会拒绝删除并说明原因
- **媒体选择器**：换图标、换壁纸都能弹出素材库弹窗，网格里直接看到缩略图与视频首帧，也能当场上传
- **设置自动保存**：改完 0.9 秒自动落库，关面板不会丢；头像栏显示「已自动保存」，保存按钮留着当"立刻存 + 关面板"；换搜索引擎这类小操作也会记住
- **自定义 CSS / JS**：设置里直接写主页样式与脚本，保存立刻生效（CSS 走独立 style 标签，JS 只在内容变化时执行一次）
- **自定义页脚**：HTML / CSS / JS 全写在一个框里，`<style>` 和 `<script>` 会自动拆出来挂到页面上；HTML 里可用 `{year}` `{month}` `{day}` `{date}` `{time}` `{title}` 占位，JS 只在内容变化时执行一次
- **主页留白**：上/左右/下边距和内容最大宽度各一根滑杆，锁屏不受影响
- **搜索框外观**：高度、圆角、字号、最大宽度都能调
- **时钟**：日期 + 星期 + 时间；时间格式可写（`15:04:05` / `15:04` / `15时04分`…），字号可调、拖到 0 就是自适应
- **图库可外部引用**：素材库里每张图都直接显示完整 `/uploads/...` 路径并带「复制」，粘到别处就能用；上传放开 png/jpg/jpeg/gif/webp/avif/bmp/svg/ico/cur 与 mp4/m4v/webm/mov
- **Docker 监控**：直连 Docker Engine API（不依赖 docker CLI），列出容器、启动/停止/重启；没权限时给出加 `docker` 组的提示
- **状态卡片**：除了普通链接卡，还能加「容器状态」卡（标题=容器名，备注实时显示 CPU / 内存占用）和「主机状态」卡（CPU / 内存总占用），角点颜色跟容器状态走
- **搜索**：多搜索引擎切换（内置 百度/Google/Bing/DuckDuckGo，可增删）
- **安全**：scrypt 加盐哈希密码、HttpOnly + SameSite Cookie、登录限流（10 分钟 10 次）、会话库内只存 token 的 sha256
- **公开模式**：未登录也能看（只读），可单独把某些分组设为「仅登录可见」
- **上传即图库**：上传的图标/壁纸自动入库，可复用、可删除
- **导入导出**：一键导出全站 JSON 备份
- **单二进制部署**：前端 `go:embed` 打包进 Go 可执行文件，无外部依赖，SQLite 免 CGO

### 路线图

- [x] **M2（部分）** Docker 容器监控（列表 / 启停 / 重启）
- [ ] **M2（剩余）** 容器日志查看、系统状态（CPU/内存/磁盘/网络）、应用快捷启动器
- [ ] **M3** Sun-Panel OpenAPI 兼容层 + 一键迁移（Sun-Panel 导出 → WaveNav）
- [ ] 多用户与权限组、站点监控探活、i18n、自动备份轮转
- [ ] 卡片角标（新窗口/内网/外网标识）、分组折叠、最近访问

---

## 快速开始

### Docker 监控的两种接法

面板要读 Docker 信息，必须有某个环节拿到 socket 权限，二选一：

**方式 A：默认直连（容器里挂 socket + 进 docker 组）**

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
group_add:
  - "994"        # docker 组的 gid，宿主机上 `getent group docker` 可查
```

能看列表也能点启停，权限 = docker 组。

**方式 B：只读代理（不想给面板权限时用，推荐）**

跑一个官方代理容器，它挂 socket，对外只放行查询、挡掉所有写操作：

```bash
docker run -d --name docker-socket-proxy --restart unless-stopped \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -p 127.0.0.1:2375:2375 \
  -e CONTAINERS=1 -e PING=1 -e POST=0 \
  tecnativa/docker-socket-proxy
```

然后给面板设置里 Docker 的 Socket 地址填 `tcp://127.0.0.1:2375`（或启动时 `WAVENAV_DOCKER_SOCK=tcp://127.0.0.1:2375`）。`POST=0` 时启停按钮会返回 403，改成 `POST=1` 即放开。`-p 127.0.0.1:2375:2375` 只监听本机回环；若用 host 网络模式，代理会听在 `0.0.0.0:2375`，局域网内其它设备也能查询（只读），介意请用端口映射方式。

**状态卡片**：编辑卡片时把「卡片类型」切成「容器状态」（下拉选容器，标题=容器名，备注实时 CPU/内存）或「主机状态」，8 秒刷新一次。

### Docker（推荐）

```bash
git clone <this-repo> wavenav && cd wavenav
docker compose up -d --build
```

打开 `http://<你的IP>:8018`，默认账号 **admin / admin123**（第一次登录后请立刻在「设置 → 账号」里改掉；账号名也能在同一个地方改，输当前密码确认即可）。

数据（SQLite + 上传的图片/视频）都在 `./data`：

```
data/
├── wavenav.db        # SQLite（WAL 模式）
├── wavenav.db-wal
└── uploads/
    └── 2026/10/      # 按年月归档，文件名是内容 sha256 前缀
```

### 源码运行

需要 Go ≥ 1.26、Node ≥ 20。

```bash
# 1. 构建前端（产物会写进 internal/web/dist，被 go:embed 打包）
cd web && npm install && npm run build && cd ..

# 2. 编译后端
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o wavenav ./cmd/server

# 3. 跑
WAVENAV_PORT=8018 WAVENAV_DATA=./data ./wavenav
```

国内网络记得：`export GOPROXY=https://goproxy.cn,direct`。

### 开发模式（热更新）

```bash
# 终端 1：后端（前端 dist 会被当成静态资源）
WAVENAV_PORT=8019 WAVENAV_DATA=./data-dev ./wavenav

# 终端 2：Vite（5179，自动代理 /api 和 /uploads 到 8019）
cd web && npm run dev
```

打开 `http://127.0.0.1:5179`。

---

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `WAVENAV_PORT` | `8018` | 监听端口 |
| `WAVENAV_DATA` | `./data` | 数据目录（db + uploads） |
| `WAVENAV_MODE` | `prod` | `dev` 时输出调试日志 |
| `WAVENAV_ADMIN` | `admin` | 首次启动创建的管理员用户名 |
| `WAVENAV_ADMIN_PASSWORD` | `admin123` | 首次启动创建的管理员密码 |
| `WAVENAV_SESSION_DAYS` | `30` | 登录态有效期（天） |
| `WAVENAV_DOCKER_SOCK` | `/var/run/docker.sock` | Docker socket 路径，也支持 `tcp://127.0.0.1:2375` 走只读代理（Docker 监控用） |

> 管理员账号只在数据库为空时创建一次，之后改这几个环境变量不会覆盖已有密码。

---

## API

全部在 `/api/v1` 下，JSON 进出，统一返回 `{ "ok": true, "data": ... }` 或 `{ "ok": false, "error": "..." }`。

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/health` | 公开 | 健康检查 |
| POST | `/auth/login` | 公开 | 登录 |
| POST | `/auth/logout` | 登录 | 退出 |
| GET | `/auth/me` | 公开 | 当前用户，未登录返回 `null` |
| GET | `/panel` | 公开* | 一次性拉全站数据（公开模式关闭时需登录） |
| PUT | `/settings` | 登录 | 整体覆盖站点设置 |
| PUT | `/auth/password` | 登录 | 改自己的密码 |
| POST | `/auth/username` | 登录 | 改登录账号名（要带当前密码确认） |
| PUT | `/auth/admin-password` | 管理员 | 重置指定用户密码 |
| POST/PUT/DELETE | `/groups[/:id]` | 登录 | 分组增改删 |
| POST/PUT/DELETE | `/items[/:id]` | 登录 | 卡片增改删 |
| POST | `/items/reorder` | 登录 | 一次性提交分组/卡片排序 |
| POST | `/upload` | 登录 | 上传图标或壁纸（`kind=icon\|wallpaper`） |
| GET | `/gallery?kind=&type=&page=&size=` | 登录 | 素材列表（`kind=icon\|wallpaper`、`type=image\|video`） |
| DELETE | `/gallery/:id` | 登录 | 删除素材（被卡片或壁纸引用时返回 409） |
| POST | `/fetch-site` | 登录 | 抓目标站点 title + favicon（带 SSRF 防护） |
| GET | `/hitokoto?c=` | 公开 | 一言（`c` 为类别，如 `d` 诗词、`k` 哲学） |
| GET | `/docker/containers` | 登录 | Docker 容器列表（不可用时返回原因） |
| GET | `/docker/stats` | 登录 | 主机 CPU/内存 + 每个容器的 CPU/内存（状态卡数据源） |
| POST | `/docker/containers/:id/:action` | 登录 | 容器 `start` / `stop` / `restart` |
| GET | `/export` | 登录 | 导出全站 JSON |

静态资源：`/uploads/*`（用户上传）、其余所有非 `/api` 路径回退到 SPA 的 `index.html`。

---

## 技术栈

- **后端**：Go 1.26、Gin、GORM、[glebarez/sqlite](https://github.com/glebarez/sqlite)（纯 Go，免 CGO）、`golang.org/x/crypto/scrypt`
- **前端**：Vue 3 + TypeScript + Vite + Tailwind CSS 3 + vue-router + sortablejs
- **部署**：单二进制 / Docker 多阶段构建（镜像 ~25MB）

### 设计要点

- **设置存 JSON blob**：`Setting{ UserID, Data }`，字段随前端演进不需要改表结构
- **`/api` 与 `/uploads` 绝不回退 SPA**：未知 API 返回 JSON 404（很多导航页踩过这个坑，反代后 API 报错变成一整页 HTML）
- **上传按内容去重**：文件名取 sha256 前缀，同用户同类型同内容只存一份；并用 `http.DetectContentType` 校验真实类型，防改扩展名绕过
- **抓 favicon 带 SSRF 防护**：先解析 IP 再拨号（防 DNS rebinding），拒绝回环、私有网段、链路本地、CGNAT
- **内外网自动切换在浏览器端判定**：后端不猜你从哪来，前端用 `location.hostname` + 可配置的私有网段规则决定用哪个 URL

---

## 和 Sun-Panel 的差异

| | Sun-Panel | WaveNav |
| --- | --- | --- |
| 开源 | v1.3.0 后闭源，PRO 收费 | MIT，全功能开源 |
| 后端 | Go + Gin + GORM | 同 |
| 前端 | Vue3 + TS + Vite + Tailwind | 同 |
| 数据库 | SQLite / MySQL / Redis | SQLite（够用、零配置） |
| 部署 | Docker | Docker / 单二进制 |
| 额外 | Docker 管理器、系统状态（PRO） | 规划在 M2 |

---

## License

MIT
