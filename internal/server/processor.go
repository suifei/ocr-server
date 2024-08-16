package server

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/doraemonkeys/paddleocr"
	"github.com/suifei/ocr-server/pkg/ocrengine"
)

type OCRProcessor struct {
	processor  *paddleocr.Ppocr
	usageCount int64
	lastUsed   time.Time
	mutex      sync.Mutex
	inUse      bool
}

type ocrTask struct {
	ImagePath string
	ImageData []byte
	Response  chan ocrResponse
}

func (s *Server) createOCRProcessor() (*OCRProcessor, error) {
	processor, err := ocrengine.NewOCREngine(s.config.OCRExePath)
	if err != nil {
		return nil, err
	}

	return &OCRProcessor{
		processor: processor.Ppocr,
		lastUsed:  time.Now(),
	}, nil
}

func (s *Server) getAvailableProcessor() *OCRProcessor {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	for {
		select {
		case <-s.shutdownChan:
			return nil
		default:
			if len(s.idleProcessors) > 0 {
				processor := s.idleProcessors[len(s.idleProcessors)-1]
				s.idleProcessors = s.idleProcessors[:len(s.idleProcessors)-1]
				s.activeProcessors = append(s.activeProcessors, processor)
				processor.inUse = true
				return processor
			}

			if len(s.activeProcessors) < s.config.MaxProcessors {
				processor, err := s.createOCRProcessor()
				if err == nil {
					processor.inUse = true
					s.activeProcessors = append(s.activeProcessors, processor)
					return processor
				}
			}

			s.processorCond.Wait()
		}
	}
}

func (s *Server) releaseProcessor(processor *OCRProcessor) {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	processor.inUse = false
	processor.lastUsed = time.Now()

	if len(s.activeProcessors) > s.config.MinProcessors {
		for i, p := range s.activeProcessors {
			if p == processor {
				s.activeProcessors = append(s.activeProcessors[:i], s.activeProcessors[i+1:]...)
				s.idleProcessors = append(s.idleProcessors, processor)
				break
			}
		}
	}

	s.processorCond.Signal()
}

func (s *Server) processTask(task ocrTask) {
	defer s.wg.Done()

	processor := s.getAvailableProcessor()
	if processor == nil {
		log.Println("No processor available, server is shutting down")
		task.Response <- ocrResponse{Error: "Server is shutting down"}
		return
	}

	log.Printf("Processing task with processor %p", processor)
	result, err := s.performOCRWithRetry(processor, task)

	if err != nil {
		log.Printf("OCR task failed: %v", err)
		task.Response <- ocrResponse{Error: err.Error()}
	} else if result.Code != paddleocr.CodeSuccess {
		log.Printf("OCR task failed with code: %s", result.Msg)
		task.Response <- ocrResponse{Error: fmt.Sprintf("OCR failed: %s", result.Msg)}
	} else {
		log.Println("OCR task completed successfully")
		task.Response <- ocrResponse{Data: result.Data}
	}

	s.releaseProcessor(processor)
}

func (s *Server) performOCRWithRetry(processor *OCRProcessor, task ocrTask) (paddleocr.Result, error) {
	maxRetries := 3
	var result paddleocr.Result
	var err error

	atomic.AddInt64(&processor.usageCount, 1)
	defer atomic.AddInt64(&processor.usageCount, -1)

	for retry := 0; retry < maxRetries; retry++ {
		processor.mutex.Lock()
		if task.ImagePath != "" {
			result, err = processor.processor.OcrFileAndParse(task.ImagePath)
		} else {
			result, err = processor.processor.OcrAndParse(task.ImageData)
		}
		processor.lastUsed = time.Now()
		processor.mutex.Unlock()

		if err == nil {
			return result, nil
		}

		log.Printf("OCR processor failed: %v. Attempting to reinitialize...", err)

		processor.mutex.Lock()
		processor.processor.Close()
		newProcessor, err := s.createOCRProcessor()
		if err != nil {
			processor.mutex.Unlock()
			log.Printf("Failed to reinitialize OCR processor: %v", err)
			time.Sleep(time.Second * time.Duration(retry+1)) // Exponential backoff
			continue
		}
		*processor = *newProcessor
		processor.inUse = true
		processor.mutex.Unlock()

		log.Printf("Successfully reinitialized OCR processor")
	}

	return result, fmt.Errorf("failed to perform OCR after %d retries", maxRetries)
}