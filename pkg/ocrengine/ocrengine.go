package ocrengine

import (
	"fmt"

	"github.com/doraemonkeys/paddleocr"
)

type OCREngine struct {
	*paddleocr.Ppocr
}

func NewOCREngine(exePath string) (*OCREngine, error) {
	processor, err := paddleocr.NewPpocr(exePath, paddleocr.OcrArgs{})
	if err != nil {
		fmt.Println("Please download the OCR executable from https://github.com/hiroi-sora/PaddleOCR-json and put it in the same folder as this program")
		return nil, err
	}
	return &OCREngine{processor}, nil
}

func (e *OCREngine) ProcessImage(imagePath string) (paddleocr.Result, error) {
	return e.OcrFileAndParse(imagePath)
}

func (e *OCREngine) ProcessImageBytes(imageData []byte) (paddleocr.Result, error) {
	return e.OcrAndParse(imageData)
}