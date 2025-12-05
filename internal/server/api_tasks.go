package server

import (
	"context"
	"encoding/json"
	"strconv"
	"net/http"
	"fmt"

	"github.com/gin-gonic/gin"

	"web_test/pkg/models"
	"web_test/pkg/queue"
	"web_test/internal/logger"
)

// 6. 取得所有執行中任務 (進度)
func GetRunningTasksHandler(c *gin.Context) {
	ctx := context.Background()
	// TODO
	// delete when get_progress finish
	tasks, err := queue.GlobalQueue.GetTasks(ctx) 
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get running tasks"})
		return
	}

	var running []models.ProgressInfo
	for _, task := range tasks {
		taskBytes, err := json.Marshal(task)
		if err != nil {
			logger.WebLog.Warnf("序列化失敗: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode task"})
			return
		}
		var tmp models.Task
		if err := json.Unmarshal([]byte(taskBytes), &tmp); err != nil {
			logger.WebLog.Warnf("GetQueueHandler: Failed to unmarshal task from queue: %v", err)
			continue
		}
		var p models.ProgressInfo
		p.TaskID,_ = strconv.Atoi(tmp.ID)
		p.TaskName = fmt.Sprintf("task %d running",tmp.ID) // Placeholder status
		p.Percent = 0                     // Placeholder percent
		p.Remaining = 0                  // Placeholder remaining time
		running = append(running, p)
	}
	c.JSON(200, running)
}
