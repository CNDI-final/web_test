package models

// TaskParams 定義任務參數
type TaskParams struct {
	NF        string `json:"nf"`
	PRVersion string `json:"pr_version"`
}

// Task 定義從 Web Server 收到的任務
type Task struct {
	ID     string       `json:"id"`
	Params []TaskParams `json:"params"`
}

// TaskResult 定義回傳給 Web Server 的結果
type TaskResult struct {
	TaskID     string `json:"task_id"`
	Status     string `json:"status"` // "success" or "failed or running"
	Logs       string `json:"logs"`
	FailedTest string `json:"failed_test,omitempty"` // 修改：單個失敗測試名稱
	Timestamp  int64  `json:"timestamp"`
}

type GitHubTask struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

type Release struct {
	Name    string `json:"name"`
	TagName string `json:"tag_name"`
}

type WorkerResponse struct {
	Summary string        `json:"summary"`
	PRs     []PullRequest `json:"prs"`
}

type ProgressInfo struct {
	TaskID    int    `json:"task_id"`
	TaskName  string `json:"task_name"`
	Percent   int    `json:"percent"`
	Remaining int    `json:"remaining"`
}

type GitHubRequest struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type RunPRRequest struct {
	Params [][]string `json:"params"`
}

type HistoryRecord struct {
	Time     string `json:"time"`
	TaskName string `json:"task_name"`
	Result   string `json:"result"`
}

type HistoryRecordsPerPage struct {
	Records    []HistoryRecord 	`json:"records"`
	CurrentPage int             `json:"current_page"`
	TotalPages int             	`json:"total_pages"`
	RecordsCount int            `json:"records_count"`
}
