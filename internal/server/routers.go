package server

import (
	"net/http"
	"strings"
	"web_test/pkg/database"

	"github.com/gin-gonic/gin"
)

// Route is the information for every URI.
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc gin.HandlerFunc
}

type Routes []Route

func applyRoutes(group *gin.RouterGroup, routes []Route) {
	for _, route := range routes {
		switch route.Method {
		case "GET":
			group.GET(route.Pattern, route.HandlerFunc)
		case "POST":
			group.POST(route.Pattern, route.HandlerFunc)
		case "PUT":
			group.PUT(route.Pattern, route.HandlerFunc)
		case "PATCH":
			group.PATCH(route.Pattern, route.HandlerFunc)
		case "DELETE":
			group.DELETE(route.Pattern, route.HandlerFunc)
		}
	}
}

// AddService registers all the API routes into the provided gin engine.
// It accepts a Redis client to inject into the WebUI package.
func AddService(engine *gin.Engine, rdb database.ResultStore) {
	// set package-level DB for handlers
	DB = rdb

	// Attach middleware to engine
	engine.Use(GinLogger())

	queueGroup := engine.Group("/api/queue")
	applyRoutes(queueGroup, QueueRoute())
	historyGroup := engine.Group("/api/history")
	applyRoutes(historyGroup, HistoryRoute())
	prsGroup := engine.Group("/api/prs")
	applyRoutes(prsGroup, PrsRoute())
	downloadGroup := engine.Group("/api/download")
	applyRoutes(downloadGroup, DownloadRoute())

	// serve static assets under a non-conflicting prefix
	engine.Static("/static", "./internal/server/public")
	engine.Static("/js", "./internal/server/public/js")
	// For SPA frontends, fallback to index.html for non-API routes using NoRoute.
	// This avoids registering a catch-all wildcard route which conflicts with /api.
	engine.NoRoute(func(c *gin.Context) {
		// if the path begins with /api, return 404 JSON to keep API semantics
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		// Serve SPA entrypoint
		c.File("./internal/server/public/index.html")
	})
}
