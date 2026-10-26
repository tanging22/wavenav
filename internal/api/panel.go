package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wavenav/internal/models"
)

type panelItem struct {
	ID         uint             `json:"id"`
	Name       string           `json:"name"`
	Desc       string           `json:"desc"`
	Icon       string           `json:"icon"`
	Sort       int              `json:"sort"`
	OpenMode   string           `json:"openMode"`
	LayoutMode string           `json:"layoutMode"`
	Type       string           `json:"type"`
	Stat       string           `json:"stat"`
	URLs       []models.ItemURL `json:"urls"`
}

type panelGroup struct {
	ID           uint        `json:"id"`
	Name         string      `json:"name"`
	Slug         string      `json:"slug"`
	Sort         int         `json:"sort"`
	Style        string      `json:"style"`
	Display      string      `json:"display"`
	HiddenPublic bool        `json:"hiddenPublic"`
	Items        []panelItem `json:"items"`
}

// loadSettings 读用户设置，解析失败就回默认值（坏数据不能把首页搞挂）。
func (s *Server) loadSettings(userID uint) models.Settings {
	var st models.Setting
	if err := s.DB.Where("user_id = ?", userID).First(&st).Error; err != nil {
		return models.DefaultSettings()
	}
	var out models.Settings
	if err := json.Unmarshal([]byte(st.Data), &out); err != nil {
		return models.DefaultSettings()
	}
	// 缺失字段兜底
	def := models.DefaultSettings()
	if out.Title == "" {
		out.Title = def.Title
	}
	// 浏览器标签标题没单独设过就跟着站点名走
	if out.TabTitle == "" {
		out.TabTitle = out.Title
	}
	if out.Layout == "" {
		out.Layout = def.Layout
	}
	if out.Wallpaper.Brightness <= 0 {
		out.Wallpaper.Brightness = 1
	}
	if out.Wallpaper.Type == "" {
		out.Wallpaper.Type = "none"
	}
	if out.LanRules == nil {
		out.LanRules = def.LanRules
	}
	if len(out.Search.Engines) == 0 {
		out.Search = def.Search
	}
	if out.Card.Radius == 0 && out.Card.Opacity == 0 && out.Card.Blur == 0 {
		out.Card = def.Card
	}
	if out.Card.DotPos == "" {
		out.Card.DotPos = def.Card.DotPos
	}
	if out.Lock.Animation == "" {
		// 老数据没有 lock 段：整段回默认（含"默认开启锁屏"），别把空字段当用户选择
		out.Lock = def.Lock
	}
	if out.Hitokoto.Source == "" {
		out.Hitokoto = def.Hitokoto
	}
	if out.Sun.Lat == 0 && out.Sun.Lng == 0 {
		out.Sun = def.Sun
	}
	if out.Docker.Socket == "" {
		out.Docker.Socket = def.Docker.Socket
	}
	if out.Clock.Align == "" {
		out.Clock.Align = def.Clock.Align
	}
	// showDate 的零值是 false，光看结构体分不出"用户关了"和"老数据没这字段"，
	// 所以回头看一眼 clock 这一段里到底有没有这个键（注意 lock 里也有个 showDate，不能全串搜）
	if !jsonHasKey(st.Data, "search", "glow") {
		out.Search.Glow = def.Search.Glow
	}
	if !jsonHasKey(st.Data, "search", "publicShow") {
		out.Search.PublicShow = def.Search.PublicShow
	}
	if !jsonHasKey(st.Data, "clock", "showDate") {
		out.Clock.ShowDate = true
	}
	if out.Clock.Format == "" {
		out.Clock.Format = def.Clock.Format
	}
	// 老数据没有这两个字段（都是 0）；用户真把间距调到 0 的话 size 不会是 0，别乱兜
	if out.Card.IconBg == "" {
		out.Card.IconBg = def.Card.IconBg
	}
	if out.Card.IconSize == 0 && out.Card.IconSpacing == 0 {
		out.Card.IconSize = def.Card.IconSize
		out.Card.IconSpacing = def.Card.IconSpacing
	}
	if out.Page == (models.Page{}) {
		out.Page = def.Page
	}
	if out.Bar == (models.Bar{}) {
		out.Bar = def.Bar
	}
	// 老数据没有 probe 字段，两个数值都是 0 就整块回默认（开关也一起打开）
	if !isHexColor(out.Text.Dark) || !isHexColor(out.Text.Light) {
		out.Text = def.Text
	}
	// 老默认浅色是白字（当时深浅都是白），浅色底上根本看不清 —— 迁到新默认深灰
	if out.Text.Light == "#ffffff" {
		out.Text.Light = def.Text.Light
	}
	if out.Probe.Interval == 0 && out.Probe.Timeout == 0 {
		out.Probe = def.Probe
	}
	// 老版本存过 list / compact，现在没有这两种了，一律按网格显示
	if out.Theme != "dark" && out.Theme != "light" && out.Theme != "auto" {
		out.Theme = "auto"
	}
	if out.Layout != "grid" && out.Layout != "icon" {
		out.Layout = "grid"
	}
	if out.Search.Height == 0 || out.Search.MaxWidth == 0 {
		out.Search.Height = def.Search.Height
		out.Search.Radius = def.Search.Radius
		out.Search.FontSize = def.Search.FontSize
		out.Search.MaxWidth = def.Search.MaxWidth
	}
	// 老数据是三段分开的页脚，合并到一个框里，前端只认 FooterCode
	if out.FooterCode == "" && (out.FooterHTML != "" || out.FooterCSS != "" || out.FooterJS != "") {
		var b strings.Builder
		if out.FooterHTML != "" {
			b.WriteString(out.FooterHTML)
		}
		if out.FooterCSS != "" {
			b.WriteString("\n<style>\n" + out.FooterCSS + "\n</style>")
		}
		if out.FooterJS != "" {
			b.WriteString("\n<script>\n" + out.FooterJS + "\n</script>")
		}
		out.FooterCode = b.String()
	}
	out.FooterHTML, out.FooterCSS, out.FooterJS = "", "", ""
	// 引擎没单独填图标时，用域名 favicon 顶上，前端就不用各自兜了
	for i := range out.Search.Engines {
		if out.Search.Engines[i].Icon != "" {
			continue
		}
		if u, err := url.Parse(out.Search.Engines[i].URL); err == nil && u.Host != "" {
			out.Search.Engines[i].Icon = u.Scheme + "://" + u.Host + "/favicon.ico"
		}
	}
	return out
}

