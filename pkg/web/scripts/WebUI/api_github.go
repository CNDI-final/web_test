package WebUI

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"web_test/pkg/models"
	"web_test/pkg/web/scripts/github"
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
		resp, err := github.FetchGitHubInfo(req.Owner, req.Repo)
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
	// TODO
	// remove when add task finished
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
	taskName := fmt.Sprintf("PR #%d [%s] - Action: %s", req.PRNumber, req.PRTitle, req.Action)
	task := models.InternalMessage{
		TaskID:   taskID, // Assign the generated unique TaskID
		TaskName: taskName,
		Params:   req.Params, // 轉發參數
	}
	taskJSON, _ := json.Marshal(task)
	DB.RPush(ctx, "task_queue", taskJSON)
	/*--------------delete above when add task finished-----------------*/
	// TODO
	// put add task there
	// taskID, err := GenerateUniqueTaskID() // Use the new unique ID generator
	// if err != nil {
	// 		c.JSON(500, gin.H{"error": "failed to generate task ID"})
	//		return
	//	}
	// add_task(taskID,req.Params)
	c.JSON(200, gin.H{"reply": "任務已加入佇列，參數已傳送。"})
}
