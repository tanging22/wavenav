package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wavenav/internal/models"
)

type urlIn struct {
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Weight int    `json:"weight"`
}

type itemIn struct {
	GroupID    uint    `json:"groupId"`
	Name       string  `json:"name" binding:"required,max=128"`
	Desc       string  `json:"desc"`
	Icon       string  `json:"icon"`
	Type       string  `json:"type"`
	Stat       string  `json:"stat"`
	OpenMode   string  `json:"openMode"`
	LayoutMode string  `json:"layoutMode"`
	Sort       *int    `json:"sort"`
	URLs       []urlIn `json:"urls"`
}

type itemPatch struct {
	GroupID    *uint    `json:"groupId"`
	Name       *string  `json:"name"`
	Desc       *string  `json:"desc"`
	Type       *string  `json:"type"`
	Stat       *string  `json:"stat"`
	Icon       *string  `json:"icon"`
	OpenMode   *string  `json:"openMode"`
	LayoutMode *string  `json:"layoutMode"`
	Sort       *int     `json:"sort"`
	URLs       *[]urlIn `json:"urls"`
}

func (s *Server) ownGroup(userID, groupID uint) error {
	var g models.Group
	return s.DB.Where("id = ? AND user_id = ?", groupID, userID).First(&g).Error
}

func validStat(t string) string {
	switch t {
	case "cpu", "mem":
		return t
	default:
		return "both"
	}
}

func validItemType(t string) string {
	switch t {
	case models.ItemContainer, models.ItemHost:
		return t
	default:
		return models.ItemLink
	}
}

func validKind(k string) string {
	switch k {
	case models.URLLan, models.URLWan, models.URLCustom:
		return k
	default:
		return models.URLDefault
	}
}

func buildURLs(itemID uint, in []urlIn) []models.ItemURL {
	out := make([]models.ItemURL, 0, len(in))
	for _, u := range in {
		if u.URL == "" {
			continue
		}
		out = append(out, models.ItemURL{
			ItemID: itemID,
			Kind:   validKind(u.Kind),
			URL:    u.URL,
			Weight: u.Weight,
		})
	}
	return out
}

// ---------- 分组 ----------

// groupIn 全指针：更新接口按 patch 语义走，前端只传哪个字段就只改哪个。
type groupIn struct {
	Name         *string `json:"name" binding:"omitempty,max=64"`
	Slug         *string `json:"slug"`
	Sort         *int    `json:"sort"`
	Style        *string `json:"style"`
	Display      *string `json:"display"`
	HiddenPublic *bool   `json:"hiddenPublic"`
}

func (s *Server) createGroup(c *gin.Context) {
	u := s.currentUser(c)
	var in groupIn
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "参数格式不对"})
		return
	}
	if in.Name == nil || strings.TrimSpace(*in.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "分组名不能为空"})
		return
	}
	g := models.Group{UserID: u.ID, Name: strings.TrimSpace(*in.Name)}
	if in.Slug != nil {
		g.Slug = *in.Slug
	}
	if in.Style != nil {
		g.Style = *in.Style
	}
	if in.Display != nil {
		switch *in.Display {
		case "card", "icon":
			g.Display = *in.Display
		default:
			g.Display = "" // 空 = 跟随全局
		}
	}
	if in.HiddenPublic != nil {
		g.HiddenPublic = *in.HiddenPublic
	}
	if in.Sort != nil {
		g.Sort = *in.Sort
	} else {
		var maxSort int
		s.DB.Model(&models.Group{}).Where("user_id = ?", u.ID).Select("COALESCE(MAX(sort),0)").Scan(&maxSort)
		g.Sort = maxSort + 1
	}
	if err := s.DB.Create(&g).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "group": g})
}

func (s *Server) updateGroup(c *gin.Context) {
	u := s.currentUser(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var g models.Group
	if err := s.DB.Where("id = ? AND user_id = ?", id, u.ID).First(&g).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "msg": "分组不存在"})
		return
	}
	var in groupIn
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "参数格式不对"})
		return
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		g.Name = strings.TrimSpace(*in.Name)
	}
	if in.Slug != nil {
		g.Slug = *in.Slug
	}
	if in.Style != nil {
		g.Style = *in.Style
	}
	if in.Display != nil {
		switch *in.Display {
		case "card", "icon":
			g.Display = *in.Display
		default:
			g.Display = "" // 空 = 跟随全局
		}
	}
	if in.HiddenPublic != nil {
		g.HiddenPublic = *in.HiddenPublic
	}
	if in.Sort != nil {
		g.Sort = *in.Sort
	}
	if err := s.DB.Save(&g).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "group": g})
}