// filterPublicURLs 公开模式下只保留外网可达的地址（default/wan/custom），
// 内网地址（lan）不出现在访客的响应里，避免泄露内网 IP 拓扑。
func filterPublicURLs(urls []models.ItemURL) []models.ItemURL {
	out := make([]models.ItemURL, 0, len(urls))
	for _, u := range urls {
		if u.Kind == models.URLLan {
			continue
		}
		out = append(out, u)
	}
	return out
}

// publicOwner 公开模式展示谁的首页：第一个管理员。
func (s *Server) publicOwner() *models.User {
	var u models.User
	if err := s.DB.Where("role = ?", models.RoleAdmin).Order("id").First(&u).Error; err != nil {
		return nil
	}
	return &u
}

func (s *Server) publicMode() bool {
	owner := s.publicOwner()
	if owner == nil {
		return false
	}
	return s.loadSettings(owner.ID).PublicMode
}

// panel 一次性返回首页需要的全部数据，避免前端 N 次请求（也便于做卡片缓存）。
func (s *Server) panel(c *gin.Context) {
	u := s.currentUser(c)
	isPublic := false
	owner := u
	if owner == nil {
		if !s.publicMode() {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "msg": "请先登录"})
			return
		}
		owner = s.publicOwner()
		if owner == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "msg": "请先登录"})
			return
		}
		isPublic = true
	}

	settings := s.loadSettings(owner.ID)
	if isPublic {
		// 访客看到的设置里不暴露会引发写操作的字段
		settings.PublicMode = true
		// 脱敏：公开模式下绝不把 Docker socket 地址、内网探测地址、内网网段、自定义页脚代码暴露给游客。
		// 这些是"能直接连到内网 / 能注入同源脚本"的高危信息，登录者才看得到。
		settings.Docker.Socket = ""
		settings.Probe.LanProbe = ""
		settings.LanRules = nil
		// 页脚：完整公开（HTML + <style> + <script> 都保留，用户明确要求游客也看到完整效果）。
		// 注意：这意味着自定义页脚代码会向所有访客执行，属用户主动选择；备案号等静态链接自然公开。
		settings.Custom.CSS = ""
		settings.Custom.JS = ""
	}

	var groups []models.Group
	if err := s.DB.Where("user_id = ?", owner.ID).Order("sort, id").Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "读取分组失败"})
		return
	}
	var items []models.Item
	if err := s.DB.Where("user_id = ?", owner.ID).Order("sort, id").Preload("URLs").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "读取卡片失败"})
		return
	}

	byGroup := map[uint][]panelItem{}
	for _, it := range items {
		urls := it.URLs
		if isPublic {
			// 公开模式只给"通用/外网"地址，内网地址（lan）不外泄
			urls = filterPublicURLs(it.URLs)
		}
		byGroup[it.GroupID] = append(byGroup[it.GroupID], panelItem{
			ID: it.ID, Name: it.Name, Desc: it.Desc, Icon: it.IconURL,
			Sort: it.Sort, OpenMode: it.OpenMode, LayoutMode: it.LayoutMode, Type: it.Type, Stat: it.Stat, URLs: urls,
		})
	}

	out := make([]panelGroup, 0, len(groups))
	for _, g := range groups {
		if isPublic && g.HiddenPublic {
			continue
		}
		gi := byGroup[g.ID]
		if gi == nil {
			gi = []panelItem{}
		}
		out = append(out, panelGroup{
			ID: g.ID, Name: g.Name, Slug: g.Slug, Sort: g.Sort, Display: g.Display,
			Style: g.Style, HiddenPublic: g.HiddenPublic, Items: gi,
		})
	}

	var user any
	if u != nil {
		user = publicUser(u)
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"settings": settings,
		"groups":   out,
		"user":     user,
		"readonly": u == nil,
	})
}

