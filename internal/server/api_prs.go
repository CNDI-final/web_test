package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"web_test/pkg/models"
)

func PrsRoute() []Route {
    return []Route{
		{
			Name:    "add github PRs",
			Method:  http.MethodPost,
			Pattern: "/add_github",
			HandlerFunc: AddGitHubTaskHandler,
		},
		{
			Name:    "get cached PRs",
			Method:  http.MethodGet,
			Pattern: "/",
			HandlerFunc: GetCachedPRsHandler,
		},
        {
			Name:    "clear PR cache",
			Method:  http.MethodPost,
			Pattern: "/clear",
			HandlerFunc: ClearPRCacheHandler,
		},
	}
}

// 4. 取得 PR 快取
func GetCachedPRsHandler(c *gin.Context) {
	ctx := context.Background()
	val, err := DB.GetPrCache(ctx)
	if err != nil {
		c.JSON(200, []interface{}{})
		return
	}
	// return raw cached JSON
	var raw interface{}
	json.Unmarshal([]byte(val), &raw)
	c.JSON(200, raw)
}

// 4.1 清除 PR 快取
func ClearPRCacheHandler(c *gin.Context) {
	ctx := context.Background()
	if err := DB.ClearPrCache(ctx); err != nil {
		c.JSON(500, gin.H{"error": "failed to clear PR cache"})
		return
	}
	c.JSON(200, gin.H{"status": "cleared"})
}

func AddGitHubTaskHandler(c *gin.Context) {
	ctx := context.Background()
	var req models.GitHubRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid payload"})
		return
	}

	defer func() {
		resp, err := FetchGitHubInfo(req.Owner, req.Repo)
		if err == nil {
			// 存快取
			prsJSON, _ := json.Marshal(resp.PRs)
			DB.SavePrCache(ctx, prsJSON)
		}
	}() // 立即執行抓取任務

	c.JSON(200, gin.H{"status": "queued"})
}