package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"web_test/internal/logger"
	"web_test/pkg/factory"
)

func main() {
	configPath := flag.String("c", "config.yml", "path to config file")
	flag.Parse()

	cfg, err := factory.ReadConfig(*configPath)
	if err != nil {
		logger.MainLog.Fatalf("Failed to load config: %+v", err)
	}

	logger.MainLog.Infof("System initializing with Redis Addr: %s", cfg.Redis.Addr)

	f := factory.NewFactory(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 初始化依賴
	redisDB := f.NewRedisDB()
	taskQueue := f.NewTaskQueue()
	logger.MainLog.Info("Dependencies initialized")

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
		exec := f.NewTaskExecutor(redisDB, taskQueue)
		logger.MainLog.Info("Executor started")
		if err := exec.Start(ctx); err != nil && err != context.Canceled {
			logger.MainLog.Errorf("Executor error: %v", err)
		}
		logger.MainLog.Info("Executor stopped")
	}()

	// 啟動 Web Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		webServer := f.NewWebServer(redisDB, taskQueue)
		if err := webServer.Start(context.Background()); err != nil {
			logger.MainLog.Errorf("Server error: %v", err)
		}
	}()

	wg.Wait()
	logger.MainLog.Info("Application shutdown complete")
}