// updateSettings 字段级合并保存：先读出库里的旧设置，再用请求里「显式带上的字段」覆盖。
// 这样多标签页/多设备同时在线时，A 页面改 docker.socket、B 页面改搜索引擎，互不覆盖对方刚写的字段。
// 只做基本校验，字段演进由前端 JSON 决定。
func (s *Server) updateSettings(c *gin.Context) {
	u := s.currentUser(c)

	// 先解析成 map，判断请求里到底带了哪些顶层字段（字段级合并的关键）。
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "参数不合法"})
		return
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawFields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "参数不合法"})
		return
	}

	// 以库里现存设置为底，请求带了的字段才覆盖。
	in := s.loadSettings(u.ID)
	if err := json.Unmarshal(body, &in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "参数不合法"})
		return
	}

	// 存之前先把上一版整体备一份，手滑或脚本覆盖还能捞回来
	s.backupSettings(u.ID)
	if len(in.Title) > 64 {
		in.Title = in.Title[:64]
	}
	in.TabTitle = clipRunes(strings.TrimSpace(in.TabTitle), 64)
	if in.TabTitle == "" {
		in.TabTitle = in.Title
	}
	switch in.Theme {
	case "dark", "light", "auto":
	default:
		// 旧版的 sun（跟随日出日落）已下线：收成跟随系统
		in.Theme = "auto"
	}
	switch in.Lock.Animation {
	case "fade", "slide", "zoom", "blur", "curtain", "flip":
	default:
		in.Lock.Animation = "slide"
	}
	switch in.Lock.Trigger {
	case "click", "swipe", "both":
	default:
		in.Lock.Trigger = "both"
	}
	in.Lock.FontSize = clampInt(in.Lock.FontSize, 0, 160)
	// 数值字段收拢到合理区间，别让前端手滑写坏样式
	in.Card.Opacity = clampInt(in.Card.Opacity, 0, 100)
	in.Card.Blur = clampInt(in.Card.Blur, 0, 40)
	in.Card.Radius = clampInt(in.Card.Radius, 0, 40)
	in.Card.IconSize = clampInt(in.Card.IconSize, 16, 120)
	in.Card.DotPos = enumOr(in.Card.DotPos, "tr", "tl", "tr", "bl", "br")
	if !isHexColor(in.Card.IconBg) {
		in.Card.IconBg = "#ffffff"
	}
	in.Card.IconBgAlpha = clampInt(in.Card.IconBgAlpha, 0, 100)
	in.Card.DotOffset = clampInt(in.Card.DotOffset, 0, 40)
	if !isHexColor(in.Text.Dark) {
		in.Text.Dark = "#f1f5f9"
	}
	if !isHexColor(in.Text.Light) {
		in.Text.Light = "#ffffff"
	}
	in.Card.IconSpacing = clampInt(in.Card.IconSpacing, 0, 80)
	in.Clock.FontSize = clampInt(in.Clock.FontSize, 0, 120)
	in.Page.PadTop = clampInt(in.Page.PadTop, 0, 160)
	in.Page.PadX = clampInt(in.Page.PadX, 0, 160)
	in.Page.PadBottom = clampInt(in.Page.PadBottom, 0, 240)
	in.Page.MaxWidth = clampInt(in.Page.MaxWidth, 320, 2400)
	// 顶栏是贴在浏览器窗口边上的，量级小得多
	in.Bar.PadX = clampInt(in.Bar.PadX, 0, 120)
	in.Bar.PadTop = clampInt(in.Bar.PadTop, 0, 80)
	in.Bar.IconSize = clampInt(in.Bar.IconSize, 18, 52)
	in.Bar.TitleSize = clampInt(in.Bar.TitleSize, 10, 30)
	in.Bar.MaxWidth = clampInt(in.Bar.MaxWidth, 0, 2400)
	// Docker socket：去掉首尾空白；tcp:// 或 unix 路径都放行，其它回默认 unix
	in.Docker.Socket = strings.TrimSpace(in.Docker.Socket)
	if in.Docker.Socket != "" && !strings.HasPrefix(in.Docker.Socket, "tcp://") && !strings.HasPrefix(in.Docker.Socket, "/") {
		in.Docker.Socket = "/var/run/docker.sock"
	}
	// 探测：只让填 http(s)，间隔和超时都夹在合理范围
	in.Probe.LanProbe = strings.TrimSpace(in.Probe.LanProbe)
	if in.Probe.LanProbe != "" && !strings.HasPrefix(in.Probe.LanProbe, "http://") && !strings.HasPrefix(in.Probe.LanProbe, "https://") {
		in.Probe.LanProbe = ""
	}
	in.Probe.LanProbe = clipRunes(in.Probe.LanProbe, 512)
	in.Probe.Interval = clampInt(in.Probe.Interval, 10, 600)
	in.Probe.Timeout = clampInt(in.Probe.Timeout, 500, 15000)
	in.Search.Height = clampInt(in.Search.Height, 24, 120)
	in.Search.Radius = clampInt(in.Search.Radius, 0, 48)
	in.Search.FontSize = clampInt(in.Search.FontSize, 10, 28)
	in.Search.MaxWidth = clampInt(in.Search.MaxWidth, 280, 1400)
	if in.Clock.Format == "" {
		in.Clock.Format = "15:04:05"
	}
	in.Clock.Format = clipRunes(in.Clock.Format, 32)
	in.Clock.Align = enumOr(in.Clock.Align, "left", "center", "right")
	in.Sun.Lat = clampFloat(in.Sun.Lat, -90, 90)
	in.Sun.Lng = clampFloat(in.Sun.Lng, -180, 180)
	// 一言的来源固定是 hitokoto.cn，别让空串把配置写坏
	if in.Hitokoto.Source == "" {
		in.Hitokoto.Source = "hitokoto"
	}
	// 问候语全空 = 用默认六句
	if in.Lock.Greet == (models.Greetings{}) {
		in.Lock.Greet = models.DefaultGreetings()
	}
	in.Lock.Greet = models.Greetings{
		Dawn:      clipRunes(in.Lock.Greet.Dawn, 20),
		Morning:   clipRunes(in.Lock.Greet.Morning, 20),
		Noon:      clipRunes(in.Lock.Greet.Noon, 20),
		Afternoon: clipRunes(in.Lock.Greet.Afternoon, 20),
		Evening:   clipRunes(in.Lock.Greet.Evening, 20),
		Night:     clipRunes(in.Lock.Greet.Night, 20),
	}
	if len(in.Custom.CSS) > 64<<10 || len(in.Custom.JS) > 64<<10 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "自定义代码太长了（上限 64KB）"})
		return
	}
	// 页脚统一进一个框，老的三个字段清掉，避免两份数据打架
	in.FooterHTML, in.FooterCSS, in.FooterJS = "", "", ""
	if len(in.FooterCode) > 64<<10 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "页脚代码太长了（上限 64KB）"})
		return
	}
	switch in.Wallpaper.Type {
	case "video", "image", "none":
	default:
		in.Wallpaper.Type = "none"
	}
	// 壁纸参数必须夹住：brightness 被写成 0 会让整屏视频变全黑，看起来就像"视频壁纸坏了"
	in.Wallpaper.Blur = clampFloat(in.Wallpaper.Blur, 0, 24)
	in.Wallpaper.Mask = clampFloat(in.Wallpaper.Mask, 0, 1)
	in.Wallpaper.Brightness = clampFloat(in.Wallpaper.Brightness, 0.2, 1.4)
	// 布局只剩网格和只图标，旧的 list/compact 数据统一收回网格
	switch in.Layout {
	case "grid", "icon":
	default:
		in.Layout = "grid"
	}
	raw, err := json.Marshal(in)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "序列化失败"})
		return
	}
	st := models.Setting{UserID: u.ID, Data: string(raw)}
	if err := s.DB.Save(&st).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "settings": in})
}

