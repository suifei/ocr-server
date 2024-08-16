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
}

func NewServer(cfg config.Config) (*Server, error) {
	s := &Server{
		config:           cfg,
		activeProcessors: make([]*OCRProcessor, 0, cfg.MaxProcessors),
		idleProcessors:   make([]*OCRProcessor, 0, cfg.MaxProcessors),
		taskQueue:        make(chan ocrTask, cfg.QueueSize),
		shutdownChan:     make(chan struct{}),
	}
	s.processorCond = sync.NewCond(&s.poolLock)
	return s, nil
}

func (s *Server) Initialize() error {
	log.Println("Initializing OCR processors...")

	for i := 0; i < s.config.MinProcessors; i++ {
		processor, err := s.createOCRProcessor()
		if err != nil {
			log.Printf("Failed to initialize processor %d: %v", i, err)
			return fmt.Errorf("failed to initialize processor %d: %w", i, err)
		}
		s.activeProcessors = append(s.activeProcessors, processor)
		log.Printf("Processor %d initialized", i)
	}

	log.Println("Warming up additional processors...")
	for i := 0; i < s.config.WarmUpCount; i++ {
		processor, err := s.createOCRProcessor()
		if err != nil {
			log.Printf("Failed to warm up processor %d: %v", i, err)
			continue
		}
		s.idleProcessors = append(s.idleProcessors, processor)
		log.Printf("Warm-up processor %d created", i)
	}

	log.Printf("%d active OCR processors initialized, %d warm-up processors ready.\n", len(s.activeProcessors), len(s.idleProcessors))
	return nil
}

func (s *Server) Start() {
	log.Printf("Starting OCR server on %s:%d with %d active processors\n",
		s.config.Addr, s.config.Port, len(s.activeProcessors))

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.config.Addr, s.config.Port),
		Handler: http.HandlerFunc(s.handleOCR),
	}

	s.wg.Add(1)
	go s.processQueue()

	go s.monitorProcessors()

	go func() {
		log.Printf("HTTP server listening on port %d", s.config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	s.waitForShutdown(server)
}

func (s *Server) waitForShutdown(server *http.Server) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutdown signal received, initiating graceful shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	close(s.shutdownChan)
	s.wg.Wait()

	s.cleanup()
	log.Println("Server stopped")
}

func (s *Server) cleanup() {
	log.Println("Cleaning up resources...")

	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	for i, p := range s.activeProcessors {
		log.Printf("Closing active processor %d", i)
		p.processor.Close()
	}
	for i, p := range s.idleProcessors {
		log.Printf("Closing idle processor %d", i)
		p.processor.Close()
	}

	s.activeProcessors = nil
	s.idleProcessors = nil

	log.Println("All resources cleaned up")
}

func (s *Server) processQueue() {
	defer s.wg.Done()
	log.Println("Task queue processor started")

	for {
		select {
		case task := <-s.taskQueue:
			s.wg.Add(1)
			go s.processTask(task)
		case <-s.shutdownChan:
			log.Println("Task queue processor shutting down")
			return
		}
	}
}

// Other methods like monitorProcessors, checkAndScaleDown, PrewarmProcessors, and HealthCheck
// will be implemented here...

func (s *Server) monitorProcessors() {
	log.Println("Processor monitor started")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Running periodic processor checks")
			s.checkAndScaleDown()
			s.PrewarmProcessors()
			s.HealthCheck()
		case <-s.shutdownChan:
			log.Println("Processor monitor shutting down")
			return
		}
	}
}

func (s *Server) checkAndScaleDown() {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	log.Println("Checking for processors to scale down")

	for i := len(s.activeProcessors) - 1; i >= s.config.MinProcessors; i-- {
		processor := s.activeProcessors[i]
		if atomic.LoadInt64(&processor.usageCount) <= s.config.DegradeThreshold &&
			time.Since(processor.lastUsed) > s.config.IdleTimeout {
			s.activeProcessors = s.activeProcessors[:i]
			s.idleProcessors = append(s.idleProcessors, processor)
			log.Printf("Moved processor to idle pool. Active: %d, Idle: %d", len(s.activeProcessors), len(s.idleProcessors))
		}
	}

	// Clean up excess idle processors
	maxIdleProcessors := s.config.MaxProcessors - len(s.activeProcessors)
	for len(s.idleProcessors) > maxIdleProcessors {
		processor := s.idleProcessors[len(s.idleProcessors)-1]
		s.idleProcessors = s.idleProcessors[:len(s.idleProcessors)-1]
		processor.processor.Close()
		log.Printf("Closed excess idle processor. Idle: %d", len(s.idleProcessors))
	}
}

func (s *Server) PrewarmProcessors() {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	log.Println("Prewarming processors")

	targetIdleCount := s.config.WarmUpCount - len(s.idleProcessors)
	for i := 0; i < targetIdleCount; i++ {
		processor, err := s.createOCRProcessor()
		if err != nil {
			log.Printf("Failed to prewarm processor: %v", err)
			continue
		}
		s.idleProcessors = append(s.idleProcessors, processor)
		log.Printf("Created new prewarm processor. Total idle: %d", len(s.idleProcessors))
	}
	log.Printf("Prewarming complete. Active: %d, Idle: %d", len(s.activeProcessors), len(s.idleProcessors))
}

func (s *Server) HealthCheck() {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	log.Println("Starting health check on all processors")

	s.healthCheckProcessors(s.activeProcessors)
	s.healthCheckProcessors(s.idleProcessors)

	log.Printf("Health check completed. Active: %d, Idle: %d", len(s.activeProcessors), len(s.idleProcessors))
}

func (s *Server) healthCheckProcessors(processors []*OCRProcessor) {
	for i, processor := range processors {
		processor.mutex.Lock()
		log.Printf("Checking health of processor %d", i)
		_, err := processor.processor.OcrAndParse([]byte("Hello, World!"))
		processor.mutex.Unlock()

		if err != nil {
			log.Printf("Processor %d failed health check: %v", i, err)
			log.Printf("Attempting to reinitialize processor %d", i)
			newProcessor, err := s.createOCRProcessor()
			if err != nil {
				log.Printf("Failed to reinitialize processor %d: %v", i, err)
				continue
			}
			*processor = *newProcessor
			log.Printf("Successfully reinitialized processor %d", i)
		} else {
			log.Printf("Processor %d passed health check", i)
		}
	}
}
