package server

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"

	"web_test/pkg/models"
	"web_test/internal/logger"
)

// 7. 歷史紀錄
func HistoryHandler(c *gin.Context) {
	ctx := context.Background()

	val, err := DB.GetHistory(ctx, 0, 100)
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
	c.JSON(200, records)
}
