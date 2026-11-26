package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/doraemonkeys/paddleocr"
	"github.com/suifei/ocr-server/internal/config"
	"github.com/suifei/ocr-server/internal/imgproc"
	"github.com/suifei/ocr-server/pkg/ocrengine"
)

type OCRProcessor struct {
	processor ocrengine.Engine
	lastUsed  time.Time
}

type ocrTask struct {
	ImagePath string
	ImageData []byte
	Response  chan ocrResponse
}

func (s *Server) createOCRProcessor(ctx context.Context) (*OCRProcessor, error) {
	var engine ocrengine.Engine
	var err error
	switch s.config.OCRBackend {
	case config.BackendPaddleOCR:
		engine, err = ocrengine.NewPaddleWorker(ctx, ocrengine.WorkerOptions{
			Python: s.config.OCRPython, DetModel: s.config.OCRDetModel, RecModel: s.config.OCRRecModel,
		})
	case config.BackendLegacy:
		// Resolve on every platform for useful diagnostics and compatibility tests.
		var path string
		path, err = ocrengine.ResolveLegacyPath(s.config.OCRExePath)
		if err != nil {
			return nil, err
		}
		if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
			return nil, fmt.Errorf("legacy 需要 Windows x86-64 版 PaddleOCR-json；请使用 paddleocr 后端")
		}
		engine, err = ocrengine.NewOCREngine(path)
	default:
		return nil, fmt.Errorf("未知 OCR 后端 %q，请使用 paddleocr 或 legacy", s.config.OCRBackend)
	}
	if err != nil {
		return nil, err
	}
	return &OCRProcessor{processor: engine, lastUsed: time.Now()}, nil
}

func (s *Server) getAvailableProcessor(ctx context.Context) (*OCRProcessor, error) {
	stop := context.AfterFunc(ctx, func() {
		s.poolLock.Lock()
		s.processorCond.Broadcast()
		s.poolLock.Unlock()
	})
	defer stop()
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	for ctx.Err() == nil {
		if n := len(s.idleProcessors); n > 0 {
			processor := s.idleProcessors[n-1]
			s.idleProcessors = s.idleProcessors[:n-1]
			s.activeProcessors = append(s.activeProcessors, processor)
			return processor, nil
		}

		if len(s.activeProcessors) < s.config.MaxProcessors {
			processor, err := s.createOCRProcessor(ctx)
			if err == nil {
				s.activeProcessors = append(s.activeProcessors, processor)
				return processor, nil
			}
			if len(s.activeProcessors) == 0 {
				return nil, err
			}
		}
		s.processorCond.Wait()
	}
	return nil, ctx.Err()
}

func (s *Server) releaseProcessor(processor *OCRProcessor) {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	for i, p := range s.activeProcessors {
		if p == processor {
			s.activeProcessors = append(s.activeProcessors[:i], s.activeProcessors[i+1:]...)
			break
		}
	}
	if processor.processor != nil {
		processor.lastUsed = time.Now()
		s.idleProcessors = append(s.idleProcessors, processor)
	}
	s.processorCond.Signal()
}

func (s *Server) processTask(ctx context.Context, task ocrTask) {
	defer s.wg.Done()
	start := time.Now()

	data, err := s.imageData(task)
	if err != nil {
		s.updateStats(time.Since(start), false)
		task.Response <- ocrResponse{Error: err.Error()}
		return
	}

	processor, err := s.getAvailableProcessor(ctx)
	if err != nil {
		log.Printf("无法取得 OCR 处理器: %v", err)
		s.updateStats(time.Since(start), false)
		task.Response <- ocrResponse{Error: "无可用 OCR 处理器"}
		return
	}
	defer s.releaseProcessor(processor)

	result, err := s.recognize(ctx, processor, data)
	if err != nil {
		log.Printf("OCR 任务失败: %v", err)
		s.updateStats(time.Since(start), false)
		task.Response <- ocrResponse{Error: "执行 OCR 失败，请查看服务日志"}
	} else if result.Code != paddleocr.CodeSuccess {
		s.updateStats(time.Since(start), false)
		task.Response <- ocrResponse{Error: fmt.Sprintf("OCR 失败: %s", result.Msg)}
	} else {
		s.updateStats(time.Since(start), true)
		task.Response <- ocrResponse{Data: result.Data}
	}
}

func (s *Server) imageData(task ocrTask) ([]byte, error) {
	data := task.ImageData
	if task.ImagePath != "" {
		var err error
		data, err = os.ReadFile(task.ImagePath)
		if err != nil {
			return nil, fmt.Errorf("无法读取图像文件")
		}
	}
	if s.config.OCRBackend != config.BackendLegacy {
		return data, nil // PP-OCRv6 uses original image colors.
	}
	img, err := imgproc.BytesToImage(data)
	if err != nil {
		return nil, fmt.Errorf("无法解码图像")
	}
	processed := imgproc.ProcessImage(img, uint8(s.config.ThresholdValue), imgproc.ThresholdMode(s.config.ThresholdMode))
	return imgproc.GrayImageToPNGBytes(processed)
}

func (s *Server) recognize(ctx context.Context, proc *OCRProcessor, data []byte) (paddleocr.Result, error) {
	result, err := proc.processor.Recognize(ctx, data)
	if err == nil {
		return result, nil
	}
	log.Printf("OCR 工作进程通信失败: %v", err)
	proc.processor.Close()
	proc.processor = nil
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	replacement, createErr := s.createOCRProcessor(ctx)
	if createErr != nil {
		return result, fmt.Errorf("重新初始化处理器失败: %w", createErr)
	}
	proc.processor = replacement.processor
	result, err = proc.processor.Recognize(ctx, data)
	if err != nil {
		proc.processor.Close()
		proc.processor = nil
	}
	return result, err
}
