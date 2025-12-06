package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"web_test/pkg/models"
	"web_test/internal/logger"
)

// 7. 歷史紀錄
func HistoryHandler(c *gin.Context) {
	ctx := context.Background()
	pageStr := c.Param("page") // Get page from path parameter
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		logger.WebLog.Errorf("HistoryHandler: Invalid page received: %s, error: %v", pageStr, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page"})
		return
	}
	var start, end int64
	start = int64(page * 100)
	end = int64(start + 99)
	val, err := DB.GetHistory(ctx, start, end)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to retrieve history"})
		return
	}

	var records []models.HistoryRecord

	for _, v := range val {
		dataBytes, err := json.Marshal(v)
		if err != nil {
			logger.WebLog.Warnf("HistoryRecord 序列化失敗: %v", err)
			// 根據您的邏輯決定要 return 還是 continue
			continue 
		}
		var rec models.HistoryRecord
		json.Unmarshal([]byte(dataBytes), &rec)
		records = append(records, rec)
	}
	var totalRecords int64
	totalRecords, err = DB.GetHistoryCount(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to retrieve history count"})
		return
	}
	totalPages := int((totalRecords + 99) / 100) // 計算總頁數，向上取整
	if page > totalPages {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page number out of range"})
		return
	}

	response := models.HistoryRecordsPerPage{
		Records:      records,
		CurrentPage:  page,
		TotalPages:   totalPages,
		RecordsCount: int(totalRecords),
	}

	c.JSON(200, response)
}
