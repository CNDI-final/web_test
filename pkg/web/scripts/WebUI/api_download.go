package WebUI

import (
	"context" // Add context import for Redis operations
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
	// TODO
	// remove when getresult finished
	// Assume task results are stored in Redis under "task_result:<taskID>"
	fileContent, err := DB.Get(ctx, fmt.Sprintf("task_result:%d", taskID)).Result()
	if err != nil {
		if err == go_redis.Nil { // go_redis.Nil means key does not exist
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Task result for ID %d not found", taskID)})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve task result"})
		}
		return
	}
	/*--------------delete above when getresult finished-----------------*/
	// TODO
	// put getresult there
	// fileContent, err := get_result(taskID)
	// if err != nil {
	//     c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve task result"})
	//     return
	// }
	fileName := fmt.Sprintf("task_%d_result.txt", taskID)

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", "text/plain")
	c.Header("Content-Length", fmt.Sprintf("%d", len(fileContent)))

	// Write the file content to the response body
	c.String(http.StatusOK, fileContent)
}