package WebUI

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
	// TODO
	// delete when get queue finished
	ctx := context.Background()
	val, err := DB.LRange(ctx, "task_queue", 0, -1).Result()
	if err != nil {
		logger.WebLog.Errorf("GetQueueHandler: Failed to retrieve task queue from Redis: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve task queue"})
		return
	}
	var tasks []models.InternalMessage
	for _, v := range val {
		var t models.InternalMessage
		if err := json.Unmarshal([]byte(v), &t); err != nil {
			logger.WebLog.Warnf("GetQueueHandler: Failed to unmarshal task from queue: %v", err)
			continue
		}
		tasks = append(tasks, t)
	}
	/*--------------delete above when delete task finish-----------------*/
	// TODO
	// put get queue there
	// var tasks []models.InternalMessage
	// tasks = get_queue()
	c.JSON(200, tasks)
}

// 2. 刪除佇列任務
func DeleteFromQueueHandler(c *gin.Context) {
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
	// TODO
	// delete when delete task finished
	for _, rawJSON := range allTasks {
		var t models.InternalMessage
		if err := json.Unmarshal([]byte(rawJSON), &t); err != nil {
			logger.MainLog.Warnf("DeleteFromQueueHandler: Failed to unmarshal task from queue: %v", err)
			continue // Skip malformed entries
		}
		if t.TaskID == targetID {
			_, err := DB.LRem(ctx, "task_queue", 1, rawJSON).Result()
			if err != nil {
				logger.MainLog.Errorf("DeleteFromQueueHandler: Failed to remove task %d from Redis: %v", targetID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task from queue"})
				return
			}
			deleted = true
			break
		}
	}
	/*--------------delete above when delete task finish-----------------*/
	// TODO
	// add new delete task there
	// delete = delete_task(targetID)
	if deleted {
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Task ID %d not found in queue", targetID)})
	}
}
