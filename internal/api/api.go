package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wavenav/internal/auth"
	"wavenav/internal/config"
	"wavenav/internal/models"
)

const sessionCookie = "wavenav_session"

type Server struct {
	DB      *gorm.DB
	Cfg     *config.Config
	WebFS   fs.FS // 前端构建产物（go:embed），dev 模式可为 nil
	limiter *loginLimiter
}

func New(db *gorm.DB, cfg *config.Config, webFS fs.FS) *Server {
	return &Server{DB: db, Cfg: cfg, WebFS: webFS, limiter: newLoginLimiter()}
}

func (s *Server) Router() *gin.Engine {
	if !s.Cfg.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	// CSP 强制生效（不是 report-only）：默认只放行同源资源 + 明确的数据 URI，
	// 拦掉一切内联脚本 / 外域脚本。页脚自定义 JS 走 new Function 不受此限制，
	// 但上传的 SVG 若被 inline 渲染也会被 img-src 白名单挡住执行。
	r.Use(func(c *gin.Context) {
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob: https:; "+
				"media-src 'self' blob:; "+
				"font-src 'self' data:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'self'; "+
				"base-uri 'self'; "+
				"form-action 'self'")
		c.Next()
	})

	v1 := r.Group("/api/v1")
	// 接口一律不许缓存。没有这句，浏览器会按启发式缓存 GET /panel，
	// 用户改完设置刷新还是旧数据 —— 表现就是"设置全都没效果"。
	v1.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Next()
	})
	{
		v1.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true, "name": "wavenav"}) })

		v1.POST("/auth/login", s.login)
		v1.POST("/auth/logout", s.logout)
		v1.GET("/auth/me", s.optionalAuth, s.me)
		v1.GET("/panel", s.optionalAuth, s.panel)
		v1.GET("/hitokoto", s.hitokoto)
	}

	// 需要登录的写操作
	a := r.Group("/api/v1", s.requireAuth)
	{
		a.PUT("/settings", s.updateSettings)
		a.POST("/auth/password", s.changePassword)
		a.POST("/auth/username", s.changeUsername)
		a.POST("/auth/admin-password", s.requireAdmin, s.adminSetPassword)

		a.POST("/groups", s.createGroup)
		a.PUT("/groups/:id", s.updateGroup)
		a.DELETE("/groups/:id", s.deleteGroup)

		a.POST("/items", s.createItem)
		a.PUT("/items/:id", s.updateItem)
		a.DELETE("/items/:id", s.deleteItem)
		a.POST("/items/reorder", s.reorderItems)

		a.POST("/upload", s.upload)
		a.GET("/gallery", s.gallery)
		a.DELETE("/gallery/:id", s.deleteGallery)

		a.POST("/fetch-site", s.fetchSite)
		a.GET("/export", s.exportData)

		a.GET("/docker/containers", s.dockerContainers)
		a.GET("/docker/stats", s.dockerStats)
		a.POST("/docker/containers/:id/:action", s.dockerAction)
	}

	// 上传素材：http.Dir 自带路径清理，不会穿越
	serveUploads := func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=604800")
		// SVG 内可嵌 <script>，inline 渲染到同源页面就是 XSS 通道。
		// 一律强制 attachment 下载，浏览器不会当文档执行；其余图片/视频照常 inline。
		if strings.HasSuffix(strings.ToLower(c.Request.URL.Path), ".svg") {
			c.Header("Content-Disposition", "attachment; filename=\"image.svg\"")
			c.Header("X-Content-Type-Options", "nosniff")
		}
		http.StripPrefix("/uploads/", http.FileServer(http.Dir(s.Cfg.UploadDir()))).ServeHTTP(c.Writer, c.Request)
	}
	r.GET("/uploads/*filepath", serveUploads)
	r.HEAD("/uploads/*filepath", serveUploads) // 视频预探测 / 探活走 HEAD，少了这条会 404

	r.NoRoute(s.serveSPA)
	return r
}

// ---------- 认证中间件 ----------

func (s *Server) optionalAuth(c *gin.Context) {
	token, _ := c.Cookie(sessionCookie)
	if token != "" {
		if u, sess, err := auth.Lookup(s.DB, token); err == nil {
			c.Set("user", u)
			c.Set("token", token)
			c.Set("session", sess)
		}
	}
	c.Next()
}

func (s *Server) requireAuth(c *gin.Context) {
	token, _ := c.Cookie(sessionCookie)
	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "msg": "请先登录"})
		return
	}
	u, sess, err := auth.Lookup(s.DB, token)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "msg": "登录已失效，请重新登录"})
		return
	}
	c.Set("user", u)
	c.Set("token", token)
	c.Set("session", sess)
	c.Next()
}

func (s *Server) requireAdmin(c *gin.Context) {
	u := s.currentUser(c)
	if u == nil || u.Role != models.RoleAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "msg": "需要管理员权限"})
		return
	}
	c.Next()
}

func (s *Server) currentUser(c *gin.Context) *models.User {
	if v, ok := c.Get("user"); ok {
		if u, ok := v.(*models.User); ok {
			return u
		}
	}
	return nil
}

func (s *Server) setSessionCookie(c *gin.Context, token string, days int) {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookie, token, days*86400, "/", "", secure, true)
}

// ---------- SPA 托管 ----------

func (s *Server) serveSPA(c *gin.Context) {
	p := c.Request.URL.Path
	// 关键：/api 与 /uploads 绝不能回退成 index.html（老站就踩了这个坑）
	if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/uploads/") {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "msg": "接口不存在"})
		return
	}
	if s.WebFS == nil {
		c.String(http.StatusOK, "前端未构建：请 cd web && npm run build，或用 WAVENAV_MODE=dev 配合 Vite")
		return
	}
	name := strings.TrimPrefix(p, "/")
	if name == "" {
		name = "index.html"
	}
	if f, err := s.WebFS.Open(name); err == nil {
		_ = f.Close()
		// go:embed 的文件 ModTime 是零值，http.FileServer 不会发 Last-Modified，
		// 浏览器就按启发式缓存放着不取 —— 结果改完前端用户看到的还是旧 bundle。
		// 带 hash 的资源放心长缓存，html 一律 no-cache 每次回来问一句。
		if strings.Contains(name, "/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "no-cache")
		}
		http.FileServerFS(s.WebFS).ServeHTTP(c.Writer, c.Request)
		return
	}
	// 其余路径一律落回 index.html（前端路由接管）
	index, err := fs.ReadFile(s.WebFS, "index.html")
	if err != nil {
		// 二进制里没打包前端（源码直接 go build 而没跑 npm run build）
		c.String(http.StatusOK, "前端未构建：请先 cd web && npm install && npm run build，再重新编译后端")
		return
	}
	// gin 的 NoRoute 会把状态预设成 404，这里必须显式改回来
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-cache, must-revalidate")
	c.Writer.Write(index)
}
