package queue

import (
	"context"
	"web_test/pkg/models"
)

var GlobalQueue TaskQueue

// TaskQueue 定義任務佇列介面
type TaskQueue interface {
	// 推送任務到佇列
	PushTask(ctx context.Context, task *models.Task) error
	// 從佇列取出任務
	PopTask(ctx context.Context) (*models.Task, error)
	// 取得佇列中的所有任務（不移除）
	GetTasks(ctx context.Context) ([]*models.Task, error)
	// 刪除指定的任務
	RemoveTask(ctx context.Context, taskID string) error
}
