package database

import (
	"context"
	"encoding/json"
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
		Addr:     addr,
		Password: password, // no password set
		DB:       db,       // use default DB
	})
	return &RedisDB{client: rdb}
}

const (
	taskResultsHashKey = "task_results"
	runningTasksSetKey = "running_tasks"
)

// SaveResult saves a task result to Redis.
func (r *RedisDB) SaveResult(ctx context.Context, result *models.TaskResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	if err := r.client.HSet(ctx, taskResultsHashKey, result.TaskID, data).Err(); err != nil {
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

// GetHistory retrieves all historical task results from Redis.
func (r *RedisDB) GetHistory(ctx context.Context) ([]*models.TaskResult, error) {
	resultsMap, err := r.client.HGetAll(ctx, taskResultsHashKey).Result()
	if err != nil {
		return nil, err
	}

	var results []*models.TaskResult
	for _, data := range resultsMap {
		var result models.TaskResult
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			// Log the error for this specific entry and continue with others
			continue
		}
		results = append(results, &result)
	}
	return results, nil
}
