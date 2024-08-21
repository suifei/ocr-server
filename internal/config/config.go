package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"github.com/suifei/ocr-server/internal/ocr"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Addr             string        `mapstructure:"addr" yaml:"addr" validate:"required"`
	Port             int           `mapstructure:"port" yaml:"port" validate:"required,min=1,max=65535"`
	OCRExePath       string        `mapstructure:"ocr_exe_path" yaml:"ocr_exe_path"`
	MinProcessors    int           `mapstructure:"min_processors" yaml:"min_processors" validate:"required,min=1"`
	MaxProcessors    int           `mapstructure:"max_processors" yaml:"max_processors" validate:"required,min=1"`
	QueueSize        int           `mapstructure:"queue_size" yaml:"queue_size" validate:"required,min=1"`
	ScaleThreshold   int64         `mapstructure:"scale_threshold" yaml:"scale_threshold" validate:"required,min=0"`
	DegradeThreshold int64         `mapstructure:"degrade_threshold" yaml:"degrade_threshold" validate:"required,min=0"`
	IdleTimeout      time.Duration `mapstructure:"idle_timeout" yaml:"idle_timeout" validate:"required"`
	WarmUpCount      int           `mapstructure:"warm_up_count" yaml:"warm_up_count" validate:"required,min=0"`
	ShutdownTimeout  time.Duration `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout" validate:"required"`
	LogFilePath      string        `mapstructure:"log_file_path" yaml:"log_file_path" validate:"required"`
	LogMaxSize       int           `mapstructure:"log_max_size" yaml:"log_max_size" validate:"required,min=1"`
	LogMaxBackups    int           `mapstructure:"log_max_backups" yaml:"log_max_backups" validate:"required,min=0"`
	LogMaxAge        int           `mapstructure:"log_max_age" yaml:"log_max_age" validate:"required,min=1"`
	LogCompress      bool          `mapstructure:"log_compress" yaml:"log_compress"`
	ThresholdMode    int           `mapstructure:"threshold_mode" yaml:"threshold_mode"`
	ThresholdValue   int           `mapstructure:"threshold_value" yaml:"threshold_value" validate:"required,min=0,max=255"`
}

func LoadConfig() (Config, error) {
	var cfg Config

	// 设置默认值
	setDefaults(&cfg)

	// 生成默认配置文件（如果不存在）
	if err := generateDefaultConfig(cfg); err != nil {
		return Config{}, fmt.Errorf("生成默认配置文件错误: %w", err)
	}

	// 读取配置文件
	if err := readConfigFile(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func setDefaults(cfg *Config) {
	cfg.Addr = "localhost"
	cfg.Port = 1111
	cfg.OCRExePath = ocr.GetOCREnginePath()
	cfg.MinProcessors = 4
	cfg.MaxProcessors = runtime.NumCPU()
	cfg.QueueSize = 100
	cfg.ScaleThreshold = 75
	cfg.DegradeThreshold = 25
	cfg.IdleTimeout = 5 * time.Minute
	cfg.WarmUpCount = 2
	cfg.ShutdownTimeout = 30 * time.Second
	cfg.LogFilePath = "ocr_server.log"
	cfg.LogMaxSize = 100
	cfg.LogMaxBackups = 3
	cfg.LogMaxAge = 28
	cfg.LogCompress = false
	cfg.ThresholdMode = 0
	cfg.ThresholdValue = 100
}

func generateDefaultConfig(cfg Config) error {
	configPath := getConfigFilePath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("序列化默认配置错误: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			return fmt.Errorf("创建配置目录错误: %w", err)
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("写入默认配置文件错误: %w", err)
		}
		fmt.Printf("已生成默认配置文件: %s\n", configPath)
	}
	return nil
}

func readConfigFile(cfg *Config) error {
	viper.SetConfigFile(getConfigFilePath())
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件错误: %w", err)
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("解析配置错误: %w", err)
	}

	return nil
}

func ValidateConfig(cfg *Config) error {
	validate := validator.New()
	return validate.Struct(cfg)
}

func getConfigFilePath() string {
	homeDir := "."
	return filepath.Join(homeDir, ".ocr-server", "config.yaml")
}
