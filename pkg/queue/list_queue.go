package queue

import (
	"context"
	"errors"
	"sync"

	"web_test/pkg/models"
)

type ListQueue struct {
	tasks []*models.Task
	mu    sync.RWMutex
}

func NewQueue() *ListQueue {
	if GlobalQueue == nil {
		GlobalQueue = &ListQueue{
			tasks: make([]*models.Task, 0),
		}
	}
	return GlobalQueue.(*ListQueue)
}

func (q *ListQueue) PushTask(ctx context.Context, task *models.Task) error {
	if task == nil || task.ID == "" {
		return errors.New("invalid task")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, task)
	return nil
}

func (q *ListQueue) PopTask(ctx context.Context) (*models.Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.tasks) == 0 {
		return nil, nil
	}
	task := q.tasks[0]
	q.tasks = q.tasks[1:]
	return task, nil
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
