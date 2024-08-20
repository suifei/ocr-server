package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/suifei/ocr-server/internal/config"
	"github.com/suifei/ocr-server/internal/server"
	"github.com/suifei/ocr-server/internal/utils"
)

var (
	version     = "1.0.0" // 版本信息
	showVersion = flag.Bool("version", false, "显示版本信息")
	
	// 新增命令行参数
	addr             = flag.String("addr", "", "服务器地址")
	port             = flag.Int("port", 0, "服务器端口")
	ocrExePath       = flag.String("ocr-exe", "", "OCR可执行文件路径")
	minProcessors    = flag.Int("min-processors", 0, "最小处理器数量")
	maxProcessors    = flag.Int("max-processors", 0, "最大处理器数量")
	queueSize        = flag.Int("queue-size", 0, "队列大小")
	scaleThreshold   = flag.Int64("scale-threshold", 0, "扩展阈值")
	degradeThreshold = flag.Int64("degrade-threshold", 0, "降级阈值")
	idleTimeout      = flag.Duration("idle-timeout", 0, "空闲超时时间")
	warmUpCount      = flag.Int("warm-up-count", 0, "预热数量")
	shutdownTimeout  = flag.Duration("shutdown-timeout", 0, "关闭超时时间")
	logFilePath      = flag.String("log-file", "", "日志文件路径")
	logMaxSize       = flag.Int("log-max-size", 0, "最大日志文件大小（MB）")
	logMaxBackups    = flag.Int("log-max-backups", 0, "最大日志文件备份数")
	logMaxAge        = flag.Int("log-max-age", 0, "最大日志文件保留天数")
	logCompress      = flag.Bool("log-compress", false, "是否压缩日志文件")
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			utils.LogError("发生严重错误: %v\n%s", r, debug.Stack())
			os.Exit(1)
		}
	}()

	flag.Parse()

	if *showVersion {
		fmt.Printf("OCR Server 版本: %s\n", version)
		os.Exit(0)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		utils.LogError("加载配置失败: %v", err)
		os.Exit(1)
	}

	// 应用命令行参数覆盖配置文件
	applyCommandLineArgs(&cfg)

	if err := config.ValidateConfig(&cfg); err != nil {
		utils.LogError("配置验证失败: %v", err)
		os.Exit(1)
	}

	utils.SetupLogger(cfg)

	utils.LogInfo("启动 OCR 服务器 (版本 %s)...", version)

	srv, err := server.NewServer(cfg)
	if err != nil {
		utils.LogError("创建服务器失败: %v", err)
		os.Exit(1)
	}

	if err := srv.Initialize(); err != nil {
		utils.LogError("初始化服务器失败: %v", err)
		os.Exit(1)
	}

	srv.Start()
}

func applyCommandLineArgs(cfg *config.Config) {
	if *addr != "" {
		cfg.Addr = *addr
	}
	if *port != 0 {
		cfg.Port = *port
	}
	if *ocrExePath != "" {
		cfg.OCRExePath = *ocrExePath
	}
	if *minProcessors != 0 {
		cfg.MinProcessors = *minProcessors
	}
	if *maxProcessors != 0 {
		cfg.MaxProcessors = *maxProcessors
	}
	if *queueSize != 0 {
		cfg.QueueSize = *queueSize
	}
	if *scaleThreshold != 0 {
		cfg.ScaleThreshold = *scaleThreshold
	}
	if *degradeThreshold != 0 {
		cfg.DegradeThreshold = *degradeThreshold
	}
	if *idleTimeout != 0 {
		cfg.IdleTimeout = *idleTimeout
	}
	if *warmUpCount != 0 {
		cfg.WarmUpCount = *warmUpCount
	}
	if *shutdownTimeout != 0 {
		cfg.ShutdownTimeout = *shutdownTimeout
	}
	if *logFilePath != "" {
		cfg.LogFilePath = *logFilePath
	}
	if *logMaxSize != 0 {
		cfg.LogMaxSize = *logMaxSize
	}
	if *logMaxBackups != 0 {
		cfg.LogMaxBackups = *logMaxBackups
	}
	if *logMaxAge != 0 {
		cfg.LogMaxAge = *logMaxAge
	}
	cfg.LogCompress = *logCompress
}