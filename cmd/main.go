package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"web_test/internal/logger"
	"web_test/pkg/factory"

	"github.com/gin-gonic/gin"
)

func main() {
	// 0. 允許從 Command Line 指定 config 路徑 (選用，更靈活)
	configPath := flag.String("c", "config.yml", "path to config file")
	flag.Parse()

	// 2. 使用 Factory 讀取設定檔
	// 這裡會執行 ReadFile -> Unmarshal -> Validate -> Print
	cfg, err := factory.ReadConfig(*configPath)
	if err != nil {
		logger.MainLog.Fatalf("Failed to load config: %+v", err)
	}

	logger.MainLog.Infof("System initializing with Redis Addr: %s", cfg.Redis.Addr)

	// 4. 建立 Factory (依賴注入容器)
	f := factory.NewFactory(cfg)

	// 6. 設定 Context 與優雅關機
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	redisDB := f.NewRedisDB()
	logger.MainLog.Info("Redis DB initialized")

	// WaitGroup 用來等待所有 goroutine 完成
	var wg sync.WaitGroup

	go func() {
		sig := <-sigChan
		logger.MainLog.Warnf("Received signal: %v, initiating shutdown...", sig)
		cancel()
	}()

	// 啟動 Executor
	wg.Add(1)
	go func() {
		defer wg.Done()
		exec := f.NewTaskExecutor(redisDB)
		logger.MainLog.Info("Executor started, waiting for tasks...")
		if err := exec.Start(ctx); err != nil && err != context.Canceled {
			logger.MainLog.Infof("Executor failed: %v", err)
		}
		logger.MainLog.Info("Executor stopped")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		engine := gin.New()
		engine.Use(gin.Recovery())

		port := ":" + cfg.WebServer.Port
		logger.MainLog.Infof("Server running at http://localhost%s", port)

		// 使用 http.Server 便於優雅關閉
		server := &http.Server{
			Addr:    port,
			Handler: engine,
		}

		// 在 context cancel 時關閉 server
		go func() {
			<-ctx.Done()
			logger.MainLog.Info("Shutting down server...")
			server.Shutdown(context.Background())
		}()

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.MainLog.Infof("Server error: %v", err)
		}
		logger.MainLog.Info("Server stopped")
	}()

	wg.Wait()
	logger.MainLog.Info("Application shutdown complete")
}
