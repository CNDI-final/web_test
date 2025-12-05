package server

import (
	"context"
	"encoding/json"
	"time"
	"fmt"

	"github.com/gin-gonic/gin"

	"web_test/pkg/models"
	"web_test/pkg/database"
)

// 7. 歷史紀錄
func HistoryHandler(c *gin.Context) {
	ctx := context.Background()

	var result []*models.TaskResult
	var err error
	val, _ := DB.GetHistory(ctx)

	var records []models.HistoryRecord

	for _, v := range val {
		var tmp models.TaskResult
		json.Unmarshal([]byte(v), &tmp)
		rec := models.HistoryRecord{
			Time:     tmp.Timestamp.Format("15:04:05"),
			TaskName: fmt.Sprintf("Task %s", tmp.TaskID),
			Result:   tmp.Status,
		}
		records = append(records, rec)
	}
	c.JSON(200, records)
}
