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
	shutdownChan     chan struct{}
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
	s := &Server{
		config:           cfg,
		activeProcessors: make([]*OCRProcessor, 0, cfg.MaxProcessors),
		idleProcessors:   make([]*OCRProcessor, 0, cfg.MaxProcessors),
		taskQueue:        make(chan ocrTask, cfg.QueueSize),
		shutdownChan:     make(chan struct{}),
		stats:            &ServerStats{},
	}
	s.processorCond = sync.NewCond(&s.poolLock)
	s.stats.AverageProcessingTime.Store(time.Duration(0))
	return s, nil
}

func (s *Server) Initialize() error {
	log.Println("初始化 OCR 处理器...")

	for i := 0; i < s.config.MinProcessors; i++ {
		processor, err := s.createOCRProcessor()
		if err != nil {
			log.Printf("初始化处理器 %d 失败: %v", i, err)
			return fmt.Errorf("初始化处理器 %d 失败: %w", i, err)
		}
		s.activeProcessors = append(s.activeProcessors, processor)
		log.Printf("处理器 %d 已初始化", i)
	}

	log.Println("预热额外处理器...")
	for i := 0; i < s.config.WarmUpCount; i++ {
		processor, err := s.createOCRProcessor()
		if err != nil {
			log.Printf("无法预热处理器 %d：%v", i, err)
			continue
		}
		s.idleProcessors = append(s.idleProcessors, processor)
		log.Printf("预热处理器 %d 已创建", i)
	}

	log.Printf("%d 个激活的 OCR 处理器已初始化，%d 个预热处理器已准备好。\n", len(s.activeProcessors), len(s.idleProcessors))
	return nil
}

func (s *Server) Start() {
	utils.LogInfo("启动 OCR 服务器于 %s:%d，激活处理器数量：%d",
		s.config.Addr, s.config.Port, len(s.activeProcessors))

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

	cancel() // 取消 context，通知所有使用该 context 的 goroutine

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		utils.LogError("服务器关闭错误: %v", err)
	}

	close(s.shutdownChan)

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
		utils.LogWarning("等待 goroutine 退出超时，强制退出")
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
		p.processor.Close()
	}
	for i, p := range s.idleProcessors {
		utils.LogInfo("关闭空闲处理器 %d", i)
		p.processor.Close()
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
			s.PrewarmProcessors()
			s.HealthCheck()
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
	newAvg := oldAvg + (processingTime-oldAvg)/time.Duration(s.stats.TotalRequests)
	s.stats.AverageProcessingTime.Store(newAvg)
}

func (s *Server) checkAndScaleDown() {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	log.Println("检查是否需要缩减处理器数量")

	for i := len(s.activeProcessors) - 1; i >= s.config.MinProcessors; i-- {
		processor := s.activeProcessors[i]
		if atomic.LoadInt64(&processor.usageCount) <= s.config.DegradeThreshold &&
			time.Since(processor.lastUsed) > s.config.IdleTimeout {
			s.activeProcessors = s.activeProcessors[:i]
			s.idleProcessors = append(s.idleProcessors, processor)
			log.Printf("处理器已移至空闲池。激活：%d，空闲：%d", len(s.activeProcessors), len(s.idleProcessors))
		}
	}

	maxIdleProcessors := s.config.MaxProcessors - len(s.activeProcessors)
	for len(s.idleProcessors) > maxIdleProcessors {
		processor := s.idleProcessors[len(s.idleProcessors)-1]
		s.idleProcessors = s.idleProcessors[:len(s.idleProcessors)-1]
		processor.processor.Close()
		log.Printf("关闭多余的空闲处理器。空闲：%d", len(s.idleProcessors))
	}
}

func (s *Server) PrewarmProcessors() {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	log.Println("预热处理器")

	targetIdleCount := s.config.WarmUpCount - len(s.idleProcessors)
	for i := 0; i < targetIdleCount; i++ {
		processor, err := s.createOCRProcessor()
		if err != nil {
			log.Printf("无法预热处理器：%v", err)
			continue
		}
		s.idleProcessors = append(s.idleProcessors, processor)
		log.Printf("创建新的预热处理器。总空闲：%d", len(s.idleProcessors))
	}
	log.Printf("预热完成。激活：%d，空闲：%d", len(s.activeProcessors), len(s.idleProcessors))
}

func (s *Server) HealthCheck() {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	log.Println("开始对所有处理器进行健康检查")

	s.healthCheckProcessors(s.activeProcessors)
	s.healthCheckProcessors(s.idleProcessors)

	log.Printf("健康检查完成。激活：%d，空闲：%d", len(s.activeProcessors), len(s.idleProcessors))
}

func (s *Server) healthCheckProcessors(processors []*OCRProcessor) {
	for i, processor := range processors {
		processor.mutex.Lock()
		log.Printf("检查处理器 %d 的健康状态", i)
		_, err := processor.processor.OcrAndParse([]byte("Hello World"))
		processor.mutex.Unlock()

		if err != nil {
			log.Printf("处理器 %d 未通过健康检查：%v", i, err)
			log.Printf("尝试重新初始化处理器 %d", i)
			newProcessor, err := s.createOCRProcessor()
			if err != nil {
				log.Printf("无法重新初始化处理器 %d：%v", i, err)
				continue
			}
			*processor = *newProcessor
			log.Printf("成功重新初始化处理器 %d", i)
		} else {
			log.Printf("处理器 %d 通过健康检查", i)
		}
	}
}
