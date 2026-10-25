package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"wavenav/internal/models"
)

func nowUnix() int64 { return time.Now().Unix() }

// exportData 全量导出，结构跟 panel 对齐，方便直接当迁移/备份文件。
func (s *Server) exportData(c *gin.Context) {
	u := s.currentUser(c)
	settings := s.loadSettings(u.ID)

	var groups []models.Group
	s.DB.Where("user_id = ?", u.ID).Order("sort, id").Find(&groups)
	var items []models.Item
	s.DB.Where("user_id = ?", u.ID).Order("sort, id").Preload("URLs").Find(&items)
	var icons []models.Icon
	s.DB.Where("user_id = ?", u.ID).Find(&icons)

	c.Header("Content-Disposition", `attachment; filename="wavenav-export.json"`)
	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"version":  1,
		"exported": nowUnix(),
		"settings": settings,
		"groups":   groups,
		"items":    items,
		"icons":    icons,
	})
}
