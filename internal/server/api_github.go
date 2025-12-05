package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"web_test/pkg/models"
	"web_test/pkg/web/scripts/github"
	"web_test/pkg/queue"
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
			// 存歷史
			recBytes, _ := json.Marshal(models.HistoryRecord{
				Time:     time.Now().Format("15:04:05"),
				TaskName: "GitHub Fetch",
				Result:   resp.Summary,
			})
			DB.LPush(ctx, "task_history", recBytes)
			// 存快取
			prsJSON, _ := json.Marshal(resp.PRs)
			DB.Set(ctx, "cached_prs", prsJSON, 0)
		}
	}() // 立即執行抓取任務

	c.JSON(200, gin.H{"status": "queued"})
}

// 4. 取得 PR 快取
func GetCachedPRsHandler(c *gin.Context) {
	ctx := context.Background()
	val, err := DB.Get(ctx, "cached_prs").Result()
	if err != nil {
		c.JSON(200, []interface{}{})
		return
	}
	// return raw cached JSON
	var raw interface{}
	json.Unmarshal([]byte(val), &raw)
	c.JSON(200, raw)
}

// 5. 執行 PR 任務 (加入佇列)
func RunPRTaskHandler(c *gin.Context) {
	if queue.GlobalQueue == nil {
		c.JSON(500, gin.H{"error": "task queue is not initialized"})
		return
	}
	ctx := context.Background()
	var req models.RunPRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid payload"})
		return
	}

	taskID, err := GenerateUniqueTaskID() // Use the new unique ID generator
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate task ID"})
		return
	}
	var params []models.TaskParams
	for nf, PRNumber := range req.Params {
		params = append(params, models.TaskParams{
			NF:   nf,
			PRVersion: PRNumber,
		})
	}
	task := models.Task{
		ID:   fmt.Sprintf("%d",taskID), // Assign the generated unique TaskID
		Params:   params, // 轉發參數
	}
	taskJSON, _ := json.Marshal(task)
	if err := queue.GlobalQueue.PushTask(ctx, &task); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to enqueue task: %v", err)})
		return
	}
	c.JSON(200, gin.H{"reply": "任務已加入佇列，參數已傳送。"})
}
