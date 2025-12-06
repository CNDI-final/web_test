package executor

import (
	"context"
	"fmt"
	"os/exec"
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
	logger.ExecutorLog.Infof("Processing task %s", task.Params)
	for _, p := range task.Params {
		paramStrs = append(paramStrs, fmt.Sprintf("%s:%s", p.NF, p.PRVersion))
	}
	logger.ExecutorLog.Infof("Processing task %s with params: [%s]", task.ID, strings.Join(paramStrs, ", "))

	// 標記任務為執行中狀態
	runningResult := &models.TaskResult{
		TaskID:    task.ID,
		Status:    "running",
		Logs:      fmt.Sprintf("Task started processing at %s", time.Now().Format(time.RFC3339)),
		Timestamp: time.Now().Unix(),
	}
	if err := e.db.SaveResult(ctx, runningResult); err != nil {
		logger.ExecutorLog.Errorf("Failed to save running status for task %s: %v", task.ID, err)
	}

	// 執行任務，獲取多個結果
	results := e.executeTask(ctx, task)

	// 儲存每個結果到 Redis
	for _, result := range results {
		if err := e.db.SaveResult(ctx, result); err != nil {
			logger.ExecutorLog.Errorf("Failed to save result for task %s: %v", task.ID, err)
			return err
		}
	}

	// 刪除 running 狀態記錄，因為任務已經完成
	if err := e.db.DeleteResult(ctx, task.ID, "running"); err != nil {
		logger.ExecutorLog.Warnf("Failed to delete running status for task %s: %v", task.ID, err)
		// 不返回錯誤，因為任務結果已經儲存
	}

	logger.ExecutorLog.Infof("Task %s completed with %d results", task.ID, len(results))
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
	// 建構命令參數
	args := []string{"-n"}
	for _, param := range task.Params {
		args = append(args, "-p", fmt.Sprintf("%s:%s", param.NF, param.PRVersion))
		logger.ExecutorLog.Infof("Adding param: NF=%s, PRVersion=%s", param.NF, param.PRVersion)
	}

	// 執行 run_task.sh，傳遞多個 -p 參數
	cmd := exec.CommandContext(ctx, "./run_task.sh", args...)
	cmd.Dir = "/home/rs/web_test" // 設定工作目錄

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
