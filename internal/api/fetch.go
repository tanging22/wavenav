package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"

	"wavenav/internal/models"
)

// ---------- SSRF 防护 ----------

// browserUA 伪装成真浏览器。很多站点和 CDN 看见 Bot 字样就直接 403，
// 之前抓不到图标有一半是 UA 的锅。
const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

// isBlockedIP 只挡真正危险的地址：回环、链路本地（169.254.0.0/16 里的云元数据）、
// 组播、未指定地址。RFC1918 私网和 CGNAT 放行 —— 导航页最常见的用法就是给自家
// NAS、路由器和容器建卡片，把这些拦掉等于功能废一半。
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()
}

// safeClient 每次拨号都重新校验解析结果，挡住 DNS rebinding。
func safeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, errors.New("域名解析为空")
			}
			for _, ip := range ips {
				if isBlockedIP(ip.IP) {
					return nil, fmt.Errorf("拒绝访问内网地址 %s", ip.IP)
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		DisableKeepAlives:     true,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("重定向次数过多")
			}
			return nil
		},
	}
}

// ---------- 抓站点信息 ----------

type fetchReq struct {
	URL string `json:"url" binding:"required"`
}

type fetchResp struct {
	Title string `json:"title"`
	Icon  string `json:"icon"`
}

func (s *Server) fetchSite(c *gin.Context) {
	u := s.currentUser(c)
	var req fetchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "请填写网址"})
		return
	}
	target := strings.TrimSpace(req.URL)
	if !strings.Contains(target, "://") {
		target = "https://" + target
	}
	base, err := url.Parse(target)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "网址格式不对"})
		return
	}

	body, finalURL, err := fetchHTML(base)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "msg": err.Error()})
		return
	}
	title, iconHref := parsePageMeta(bytes.NewReader(body), finalURL)

	out := fetchResp{Title: title, Icon: ""}
	origin := &url.URL{Scheme: finalURL.Scheme, Host: finalURL.Host}

	// 候选顺序：页面声明的 icon → 根目录几个老站点常用的位置
	var cands []string
	if iconHref != "" {
		if ref, err := url.Parse(iconHref); err == nil {
			abs := finalURL.ResolveReference(ref)
			if abs.Scheme == "http" || abs.Scheme == "https" {
				cands = append(cands, abs.String())
			}
		}
	}
	for _, p := range []string{"/favicon.ico", "/favicon.png", "/apple-touch-icon.png"} {
		cands = append(cands, origin.String()+p)
	}
	for _, cand := range cands {
		if out.Icon != "" {
			break
		}
		out.Icon = s.downloadIcon(u.ID, cand)
	}
	// 一个都下不下来：把最可能要的那个外链交回去，前端 img 自己试
	if out.Icon == "" {
		out.Icon = origin.String() + "/favicon.ico"
	}
	if out.Title == "" {
		out.Title = base.Hostname()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "title": out.Title, "icon": out.Icon})
}

// fetchHTML 拿页面 HTML；https 不通时自动退回 http（内网服务常常只开 http）。
func fetchHTML(base *url.URL) ([]byte, *url.URL, error) {
	try := func(u *url.URL) ([]byte, *url.URL, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, nil, errors.New("网址格式不对")
		}
		req.Header.Set("User-Agent", browserUA)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		resp, err := safeClient(12 * time.Second).Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, nil, fmt.Errorf("目标站返回 HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
		if err != nil {
			return nil, nil, errors.New("读取目标页面失败")
		}
		return body, resp.Request.URL, nil
	}

	body, final, err := try(base)
	if err == nil {
		return body, final, nil
	}
	if base.Scheme == "https" {
		alt := *base
		alt.Scheme = "http"
		if b2, f2, e2 := try(&alt); e2 == nil {
			return b2, f2, nil
		}
	}
	return nil, nil, fmt.Errorf("连不上这个地址（%v）；内网服务请确认它和后端在同一网络，且端口已开", err)
}

// parsePageMeta 用流式 tokenizer，只读前 512KB，不整页进内存。
func parsePageMeta(r io.Reader, base *url.URL) (title, iconHref string) {
	z := html.NewTokenizer(r)
	for {
		switch z.Next() {
		case html.ErrorToken:
			return
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "title":
				if z.Next() == html.TextToken {
					title = strings.Join(strings.Fields(string(z.Text())), " ")
				}
			case "link":
				var rel, href string
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					switch strings.ToLower(string(k)) {
					case "rel":
						rel = strings.ToLower(string(v))
					case "href":
						href = strings.TrimSpace(string(v))
					}
				}
				if strings.Contains(rel, "icon") && href != "" && iconHref == "" {
					iconHref = href
				}
			}
		}
	}
}

// downloadIcon 把图标转存到本地，失败就返回空（调用方回落外链）。
func (s *Server) downloadIcon(userID uint, iconURL string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, iconURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/svg+xml,*/*;q=0.8")
	resp, err := safeClient(8 * time.Second).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ""
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil || len(buf) == 0 {
		return ""
	}

	ext := ".png"
	switch {
	case bytes.HasPrefix(buf, []byte("\x89PNG")):
		ext = ".png"
	case bytes.HasPrefix(buf, []byte("\xff\xd8")):
		ext = ".jpg"
	case bytes.HasPrefix(buf, []byte("GIF8")):
		ext = ".gif"
	case bytes.HasPrefix(buf, []byte("RIFF")):
		ext = ".webp"
	case bytes.HasPrefix(buf, []byte("<svg")) || bytes.Contains(buf[:min(len(buf), 256)], []byte("<svg")):
		ext = ".svg"
	case bytes.HasPrefix(buf, []byte("\x00\x00\x01\x00")):
		ext = ".ico"
	default:
		// 认不出的二进制别急着丢：Content-Type 说是图片就按 png 存
		ct := strings.ToLower(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
		if !strings.HasPrefix(ct, "image/") {
			return ""
		}
		switch ct {
		case "image/jpeg":
			ext = ".jpg"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		case "image/svg+xml":
			ext = ".svg"
		case "image/x-icon", "image/vnd.microsoft.icon":
			ext = ".ico"
		default:
			ext = ".png"
		}
	}
	res, status, _ := s.saveUpload(userID, models.KindIcon, "favicon"+ext, bytes.NewReader(buf), int64(len(buf)))
	if status != http.StatusOK || res == nil {
		return ""
	}
	return res.URL
}
