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

	IncrementTaskID(ctx context.Context) (int, error)
	// 儲存歷史紀錄
	SaveHistory(ctx context.Context, record *models.HistoryRecord) error
	// 取得所有任務歷史紀錄
	GetHistory(ctx context.Context, start, end int64) ([]*models.HistoryRecord, error)
	// 儲存PR快取
	SavePrCache(ctx context.Context, Prs []byte) error
	// 取得PR快取
	GetPrCache(ctx context.Context) ([]byte, error)
	// 清除PR快取
	ClearPrCache(ctx context.Context) error
}