// deleteGroup 删分组时组内卡片一起删（连同地址），事务保证不留孤儿。
func (s *Server) deleteGroup(c *gin.Context) {
	u := s.currentUser(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var g models.Group
	if err := s.DB.Where("id = ? AND user_id = ?", id, u.ID).First(&g).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "msg": "分组不存在"})
		return
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var ids []uint
		if err := tx.Model(&models.Item{}).Where("group_id = ? AND user_id = ?", g.ID, u.ID).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) > 0 {
			if err := tx.Where("item_id IN ?", ids).Delete(&models.ItemURL{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", ids).Delete(&models.Item{}).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&g).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- 卡片 ----------

func (s *Server) createItem(c *gin.Context) {
	u := s.currentUser(c)
	var in itemIn
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "卡片名称不能为空"})
		return
	}
	if in.GroupID == 0 || s.ownGroup(u.ID, in.GroupID) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "分组不存在"})
		return
	}
	icon := s.localizeIcon(u.ID, in.Icon)
	it := models.Item{
		UserID: u.ID, GroupID: in.GroupID, Name: strings.TrimSpace(in.Name), Desc: in.Desc,
		IconURL: icon, Type: validItemType(in.Type), Stat: validStat(in.Stat),
		OpenMode: defStr(in.OpenMode, "new"), LayoutMode: in.LayoutMode,
	}
	if in.Sort != nil {
		it.Sort = *in.Sort
	} else {
		var maxSort int
		s.DB.Model(&models.Item{}).Where("group_id = ?", in.GroupID).Select("COALESCE(MAX(sort),0)").Scan(&maxSort)
		it.Sort = maxSort + 1
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&it).Error; err != nil {
			return err
		}
		urls := buildURLs(it.ID, in.URLs)
		if len(urls) > 0 {
			return tx.Create(&urls).Error
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "创建失败"})
		return
	}
	s.DB.Preload("URLs").First(&it, it.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "item": it})
}

func (s *Server) updateItem(c *gin.Context) {
	u := s.currentUser(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var it models.Item
	if err := s.DB.Where("id = ? AND user_id = ?", id, u.ID).Preload("URLs").First(&it).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "msg": "卡片不存在"})
		return
	}
	var p itemPatch
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "参数不合法"})
		return
	}
	if p.GroupID != nil {
		if s.ownGroup(u.ID, *p.GroupID) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "目标分组不存在"})
			return
		}
		it.GroupID = *p.GroupID
	}
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "卡片名称不能为空"})
			return
		}
		it.Name = name
	}
	if p.Desc != nil {
		it.Desc = *p.Desc
	}
	if p.Icon != nil {
		it.IconURL = s.localizeIcon(u.ID, *p.Icon)
	}
	if p.Type != nil {
		it.Type = validItemType(*p.Type)
	}
	if p.Stat != nil {
		it.Stat = validStat(*p.Stat)
	}
	if p.OpenMode != nil {
		it.OpenMode = defStr(*p.OpenMode, "new")
	}
	if p.LayoutMode != nil {
		it.LayoutMode = *p.LayoutMode
	}
	if p.Sort != nil {
		it.Sort = *p.Sort
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&it).Error; err != nil {
			return err
		}
		if p.URLs != nil {
			if err := tx.Where("item_id = ?", it.ID).Delete(&models.ItemURL{}).Error; err != nil {
				return err
			}
			urls := buildURLs(it.ID, *p.URLs)
			if len(urls) > 0 {
				return tx.Create(&urls).Error
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "保存失败"})
		return
	}
	s.DB.Preload("URLs").First(&it, it.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "item": it})
}

func (s *Server) deleteItem(c *gin.Context) {
	u := s.currentUser(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var it models.Item
	if err := s.DB.Where("id = ? AND user_id = ?", id, u.ID).First(&it).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "msg": "卡片不存在"})
		return
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("item_id = ?", it.ID).Delete(&models.ItemURL{}).Error; err != nil {
			return err
		}
		return tx.Delete(&it).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type reorderReq struct {
	Items []struct {
		ID      uint `json:"id"`
		GroupID uint `json:"groupId"`
		Sort    int  `json:"sort"`
	} `json:"items"`
	Groups []struct {
		ID   uint `json:"id"`
		Sort int  `json:"sort"`
	} `json:"groups"`
}

// reorderItems 一次事务里落盘所有排序，拖完立刻保存。
func (s *Server) reorderItems(c *gin.Context) {
	u := s.currentUser(c)
	var req reorderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "参数不合法"})
		return
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		for _, it := range req.Items {
			updates := map[string]any{"sort": it.Sort}
			if it.GroupID != 0 {
				updates["group_id"] = it.GroupID
			}
			if err := tx.Model(&models.Item{}).Where("id = ? AND user_id = ?", it.ID, u.ID).Updates(updates).Error; err != nil {
				return err
			}
		}
		for _, g := range req.Groups {
			if err := tx.Model(&models.Group{}).Where("id = ? AND user_id = ?", g.ID, u.ID).Update("sort", g.Sort).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "排序保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func defStr(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// localizeIcon：保存卡片时把外链图标转存到本地（持久化），失败就原样保留外链。
func (s *Server) localizeIcon(userID uint, icon string) string {
	icon = strings.TrimSpace(icon)
	if !strings.HasPrefix(icon, "http://") && !strings.HasPrefix(icon, "https://") {
		return icon // 已经是本地图标或 emoji，直接用
	}
	if local := s.downloadIcon(userID, icon); local != "" {
		return local
	}
	return icon
}
