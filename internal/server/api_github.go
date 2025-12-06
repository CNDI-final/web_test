package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	"web_test/internal/logger"
	"web_test/pkg/models"
)

// 3. 加入 GitHub 抓取任務
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

// 5. 執行 PR 任務 (加入佇列)
func RunPRTaskHandler(c *gin.Context) {
	if TaskQ == nil {
		c.JSON(500, gin.H{"error": "task queue is not initialized"})
		return
	}
	ctx := context.Background()
	var req models.RunPRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WebLog.Errorf("Failed to bind JSON: %v", err)
		c.JSON(400, gin.H{"error": "invalid payload"})
		return
	}
	logger.WebLog.Infof("Received request: %+v", req)
	logger.WebLog.Infof("Params slice length: %d", len(req.Params))
	logger.WebLog.Infof("Params content: %+v", req.Params)
	if len(req.Params) == 0 {
		c.JSON(400, gin.H{"error": "params cannot be empty"})
		return
	}

	taskID, err := GenerateUniqueTaskID() // Use the new unique ID generator
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate task ID"})
		return
	}
	var params []models.TaskParams
	for _, pair := range req.Params {
		if len(pair) < 2 {
			continue 
		}
		nf := string(pair[0])
		prVersion := string(pair[1])
		logger.WebLog.Infof("Processing NF: %s, PRVersion: %s", nf, prVersion)
		params = append(params, models.TaskParams{
			NF:        nf,
			PRVersion: prVersion,
		})
	}
	if len(params) == 0 {
		c.JSON(400, gin.H{"error": "no valid params provided"})
		return
	}
	task := models.Task{
		ID:     fmt.Sprintf("%d", taskID), // Assign the generated unique TaskID
		Params: params,                    // 轉發參數
	}
	logger.WebLog.Infof("Enqueuing PR task %s with %d params", task.ID, len(params))
	if err := TaskQ.PushTask(ctx, &task); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to enqueue task: %v", err)})
		return
	}
	c.JSON(200, gin.H{"reply": "任務已加入佇列，參數已傳送。"})
}
