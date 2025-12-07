package server

import (
	"context"
	"encoding/json"
	"fmt"      // Add fmt for Sprintf
	"net/http" // Add http for status codes
	"strconv"

	"web_test/internal/logger" // Import logger
	"web_test/pkg/models"

	"github.com/gin-gonic/gin"
)

func QueueRoute() []Route {
    return []Route{
		{
			Name:    "get queue",
			Method:  http.MethodGet,
			Pattern: "/list",
			HandlerFunc: GetQueueHandler,
		},
		{
			Name:    "remove from queue",
			Method:  http.MethodDelete,
			Pattern: "/delete/:taskID",
			HandlerFunc: DeleteFromQueueHandler,
		},
        {
			Name:    "run PR task",
			Method:  http.MethodPost,
			Pattern: "/run-pr",
			HandlerFunc: RunPRTaskHandler,
		},
	}
}

// 1. 取得佇列
func GetQueueHandler(c *gin.Context) {
	if TaskQ == nil {
		c.JSON(500, gin.H{"error": "task queue is not initialized"})
		return
	}
	ctx := context.Background()

	var return_tasks []models.TaskResult
	running_tasks, err := DB.GetRunningTasks(ctx)
	if err != nil {
		logger.WebLog.Errorf("GetQueueHandler: Failed to get running tasks: %v", err)
		c.JSON(500, gin.H{"error": "failed to get running tasks"})
		return
	}
	for _, rt := range running_tasks {
		taskResult := models.TaskResult{
			TaskID: rt.TaskID,
			Status: "running",
			Params: rt.Params,
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
			TaskID: tmp.ID,
			Status: "queueing", // Placeholder status
			Params: tmp.Params,
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
