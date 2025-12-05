package factory

import (
	"fmt"
	"os"

	"web_test/pkg/executor"
	"web_test/pkg/queue"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Factory 負責依賴注入的容器
type Factory struct {
	cfg *Config
}

// ReadConfig 讀取 YAML 設定檔
func ReadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 設定預設值
	if cfg.Redis.TaskQueueKey == "" {
		cfg.Redis.TaskQueueKey = "task_queue"
	}
	if cfg.Redis.ResultKeyPrefix == "" {
		cfg.Redis.ResultKeyPrefix = "task_result"
	}
	if cfg.Redis.DB == 0 {
		cfg.Redis.DB = 0
	}
	if cfg.Executor.TaskTimeout == "" {
		cfg.Executor.TaskTimeout = "300s"
	}
	if cfg.Executor.RetryDelay == "" {
		cfg.Executor.RetryDelay = "1s"
	}

	cfg.Print()
	return cfg, nil
}

// InitConfigFactory 負責底層讀檔與解析
func InitConfigFactory(path string, cfg *Config) error {

	content, err := os.ReadFile(path)
	if err != nil {
		return errors.Errorf("[Factory] ReadFile error: %+v", err)
	}

	// 注意：這裡用 yaml.Unmarshal，所以 Struct Tag 必須是 `yaml:"..."`
	if err := yaml.Unmarshal(content, cfg); err != nil {
		return errors.Errorf("[Factory] Unmarshal error: %+v", err)
	}

	return nil
}

func NewFactory(cfg *Config) *Factory {
	return &Factory{
		cfg: cfg,
	}
}

func (f *Factory) NewTaskExecutor() *executor.TaskExecutor {
	// 1. 初始化 ListQueue（記憶體佇列）
	var taskQueue queue.TaskQueue = queue.NewQueue()

	// 2. 初始化 Executor
	exec := executor.NewTaskExecutor(taskQueue)

	return exec
}
