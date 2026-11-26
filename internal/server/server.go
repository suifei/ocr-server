package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/suifei/ocr-server/internal/config"
	"github.com/suifei/ocr-server/internal/utils"
)

type Server struct {
	config           config.Config
	activeProcessors []*OCRProcessor
	idleProcessors   []*OCRProcessor
	taskQueue        chan ocrTask
	poolLock         sync.Mutex
	processorCond    *sync.Cond
	wg               sync.WaitGroup
	stats            *ServerStats
}
type ServerStats struct {
	TotalRequests         int64
	SuccessfulRequests    int64
	FailedRequests        int64
	AverageProcessingTime atomic.Value // stores time.Duration
}

func NewServer(cfg config.Config) (*Server, error) {
	if cfg.MinProcessors < 1 || cfg.MaxProcessors < cfg.MinProcessors || cfg.QueueSize < 1 {
		return nil, fmt.Errorf("处理器配置错误：要求 1 <= min_processors <= max_processors，且 queue_size >= 1")
	}
	s := &Server{
		config:           cfg,
		activeProcessors: make([]*OCRProcessor, 0, cfg.MaxProcessors),
		idleProcessors:   make([]*OCRProcessor, 0, cfg.MaxProcessors),
		taskQueue:        make(chan ocrTask, cfg.QueueSize),
		stats:            &ServerStats{},
	}
	s.processorCond = sync.NewCond(&s.poolLock)
	s.stats.AverageProcessingTime.Store(time.Duration(0))
	return s, nil
}

func (s *Server) Initialize() error {
	ctx := context.Background()
	log.Println("初始化 OCR 处理器...")

	for i := 0; i < s.config.MinProcessors; i++ {
		processor, err := s.createOCRProcessor(context.Background())
		if err != nil {
			log.Printf("初始化处理器 %d 失败: %v", i, err)
			s.cleanup()
			return fmt.Errorf("初始化处理器 %d 失败: %w", i, err)
		}
		s.idleProcessors = append(s.idleProcessors, processor)
		log.Printf("处理器 %d 已初始化", i)
	}

	s.PrewarmProcessors(ctx)
	log.Printf("%d 个 OCR 处理器已准备好。", len(s.idleProcessors))
	return nil
}

func (s *Server) Start() {
	utils.LogInfo("启动 OCR 服务器于 %s:%d，激活处理器数量：%d",
		s.config.Addr, s.config.Port, len(s.idleProcessors))

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.config.Addr, s.config.Port),
		Handler: http.HandlerFunc(s.handleOCR),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.wg.Add(1)
	go s.processQueue(ctx)

	s.wg.Add(1)
	go s.monitorProcessors(ctx)

	go func() {
		utils.LogInfo("HTTP 服务器监听端口 %d", s.config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.LogError("HTTP 服务器错误: %v", err)
		}
	}()

	s.waitForShutdown(ctx, cancel, server)
}

func (s *Server) waitForShutdown(ctx context.Context, cancel context.CancelFunc, server *http.Server) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	utils.LogInfo("接收到关闭信号，开始优雅关闭...")

	cancel()
	s.poolLock.Lock()
	s.processorCond.Broadcast()
	s.poolLock.Unlock()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		utils.LogError("服务器关闭错误: %v", err)
	}

	// 等待所有 goroutine 完成，但设置一个超时
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		utils.LogInfo("所有 goroutine 已正常退出")
	case <-time.After(s.config.ShutdownTimeout):
		utils.LogWarning("等待 goroutine 退出超时，关闭空闲处理器")
		// Active jobs still own their engines; avoid a concurrent Close/restart.
		s.closeIdleProcessors()
		return
	}

	s.cleanup()
	utils.LogInfo("服务器已停止")
}

func (s *Server) cleanup() {
	utils.LogInfo("清理资源...")

	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	for i, p := range s.activeProcessors {
		utils.LogInfo("关闭活跃处理器 %d", i)
		if p.processor != nil {
			p.processor.Close()
		}
	}
	for i, p := range s.idleProcessors {
		utils.LogInfo("关闭空闲处理器 %d", i)
		if p.processor != nil {
			p.processor.Close()
		}
	}

	s.activeProcessors = nil
	s.idleProcessors = nil

	utils.LogInfo("所有资源已清理")
}

func (s *Server) monitorProcessors(ctx context.Context) {
	defer s.wg.Done()
	utils.LogInfo("处理器监控已启动")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			utils.LogInfo("运行定期处理器检查")
			s.checkAndScaleDown()
			s.PrewarmProcessors(ctx)
		case <-ctx.Done():
			utils.LogInfo("处理器监控正在关闭")
			return
		}
	}
}

func (s *Server) processQueue(ctx context.Context) {
	defer s.wg.Done()
	utils.LogInfo("任务队列处理器已启动")

	for {
		select {
		case task := <-s.taskQueue:
			s.wg.Add(1)
			go s.processTask(ctx, task)
		case <-ctx.Done():
			utils.LogInfo("任务队列处理器正在关闭")
			return
		}
	}
}

func (s *Server) updateStats(processingTime time.Duration, success bool) {
	atomic.AddInt64(&s.stats.TotalRequests, 1)
	if success {
		atomic.AddInt64(&s.stats.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&s.stats.FailedRequests, 1)
	}

	// 更新平均处理时间
	oldAvg := s.stats.AverageProcessingTime.Load().(time.Duration)
	newAvg := oldAvg + (processingTime-oldAvg)/time.Duration(atomic.LoadInt64(&s.stats.TotalRequests))
	s.stats.AverageProcessingTime.Store(newAvg)
}

func (s *Server) checkAndScaleDown() {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	keep := max(0, s.config.MinProcessors-len(s.activeProcessors))
	for i := len(s.idleProcessors) - 1; i >= 0 && len(s.idleProcessors) > keep; i-- {
		processor := s.idleProcessors[i]
		if time.Since(processor.lastUsed) > s.config.IdleTimeout {
			processor.processor.Close()
			s.idleProcessors = append(s.idleProcessors[:i], s.idleProcessors[i+1:]...)
		}
	}
}

func (s *Server) PrewarmProcessors(ctx context.Context) {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	target := min(s.config.MaxProcessors, s.config.MinProcessors+s.config.WarmUpCount)
	for len(s.activeProcessors)+len(s.idleProcessors) < target {
		p, err := s.createOCRProcessor(ctx)
		if err != nil {
			log.Printf("无法预热处理器：%v", err)
			break
		}
		s.idleProcessors = append(s.idleProcessors, p)
		s.processorCond.Signal()
	}
}

func (s *Server) closeIdleProcessors() {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()
	for _, p := range s.idleProcessors {
		p.processor.Close()
	}
	s.idleProcessors = nil
}
