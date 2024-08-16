package config

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/suifei/ocr-server/internal/ocr"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Addr             string        `yaml:"addr"`
	Port             int           `yaml:"port"`
	OCRExePath       string        `yaml:"ocr_exe_path"`
	MinProcessors    int           `yaml:"min_processors"`
	MaxProcessors    int           `yaml:"max_processors"`
	QueueSize        int           `yaml:"queue_size"`
	ScaleThreshold   int64         `yaml:"scale_threshold"`
	DegradeThreshold int64         `yaml:"degrade_threshold"`
	IdleTimeout      time.Duration `yaml:"idle_timeout"`
	WarmUpCount      int           `yaml:"warm_up_count"`
	ShutdownTimeout  time.Duration `yaml:"shutdown_timeout"`
	LogFilePath      string        `yaml:"log_file_path"`
	LogMaxSize       int           `yaml:"log_max_size"`
	LogMaxBackups    int           `yaml:"log_max_backups"`
	LogMaxAge        int           `yaml:"log_max_age"`
	LogCompress      bool          `yaml:"log_compress"`
}

func LoadConfig() (Config, error) {
	var configFile string
	var showHelp bool

	// Define default configuration
	cfg := Config{
		Addr:             "localhost",
		Port:             1111,
		OCRExePath:       ocr.GetOCREnginePath(),
		MinProcessors:    4,
		MaxProcessors:    runtime.NumCPU(),
		QueueSize:        100,
		ScaleThreshold:   75,
		DegradeThreshold: 25,
		IdleTimeout:      5 * time.Minute,
		WarmUpCount:      2,
		ShutdownTimeout:  30 * time.Second,
		LogFilePath:      "ocr_server.log",
		LogMaxSize:       100,
		LogMaxBackups:    3,
		LogMaxAge:        28,
		LogCompress:      true,
	}

	// Define flags
	flag.StringVar(&configFile, "config", "", "Path to configuration file (optional)")
	flag.BoolVar(&showHelp, "help", false, "Show help message")

	// Parse flags
	flag.Parse()

	// Show help if requested
	if showHelp {
		printUsage()
		os.Exit(0)
	}

	// Load config file if specified
	if configFile != "" {
		if err := loadConfigFile(&cfg, configFile); err != nil {
			return cfg, fmt.Errorf("error loading config file: %w", err)
		}
	}

	// Override with command line flags
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			cfg.Addr = f.Value.String()
		case "port":
			cfg.Port, _ = f.Value.(flag.Getter).Get().(int)
		case "ocr":
			cfg.OCRExePath = f.Value.String()
		case "min-processors":
			cfg.MinProcessors, _ = f.Value.(flag.Getter).Get().(int)
		case "max-processors":
			cfg.MaxProcessors, _ = f.Value.(flag.Getter).Get().(int)
		case "queue-size":
			cfg.QueueSize, _ = f.Value.(flag.Getter).Get().(int)
		case "scale-threshold":
			cfg.ScaleThreshold, _ = f.Value.(flag.Getter).Get().(int64)
		case "degrade-threshold":
			cfg.DegradeThreshold, _ = f.Value.(flag.Getter).Get().(int64)
		case "idle-timeout":
			cfg.IdleTimeout, _ = f.Value.(flag.Getter).Get().(time.Duration)
		case "warm-up-count":
			cfg.WarmUpCount, _ = f.Value.(flag.Getter).Get().(int)
		case "shutdown-timeout":
			cfg.ShutdownTimeout, _ = f.Value.(flag.Getter).Get().(time.Duration)
		case "log-file":
			cfg.LogFilePath = f.Value.String()
		case "log-max-size":
			cfg.LogMaxSize, _ = f.Value.(flag.Getter).Get().(int)
		case "log-max-backups":
			cfg.LogMaxBackups, _ = f.Value.(flag.Getter).Get().(int)
		case "log-max-age":
			cfg.LogMaxAge, _ = f.Value.(flag.Getter).Get().(int)
		case "log-compress":
			cfg.LogCompress, _ = f.Value.(flag.Getter).Get().(bool)
		}
	})
	// Ensure OCR engine is installed
	if !ocr.IsOCREngineInstalled() {
		ocrPath, err := ocr.EnsureOCREngine()
		if err != nil {
			return cfg, fmt.Errorf("failed to ensure OCR engine: %w", err)
		}
		cfg.OCRExePath = ocrPath
	}
	// Apply constraints
	cfg.MinProcessors = int(math.Max(float64(cfg.MinProcessors), 4))
	cfg.MaxProcessors = int(math.Max(float64(cfg.MaxProcessors), float64(cfg.MinProcessors)))

	return cfg, nil
}

func loadConfigFile(cfg *Config, filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return fmt.Errorf("error parsing config file: %w", err)
	}

	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage of OCR Server:\n")
	fmt.Fprintf(os.Stderr, "  -config string\n")
	fmt.Fprintf(os.Stderr, "        Path to configuration file (optional)\n")
	fmt.Fprintf(os.Stderr, "  -help\n")
	fmt.Fprintf(os.Stderr, "        Show this help message\n\n")
	fmt.Fprintf(os.Stderr, "Other flags (override config file settings):\n")
	flag.PrintDefaults()
}

func init() {
	// Define all flags here, but don't set default values
	flag.String("addr", "", "Address to run the server on")
	flag.Int("port", 0, "Port to run the server on")
	flag.String("ocr", "", "Path to the OCR executable")
	flag.Int("min-processors", 0, "Minimum number of OCR processors")
	flag.Int("max-processors", 0, "Maximum number of OCR processors")
	flag.Int("queue-size", 0, "Size of task queue")
	flag.Int64("scale-threshold", 0, "Threshold to scale up processors (tasks per processor)")
	flag.Int64("degrade-threshold", 0, "Threshold to scale down processors (tasks per processor)")
	flag.Duration("idle-timeout", 0, "Idle time before scaling down a processor")
	flag.Int("warm-up-count", 0, "Number of processors to warm up")
	flag.Duration("shutdown-timeout", 0, "Timeout for graceful shutdown")
	flag.String("log-file", "", "Path to log file")
	flag.Int("log-max-size", 0, "Maximum size of log file before rotation (MB)")
	flag.Int("log-max-backups", 0, "Maximum number of old log files to retain")
	flag.Int("log-max-age", 0, "Maximum number of days to retain old log files")
	flag.Bool("log-compress", false, "Compress rotated log files")
}
