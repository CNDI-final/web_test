package executor

import (
	"context"
	"testing"
	"time"

	"web_test/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ResultStore 定義結果儲存介面
type ResultStore interface {
	SaveResult(ctx context.Context, result *models.TaskResult) error
	GetResult(ctx context.Context, taskID string) (*models.TaskResult, error)
	DeleteResult(ctx context.Context, taskID string, status string) error
}

// MockTaskQueue 模擬任務佇列
type MockTaskQueue struct {
	mock.Mock
}

func (m *MockTaskQueue) PushTask(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskQueue) PopTask(ctx context.Context) (*models.Task, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskQueue) GetTasks(ctx context.Context) ([]*models.Task, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskQueue) RemoveTask(ctx context.Context, taskID string) error {
	args := m.Called(ctx, taskID)
	return args.Error(0)
}

// MockRedisDB 模擬 Redis 數據庫（實現 ResultStore 介面）
type MockRedisDB struct {
	mock.Mock
	SavedResults map[string]*models.TaskResult
}

func NewMockRedisDB() *MockRedisDB {
	return &MockRedisDB{
		SavedResults: make(map[string]*models.TaskResult),
	}
}

func (m *MockRedisDB) SaveResult(ctx context.Context, result *models.TaskResult) error {
	args := m.Called(ctx, result)
	if args.Error(0) == nil {
		m.SavedResults[result.TaskID] = result
	}
	return args.Error(0)
}

func (m *MockRedisDB) GetResult(ctx context.Context, taskID string) (*models.TaskResult, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskResult), args.Error(1)
}

func (m *MockRedisDB) DeleteResult(ctx context.Context, taskID string, status string) error {
	args := m.Called(ctx, taskID, status)
	return args.Error(0)
}

// TestProcessTaskSuccess 測試成功處理任務
func TestProcessTaskSuccess(t *testing.T) {
	// 準備
	mockQueue := new(MockTaskQueue)
	mockDB := NewMockRedisDB()

	task := &models.Task{
		ID: "task-123",
		Params: []models.TaskParams{
			{NF: "amf", PRVersion: "70"},
			{NF: "smf", PRVersion: "80"},
		},
	}

	// 設置 mock 期望
	mockQueue.On("PopTask", mock.Anything).Return(task, nil)
	mockDB.On("SaveResult", mock.Anything, mock.MatchedBy(func(r *models.TaskResult) bool {
		return r.TaskID == task.ID && r.Status == "running"
	})).Return(nil)
	mockDB.On("SaveResult", mock.Anything, mock.MatchedBy(func(r *models.TaskResult) bool {
		return r.TaskID == task.ID && r.Status == "success"
	})).Return(nil)
	mockDB.On("DeleteResult", mock.Anything, task.ID, "running").Return(nil)

	// 創建 executor（直接使用 mock，因為已實現介面）
	executor := &TaskExecutor{
		queue: mockQueue,
		db:    mockDB,
	}

	// 執行
	ctx := context.Background()
	err := executor.processNextTask(ctx)

	// 驗證
	assert.NoError(t, err)
	assert.NotNil(t, mockDB.SavedResults[task.ID])
	assert.Equal(t, "success", mockDB.SavedResults[task.ID].Status)
	mockQueue.AssertExpectations(t)
	mockDB.AssertExpectations(t)
}

// TestProcessTaskWithFailedTests 測試包含失敗測試的任務
func TestProcessTaskWithFailedTests(t *testing.T) {
	// 準備
	mockQueue := new(MockTaskQueue)
	mockDB := NewMockRedisDB()

	task := &models.Task{
		ID: "task-456",
		Params: []models.TaskParams{
			{NF: "amf", PRVersion: "70"},
		},
	}

	// 模擬腳本輸出（包含失敗測試）
	failedOutput := `
TestAMF started
Running test cases...
FAIL: TestAMF_Case1
Error details here
TestAMF completed
`

	// 設置 mock 期望
	mockQueue.On("PopTask", mock.Anything).Return(task, nil)
	mockDB.On("SaveResult", mock.Anything, mock.MatchedBy(func(r *models.TaskResult) bool {
		return r.TaskID == task.ID && r.Status == "running"
	})).Return(nil)
	mockDB.On("SaveResult", mock.Anything, mock.MatchedBy(func(r *models.TaskResult) bool {
		return r.TaskID == task.ID && r.Status == "failed"
	})).Return(nil)
	mockDB.On("DeleteResult", mock.Anything, task.ID, "running").Return(nil)

	// 測試 parseFailedTestLogs 函數
	failedLogs := parseFailedTestLogs(failedOutput)

	// 驗證
	assert.NotEmpty(t, failedLogs, "應該解析到失敗的測試")
}