// enumOr 从白名单里挑一个，不认识的回第一个（也就是默认值）
func enumOr(v string, allowed ...string) string {
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return allowed[0]
}

// jsonHasKey 看一段 JSON 里某对象的某个键在不在（用来区分"老数据没这字段"和"用户设成零值"）
func jsonHasKey(data, obj, key string) bool {
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &top); err != nil {
		return false
	}
	raw, ok := top[obj]
	if !ok {
		return false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	_, has := m[key]
	return has
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clipRunes 按字符（不是字节）截断，免得中文被砍一半
// isHexColor 只认 #rrggbb —— 前端 <input type="color"> 给的就是这个格式。
func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// backupSettings 每次覆盖设置前，把当前值整份抄到磁盘，保留最近 20 份。
// 之前用 curl 测接口时整体覆盖过一次，把人家的自定义全冲了，这个就是补那个窟窿的。
func (s *Server) backupSettings(userID uint) {
	var set models.Setting
	if err := s.DB.Where("user_id = ?", userID).First(&set).Error; err != nil || set.Data == "" {
		return
	}
	dir := filepath.Join(filepath.Dir(s.Cfg.DBPath()), "backup")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	name := fmt.Sprintf("settings-%s.json", time.Now().Format("20060102-150405"))
	_ = os.WriteFile(filepath.Join(dir, name), []byte(set.Data), 0o644)
	// 只留最近 20 份
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) <= 20 {
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names[:len(names)-20] {
		_ = os.Remove(filepath.Join(dir, n))
	}
}
