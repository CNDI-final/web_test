package WebUI

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	go_redis "github.com/redis/go-redis/v9"
)

// DB is the injected Redis client used by handlers.
var DB *go_redis.Client

// Route is the information for every URI.
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc gin.HandlerFunc
}

type Routes []Route

// AddService registers all the API routes into the provided gin engine.
// It accepts a Redis client to inject into the WebUI package.
func AddService(engine *gin.Engine, rdb *go_redis.Client) *gin.RouterGroup {
	// set package-level DB for handlers
	DB = rdb

	// Attach middleware to engine
	engine.Use(GinLogger())

	group := engine.Group("/api")
	// register routes directly with gin handlers
	group.GET("/queue/list", GetQueueHandler)
	group.DELETE("/queue/delete/:taskID", DeleteFromQueueHandler)
	group.GET("/history", HistoryHandler)
	group.POST("/queue/add_github", AddGitHubTaskHandler)
	group.GET("/prs", GetCachedPRsHandler)
	group.POST("/run-pr", RunPRTaskHandler)
	group.GET("/running", GetRunningTasksHandler)
	group.GET("/download/:taskID", DownloadTextFileHandler) // Route for downloading a text file with a taskID

	// serve static assets under a non-conflicting prefix
	engine.Static("/static", "./frontend")
	engine.Static("/js", "./frontend/js")
	// For SPA frontends, fallback to index.html for non-API routes using NoRoute.
	// This avoids registering a catch-all wildcard route which conflicts with /api.
	engine.NoRoute(func(c *gin.Context) {
		// if the path begins with /api, return 404 JSON to keep API semantics
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		// Serve SPA entrypoint
		c.File("./frontend/index.html")
	})

	return group
}
