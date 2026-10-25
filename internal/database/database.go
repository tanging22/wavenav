package database

import (
	"encoding/json"
	"log"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wavenav/internal/auth"
	"wavenav/internal/config"
	"wavenav/internal/models"
)

// Open 打开 SQLite 并自动迁移。SQLite 用 glebarez/sqlite（纯 Go，免 CGO）。
func Open(cfg *config.Config) (*gorm.DB, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.UploadDir(), 0o755); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(cfg.DBPath()+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(
		&models.User{}, &models.Session{}, &models.Group{},
		&models.Item{}, &models.ItemURL{}, &models.Icon{}, &models.Setting{},
	); err != nil {
		return nil, err
	}
	return db, nil
}

// Seed 首次启动创建管理员 + 一个默认分组 + 默认设置。已有数据时什么都不做。
func Seed(db *gorm.DB, cfg *config.Config) error {
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	log.Printf("[seed] 首次启动，创建管理员 %q", cfg.AdminUser)
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	u := models.User{
		Username:     cfg.AdminUser,
		PasswordHash: hash,
		Role:         models.RoleAdmin,
		Nickname:     "管理员",
	}
	if err := db.Create(&u).Error; err != nil {
		return err
	}

	g := models.Group{UserID: u.ID, Name: "常用", Slug: "common", Sort: 0}
	if err := db.Create(&g).Error; err != nil {
		return err
	}

	type seedItem struct {
		item models.Item
		urls []models.ItemURL
	}
	seeds := []seedItem{
		{
			item: models.Item{UserID: u.ID, GroupID: g.ID, Name: "fnOS 管理", Desc: "NAS 管理后台", Sort: 0, OpenMode: "new"},
			urls: []models.ItemURL{{Kind: models.URLLan, URL: "http://192.168.1.2:5666"}},
		},
		{
			item: models.Item{UserID: u.ID, GroupID: g.ID, Name: "GitHub", Desc: "代码托管", IconURL: "https://github.com/favicon.ico", Sort: 1, OpenMode: "new"},
			urls: []models.ItemURL{{Kind: models.URLDefault, URL: "https://github.com"}},
		},
	}
	for i := range seeds {
		if err := db.Create(&seeds[i].item).Error; err != nil {
			return err
		}
		for j := range seeds[i].urls {
			seeds[i].urls[j].ItemID = seeds[i].item.ID
			if err := db.Create(&seeds[i].urls[j]).Error; err != nil {
				return err
			}
		}
	}

	s := models.DefaultSettings()
	s.FooterHTML = `<a href="https://beian.miit.gov.cn/" target="_blank">备案号待填</a>`
	raw, _ := json.Marshal(s)
	return db.Create(&models.Setting{UserID: u.ID, Data: string(raw)}).Error
}
