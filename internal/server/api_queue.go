package server

import (
	"context"
	"encoding/json"
	"fmt"      // Add fmt for Sprintf
	"net/http" // Add http for status codes
	"strconv"

	"github.com/gin-gonic/gin"
	"web_test/internal/logger" // Import logger
	"web_test/pkg/models"
)

// 1. 取得佇列
func GetQueueHandler(c *gin.Context) {
	if TaskQ == nil {
		c.JSON(500, gin.H{"error": "task queue is not initialized"})
		return
	}
	ctx := context.Background()

	var return_tasks []models.TaskResult
	running_tasks,err := DB.GetRunningTasks(ctx)
	if err != nil {
		logger.WebLog.Errorf("GetQueueHandler: Failed to get running tasks: %v", err)
		c.JSON(500, gin.H{"error": "failed to get running tasks"})
		return
	}
	for _, rt := range running_tasks {
		taskResult := models.TaskResult{
			TaskID:    rt.TaskID,
			Status:    "running",
		}
		return_tasks = append(return_tasks, taskResult)
	}

	tasks, err := TaskQ.GetTasks(ctx)
	if err != nil {
		logger.WebLog.Errorf("GetQueueHandler: Failed to get tasks from queue: %v", err)
		c.JSON(500, gin.H{"error": "failed to get tasks from queue"})
		return
	}
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
		taskResult := models.TaskResult{
			TaskID:    tmp.ID,
			Status:    "queueing", // Placeholder status
		}
		return_tasks = append(return_tasks, taskResult)
	}
	c.JSON(200, return_tasks)
}

// 2. 刪除佇列任務
func DeleteFromQueueHandler(c *gin.Context) {
	if TaskQ == nil {
		c.JSON(500, gin.H{"error": "task queue is not initialized"})
		return
	}
	ctx := context.Background()
	taskIDStr := c.Param("taskID") // Get taskID from path parameter
	targetID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		logger.WebLog.Errorf("DeleteFromQueueHandler: Invalid task ID received: %s, error: %v", taskIDStr, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	deleted := false
	if err := TaskQ.RemoveTask(ctx, taskIDStr); err == nil {
		deleted = true
	} else {
		logger.WebLog.Errorf("DeleteFromQueueHandler: Failed to remove task %d from queue: %v", targetID, err)
	}
	if deleted {
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Task ID %d not found in queue", targetID)})
	}
}
