package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"web_test/internal/logger"
	"web_test/pkg/database"
	"web_test/pkg/queue"

	"github.com/gin-gonic/gin"
)

type WebServer struct {
	port      string
	engine    *gin.Engine
	server    *http.Server
	redisDB   *database.RedisDB
	taskQueue queue.TaskQueue
}

func NewWebServer(port string, redisDB *database.RedisDB, taskQueue queue.TaskQueue) *WebServer {
	engine := gin.New()
	engine.Use(gin.Recovery())

	ws := &WebServer{
		port:      port,
		engine:    engine,
		redisDB:   redisDB,
		taskQueue: taskQueue,
		server: &http.Server{
			Addr:    ":" + port,
			Handler: engine,
		},
	}

	// 設定全局 DB（供 handler 使用）
	DB = redisDB

	// 註冊路由
	ws.setupRoutes()

	return ws
}

func (ws *WebServer) setupRoutes() {
	// 健康檢查
	ws.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 使用 AddService 註冊所有 API 路由
	AddService(ws.engine, ws.redisDB)
}

// Run 啟動伺服器（使用 sync.WaitGroup）
func (ws *WebServer) Run(wg *sync.WaitGroup) error {
	wg.Add(1)
	go ws.startServer(wg)
	logger.MainLog.Infof("Web server started on port %s", ws.port)
	return nil
}

func (ws *WebServer) startServer(wg *sync.WaitGroup) {
	defer wg.Done()

	if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.MainLog.Errorf("Web server error: %v", err)
	}
	logger.MainLog.Infof("Web server (listen on %s) stopped", ws.server.Addr)
}

// Start 啟動伺服器（使用 context）
func (ws *WebServer) Start(ctx context.Context) error {
	logger.MainLog.Infof("Server running at http://localhost:%s", ws.port)

	// 監聽 context 取消信號
	go func() {
		<-ctx.Done()
		logger.MainLog.Info("Shutting down server...")
		ws.Stop()
	}()

	if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// Stop 優雅關閉伺服器
func (ws *WebServer) Stop() {
	const defaultShutdownTimeout time.Duration = 2 * time.Second

	toCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	if err := ws.server.Shutdown(toCtx); err != nil {
		logger.MainLog.Errorf("Could not close web server: %v", err)
	}
}
