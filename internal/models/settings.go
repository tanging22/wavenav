package models

// Settings 每个用户一份，整体 JSON 存 Setting.Data。
// 字段只增不改：前端新增开关时后端不用改表，坏数据用 DefaultSettings 兜底。
type Settings struct {
	Title     string    `json:"title"`    // 页面上显示的站点名
	TabTitle  string    `json:"tabTitle"` // 浏览器标签上的标题，留空就用 Title
	Logo      string    `json:"logo"`
	Theme     string    `json:"theme"` // dark | light | sun（跟随日出日落）
	Wallpaper Wallpaper `json:"wallpaper"`
	Clock     Clock     `json:"clock"`
	Search    Search    `json:"search"`
	Card      Card      `json:"card"`
	Lock      Lock      `json:"lock"`
	Hitokoto  Hitokoto  `json:"hitokoto"`
	Sun       Sun       `json:"sun"`
	Custom    Custom    `json:"custom"`
	Docker    Docker    `json:"docker"`
	// 页脚：一个框写到底，HTML 里可以顺手夹 <style> / <script>
	FooterCode string `json:"footerCode"`
	// 兼容老数据，读的时候会合并进 FooterCode
	FooterHTML string   `json:"footerHtml"`
	FooterCSS  string   `json:"footerCss"`
	FooterJS   string   `json:"footerJs"`
	Page       Page     `json:"page"`
	Bar        Bar      `json:"bar"`
	Text       Text     `json:"text"`
	Probe      Probe    `json:"probe"`
	LanRules   []string `json:"lanRules"`
	Layout     string   `json:"layout"`
	PublicMode bool     `json:"publicMode"`
	Lang       string   `json:"lang"`
	Favicon    string   `json:"favicon"`
}

type Wallpaper struct {
	Type       string  `json:"type"` // video | image | none
	Src        string  `json:"src"`
	Blur       float64 `json:"blur"`
	Mask       float64 `json:"mask"`
	Brightness float64 `json:"brightness"`
	Smooth     bool    `json:"smooth"`
}

type Clock struct {
	Show     bool   `json:"show"`
	Seconds  bool   `json:"seconds"`
	Format   string `json:"format"`   // Go 风格 "15:04:05"，前端再翻成自己那套
	FontSize int    `json:"fontSize"` // px，0 = 按屏幕自适应
	Align    string `json:"align"`    // left | center | right
	ShowDate bool   `json:"showDate"`
}

// Page 主页内容区的留白，四边各自可调。
type Text struct {
	Dark  string `json:"dark"`
	Light string `json:"light"`
}

type Page struct {
	PadTop    int `json:"padTop"`
	PadX      int `json:"padX"`
	PadBottom int `json:"padBottom"`
	MaxWidth  int `json:"maxWidth"`
}

// Bar 是主页顶栏（标题 + 操作按钮）自己的定位参数。
// 它贴在浏览器窗口边缘，不受「主页留白」那套影响。
type Bar struct {
	Fixed     bool `json:"fixed"`     // true = 固定在视口边缘（滚不滚都在）
	PadX      int  `json:"padX"`      // 左右留白
	PadTop    int  `json:"padTop"`    // 上留白
	IconSize  int  `json:"iconSize"`  // 圆形按钮直径
	TitleSize int  `json:"titleSize"` // 标题字号
	MaxWidth  int  `json:"maxWidth"`  // 顶栏内容最大宽度，0 = 铺满不限宽（独立于主页留白）
}

// Probe 是前台可达性探测：判断当前设备在内网还是外网，顺带在卡片角上显示延迟。
type Probe struct {
	Enabled     bool   `json:"enabled"`     // 总开关
	LanProbe    string `json:"lanProbe"`    // 填一个只有内网能通上的地址，通了就算内网
	ShowLatency bool   `json:"showLatency"` // 卡片右上角绿/黄/红点
	Interval    int    `json:"interval"`    // 秒，探测间隔
	Timeout     int    `json:"timeout"`     // 毫秒，单次超时
}

type Search struct {
	Show       bool           `json:"show"`
	PublicShow bool           `json:"publicShow"` // 公开（未登录）时是否也显示搜索框
	Engine     string         `json:"engine"`
	Engines    []SearchEngine `json:"engines"`
	Height     int            `json:"height"`   // 框高 px
	Radius     int            `json:"radius"`   // 圆角 px
	FontSize   int            `json:"fontSize"` // 字号 px
	MaxWidth   int            `json:"maxWidth"` // 最宽 px
	Glow       bool           `json:"glow"`     // 搜索框边框淡淡泛光
}

