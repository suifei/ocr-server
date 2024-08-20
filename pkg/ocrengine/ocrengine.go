package ocrengine

import (
	"fmt"
	"time"

	"github.com/doraemonkeys/paddleocr"
	"github.com/suifei/ocr-server/internal/utils"
)

type OCREngine struct {
	*paddleocr.Ppocr
	ExecutionTime time.Duration
}

func NewOCREngine(exePath string) (*OCREngine, error) {
	startTime := time.Now()
	enable_mkldnn := true
	processor, err := paddleocr.NewPpocr(exePath, paddleocr.OcrArgs{
		EnableMkldnn:  &enable_mkldnn,
	})
	if err != nil {
		utils.LogError("创建 OCR 引擎失败: exePath=%s, error=%v", exePath, err)
		return nil, fmt.Errorf("创建 OCR 引擎失败: %w", err)
	}
	executionTime := time.Since(startTime)
	utils.LogInfo("OCR 引擎创建成功:%v", executionTime)
	return &OCREngine{processor, executionTime}, nil
}

func (e *OCREngine) ProcessImage(imagePath string) (paddleocr.Result, error) {
	startTime := time.Now()
	result, err := e.OcrFileAndParse(imagePath)
	executionTime := time.Since(startTime)
	e.ExecutionTime = executionTime

	if err != nil {
		utils.LogError("处理图像失败: imagePath=%s, executionTime=%v, error=%v", imagePath, executionTime, err)
		return result, fmt.Errorf("处理图像失败: %w", err)
	}

	utils.LogInfo("图像处理成功: imagePath=%s, executionTime=%v, resultCount=%d", imagePath, executionTime, len(result.Data))

	return result, nil
}

func (e *OCREngine) ProcessImageBytes(imageData []byte) (paddleocr.Result, error) {
	startTime := time.Now()
	result, err := e.OcrAndParse(imageData)
	executionTime := time.Since(startTime)
	e.ExecutionTime = executionTime

	if err != nil {
		utils.LogError("处理图像数据失败: dataSize=%d, executionTime=%v, error=%v", len(imageData), executionTime, err)
		return result, fmt.Errorf("处理图像数据失败: %w", err)
	}

	utils.LogInfo("图像数据处理成功: dataSize=%d, executionTime=%v, resultCount=%d", len(imageData), executionTime, len(result.Data))

	return result, nil
}
