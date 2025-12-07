package server

import (
	"archive/zip" // 引入 zip 壓縮包處理庫
	"bytes"      // 引入 bytes 處理記憶體緩衝區
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	go_redis "github.com/redis/go-redis/v9"
)

func DownloadRoute() []Route {
    return []Route{
		{
			Name:    "download all logs",
			Method:  http.MethodGet,
			Pattern: "/:taskID",
			HandlerFunc: DownloadAllLogHandler,
		},
		{
			Name:    "download single log",
			Method:  http.MethodGet,
			Pattern: "/single/:taskID/:failedTest",
			HandlerFunc: DownloadSingleLogHandler,
		},
        {
			Name:    "get task result",
			Method:  http.MethodGet,
			Pattern: "/task/:taskID",
			HandlerFunc: GetTaskResultHandler,
		},
	}
}

func DownloadAllLogHandler(c *gin.Context) {
	taskIDStr := c.Param("taskID")
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	ctx := context.Background()
	taskResult, err := DB.GetResult(ctx, taskIDStr) 
	if err != nil {
		if err == go_redis.Nil { // go_redis.Nil means key does not exist
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Task result for ID %d not found", taskID)})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve task result"})
		}
		return
	}
    
    // 檢查指標是否為 nil
    if taskResult == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task result is empty"})
		return
	}

	// 2. 建立記憶體緩衝區來儲存 ZIP 內容
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 3. 將 Logs 陣列中的每個 Log 內容寫入 ZIP 檔案
    // 注意：taskResult 是指標，我們需要使用 taskResult.Logs 存取欄位
	for i, logContent := range taskResult.Logs {
		fileName := fmt.Sprintf("log_%d.txt", i+1) // 預設檔案名稱
        
        if len(taskResult.FailedTests) > i && taskResult.FailedTests[i] != "" {
            // 使用輔助函式替換不適合檔名的字元
            cleanedTestName := replaceBadChars(taskResult.FailedTests[i]) 
            fileName = fmt.Sprintf("%s.log", cleanedTestName) 
        }

		header := &zip.FileHeader{
			Name:   fileName,
			Method: zip.Deflate,
		}

		fileWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create file header in ZIP: %v", err)})
			return
		}

		_, err = fileWriter.Write([]byte(logContent))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to write log content to ZIP: %v", err)})
			return
		}
	}

    // 額外寫入一個 JSON 結果檔 (Metadata)
    jsonFileName := fmt.Sprintf("task_%d_metadata.json", taskID)
    jsonHeader := &zip.FileHeader{
        Name: jsonFileName,
        Method: zip.Deflate,
    }
    jsonWriter, err := zipWriter.CreateHeader(jsonHeader)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create JSON file header in ZIP"})
        return
    }
    // Marshal taskResult (解除指標引用後使用)
    jsonBytesWithIndent, _ := json.MarshalIndent(*taskResult, "", "  ")
    jsonWriter.Write(jsonBytesWithIndent)


	// 4. 關閉 zipWriter
	err = zipWriter.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to close ZIP writer: %v", err)})
		return
	}
    
    // 5. 設定 HTTP Header 並傳送 ZIP 檔案
	zipFileName := fmt.Sprintf("task_%d_logs.zip", taskID) 

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", zipFileName))
	c.Header("Content-Type", "application/zip") 
	c.Header("Content-Length", fmt.Sprintf("%d", buf.Len()))

	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

func DownloadSingleLogHandler(c *gin.Context) {
	// 1. 獲取參數
    taskIDStr := c.Param("taskID")
    // 注意：這裡需跟你的 Router 定義一致，例如 router.GET("/download/:taskID/:failedTest", ...)
    targetTestName := c.Param("failedTest") 

    taskID, err := strconv.Atoi(taskIDStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
        return
    }
    if targetTestName == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid failed test name"})
        return
    }

    // 2. 從 Redis (或其他 DB) 獲取資料
    ctx := context.Background()
    taskResult, err := DB.GetResult(ctx, taskIDStr) 
    if err != nil {
        if err == go_redis.Nil {
            c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Task result for ID %d not found", taskID)})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve task result"})
        }
        return
    }
    
    if taskResult == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Task result is empty"})
        return
    }

    // 3. 搜尋對應的 Log 內容
    var logContent string
    found := false

    // 假設 FailedTests 和 Logs 是一一對應的 (Index 相同)
    for i, testName := range taskResult.FailedTests {
        // 安全檢查：避免 Logs 陣列比 FailedTests 短導致 panic
        if i >= len(taskResult.Logs) {
            break
        }

        // 比對名稱 (完全符合)
        if testName == targetTestName {
            logContent = taskResult.Logs[i]
            found = true
            break
        }
    }

    if !found {
        c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Log not found for test: %s", targetTestName)})
        return
    }

    // 4. 設定檔名與 Header
    // 使用輔助函式清理檔名 (移除 / \ : 等字元)
    cleanName := replaceBadChars(targetTestName)
    fileName := fmt.Sprintf("%s.log", cleanName)

    // 設定瀏覽器下載行為
    // "attachment" 會強迫瀏覽器跳出下載視窗，而不是直接在瀏覽器開啟
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
    c.Header("Content-Type", "text/plain; charset=utf-8")
    // 如果知道長度可以設，不知道也沒關係，Gin 會自動處理
    c.Header("Content-Length", fmt.Sprintf("%d", len(logContent)))

    // 5. 直接回傳字串內容
    c.String(http.StatusOK, logContent)
}

// 輔助函式，用於替換檔案名稱中不適合的字元
func replaceBadChars(s string) string {
    // 由於 strings 已引入，此處 code 運行正常
    s = strings.ReplaceAll(s, "/", "_")
    s = strings.ReplaceAll(s, "\\", "_")
    s = strings.ReplaceAll(s, ":", "-")
    s = strings.ReplaceAll(s, " ", "_")
    
    if len(s) > 100 {
        s = s[:100] 
    }
    return s
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
