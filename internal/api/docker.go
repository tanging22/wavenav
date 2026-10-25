package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 容器状态监控：不引 docker SDK（几十兆），直接对 unix socket 讲 Docker Engine API。
// 权限沿用宿主 socket 权限，容器里跑需要挂 -v /var/run/docker.sock:/var/run/docker.sock:ro
// （要启停容器就去掉 :ro）。

const dockerAPIVersion = "v1.41"

var dockerIDSafe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

type dockerPort struct {
	IP          string `json:"IP"`
	PrivatePort int    `json:"PrivatePort"`
	PublicPort  int    `json:"PublicPort"`
	Type        string `json:"Type"`
}

type dockerRawContainer struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`
	Status string            `json:"Status"`
	Ports  []dockerPort      `json:"Ports"`
	Labels map[string]string `json:"Labels"`
}

type dockerContainerOut struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	State   string   `json:"state"`
	Status  string   `json:"status"`
	Ports   []string `json:"ports"`
	Project string   `json:"project"`
}

func dockerSocketPath(cfgSocket string) string {
	if v := os.Getenv("WAVENAV_DOCKER_SOCK"); v != "" {
		return v
	}
	if cfgSocket != "" {
		return cfgSocket
	}
	return "/var/run/docker.sock"
}

// validUnixPath 只放行本地 unix socket 路径，防止有人拿它当任意文件读取。
func validUnixPath(p string) bool {
	if !strings.HasPrefix(p, "/") || len(p) > 200 {
		return false
	}
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '/', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// validDockerAddr 放行两种地址：unix socket 路径，或 tcp://host:port（socat 中继）。
func validDockerAddr(a string) bool {
	if !strings.HasPrefix(a, "tcp://") {
		return validUnixPath(a)
	}
	host := strings.TrimPrefix(a, "tcp://")
	if host == "" || len(host) > 120 {
		return false
	}
	h, port, err := net.SplitHostPort(host)
	if err != nil || h == "" || port == "" {
		return false
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return false
	}
	for _, r := range h {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '-', r == '_', r == ':', r == '[' || r == ']':
		default:
			return false
		}
	}
	return true
}

func dockerClient(sock string) *http.Client {
	if strings.HasPrefix(sock, "tcp://") {
		return &http.Client{Timeout: 8 * time.Second}
	}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 3 * time.Second}
			return d.DialContext(ctx, "unix", sock)
		},
		DisableKeepAlives: true,
	}
	return &http.Client{Transport: tr, Timeout: 8 * time.Second}
}

func (s *Server) dockerReq(sock string, method, path string) ([]byte, int, error) {
	if !validDockerAddr(sock) {
		return nil, 0, errors.New("socket 路径不合法")
	}
	base := "http://docker"
	if strings.HasPrefix(sock, "tcp://") {
		base = "http://" + strings.TrimPrefix(sock, "tcp://")
	}
	req, err := http.NewRequest(method, base+"/"+dockerAPIVersion+path, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := dockerClient(sock).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return body, resp.StatusCode, nil
}

// dockerContainers 列出所有容器（含已停止）。
func (s *Server) dockerContainers(c *gin.Context) {
	u := s.currentUser(c)
	sock := dockerSocketPath(s.loadSettings(u.ID).Docker.Socket)

	body, code, err := s.dockerReq(sock, http.MethodGet, "/containers/json?all=1")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":        true,
			"available": false,
			"msg":       dockerErrHint(err, sock),
		})
		return
	}
	if code != http.StatusOK {
		c.JSON(http.StatusOK, gin.H{"ok": true, "available": false, "msg": fmt.Sprintf("Docker 返回 %d", code)})
		return
	}

	var raw []dockerRawContainer
	if err := json.Unmarshal(body, &raw); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "available": false, "msg": "解析 Docker 响应失败"})
		return
	}

	out := make([]dockerContainerOut, 0, len(raw))
	for _, ct := range raw {
		name := ""
		if len(ct.Names) > 0 {
			name = strings.TrimPrefix(ct.Names[0], "/")
		}
		ports := make([]string, 0, len(ct.Ports))
		for _, p := range ct.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d→%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			} else {
				ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
			}
		}
		out = append(out, dockerContainerOut{
			ID:      ct.ID,
			Name:    name,
			Image:   ct.Image,
			State:   ct.State,
			Status:  ct.Status,
			Ports:   ports,
			Project: ct.Labels["com.docker.compose.project"],
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "available": true, "containers": out})
}

// dockerAction 启停容器。只允许 start/stop/restart，id 走白名单正则。
func (s *Server) dockerAction(c *gin.Context) {
	u := s.currentUser(c)
	id := c.Param("id")
	action := c.Param("action")
	if !dockerIDSafe.MatchString(id) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "容器 id 不合法"})
		return
	}
	switch action {
	case "start", "stop", "restart":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "msg": "不支持的操作"})
		return
	}

	sock := dockerSocketPath(s.loadSettings(u.ID).Docker.Socket)
	_, code, err := s.dockerReq(sock, http.MethodPost, "/containers/"+id+"/"+action)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "msg": dockerErrHint(err, sock)})
		return
	}
	// Docker 对启停返回 204，已经在跑/已经停也返回 304
	if code == http.StatusNoContent || code == http.StatusNotModified {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"ok": false, "msg": fmt.Sprintf("Docker 返回 %d", code)})
}

func dockerErrHint(err error, sock string) string {
	if errors.Is(err, os.ErrNotExist) {
		return "找不到 " + sock + "，这台机器可能没装 Docker"
	}
	if errors.Is(err, os.ErrPermission) {
		return "没有权限读 " + sock + "，把运行用户加进 docker 组"
	}
	msg := err.Error()
	if strings.HasPrefix(sock, "tcp://") && strings.Contains(msg, "connection refused") {
		return "连不上 " + sock + "，转发容器（socat）没在跑？"
	}
	if strings.Contains(msg, "permission denied") {
		return "没有权限读 " + sock + "（permission denied）"
	}
	if strings.Contains(msg, "no such file") {
		return "找不到 " + sock + "，这台机器可能没装 Docker"
	}
	return "连不上 Docker：" + msg
}

// ---------- 状态卡：容器 CPU/内存 + 主机 CPU/内存 ----------

type dockerStatsOut struct {
	CPUPercent float64 `json:"cpuPercent"`
	MemUsed    float64 `json:"memUsed"`  // 字节
	MemLimit   float64 `json:"memLimit"` // 字节
	State      string  `json:"state"`
}

// readCPUTimes 读 /proc/stat 首行，返回 busy/total 两个 jiffies 累计值
func readCPUTimes() (busy, total uint64) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(strings.SplitN(string(b), "\n", 2)[0])
	if len(fields) < 5 {
		return 0, 0
	}
	var vals []uint64
	for _, f := range fields[1:] {
		v, _ := strconv.ParseUint(f, 10, 64)
		vals = append(vals, v)
	}
	for _, v := range vals {
		total += v
	}
	idle := uint64(0)
	if len(vals) > 3 {
		idle += vals[3]
	}
	if len(vals) > 4 {
		idle += vals[4] // iowait
	}
	return total - idle, total
}

func hostCPUPercent() float64 {
	b0, t0 := readCPUTimes()
	time.Sleep(200 * time.Millisecond)
	b1, t1 := readCPUTimes()
	dt := float64(t1 - t0)
	if dt <= 0 {
		return 0
	}
	pct := float64(b1-b0) / dt * 100
	return math.Round(pct*10) / 10
}

func hostMemStats() (used, total float64) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var avail float64
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		v, _ := strconv.ParseFloat(f[1], 64)
		switch f[0] {
		case "MemTotal:":
			total = v * 1024
		case "MemAvailable:":
			avail = v * 1024
		}
	}
	return total - avail, total
}

// parseContainerStats 从 stream=false 的 stats JSON 里算 CPU% 和内存
func parseContainerStats(body []byte) (cpuPct float64, memUsed, memLimit float64) {
	var st struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage  uint64   `json:"total_usage"`
				PercpuUsage []uint64 `json:"percpu_usage"`
			} `json:"cpu_usage"`
			OnlineCPUs  int    `json:"online_cpus"`
			SystemUsage uint64 `json:"system_cpu_usage"`
		} `json:"cpu_stats"`
		PreCPU struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemStats struct {
			Usage float64            `json:"usage"`
			Limit float64            `json:"limit"`
			Stats map[string]float64 `json:"stats"`
		} `json:"memory_stats"`
	}
	if json.Unmarshal(body, &st) != nil {
		return 0, 0, 0
	}
	cpuDelta := float64(st.CPUStats.CPUUsage.TotalUsage) - float64(st.PreCPU.CPUUsage.TotalUsage)
	sysDelta := float64(st.CPUStats.SystemUsage) - float64(st.PreCPU.SystemUsage)
	online := float64(st.CPUStats.OnlineCPUs)
	if online == 0 {
		online = float64(len(st.CPUStats.CPUUsage.PercpuUsage))
	}
	if sysDelta > 0 && online > 0 {
		cpuPct = math.Round(cpuDelta/sysDelta*online*1000) / 10
	}
	memUsed, memLimit = st.MemStats.Usage, st.MemStats.Limit
	if c, ok := st.MemStats.Stats["cache"]; ok && c > 0 {
		memUsed -= c
	} else if i, ok := st.MemStats.Stats["inactive_file"]; ok {
		memUsed -= i
	}
	if memUsed < 0 {
		memUsed = 0
	}
	return
}

// dockerStats 一次返回主机 CPU/内存 + 每个容器的 CPU/内存。
// 容器 stats 是逐个接口，这里并发拉，proxy（CONTAINERS=1）也放行 /containers/*/stats。
func (s *Server) dockerStats(c *gin.Context) {
	u := s.currentUser(c)
	sock := dockerSocketPath(s.loadSettings(u.ID).Docker.Socket)

	cpuPct := hostCPUPercent()
	memUsed, memTotal := hostMemStats()
	host := gin.H{"cpuPercent": cpuPct, "memUsed": memUsed, "memTotal": memTotal}

	body, code, err := s.dockerReq(sock, http.MethodGet, "/containers/json?all=1")
	if err != nil || code != http.StatusOK {
		msg := dockerErrHint(err, sock)
		if err == nil {
			msg = fmt.Sprintf("Docker 返回 %d", code)
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "available": false, "host": host, "containers": []dockerStatsOut{}, "msg": msg})
		return
	}
	var raw []dockerRawContainer
	if json.Unmarshal(body, &raw) != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "available": false, "host": host, "containers": []dockerStatsOut{}, "msg": "解析 Docker 响应失败"})
		return
	}

	type namedStat struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		dockerStatsOut
	}
	out := make([]namedStat, 0, len(raw))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, ct := range raw {
		name := ""
		if len(ct.Names) > 0 {
			name = strings.TrimPrefix(ct.Names[0], "/")
		}
		wg.Add(1)
		go func(id, name, state string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			b, _, err := s.dockerReq(sock, http.MethodGet, "/containers/"+id+"/stats?stream=false")
			if err != nil {
				return
			}
			cpu, mem, limit := parseContainerStats(b)
			mu.Lock()
			st := namedStat{ID: id, Name: name}
			st.CPUPercent, st.MemUsed, st.MemLimit, st.State = cpu, mem, limit, state
			out = append(out, st)
			mu.Unlock()
		}(ct.ID, name, ct.State)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	c.JSON(http.StatusOK, gin.H{"ok": true, "available": true, "host": host, "containers": out})
}
