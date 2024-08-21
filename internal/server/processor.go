package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/doraemonkeys/paddleocr"
	"github.com/suifei/ocr-server/internal/imgproc"
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

func (s *Server) getAvailableProcessor(ctx context.Context) *OCRProcessor {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	for {
		select {
		case <-ctx.Done():
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
func (s *Server) processTask(ctx context.Context, task ocrTask) {
	defer s.wg.Done()

	startTime := time.Now()

	processor := s.getAvailableProcessor(ctx)
	if processor == nil {
		log.Println("无可用处理器，服务器正在关闭")
		task.Response <- ocrResponse{Error: "服务器正在关闭"}
		s.updateStats(time.Since(startTime), false)
		return
	}

	log.Printf("使用处理器 %p 处理任务", processor)
	result, err := s.performOCRWithRetry(ctx, processor, task)

	if err != nil {
		log.Printf("OCR 任务失败: %v", err)
		task.Response <- ocrResponse{Error: err.Error()}
		s.updateStats(time.Since(startTime), false)
	} else if result.Code != paddleocr.CodeSuccess {
		log.Printf("OCR 任务失败，错误代码: %s", result.Msg)
		task.Response <- ocrResponse{Error: fmt.Sprintf("OCR 失败: %s", result.Msg)}
		s.updateStats(time.Since(startTime), false)
	} else {
		log.Println("OCR 任务成功完成")
		task.Response <- ocrResponse{Data: result.Data}
		s.updateStats(time.Since(startTime), true)
	}

	s.releaseProcessor(processor)
}

func (s *Server) performOCRWithRetry(ctx context.Context, processor *OCRProcessor, task ocrTask) (paddleocr.Result, error) {
	var result paddleocr.Result
	var err error

	operation := func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			atomic.AddInt64(&processor.usageCount, 1)
			defer atomic.AddInt64(&processor.usageCount, -1)

			processor.mutex.Lock()
			defer processor.mutex.Unlock()

			var buff []byte

			if task.ImagePath != "" {
				buff, err = os.ReadFile(task.ImagePath)
			} else {
				buff = task.ImageData
			}
			// 二值化
			threshold := s.config.ThresholdValue
			thresholdMode := imgproc.ThresholdMode(s.config.ThresholdMode)
			img, _ := imgproc.BytesToImage(buff)
			processedImg := imgproc.ProcessImage(img, uint8(threshold), thresholdMode)
			imgdata, _ := imgproc.GrayImageToPNGBytes(processedImg)
			task.ImageData = imgdata
			result, err = processor.processor.OcrAndParse(task.ImageData)

			processor.lastUsed = time.Now()

			if err != nil {
				log.Printf("OCR 处理器失败: %v。尝试重新初始化...", err)
				processor.processor.Close()
				newProcessor, initErr := s.createOCRProcessor()
				if initErr != nil {
					log.Printf("重新初始化 OCR 处理器失败: %v", initErr)
					return err // 返回原始错误，让 backoff 重试
				}
				*processor = *newProcessor
				processor.inUse = true
				log.Printf("成功重新初始化 OCR 处理器")
				return err // 返回原始错误，让 backoff 重试
			}

			return nil
		}
	}

	backOff := backoff.NewExponentialBackOff()
	backOff.MaxElapsedTime = 2 * time.Minute

	err = backoff.Retry(operation, backoff.WithContext(backOff, ctx))
	if err != nil {
		return result, fmt.Errorf("执行 OCR 失败: %w", err)
	}

	return result, nil
}
