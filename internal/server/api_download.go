package server

import (
	"context" // Add context import for Redis operations
	"encoding/json"
	"fmt"
	"net/http"
	"strconv" // Add strconv for string to int conversion

	"github.com/gin-gonic/gin"
	go_redis "github.com/redis/go-redis/v9" // Import go_redis for go_redis.Nil
)

// DownloadTextFileHandler serves a text file associated with a taskID for download.
func DownloadTextFileHandler(c *gin.Context) {
	taskIDStr := c.Param("taskID")
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	ctx := context.Background()
	fileContent, err := DB.GetResult(ctx, taskIDStr)
	if err != nil {
		if err == go_redis.Nil { // go_redis.Nil means key does not exist
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Task result for ID %d not found", taskID)})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve task result"})
		}
		return
	}

	if fileContent == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task result is empty"})
		return
	}

	jsonBytes, err := json.MarshalIndent(fileContent, "", "  ") // Indent 讓 JSON 排版好讀
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize task result"})
		return
	}

	downloadContent := string(jsonBytes)

	fileName := fmt.Sprintf("task_%d_result.json", taskID) // 副檔名改 json

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Content-Length", fmt.Sprintf("%d", len(downloadContent)))

	c.String(http.StatusOK, downloadContent)
}

// GetTaskResultHandler returns task result JSON for preview usage.
func GetTaskResultHandler(c *gin.Context) {
	taskID := c.Param("taskID")
	ctx := context.Background()
	result, err := DB.GetResult(ctx, taskID)
	if err != nil {
		if err == go_redis.Nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Task result for ID %s not found", taskID)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve task result"})
		return
	}
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Task result for ID %s not found", taskID)})
		return
	}
	c.JSON(http.StatusOK, result)
}
