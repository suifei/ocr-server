package ocr

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gen2brain/go-unarr"
)

const (
	ocrURL      = "https://github.com/hiroi-sora/PaddleOCR-json/releases/download/v1.4.0/PaddleOCR-json_v1.4.0_windows_x86-64.7z"
	ocrFileName = "PaddleOCR-json_v1.4.0_windows_x86-64.7z"
	ocrExeName  = "PaddleOCR-json_v1.4.0/PaddleOCR-json.exe"
	resDir      = "res"
)

func EnsureOCREngine() (string, error) {
	ocrPath := filepath.Join(resDir, ocrExeName)

	if _, err := os.Stat(ocrPath); err == nil {
		fmt.Println("OCR 引擎已存在。")
		return ocrPath, nil
	}

	fmt.Println("未找到 OCR 引擎。开始下载过程...")

	var proxyURL string
	fmt.Print("输入代理 URL (留空则直接下载): ")
	fmt.Scanln(&proxyURL)

	if err := downloadOCRWithRetry(proxyURL); err != nil {
		return "", fmt.Errorf("下载 OCR 引擎失败: %w", err)
	}

	if err := extractArchive(); err != nil {
		return "", fmt.Errorf("提取 OCR 引擎失败: %w", err)
	}

	fmt.Println("OCR 引擎安装成功。")
	return ocrPath, nil
}

func downloadOCRWithRetry(proxyURL string) error {
	if _, err := os.Stat(resDir); os.IsNotExist(err) {
		if err := os.MkdirAll(resDir, 0755); err != nil {
			return fmt.Errorf("创建 res 目录失败: %w", err)
		}
	}

	client := &http.Client{}
	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			return fmt.Errorf("无效的代理 URL: %w", err)
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURLParsed)}
	}

	filePath := filepath.Join(resDir, ocrFileName)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	resumePos := fileInfo.Size()

	operation := func() error {
		req, err := http.NewRequest("GET", ocrURL, nil)
		if err != nil {
			return fmt.Errorf("创建请求失败: %w", err)
		}

		if resumePos > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumePos))
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("发送请求失败: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			return fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
		}

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}

		return nil
	}

	backOff := backoff.NewExponentialBackOff()
	backOff.MaxElapsedTime = 5 * time.Minute

	err = backoff.Retry(operation, backOff)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	fmt.Println("\n下载成功完成。")
	return nil
}
func extractArchive() error {
	archivePath := filepath.Join(resDir, ocrFileName)

	a, err := unarr.NewArchive(archivePath)
	if err != nil {
		return fmt.Errorf("打开压缩文件失败: %w", err)
	}
	defer a.Close()

	_, err = a.Extract(resDir)

	if err != nil {
		fmt.Println("提取失败: %v\n", err)
	}

	go func() {
		time.Sleep(10 * time.Second)

		// Remove the archive file after extraction
		if err := os.Remove(archivePath); err != nil {
			fmt.Printf("警告: 删除文件失败: %v\n", err)
		}
	}()

	fmt.Println("提取成功完成。")
	return nil
}

func IsOCREngineInstalled() bool {
	_, err := os.Stat(filepath.Join(resDir, ocrExeName))
	return err == nil
}

func GetOCREnginePath() string {
	return filepath.Join(resDir, ocrExeName)
}

// ProgressReader is a custom io.Reader that reports progress
type ProgressReader struct {
	Reader     io.Reader
	Total      int64
	Current    int64
	OnProgress func(int64)
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.Current += int64(n)
	if pr.OnProgress != nil {
		pr.OnProgress(pr.Current)
	}
	return n, err
}
