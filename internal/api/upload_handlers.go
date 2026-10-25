package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wavenav/internal/models"
)

const (
	maxImageBytes = 10 << 20  // 10MB
	maxVideoBytes = 300 << 20 // 300MB
)

// mimeByExt 白名单：扩展名 -> 允许的实测 MIME 前缀
var mimeByExt = map[string]string{
	".png":  "image/png",
	".apng": "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".jfif": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".svg":  "svg",
	".ico":  "image/x-icon",
	".cur":  "image/x-icon",
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".webm": "video/webm",
	".mov":  "video/quicktime",
}

// mediaFamily 把 MIME 收敛成 image / video / 其它，用于放宽同族格式的校验。
func mediaFamily(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case mime == "svg":
		return "image"
	}
	return mime
}

type uploadResult struct {
	ID     uint   `json:"id"`
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	Mime   string `json:"mime"`
	Sha256 string `json:"-"`
}

// saveUpload 把上传流写到磁盘并登记到图库。
func (s *Server) saveUpload(userID uint, kind, origName string, r io.Reader, size int64) (*uploadResult, int, string) {
	ext := strings.ToLower(filepath.Ext(origName))
	want, allowed := mimeByExt[ext]
	if !allowed {
		return nil, http.StatusBadRequest, "不支持的文件类型（图片 png/jpg/jpeg/webp/gif/svg/ico/avif/bmp，视频 mp4/m4v/webm/mov）"
	}
	limit := int64(maxImageBytes)
	if ext == ".mp4" || ext == ".webm" || ext == ".m4v" || ext == ".mov" {
		limit = maxVideoBytes
	}
	if size > limit {
		return nil, http.StatusBadRequest, fmt.Sprintf("文件超过 %dMB 限制", limit>>20)
	}

	// 落盘到临时文件，边写边算 hash 边验头
	now := time.Now()
	dir := filepath.Join(s.Cfg.UploadDir(), now.Format("2006/01"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, http.StatusInternalServerError, "创建目录失败"
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return nil, http.StatusInternalServerError, "创建临时文件失败"
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	h := sha256.New()
	head := make([]byte, 512)
	var written int64
	n, err := io.ReadFull(io.TeeReader(io.LimitReader(r, limit+1), h), head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, http.StatusBadRequest, "读取上传内容失败"
	}
	head = head[:n]
	if n == 0 {
		return nil, http.StatusBadRequest, "空文件"
	}
	if _, err := tmp.Write(head); err != nil {
		return nil, http.StatusInternalServerError, "写入失败"
	}
	written = int64(n)
	rest, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, limit+1-written))
	if err != nil {
		return nil, http.StatusBadRequest, "读取上传内容失败"
	}
	written += rest
	if written > limit {
		return nil, http.StatusRequestEntityTooLarge, fmt.Sprintf("文件超过 %dMB 限制", limit>>20)
	}

	detected := http.DetectContentType(head)
	if want == "svg" {
		if !strings.Contains(strings.ToLower(string(head)), "<svg") {
			return nil, http.StatusBadRequest, "不是有效的 SVG"
		}
	} else if mediaFamily(detected) != mediaFamily(want) {
		// 不逐字节比对具体 MIME：avif/bmp/cur 这类 http.DetectContentType 常认不准，
		// 只要大类还是同一个（图片归图片、视频归视频）就放行，
		// 把 txt 改名的假图标照样挡在外面。
		return nil, http.StatusBadRequest, "文件内容与扩展名不符：" + detected
	}
	if err := tmp.Sync(); err != nil {
		return nil, http.StatusInternalServerError, "落盘失败"
	}

	sum := hex.EncodeToString(h.Sum(nil))

	// 同一用户同一类型的相同文件直接复用
	var exist models.Icon
	if err := s.DB.Where("user_id = ? AND kind = ? AND sha256 = ?", userID, kind, sum).First(&exist).Error; err == nil {
		return &uploadResult{ID: exist.ID, Kind: exist.Kind, Path: exist.Path, URL: "/uploads/" + exist.Path, Size: exist.Size, Mime: exist.Mime, Sha256: exist.Sha256}, http.StatusOK, ""
	}

	suffix := make([]byte, 3)
	rand.Read(suffix)
	name := sum[:12] + "-" + hex.EncodeToString(suffix) + ext
	rel := filepath.Join(now.Format("2006/01"), name)
	finalPath := filepath.Join(s.Cfg.UploadDir(), rel)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return nil, http.StatusInternalServerError, "保存文件失败"
	}

	ic := models.Icon{
		UserID: userID, Kind: kind, Path: filepath.ToSlash(rel),
		Sha256: sum, Size: written, Mime: detected,
	}
	if err := s.DB.Create(&ic).Error; err != nil {
		os.Remove(finalPath)
		return nil, http.StatusInternalServerError, "登记失败"
	}
	return &uploadResult{ID: ic.ID, Kind: kind, Path: ic.Path, URL: "/uploads/" + ic.Path, Size: written, Mime: detected, Sha256: sum}, http.StatusOK, ""
}