// TestExecuteTaskSuccess 測試 executeTask 成功情況
func TestExecuteTaskSuccess(t *testing.T) {
	mockDB := NewMockRedisDB()
	mockQueue := new(MockTaskQueue)

	executor := &TaskExecutor{
		queue: mockQueue,
		db:    mockDB,
	}

	task := &models.Task{
		ID: "task-789",
		Params: []models.TaskParams{
			{NF: "upf", PRVersion: "65"},
		},
	}

	// 執行
	results := executor.executeTask(context.Background(), task)

	// 驗證
	assert.NotEmpty(t, results)
	assert.Equal(t, task.ID, results[0].TaskID)
}

// TestParseFailedTestLogs 測試解析失敗測試日誌
func TestParseFailedTestLogs(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected map[string]string
	}{
		{
			name: "單個失敗測試",
			output: `TestAMF started
Running test case 1...
FAIL: TestAMF
Error: connection timeout
TestAMF completed`,
			expected: map[string]string{
				"TestAMF": "包含失敗標記",
			},
		},
		{
			name: "無失敗測試",
			output: `TestAMF started
Running test case 1...
PASS: TestAMF
TestAMF completed`,
			expected: map[string]string{},
		},
		{
			name: "多個測試",
			output: `TestAMF started
FAIL: TestAMF
Error here
TestSMF started
PASS: TestSMF
Done`,
			expected: map[string]string{
				"TestAMF": "包含失敗標記",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFailedTestLogs(tt.output)
			if len(tt.expected) == 0 {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
				for key := range tt.expected {
					assert.Contains(t, result, key)
				}
			}
		})
	}
}

// TestProcessTaskContextCancellation 測試 context 取消
func TestProcessTaskContextCancellation(t *testing.T) {
	mockQueue := new(MockTaskQueue)
	mockDB := NewMockRedisDB()

	// 模擬 PopTask 被取消
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	mockQueue.On("PopTask", ctx).Return(nil, context.Canceled)

	executor := &TaskExecutor{
		queue: mockQueue,
		db:    mockDB,
	}

	// 執行
	err := executor.processNextTask(ctx)

	// 驗證
	assert.Equal(t, context.Canceled, err)
}

// TestIntegrationSuccessfulTaskProcessing 集成測試：成功處理整個任務流程
func TestIntegrationSuccessfulTaskProcessing(t *testing.T) {
	mockQueue := new(MockTaskQueue)
	mockDB := NewMockRedisDB()

	task := &models.Task{
		ID: "integration-task-001",
		Params: []models.TaskParams{
			{NF: "amf", PRVersion: "70"},
			{NF: "smf", PRVersion: "80"},
		},
	}

	// 期望順序：PopTask -> SaveResult (running) -> SaveResult (success) -> DeleteResult
	mockQueue.On("PopTask", mock.Anything).Return(task, nil).Once()
	mockDB.On("SaveResult", mock.Anything, mock.MatchedBy(func(r *models.TaskResult) bool {
		return r.Status == "running"
	})).Return(nil).Once()
	mockDB.On("SaveResult", mock.Anything, mock.MatchedBy(func(r *models.TaskResult) bool {
		return r.Status == "success"
	})).Return(nil).Once()
	mockDB.On("DeleteResult", mock.Anything, task.ID, "running").Return(nil).Once()

	executor := &TaskExecutor{
		queue: mockQueue,
		db:    mockDB,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 執行
	err := executor.processNextTask(ctx)

	// 驗證
	assert.NoError(t, err)
	assert.Equal(t, 1, len(mockDB.SavedResults))
	assert.Equal(t, task.ID, mockDB.SavedResults[task.ID].TaskID)

	mockQueue.AssertExpectations(t)
	mockDB.AssertExpectations(t)
}
