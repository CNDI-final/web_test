package WebUI

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"

	"web_test/pkg/models"
)

// 7. 歷史紀錄
func HistoryHandler(c *gin.Context) {
	// TODO
	// remove when get_history finished
	ctx := context.Background()
	val, _ := DB.LRange(ctx, "task_history", 0, 99).Result()
	var records []models.HistoryRecord
	for _, v := range val {
		var rec models.HistoryRecord
		json.Unmarshal([]byte(v), &rec)
		records = append(records, rec)
	}
	c.JSON(200, records)
	/*--------------delete above when get_history finished-----------------*/
	// TODO
	// put get_history there
	// var records []models.HistoryRecord
	// records = get_history()
	// c.JSON(200, records)
}
