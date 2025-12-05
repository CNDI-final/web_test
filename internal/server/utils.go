package server

import (
	"context"
	"fmt"
	"web_test/internal/logger"
)

// GenerateUniqueTaskID generates a unique task ID using a Redis counter.
// It increments a counter in Redis and returns the new value.
func GenerateUniqueTaskID() (int, error) {
	ctx := context.Background()
	// Increment the task ID counter in Redis
	taskID, err := DB.IncrementTaskID(ctx)
	if err != nil {
		logger.WebLog.Errorf("failed to generate unique task ID from Redis: %v", err) // Log the error
		return 0, fmt.Errorf("failed to generate unique task ID from Redis: %w", err)
	}
	return int(taskID), nil
}
