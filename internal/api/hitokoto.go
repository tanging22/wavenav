package api

import (
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 一言是外网接口，做了 3 分钟进程内缓存 + 兜底句子：
// 断网、超时、被墙都不能让首页右上角变空白。

type hitokotoPayload struct {
	Hitokoto string `json:"hitokoto"`
	From     string `json:"from"`
	FromWho  string `json:"from_who"`
}

var hkFallbacks = []string{
	"海那么大，装得下你所有的情绪。",
	"今天也辛苦了，先喝口水吧。",
	"慢慢来，比较快。",
	"数据海洋深处，灯一直亮着。",
	"别急，好的东西都沉在底下。",
	"你已经做得比想象中好了。",
}

// 按类别分桶缓存：c=d 和 c=a 不能共用同一条句子
type hkEntry struct {
	text string
	from string
	at   time.Time
}

var (
	hkMu    sync.Mutex
	hkCache = make(map[string]hkEntry)
	hkHTTPC = &http.Client{Timeout: 6 * time.Second}
)

func (s *Server) hitokoto(c *gin.Context) {
	category := c.Query("c")

	hkMu.Lock()
	if e, ok := hkCache[category]; ok && time.Since(e.at) < 3*time.Minute {
		hkMu.Unlock()
		c.JSON(http.StatusOK, gin.H{"ok": true, "text": e.text, "from": e.from, "cached": true})
		return
	}
	hkMu.Unlock()

	text, from := fetchHitokoto(category)
	if text == "" {
		text = hkFallbacks[rand.Intn(len(hkFallbacks))]
		c.JSON(http.StatusOK, gin.H{"ok": true, "text": text, "from": "", "fallback": true})
		return
	}

	hkMu.Lock()
	if len(hkCache) > 16 { // 类别是用户自己填的，别让它无限长
		hkCache = make(map[string]hkEntry)
	}
	hkCache[category] = hkEntry{text: text, from: from, at: time.Now()}
	hkMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"ok": true, "text": text, "from": from})
}

func fetchHitokoto(category string) (string, string) {
	url := "https://v1.hitokoto.cn/?encode=json&charset=utf-8"
	if category != "" {
		// 只接受单个字母类别，避免拼出奇怪的 query
		ok := len(category) <= 20
		for _, r := range category {
			if !(r >= 'a' && r <= 'z') && r != '|' {
				ok = false
				break
			}
		}
		if ok {
			url += "&c=" + category
		}
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", "WaveNav/1.0 (+hitokoto)")
	resp, err := hkHTTPC.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", ""
	}
	var p hitokotoPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return "", ""
	}
	from := p.From
	if p.FromWho != "" && from != "" {
		from = p.FromWho + "《" + p.From + "》"
	}
	return p.Hitokoto, from
}
