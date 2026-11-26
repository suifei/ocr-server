package ocrengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/doraemonkeys/paddleocr"
)

const LegacyRelativePath = "res/PaddleOCR-json_v1.4.0/PaddleOCR-json.exe"

// Relative paths are checked in the working directory first (existing configs),
// then alongside the server executable (shortcut/service launch).
func resolveLegacyPath(requested, launchDir, binaryDir string) (string, error) {
	if requested == "" {
		requested = LegacyRelativePath
	}
	candidates := []string{requested}
	if !filepath.IsAbs(requested) {
		candidates = []string{filepath.Join(launchDir, requested), filepath.Join(binaryDir, requested)}
	}
	for _, path := range candidates {
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() {
			return filepath.Abs(path)
		}
	}
	return "", fmt.Errorf("找不到旧版 PaddleOCR-json 可执行文件，已检查 %q；请解压完整 Windows 引擎目录并用 -ocr-exe 指定 exe 的绝对路径，或改用 paddleocr 后端", candidates)
}

func ResolveLegacyPath(requested string) (string, error) {
	launch, err := os.Getwd()
	if err != nil {
		return "", err
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return resolveLegacyPath(requested, launch, filepath.Dir(executable))
}

func (e *OCREngine) Recognize(ctx context.Context, image []byte) (paddleocr.Result, error) {
	if err := ctx.Err(); err != nil {
		return paddleocr.Result{}, err
	}
	return e.OcrAndParse(image)
}
