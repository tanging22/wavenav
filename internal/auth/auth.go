package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"
	"gorm.io/gorm"

	"wavenav/internal/models"
)

const (
	saltLen = 16
	keyLen  = 32
	// scrypt 参数：N=32768 在 NAS 上单次约 50~80ms，足够抗暴力又不会拖慢登录
	scryptN = 32768
	scryptR = 8
	scryptP = 1
)

var ErrInvalidCredentials = errors.New("用户名或密码错误")

// HashPassword 返回 `scrypt$N$r$p$salt$hash`，参数随哈希一起存，以后调参不破坏老密码。
func HashPassword(pw string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk, err := scrypt.Key([]byte(pw), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("scrypt$%d$%d$%d$%s$%s",
		scryptN, scryptR, scryptP,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk),
	), nil
}

// VerifyPassword 常量时间比较。
func VerifyPassword(pw, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "scrypt" {
		return false
	}
	var n, r, p int
	if _, err := fmt.Sscanf(parts[1], "%d", &n); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(parts[2], "%d", &r); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "%d", &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got, err := scrypt.Key([]byte(pw), salt, n, r, p, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewSession 生成随机 token，把 hash 存库，明文只回给调用方写 Cookie。
func NewSession(db *gorm.DB, userID uint, days int) (string, *models.Session, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	if days <= 0 {
		days = 30
	}
	s := &models.Session{
		TokenHash: hashToken(token),
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Duration(days) * 24 * time.Hour),
	}
	if err := db.Create(s).Error; err != nil {
		return "", nil, err
	}
	return token, s, nil
}

// Lookup 用明文 token 反查用户；过期会话顺手删掉。
func Lookup(db *gorm.DB, token string) (*models.User, *models.Session, error) {
	if token == "" {
		return nil, nil, ErrInvalidCredentials
	}
	var s models.Session
	if err := db.Where("token_hash = ?", hashToken(token)).First(&s).Error; err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if time.Now().After(s.ExpiresAt) {
		db.Delete(&s)
		return nil, nil, ErrInvalidCredentials
	}
	var u models.User
	if err := db.First(&u, s.UserID).Error; err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	return &u, &s, nil
}

// Login 校验账号密码并建会话。
func Login(db *gorm.DB, username, password string, days int) (*models.User, string, error) {
	var u models.User
	if err := db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if !VerifyPassword(password, u.PasswordHash) {
		return nil, "", ErrInvalidCredentials
	}
	token, _, err := NewSession(db, u.ID, days)
	if err != nil {
		return nil, "", err
	}
	return &u, token, nil
}

// DeleteSession 登出。
func DeleteSession(db *gorm.DB, token string) error {
	return db.Where("token_hash = ?", hashToken(token)).Delete(&models.Session{}).Error
}

// DeleteOtherSessions 改密后踢掉其它设备，只留当前这条。
func DeleteOtherSessions(db *gorm.DB, userID uint, keepToken string) error {
	q := db.Where("user_id = ?", userID)
	if keepToken != "" {
		q = q.Where("token_hash <> ?", hashToken(keepToken))
	}
	return q.Delete(&models.Session{}).Error
}

// CleanupExpired 定时清理过期会话。
func CleanupExpired(db *gorm.DB) error {
	return db.Where("expires_at < ?", time.Now()).Delete(&models.Session{}).Error
}
