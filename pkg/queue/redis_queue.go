package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"web_test/pkg/models" // 假設 module name 為 my-task-server

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// RedisQueue 是 TaskQueue 的具體實作
type RedisQueue struct {
	client    *redis.Client
	taskKey   string
	resultKey string
	logger    *logrus.Entry
}

func NewRedisQueue(addr, taskKey string) *RedisQueue {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &RedisQueue{
		client:    rdb,
		taskKey:   taskKey,
		resultKey: "task_result",
		logger:    logrus.WithField("component", "RedisQueue"),
	}
}

// PushTask 推送任務到佇列
func (q *RedisQueue) PushTask(ctx context.Context, task *models.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		q.logger.Errorf("Failed to marshal task: %v", err)
		return err
	}

	// 使用 RPush 將任務推到佇列尾部
	if err := q.client.RPush(ctx, q.taskKey, data).Err(); err != nil {
		q.logger.Errorf("Failed to push task to queue: %v", err)
		return err
	}

	q.logger.Infof("Task %s pushed to queue", task.ID)
	return nil
}

// PopTask 從佇列取出任務
func (q *RedisQueue) PopTask(ctx context.Context) (*models.Task, error) {
	res, err := q.client.BLPop(ctx, 0, q.taskKey).Result()
	if err != nil {
		return nil, err
	}

	var task models.Task
	if err := json.Unmarshal([]byte(res[1]), &task); err != nil {
		q.logger.Errorf("Failed to unmarshal task: %v", err)
		return nil, err
	}

	q.logger.Infof("Task %s popped from queue", task.ID)
	return &task, nil
}

// SaveResult 儲存任務結果
func (q *RedisQueue) SaveResult(ctx context.Context, result *models.TaskResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		q.logger.Errorf("Failed to marshal result: %v", err)
		return err
	}

	suffix := result.FailedTest
	if suffix == "" {
		suffix = "success"
	}
	key := fmt.Sprintf("%s:%s:%s", q.resultKey, result.TaskID, suffix)

	// 設定 1 小時過期
	if err := q.client.Set(ctx, key, data, time.Hour).Err(); err != nil {
		q.logger.Errorf("Failed to save result: %v", err)
		return err
	}

	q.logger.Infof("Result for task %s (%s) saved", result.TaskID, suffix)
	return nil
}

// GetResults 取得任務的所有結果（支援多個）
func (q *RedisQueue) GetResults(ctx context.Context, taskID string) ([]*models.TaskResult, error) {
	pattern := fmt.Sprintf("%s:%s:*", q.resultKey, taskID)
	keys, err := q.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	var results []*models.TaskResult
	for _, key := range keys {
		data, err := q.client.Get(ctx, key).Result()
		if err != nil {
			q.logger.Warnf("Failed to get data for key %s: %v", key, err)
			continue
		}

		var result models.TaskResult
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			q.logger.Warnf("Failed to unmarshal result for key %s: %v", key, err)
			continue
		}

		results = append(results, &result)
	}

	return results, nil
}

// DeleteResult 刪除任務指定狀態的結果
func (q *RedisQueue) DeleteResult(ctx context.Context, taskID string, status string) error {
	// 建構要刪除的 key
	key := fmt.Sprintf("%s:%s:%s", q.resultKey, taskID, status)

	// 刪除指定的 key
	if err := q.client.Del(ctx, key).Err(); err != nil {
		q.logger.Errorf("Failed to delete result for task %s status %s: %v", taskID, status, err)
		return err
	}

	q.logger.Infof("Deleted result key for task %s status %s", taskID, status)
	return nil
}

// GetQueueLength 取得佇列長度
func (q *RedisQueue) GetQueueLength(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, q.taskKey).Result()
}

// GetTasks 取得佇列中的所有任務（不移除）
func (q *RedisQueue) GetTasks(ctx context.Context) ([]*models.Task, error) {
	data, err := q.client.LRange(ctx, q.taskKey, 0, -1).Result()
	if err != nil {
		q.logger.Errorf("Failed to get tasks from queue: %v", err)
		return nil, err
	}

	var tasks []*models.Task
	for _, item := range data {
		var task models.Task
		if err := json.Unmarshal([]byte(item), &task); err != nil {
			q.logger.Warnf("Failed to unmarshal task: %v", err)
			continue
		}
		tasks = append(tasks, &task)
	}

	q.logger.Infof("Retrieved %d tasks from queue", len(tasks))
	return tasks, nil
}

// RemoveTask 刪除指定的任務
func (q *RedisQueue) RemoveTask(ctx context.Context, taskID string) error {
	// 獲取隊列中的所有任務
	data, err := q.client.LRange(ctx, q.taskKey, 0, -1).Result()
	if err != nil {
		q.logger.Errorf("Failed to get tasks from queue for removal: %v", err)
		return err
	}

	// 找到匹配的任務並移除
	for _, item := range data {
		var task models.Task
		if err := json.Unmarshal([]byte(item), &task); err != nil {
			q.logger.Warnf("Failed to unmarshal task during removal: %v", err)
			continue
		}

		if task.ID == taskID {
			// 使用 LREM 移除第一個匹配的元素
			if err := q.client.LRem(ctx, q.taskKey, 1, item).Err(); err != nil {
				q.logger.Errorf("Failed to remove task %s from queue: %v", taskID, err)
				return err
			}
			q.logger.Infof("Task %s removed from queue", taskID)
			return nil
		}
	}

	q.logger.Warnf("Task %s not found in queue", taskID)
	return fmt.Errorf("task %s not found in queue", taskID)
}

// GetRunningTasks 取得所有正在運行的任務
func (q *RedisQueue) GetRunningTasks(ctx context.Context) ([]*models.TaskResult, error) {
	pattern := fmt.Sprintf("%s:*:running", q.resultKey)
	keys, err := q.client.Keys(ctx, pattern).Result()
	if err != nil {
		q.logger.Errorf("Failed to get running task keys: %v", err)
		return nil, err
	}

	var runningTasks []*models.TaskResult
	for _, key := range keys {
		data, err := q.client.Get(ctx, key).Result()
		if err != nil {
			q.logger.Warnf("Failed to get data for key %s: %v", key, err)
			continue
		}

		var result models.TaskResult
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			q.logger.Warnf("Failed to unmarshal running task for key %s: %v", key, err)
			continue
		}

		runningTasks = append(runningTasks, &result)
	}

	q.logger.Infof("Retrieved %d running tasks", len(runningTasks))
	return runningTasks, nil
}
