package models

import "time"

// User 账号。Role: admin | user
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Role         string    `gorm:"size:16;default:user" json:"role"`
	Nickname     string    `gorm:"size:64" json:"nickname"`
	Avatar       string    `gorm:"size:512" json:"avatar"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Session 会话。Cookie 里放明文 token，库里只存 sha256。
type Session struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	TokenHash string    `gorm:"uniqueIndex;size:64;not null" json:"-"`
	UserID    uint      `gorm:"index;not null" json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"-"`
}

// Group 卡片分组
type Group struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID uint   `gorm:"index;not null" json:"-"`
	Name   string `gorm:"size:64;not null" json:"name"`
	Slug   string `gorm:"size:64" json:"slug"`
	Sort   int    `json:"sort"`
	Style  string `gorm:"type:text" json:"style"`
	// 本组的显示方式：空=跟随全局布局，card=卡片，icon=只图标
	Display      string    `gorm:"size:16" json:"display"`
	HiddenPublic bool      `json:"hiddenPublic"`
	CreatedAt    time.Time `json:"-"`
}

// Item 导航卡片
type Item struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"-"`
	GroupID    uint      `gorm:"index" json:"groupId"`
	Name       string    `gorm:"size:128;not null" json:"name"`
	Desc       string    `gorm:"size:512" json:"desc"`
	IconURL    string    `gorm:"size:1024" json:"icon"`
	Sort       int       `json:"sort"`
	Type       string    `gorm:"size:16;default:link" json:"type"` // link | container | host
	Stat       string    `gorm:"size:8;default:both" json:"stat"`  // 状态卡显示内容：both | cpu | mem
	LayoutMode string    `gorm:"size:16" json:"layoutMode"`
	OpenMode   string    `gorm:"size:16;default:new" json:"openMode"`
	URLs       []ItemURL `gorm:"foreignKey:ItemID;constraint:OnDelete:CASCADE" json:"urls"`
	CreatedAt  time.Time `json:"-"`
}

// 卡片类型：普通链接、容器状态、主机状态
const (
	ItemLink      = "link"
	ItemContainer = "container"
	ItemHost      = "host"
)

// ItemURL 同一张卡片可以有多个地址，按 Kind 区分用途。
// Kind: default | lan | wan | custom
type ItemURL struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	ItemID uint   `gorm:"index;not null" json:"-"`
	Kind   string `gorm:"size:16;default:'default'" json:"kind"`
	URL    string `gorm:"size:1024;not null" json:"url"`
	Weight int    `json:"weight"`
}

// Icon 图库素材（图标 / 壁纸）
type Icon struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"-"`
	Kind      string    `gorm:"size:16;index" json:"kind"`
	Path      string    `gorm:"size:512" json:"path"`
	ThumbPath string    `gorm:"size:512" json:"thumb"`
	Sha256    string    `gorm:"size:64;index" json:"sha256"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Size      int64     `json:"size"`
	Mime      string    `gorm:"size:64" json:"mime"`
	CreatedAt time.Time `json:"createdAt"`
}

// Setting 每个用户的设置，整体一个 JSON blob，结构随前端演进而不用改表。
type Setting struct {
	UserID uint   `gorm:"primaryKey" json:"-"`
	Data   string `gorm:"type:text" json:"data"`
}

// 角色
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// 地址类型
const (
	URLDefault = "default"
	URLLan     = "lan"
	URLWan     = "wan"
	URLCustom  = "custom"
)

// 素材类型
const (
	KindIcon      = "icon"
	KindWallpaper = "wallpaper"
)
