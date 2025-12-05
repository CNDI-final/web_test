package factory

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
)

type Config struct {
	App       AppConfig      `yaml:"app" valid:"required"`
	Redis     RedisConfig    `yaml:"redis" valid:"required"`
	WebServer WebServer      `yaml:"webserver" valid:"required"`
	Executor  ExecutorConfig `yaml:"executor"`
}

type AppConfig struct {
	LogLevel string `yaml:"log_level" valid:"required"`
}

type RedisConfig struct {
	Addr            string `yaml:"addr" valid:"required"`
	TaskQueueKey    string `yaml:"task_queue_key"`
	ResultKeyPrefix string `yaml:"result_key_prefix"`
	Password        string `yaml:"password"`
	DB              int    `yaml:"db"`
}

type WebServer struct {
	Port string `yaml:"port" valid:"required"`
}

type ExecutorConfig struct {
	TaskTimeout string `yaml:"task_timeout"`
	RetryDelay  string `yaml:"retry_delay"`
}

// Print 輸出載入的設定
func (c *Config) Print() {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		logrus.Error("Failed to marshal config")
		return
	}
	logrus.Infof("Loaded Configuration:\n%s", string(b))
}
