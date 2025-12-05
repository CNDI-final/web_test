package database

import (
	"context"
	"web_test/pkg/models"
)
// ResultStore 定義結果儲存介面
type ResultStore interface {
	// 儲存任務結果
	SaveResult(ctx context.Context, result *models.TaskResult) error
	// 取得任務結果
	GetResult(ctx context.Context, taskID string) (*models.TaskResult, error)
	// 取得所有正在運行的任務
	GetRunningTasks(ctx context.Context) ([]*models.TaskResult, error)
	// 刪除任務指定狀態的結果
	DeleteResult(ctx context.Context, taskID string, status string) error
	// 取得所有任務歷史紀錄
	GetHistory(ctx context.Context) ([]*models.TaskResult, error)
}
