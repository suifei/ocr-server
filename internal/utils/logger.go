package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/suifei/ocr-server/internal/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	infoLogger    *log.Logger
	warningLogger *log.Logger
	errorLogger   *log.Logger
)

func init() {
	// 设置默认的日志记录器
	infoLogger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lmicroseconds)
	warningLogger = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime|log.Lmicroseconds)
	errorLogger = log.New(os.Stdout, "ERROR: ", log.Ldate|log.Ltime|log.Lmicroseconds)
}
func SetupLogger(cfg config.Config) {
	logFile := &lumberjack.Logger{
		Filename:   cfg.LogFilePath,
		MaxSize:    cfg.LogMaxSize,
		MaxBackups: cfg.LogMaxBackups,
		MaxAge:     cfg.LogMaxAge,
		Compress:   cfg.LogCompress,
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	infoLogger = log.New(multiWriter, "INFO: ", log.Ldate|log.Ltime|log.Lmicroseconds)
	warningLogger = log.New(multiWriter, "WARNING: ", log.Ldate|log.Ltime|log.Lmicroseconds)
	errorLogger = log.New(multiWriter, "ERROR: ", log.Ldate|log.Ltime|log.Lmicroseconds)

	log.Println("日志系统初始化完成")
}

func LogInfo(format string, v ...interface{}) {
	logWithCaller(infoLogger, format, v...)
}

func LogWarning(format string, v ...interface{}) {
	logWithCaller(warningLogger, format, v...)
}

func LogError(format string, v ...interface{}) {
	logWithCaller(errorLogger, format, v...)
}

func logWithCaller(logger *log.Logger, format string, v ...interface{}) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}
	_, file = filepath.Split(file)
	msg := fmt.Sprintf(format, v...)
	logger.Printf("%s:%d: %s", file, line, msg)
}
