package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"web_test/pkg/models"

	"github.com/redis/go-redis/v9"
)

// RedisDB implements the ResultStore interface with a Redis backend.
type RedisDB struct {
	client *redis.Client
}

// NewRedisDB creates a new RedisDB instance.
// It takes the Redis server address (e.g., "localhost:6379") and password.
func NewRedisDB(addr, password string, db int) *RedisDB {
	rdb := redis.NewClient(&redis.Options{
		Addr:             addr,
		Password:         password, // no password set
		DB:               db,       // use default DB
		DisableIndentity: true,     // Disable RESP3 identity feature
		Protocol:         2,
	})
	return &RedisDB{client: rdb}
}

const (
	taskResultsHashKey = "task_results"
	runningTasksSetKey = "running_tasks"
	taskIDCounterKey   = "task_id_counter"
	historyListKey     = "task_history_list" // Use a list for history to maintain order
	prCacheKey         = "pr_cache"
)

var taipeiLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

// SaveResult saves a task result to Redis.
func (r *RedisDB) SaveResult(ctx context.Context, result *models.TaskResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	if err = r.client.HSet(ctx, taskResultsHashKey, result.TaskID, data).Err(); err != nil {
		return err
	}

	// If the task is running, add it to the running tasks set
	if result.Status == "running" {
		if err := r.client.SAdd(ctx, runningTasksSetKey, result.TaskID).Err(); err != nil {
			return err
		}
	} else {
		// If the task is completed or failed, remove it from the running tasks set
		if err := r.client.SRem(ctx, runningTasksSetKey, result.TaskID).Err(); err != nil {
			return err
		}
		r.SaveHistory(ctx, &models.HistoryRecord{
			Time:     time.Unix(result.Timestamp, 0).In(taipeiLocation).Format("2006-01-02 15:04:05"),
			Params:   result.Params,
			TaskName: fmt.Sprintf("Test Task %s", result.TaskID),
			Result:   result.Status,
		})
	}

	return nil
}

// GetResult retrieves a task result from Redis.
func (r *RedisDB) GetResult(ctx context.Context, taskID string) (*models.TaskResult, error) {
	data, err := r.client.HGet(ctx, taskResultsHashKey, taskID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Not found
		}
		return nil, err
	}

	var result models.TaskResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRunningTasks retrieves all running tasks from Redis.
func (r *RedisDB) GetRunningTasks(ctx context.Context) ([]*models.TaskResult, error) {
	taskIDs, err := r.client.SMembers(ctx, runningTasksSetKey).Result()
	if err != nil {
		return nil, err
	}

	var tasks []*models.TaskResult
	for _, taskID := range taskIDs {
		result, getErr := r.GetResult(ctx, taskID)
		if getErr != nil {
			// Log or handle the error, maybe the task result was deleted
			// but the running task entry was not cleaned up.
			continue
		}
		if result != nil {
			tasks = append(tasks, result)
		}
	}
	return tasks, nil
}

// DeleteResult deletes a task result from Redis.
func (r *RedisDB) DeleteResult(ctx context.Context, taskID string, status string) error {
	if status == "running" {
		return r.client.SRem(ctx, runningTasksSetKey, taskID).Err()
	} else {
		return r.client.HDel(ctx, taskResultsHashKey, taskID).Err()
	}
}

// SaveHistory saves a history record to Redis.
func (r *RedisDB) SaveHistory(ctx context.Context, record *models.HistoryRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	// Use RPush to add to the end of the list
	if err := r.client.LPush(ctx, historyListKey, data).Err(); err != nil {
		return err
	}
	return nil
}

// GetHistory retrieves all historical task records from Redis.
func (r *RedisDB) GetHistory(ctx context.Context, start, end int64) ([]*models.HistoryRecord, error) {
	// Use LRange to get all elements from the list
	data, err := r.client.LRange(ctx, historyListKey, start, end).Result()
	if err != nil {
		return nil, err
	}

	history := make([]*models.HistoryRecord, 0, len(data))
	for _, item := range data {
		var record models.HistoryRecord
		if err := json.Unmarshal([]byte(item), &record); err != nil {
			// Log the error for this specific entry and continue with others
			continue
		}
		history = append(history, &record)
	}
	return history, nil
}

// SavePrCache saves the PRs cache to Redis.
func (r *RedisDB) SavePrCache(ctx context.Context, Prs []byte) error {
	return r.client.Set(ctx, prCacheKey, Prs, 0).Err()
}

// GetPrCache retrieves the PRs cache from Redis.
func (r *RedisDB) GetPrCache(ctx context.Context) ([]byte, error) {
	data, err := r.client.Get(ctx, prCacheKey).Bytes()
	if err == redis.Nil {
		return nil, nil // Key does not exist
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ClearPrCache removes the cached PR data.
func (r *RedisDB) ClearPrCache(ctx context.Context) error {
	return r.client.Del(ctx, prCacheKey).Err()
}

func (r *RedisDB) IncrementTaskID(ctx context.Context) (int, error) {
	result, err := r.client.Incr(ctx, taskIDCounterKey).Result()
	if err != nil {
		return 0, err
	}
	return int(result), nil
}
