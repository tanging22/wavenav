package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"wavenav/internal/api"
	"wavenav/internal/auth"
	"wavenav/internal/config"
	"wavenav/internal/database"
	"wavenav/internal/web"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("[wavenav] 打开数据库失败: %v", err)
	}
	if err := database.Seed(db, cfg); err != nil {
		log.Fatalf("[wavenav] 初始化数据失败: %v", err)
	}

	// 过期会话每小时扫一次，进程内定时器就够（单实例部署）
	go func() {
		tick := time.NewTicker(time.Hour)
		defer tick.Stop()
		for {
			if err := auth.CleanupExpired(db); err != nil {
				log.Printf("[wavenav] 清理会话失败: %v", err)
			}
			<-tick.C
		}
	}()

	srv := api.New(db, cfg, web.FS())
	httpSrv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("[wavenav] 监听 http://0.0.0.0:%d （数据目录 %s）", cfg.Port, cfg.DataDir)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[wavenav] 启动失败: %v", err)
		}
	}()

	// 优雅退出：给正在跑的上传留 10 秒收尾
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[wavenav] 正在关闭…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("[wavenav] 关闭超时: %v", err)
	}
}
