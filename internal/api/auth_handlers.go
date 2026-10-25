package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"wavenav/internal/auth"
	"wavenav/internal/models"
)

// loginLimiter 进程内登录限流：同一 IP 10 分钟内失败 10 次锁 10 分钟。
// NAS 自用场景不需要 Redis。
type loginLimiter struct {
	mu       sync.Mutex
	fails    map[string][]time.Time
	window   time.Duration
	maxFails int
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{
		fails:    map[string][]time.Time{},
		window:   10 * time.Minute,
		maxFails: 10,
	}
}

func (l *loginLimiter) blocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.fails[ip][:0]
	for _, t := range l.fails[ip] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	l.fails[ip] = kept
	return len(kept) >= l.maxFails
}

func (l *loginLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[ip] = append(l.fails[ip], time.Now())
}

func (l *loginLimiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, ip)
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Remember bool   `json:"remember"`
}

func (s *Server) login(c *gin.Context) {
	ip := c.ClientIP()
	if s.limiter.blocked(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"ok": false, "msg": "失败次数过多，请 10 分钟后再试"})
		return
	}
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "账号密码不能为空"})
		return
	}
	u, token, err := auth.Login(s.DB, req.Username, req.Password, s.Cfg.SessionDays)
	if err != nil {
		s.limiter.fail(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "msg": "用户名或密码错误"})
		return
	}
	s.limiter.reset(ip)
	days := 1
	if req.Remember {
		days = s.Cfg.SessionDays
	}
	// 重新签发一个期限符合"记住我"的会话
	_ = auth.DeleteSession(s.DB, token)
	token, _, err = auth.NewSession(s.DB, u.ID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "创建会话失败"})
		return
	}
	s.setSessionCookie(c, token, days)
	c.JSON(http.StatusOK, gin.H{"ok": true, "user": publicUser(u)})
}

func (s *Server) logout(c *gin.Context) {
	if token, err := c.Cookie(sessionCookie); err == nil && token != "" {
		_ = auth.DeleteSession(s.DB, token)
	}
	s.setSessionCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) me(c *gin.Context) {
	u := s.currentUser(c)
	if u == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "user": nil, "publicMode": s.publicMode()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "user": publicUser(u)})
}

type passwordReq struct {
	Old string `json:"old"`
	New string `json:"new" binding:"required,min=8,max=72"`
}

func (s *Server) changePassword(c *gin.Context) {
	u := s.currentUser(c)
	var req passwordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "新密码至少 8 位"})
		return
	}
	if !auth.VerifyPassword(req.Old, u.PasswordHash) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "原密码不正确"})
		return
	}
	hash, err := auth.HashPassword(req.New)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "生成密码失败"})
		return
	}
	if err := s.DB.Model(u).Update("password_hash", hash).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "保存失败"})
		return
	}
	token, _ := c.Get("token")
	if t, ok := token.(string); ok {
		_ = auth.DeleteOtherSessions(s.DB, u.ID, t)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type usernameReq struct {
	Username string `json:"username" binding:"required,min=2,max=32"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// validUsername 允许字母、数字、下划线、连字符和中文，2-32 位。
func validUsername(s string) bool {
	if len([]rune(s)) < 2 || len([]rune(s)) > 32 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_', r == '-', r == '.', r == '@':
		case r >= 0x4e00 && r <= 0x9fff: // 常用汉字
		default:
			return false
		}
	}
	return true
}

// changeUsername 改登录账号（用户名）。密码确认后生效，会话不受影响。
func (s *Server) changeUsername(c *gin.Context) {
	u := s.currentUser(c)
	var req usernameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "账号 2-32 位，密码至少 8 位"})
		return
	}
	name := strings.TrimSpace(req.Username)
	if !validUsername(name) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "账号只能用字母、数字、下划线、连字符或中文"})
		return
	}
	if name == u.Username {
		c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "账号没变"})
		return
	}
	if !auth.VerifyPassword(req.Password, u.PasswordHash) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "密码不正确"})
		return
	}
	var count int64
	s.DB.Model(&models.User{}).Where("username = ?", name).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "msg": "这个账号已经有人用了"})
		return
	}
	if err := s.DB.Model(u).Update("username", name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "username": name})
}

type adminPasswordReq struct {
	Username string `json:"username" binding:"required"`
	New      string `json:"new" binding:"required,min=8,max=72"`
}

// adminSetPassword 管理员重置任意账号密码（M1 用于"忘了密码"自救）。
func (s *Server) adminSetPassword(c *gin.Context) {
	var req adminPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "参数不合法"})
		return
	}
	var target models.User
	if err := s.DB.Where("username = ?", req.Username).First(&target).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "msg": "账号不存在"})
		return
	}
	hash, err := auth.HashPassword(req.New)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "生成密码失败"})
		return
	}
	if err := s.DB.Model(&target).Update("password_hash", hash).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "保存失败"})
		return
	}
	_ = auth.DeleteOtherSessions(s.DB, target.ID, "")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// publicUser 对外输出去掉哈希与内部字段
func publicUser(u *models.User) gin.H {
	return gin.H{
		"id":       u.ID,
		"username": u.Username,
		"nickname": u.Nickname,
		"avatar":   u.Avatar,
		"role":     u.Role,
	}
}