func (s *Server) upload(c *gin.Context) {
	u := s.currentUser(c)
	kind := c.DefaultPostForm("kind", models.KindIcon)
	if kind != models.KindIcon && kind != models.KindWallpaper {
		kind = models.KindIcon
	}
	// 注意：c.Request.Body 需先包 MaxBytesReader 才能挡住超大请求
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxVideoBytes+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "没有收到文件"})
		return
	}
	defer file.Close()

	res, status, msg := s.saveUpload(u.ID, kind, header.Filename, file, header.Size)
	if status != http.StatusOK && res == nil {
		c.JSON(status, gin.H{"ok": false, "msg": msg})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "icon": res})
}

// gallery 图库列表，按类型筛选 + 简单分页。
func (s *Server) gallery(c *gin.Context) {
	u := s.currentUser(c)
	kind := c.Query("kind")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "40"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 40
	}
	q := s.DB.Model(&models.Icon{}).Where("user_id = ?", u.ID)
	if kind == models.KindIcon || kind == models.KindWallpaper {
		q = q.Where("kind = ?", kind)
	}
	// 按媒体类型筛：图库/壁纸混在一起时前端要能只看图或只看视频
	switch c.Query("type") {
	case "image":
		q = q.Where("mime LIKE ?", "image/%")
	case "video":
		q = q.Where("mime LIKE ?", "video/%")
	}
	var total int64
	q.Count(&total)
	var list []models.Icon
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "读取失败"})
		return
	}
	for i := range list {
		list[i].Path = "/uploads/" + list[i].Path
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "total": total, "page": page, "size": size, "list": list})
}

func (s *Server) deleteGallery(c *gin.Context) {
	u := s.currentUser(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var ic models.Icon
	if err := s.DB.Where("id = ? AND user_id = ?", id, u.ID).First(&ic).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "msg": "素材不存在"})
		return
	}
	// 还被卡片用着就别删，免得悄悄弄坏别人的图标
	ref := "/uploads/" + ic.Path
	var useCount int64
	s.DB.Model(&models.Item{}).Where("user_id = ? AND icon_url = ?", u.ID, ref).Count(&useCount)
	if useCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "msg": "还有 " + strconv.FormatInt(useCount, 10) + " 张卡片在用这个文件，先换掉再删"})
		return
	}
	// 正被拿来当壁纸的也别删
	var st models.Setting
	if err := s.DB.Where("user_id = ?", u.ID).First(&st).Error; err == nil {
		var set models.Settings
		if json.Unmarshal([]byte(st.Data), &set) == nil && set.Wallpaper.Src == ref {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "msg": "这张正在当主页壁纸用，先换一张再删"})
			return
		}
	}
	if err := s.DB.Delete(&ic).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "msg": "删除失败"})
		return
	}
	_ = os.Remove(filepath.Join(s.Cfg.UploadDir(), filepath.FromSlash(ic.Path)))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
