package config

import (
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/viper"
	"github.com/go-playground/validator/v10"
	"github.com/suifei/ocr-server/internal/ocr"
)

type Config struct {
	Addr             string        `mapstructure:"addr" validate:"required"`
	Port             int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	OCRExePath       string        `mapstructure:"ocr_exe_path"`
	MinProcessors    int           `mapstructure:"min_processors" validate:"required,min=1"`
	MaxProcessors    int           `mapstructure:"max_processors" validate:"required,min=1"`
	QueueSize        int           `mapstructure:"queue_size" validate:"required,min=1"`
	ScaleThreshold   int64         `mapstructure:"scale_threshold" validate:"required,min=0"`
	DegradeThreshold int64         `mapstructure:"degrade_threshold" validate:"required,min=0"`
	IdleTimeout      time.Duration `mapstructure:"idle_timeout" validate:"required"`
	WarmUpCount      int           `mapstructure:"warm_up_count" validate:"required,min=0"`
	ShutdownTimeout  time.Duration `mapstructure:"shutdown_timeout" validate:"required"`
	LogFilePath      string        `mapstructure:"log_file_path" validate:"required"`
	LogMaxSize       int           `mapstructure:"log_max_size" validate:"required,min=1"`
	LogMaxBackups    int           `mapstructure:"log_max_backups" validate:"required,min=0"`
	LogMaxAge        int           `mapstructure:"log_max_age" validate:"required,min=1"`
	LogCompress      bool          `mapstructure:"log_compress"`
}

func LoadConfig() (Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/ocr-server/")
	viper.AddConfigPath("$HOME/.ocr-server")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, fmt.Errorf("读取配置文件错误: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("解析配置错误: %w", err)
	}

	// 设置默认值
	setDefaults(&cfg)

	// 验证配置
	if err := validateConfig(&cfg); err != nil {
		return Config{}, fmt.Errorf("配置验证错误: %w", err)
	}

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Addr == "" {
		cfg.Addr = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 1111
	}
	if cfg.OCRExePath == "" {
		cfg.OCRExePath = ocr.GetOCREnginePath()
	}
	if cfg.MinProcessors == 0 {
		cfg.MinProcessors = 4
	}
	if cfg.MaxProcessors == 0 {
		cfg.MaxProcessors = runtime.NumCPU()
	}
	if cfg.QueueSize == 0 {
		cfg.QueueSize = 100
	}
	if cfg.ScaleThreshold == 0 {
		cfg.ScaleThreshold = 75
	}
	if cfg.DegradeThreshold == 0 {
		cfg.DegradeThreshold = 25
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 5 * time.Minute
	}
	if cfg.WarmUpCount == 0 {
		cfg.WarmUpCount = 2
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	if cfg.LogFilePath == "" {
		cfg.LogFilePath = "ocr_server.log"
	}
	if cfg.LogMaxSize == 0 {
		cfg.LogMaxSize = 100
	}
	if cfg.LogMaxBackups == 0 {
		cfg.LogMaxBackups = 3
	}
	if cfg.LogMaxAge == 0 {
		cfg.LogMaxAge = 28
	}
}

func validateConfig(cfg *Config) error {
	validate := validator.New()
	return validate.Struct(cfg)
}