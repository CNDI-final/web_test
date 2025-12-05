package queue

import (
	"context"
	"errors"
	"sync"

	"web_test/pkg/models"
)

type ListQueue struct {
	tasks    []*models.Task
	mu       sync.RWMutex
	notEmpty chan struct{} // 用於通知有新任務
}

func NewQueue() *ListQueue {
	if GlobalQueue == nil {
		GlobalQueue = &ListQueue{
			tasks:    make([]*models.Task, 0),
			notEmpty: make(chan struct{}, 1), // 使用緩衝 channel 避免阻塞
		}
	}
	return GlobalQueue.(*ListQueue)
}

func (q *ListQueue) PushTask(ctx context.Context, task *models.Task) error {
	if task == nil || task.ID == "" {
		return errors.New("invalid task")
	}
	q.mu.Lock()
	q.tasks = append(q.tasks, task)
	q.mu.Unlock()

	// 通知有新任務（非阻塞）
	select {
	case q.notEmpty <- struct{}{}:
	default:
	}

	return nil
}

func (q *ListQueue) PopTask(ctx context.Context) (*models.Task, error) {
	for {
		q.mu.Lock()
		if len(q.tasks) > 0 {
			task := q.tasks[0]
			q.tasks = q.tasks[1:]
			q.mu.Unlock()
			return task, nil
		}
		q.mu.Unlock()

		// 阻塞等待新任務或 context 取消
		select {
		case <-q.notEmpty:
			// 有新任務，繼續循環檢查
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (q *ListQueue) GetTasks(ctx context.Context) ([]*models.Task, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]*models.Task, len(q.tasks))
	copy(out, q.tasks)
	return out, nil
}

func (q *ListQueue) RemoveTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return errors.New("taskID is empty")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, t := range q.tasks {
		if t.ID == taskID {
			q.tasks = append(q.tasks[:i], q.tasks[i+1:]...)
			return nil
		}
	}
	return errors.New("task not found")
}
