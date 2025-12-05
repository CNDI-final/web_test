package WebUI

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"

	"web_test/pkg/models"
)

// 6. 取得所有執行中任務 (進度)
func GetRunningTasksHandler(c *gin.Context) {
	ctx := context.Background()
	// TODO
	// delete when get_progress finish
	keys, err := DB.Keys(ctx, "progress:*").Result()
	if err != nil {
		c.JSON(200, []models.ProgressInfo{})
		return
	}

	var running []models.ProgressInfo
	for _, key := range keys {
		val, _ := DB.Get(ctx, key).Result()
		var p models.ProgressInfo
		json.Unmarshal([]byte(val), &p)
		running = append(running, p)
	}
	c.JSON(200, running)
	/*--------------delete above when get_progress finish-----------------*/
	// TODO
	// put get_progress there
	// var running []models.ProgressInfo
	// running = get_progress()
	// c.JSON(200, running)
}
