package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"web_test/internal/logger"
	"web_test/pkg/database"
	"web_test/pkg/models"
	"web_test/pkg/queue"
)

type TaskExecutor struct {
	queue queue.TaskQueue
	db    database.ResultStore
}

func NewTaskExecutor(db database.ResultStore, q queue.TaskQueue) *TaskExecutor {
	return &TaskExecutor{
		db:    db,
		queue: q,
	}
}

// Start 啟動 executor,持續處理任務
func (e *TaskExecutor) Start(ctx context.Context) error {
	logger.ExecutorLog.Info("Executor started, waiting for tasks...")

	for {
		select {
		case <-ctx.Done():
			logger.ExecutorLog.Info("Executor stopped")
			return ctx.Err()
		default:
			if err := e.processNextTask(ctx); err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return err
				}
				logger.ExecutorLog.Errorf("Error processing task: %v", err)
				time.Sleep(time.Second)
			}
		}
	}
}

func (e *TaskExecutor) processNextTask(ctx context.Context) error {
	// 從佇列取出任務 (會阻塞等待)
	task, err := e.queue.PopTask(ctx)
	if err != nil {
		return err
	}

	// 建構日誌訊息
	var paramStrs []string
	for _, p := range task.Params {
		paramStrs = append(paramStrs, fmt.Sprintf("%s:%s", p.NF, p.PRVersion))
	}
	logger.ExecutorLog.Infof("Processing task %s with params: [%s]", task.ID, strings.Join(paramStrs, ", "))

	// 標記任務為執行中狀態
	runningResult := &models.TaskResult{
		TaskID:    task.ID,
		Status:    "running",
		Timestamp: time.Now().Unix(),
	}

	if err := e.db.SaveResult(ctx, runningResult); err != nil {
		logger.ExecutorLog.Errorf("Failed to save running status for task %s: %v", task.ID, err)
	}

	// 執行任務，獲取多個結果
	e.executeTask(ctx, task)

	defer func() {
		if err := e.db.DeleteResult(context.Background(), task.ID, "running"); err != nil {
			logger.ExecutorLog.Errorf("Failed to delete running status for task %s: %v", task.ID, err)
		} else {
			logger.ExecutorLog.Infof("Task %s completed, running status deleted", task.ID)
		}
	}()

	return nil
}

func (e *TaskExecutor) executeTask(ctx context.Context, task *models.Task) {
	IsSuccess := e.cmdrun(ctx, task)

	if IsSuccess {
		result := &models.TaskResult{
			TaskID:    task.ID,
			Status:    "Success",
			Timestamp: time.Now().Unix(),
		}
		e.db.SaveResult(ctx, result)
	} else {
		e.handleFailedTests(ctx, task)
	}

}

func (e *TaskExecutor) cmdrun(ctx context.Context, task *models.Task) bool {
	// 建構命令參數
	args := []string{"-n"}
	for _, param := range task.Params {
		args = append(args, "-p", fmt.Sprintf("%s:%s", param.NF, param.PRVersion))
	}

	wd, _ := os.Getwd()
	scriptPath := filepath.Clean(filepath.Join(wd, "run_task.sh"))

	// 執行 run_task.sh，傳遞多個 -p 參數
	cmd := exec.CommandContext(ctx, "sudo", append([]string{scriptPath}, args...)...)

	var logBuffer bytes.Buffer

	multiWriter := io.MultiWriter(os.Stdout, &logBuffer)

	cmd.Stdout = multiWriter
	cmd.Stderr = multiWriter

	err := cmd.Run()

	// 如果命令失敗，返回錯誤
	if err != nil {
		return false
	}

	return true
}

func (e *TaskExecutor) handleFailedTests(ctx context.Context, task *models.Task) {
	wd, _ := os.Getwd()
	failuresPath := filepath.Clean(filepath.Join(wd, "logs", "failures.json"))

	logger.ExecutorLog.Infof("Reading failures from: %s", failuresPath)

	// 讀取 failures.json
	data, err := os.ReadFile(failuresPath)
	if err != nil {
		logger.ExecutorLog.Errorf("Failed to read failures.json: %v", err)
		// 如果找不到 failures.json，存儲通用失敗結果
		result := &models.TaskResult{
			TaskID:      task.ID,
			Status:      "Failed",
			Logs:        []string{"Task execution failed, but failures.json not found"},
			FailedTests: []string{"JsonNotFound"},
			Timestamp:   time.Now().Unix(),
		}
		e.db.SaveResult(ctx, result)
		return
	}

	// 解析 JSON
	var failureData struct {
		FailedTests []string `json:"failed_tests"`
	}
	if err := json.Unmarshal(data, &failureData); err != nil {
		logger.ExecutorLog.Errorf("Failed to parse failures.json: %v", err)
		return
	}

	logger.ExecutorLog.Infof("Found %d failed tests", len(failureData.FailedTests))

	var allLogs []string
	var failedTestNames []string

	// 為每個失敗的測試讀取 log 檔案並存儲
	logsDir := filepath.Clean(filepath.Join(wd, "logs"))
	for _, testLogFile := range failureData.FailedTests {
		logFilePath := filepath.Join(logsDir, testLogFile)

		logger.ExecutorLog.Infof("Reading log file: %s", logFilePath)

		// 讀取 log 檔案內容
		logContent, err := os.ReadFile(logFilePath)
		if err != nil {
			logger.ExecutorLog.Errorf("Failed to read log file %s: %v", logFilePath, err)
			allLogs = append(allLogs, fmt.Sprintf("Failed to read log file %s: %v", logFilePath, err))
			continue
		}

		// 取出測試名稱（移除 .log 副檔名）
		testName := strings.TrimSuffix(testLogFile, ".log")
		failedTestNames = append(failedTestNames, testName)

		allLogs = append(allLogs, string(logContent))

		logger.ExecutorLog.Infof("Successfully read log file for failed test: %s", testName)
	}

	result := &models.TaskResult{
		TaskID:      task.ID,
		Status:      "Failed",
		Logs:        allLogs,
		FailedTests: failedTestNames,
		Timestamp:   time.Now().Unix(),
	}

	if err := e.db.SaveResult(ctx, result); err != nil {
		logger.ExecutorLog.Errorf("Failed to save result for task %s: %v", task.ID, err)
		return
	}

	logger.ExecutorLog.Infof("Successfully saved result for task %s with %d failed tests", task.ID, len(failedTestNames))

}