type SearchEngine struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`  // 用 %s 占位
	Icon string `json:"icon"` // 引擎图标，抓站点 favicon 或手填
}

// Card 卡片外观。Opacity 是背景不透明度百分比，Blur 是磨砂半径（默认 0 = 不磨砂）。
type Card struct {
	Opacity     int    `json:"opacity"`
	Blur        int    `json:"blur"`
	Radius      int    `json:"radius"`
	IconSize    int    `json:"iconSize"`    // 「只显示图标」模式下单个图标的边长
	IconSpacing int    `json:"iconSpacing"` // 「只显示图标」模式下图标之间的距离
	DotPos      string `json:"dotPos"`      // 延迟小点的位置：tl | tr | bl | br
	// 只图标模式：图标本身可能就是一张带透明通道的 PNG，
	// 垫一层底色 + 可调不透明度，透明的地方才看得清。
	IconBg      string `json:"iconBg"`      // #rrggbb
	IconBgAlpha int    `json:"iconBgAlpha"` // 0-100，0 就是全透明（默认）
	DotOffset   int    `json:"dotOffset"`   // 延迟小点离边缘的距离 px
}

// Lock 锁屏式首页：先只有时间、一言和问候，解锁后才露出卡片。
type Lock struct {
	Enabled   bool      `json:"enabled"`
	Animation string    `json:"animation"` // fade | slide | zoom | blur | curtain | flip
	Trigger   string    `json:"trigger"`   // click | swipe | both
	ShowDate  bool      `json:"showDate"`
	Greet     Greetings `json:"greet"`
	FontSize  int       `json:"fontSize"` // 锁屏时间字号，0=自适应
}

// Greetings 底部问候语，按时段可自定义。留空用默认。
type Greetings struct {
	Dawn      string `json:"dawn"`      // 0-5
	Morning   string `json:"morning"`   // 5-11
	Noon      string `json:"noon"`      // 11-14
	Afternoon string `json:"afternoon"` // 14-18
	Evening   string `json:"evening"`   // 18-23
	Night     string `json:"night"`     // 23-24
}

// DefaultGreetings 默认问候语。
func DefaultGreetings() Greetings {
	return Greetings{
		Dawn:      "还没睡呀",
		Morning:   "早上好",
		Noon:      "中午好",
		Afternoon: "下午好",
		Evening:   "晚上好",
		Night:     "夜深了",
	}
}

// Hitokoto 一言。Source 目前支持 hitokoto / 一言不发了就回兜底。
type Hitokoto struct {
	Enabled  bool   `json:"enabled"`
	Source   string `json:"source"`
	Category string `json:"category"` // 空=随机，可填 i k a b c d e f g h j l
}

// Sun 日出日落需要的坐标。
type Sun struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Custom 站长自己写的主页 CSS / JS。
type Custom struct {
	CSS string `json:"css"`
	JS  string `json:"js"`
}

// Docker 容器状态监控。Socket 为空时用环境变量或默认路径。
type Docker struct {
	Enabled bool   `json:"enabled"`
	Socket  string `json:"socket"`
}

// DefaultSettings 首次启动的默认设置。lanRules 来自 RFC1918 + 常见私有段。
func DefaultSettings() Settings {
	return Settings{
		Title:    "鲸鲸导航",
		TabTitle: "鲸鲸导航",
		Theme:    "dark",
		Wallpaper: Wallpaper{
			Type:       "none",
			Blur:       0,
			Mask:       0.25,
			Brightness: 1,
			Smooth:     true,
		},
		Clock: Clock{Show: true, Seconds: true, Format: "15:04:05", FontSize: 0, Align: "center", ShowDate: true},
		Card:  Card{Opacity: 8, Blur: 0, Radius: 18, IconSize: 56, IconSpacing: 18, DotPos: "tr", IconBg: "#ffffff", IconBgAlpha: 0, DotOffset: 8},
		Page:  Page{PadTop: 28, PadX: 24, PadBottom: 40, MaxWidth: 1152},
		Bar:   Bar{Fixed: true, PadX: 18, PadTop: 14, IconSize: 34, TitleSize: 17, MaxWidth: 0},
		Probe: Probe{Enabled: true, LanProbe: "", ShowLatency: true, Interval: 60, Timeout: 3000},
		Text:  Text{Dark: "#f1f5f9", Light: "#1e293b"},
		Lock: Lock{
			Enabled:   true,
			Animation: "slide",
			Trigger:   "both",
			ShowDate:  true,
			Greet:     DefaultGreetings(),
		},
		Hitokoto: Hitokoto{Enabled: true, Source: "hitokoto", Category: ""},
		Sun:      Sun{Lat: 0, Lng: 0},
		Docker:   Docker{Enabled: false, Socket: "/var/run/docker.sock"},
		Layout:   "grid",
		Lang:     "zh-CN",
		Favicon:  "",
		Search: Search{
			Show:       true,
			PublicShow: true,
			Engine:     "baidu",
			Height:     52,
			Radius:     16,
			FontSize:   14,
			MaxWidth:   640,
			Glow:       true,
			Engines: []SearchEngine{
				{Key: "baidu", Name: "百度", URL: "https://www.baidu.com/s?wd=%s", Icon: "https://www.baidu.com/favicon.ico"},
				{Key: "google", Name: "Google", URL: "https://www.google.com/search?q=%s", Icon: "https://www.google.com/favicon.ico"},
				{Key: "bing", Name: "必应", URL: "https://www.bing.com/search?q=%s", Icon: "https://www.bing.com/favicon.ico"},
				{Key: "ddg", Name: "DuckDuckGo", URL: "https://duckduckgo.com/?q=%s", Icon: "https://duckduckgo.com/favicon.ico"},
			},
		},
		LanRules: []string{"192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12", "127.0.0.0/8", "169.254.0.0/16"},
	}
}
