package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"web_test/pkg/models"
	"web_test/pkg/queue"

	"github.com/sirupsen/logrus"
)

type TaskExecutor struct {
	queue  queue.TaskQueue
	logger *logrus.Entry
}

func NewTaskExecutor(q queue.TaskQueue) *TaskExecutor {
	return &TaskExecutor{
		queue:  q,
		logger: logrus.WithField("component", "TaskExecutor"),
	}
}

// Start 啟動 executor,持續處理任務
func (e *TaskExecutor) Start(ctx context.Context) error {
	e.logger.Info("Executor started, waiting for tasks...")

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("Executor stopped")
			return ctx.Err()
		default:
			if err := e.processNextTask(ctx); err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return err
				}
				e.logger.Errorf("Error processing task: %v", err)
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

	e.logger.Infof("Processing task %s (NF: %s, PRVersion: %s)",
		task.ID, task.Params.NF, task.Params.PRVersion)

	// 標記任務為執行中狀態
	runningResult := &models.TaskResult{
		TaskID:    task.ID,
		Status:    "running",
		Logs:      fmt.Sprintf("Task started processing at %s", time.Now().Format(time.RFC3339)),
		Timestamp: time.Now().Unix(),
	}
	if err := e.queue.SaveResult(ctx, runningResult); err != nil {
		e.logger.Errorf("Failed to save running status for task %s: %v", task.ID, err)
		// 不返回錯誤，繼續執行任務
	}

	// 執行任務，獲取多個結果
	results := e.executeTask(ctx, task)

	// 儲存每個結果到 Redis
	for _, result := range results {
		if err := e.queue.SaveResult(ctx, result); err != nil {
			e.logger.Errorf("Failed to save result for task %s: %v", task.ID, err)
			return err
		}
	}

	// 刪除 running 狀態記錄，因為任務已經完成
	if err := e.queue.DeleteResult(ctx, task.ID, "running"); err != nil {
		e.logger.Warnf("Failed to delete running status for task %s: %v", task.ID, err)
		// 不返回錯誤，因為任務結果已經儲存
	}

	e.logger.Infof("Task %s completed with %d results", task.ID, len(results))
	return nil
}

func (e *TaskExecutor) executeTask(ctx context.Context, task *models.Task) []*models.TaskResult {
	logs, failedTestLogs, err := e.doActualWork(ctx, task)

	var results []*models.TaskResult

	if err != nil || len(failedTestLogs) > 0 {
		// 失敗情況：為每個失敗測試創建一個 result
		for failedTest, testLogs := range failedTestLogs {
			result := &models.TaskResult{
				TaskID:     task.ID,
				Status:     "failed",
				Logs:       testLogs, // 特定測試的 logs
				FailedTest: failedTest,
				Timestamp:  time.Now().Unix(),
			}
			results = append(results, result)
		}
		if err != nil && len(failedTestLogs) == 0 {
			// 如果有錯誤但沒有解析到失敗測試，創建一個通用失敗 result
			result := &models.TaskResult{
				TaskID:    task.ID,
				Status:    "failed",
				Logs:      fmt.Sprintf("Task failed: %v\nLogs: %s", err, logs),
				Timestamp: time.Now().Unix(),
			}
			results = append(results, result)
		}
	} else {
		// 成功情況：創建一個成功 result
		result := &models.TaskResult{
			TaskID:    task.ID,
			Status:    "success",
			Logs:      logs,
			Timestamp: time.Now().Unix(),
		}
		results = append(results, result)
	}

	return results
}

func (e *TaskExecutor) doActualWork(ctx context.Context, task *models.Task) (string, map[string]string, error) {
	e.logger.Infof("Starting work for NF: %s, PRVersion: %s", task.Params.NF, task.Params.PRVersion)

	// 執行 run_task.sh，傳遞參數
	cmd := exec.CommandContext(ctx, "./run_task.sh", "-n", "-p", fmt.Sprintf("%s:%s", task.Params.NF, task.Params.PRVersion))
	cmd.Dir = "/home/rs/test" // 設定工作目錄

	// 執行命令並捕獲輸出
	output, err := cmd.CombinedOutput()
	logs := string(output)

	// 解析失敗測試及其對應 logs
	failedTestLogs := parseFailedTestLogs(logs)

	// 如果命令失敗，返回錯誤
	if err != nil {
		return logs, failedTestLogs, fmt.Errorf("script execution failed: %v", err)
	}

	return logs, failedTestLogs, nil
}

// parseFailedTestLogs 從腳本輸出中解析失敗測試及其對應 logs
func parseFailedTestLogs(output string) map[string]string {
	lines := strings.Split(output, "\n")
	failedLogs := make(map[string]string)
	var currentTest string
	var testLogs []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Test") && len(trimmed) > 4 { // 簡單匹配測試開始，如 "TestUPF"
			// 處理前一個測試
			if currentTest != "" && len(testLogs) > 0 {
				// 檢查是否失敗
				for _, l := range testLogs {
					if strings.Contains(l, "FAIL: "+currentTest) {
						failedLogs[currentTest] = strings.Join(testLogs, "\n")
						break
					}
				}
			}
			// 開始新測試
			currentTest = trimmed
			testLogs = []string{line}
		} else {
			if currentTest != "" {
				testLogs = append(testLogs, line)
			}
		}
	}

	// 處理最後一個測試
	if currentTest != "" && len(testLogs) > 0 {
		for _, l := range testLogs {
			if strings.Contains(l, "FAIL: "+currentTest) {
				failedLogs[currentTest] = strings.Join(testLogs, "\n")
				break
			}
		}
	}

	return failedLogs
}
