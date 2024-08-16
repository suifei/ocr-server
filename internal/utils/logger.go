package utils

import (
	"io"
	"log"
	"github.com/suifei/ocr-server/internal/config"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

func SetupLogger(cfg config.Config) {
	log.SetOutput(io.MultiWriter(os.Stdout, &lumberjack.Logger{
		Filename:   cfg.LogFilePath,
		MaxSize:    cfg.LogMaxSize,
		MaxBackups: cfg.LogMaxBackups,
		MaxAge:     cfg.LogMaxAge,
		Compress:   cfg.LogCompress,
	}))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Logger initialized")
}