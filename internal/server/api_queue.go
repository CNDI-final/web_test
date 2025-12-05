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
	"web_test/pkg/queue"
)

// 1. 取得佇列
func GetQueueHandler(c *gin.Context) {
	if queue.GlobalQueue == nil {
		c.JSON(500, gin.H{"error": "task queue is not initialized"})
		return
	}
	ctx := context.Background()
	var tasks []*models.Task
	var err error
	tasks, err := queue.GlobalQueue.GetTasks(ctx)
	if err != nil {
		logger.WebLog.Errorf("GetQueueHandler: Failed to get tasks from queue: %v", err)
		c.JSON(500, gin.H{"error": "failed to get tasks from queue"})
		return
	}
	var return_tasks []models.InternalMessage
	for _, task := range tasks {
		var t models.InternalMessage
		var tmp models.Task
		if err := json.Unmarshal([]byte(task), &tmp); err != nil {
			logger.WebLog.Warnf("GetQueueHandler: Failed to unmarshal task from queue: %v", err)
			continue
		}
		params := make(map[string]string)
		for _, p := range task.Params {
			params[p.NF] = p.PRVersion
		}
		t := models.InternalMessage{
			TaskID: strconv.Atoi(tmp.ID),
			Params: params,
		}
		return_tasks = append(return_tasks, t)
	}
	c.JSON(200, return_tasks)
}

// 2. 刪除佇列任務
func DeleteFromQueueHandler(c *gin.Context) {
	if queue.GlobalQueue == nil {
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

	allTasks, err := DB.LRange(ctx, "task_queue", 0, -1).Result()
	if err != nil {
		logger.WebLog.Errorf("DeleteFromQueueHandler: Failed to retrieve task queue from Redis: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve task queue"})
		return
	}

	deleted := false
	if err := queue.GlobalQueue.RemoveTask(ctx, taskIDStr); err == nil {
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
