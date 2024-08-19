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
