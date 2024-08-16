package ocr

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/gen2brain/go-unarr"
)

const (
	ocrURL      = "https://github.com/hiroi-sora/PaddleOCR-json/releases/download/v1.4.0/PaddleOCR-json_v1.4.0_windows_x86-64.7z"
	ocrFileName = "PaddleOCR-json_v1.4.0_windows_x86-64.7z"
	ocrExeName  = "PaddleOCR-json.exe"
	resDir      = "res/PaddleOCR-json_v1.4.0"
)

func EnsureOCREngine() (string, error) {
	ocrPath := filepath.Join(resDir, ocrExeName)
	if _, err := os.Stat(ocrPath); err == nil {
		fmt.Println("OCR engine already exists.")
		return ocrPath, nil
	}

	fmt.Println("OCR engine not found. Starting download process...")

	var proxyURL string
	fmt.Print("Enter proxy URL (leave empty for direct download): ")
	fmt.Scanln(&proxyURL)

	if err := downloadOCR(proxyURL); err != nil {
		return "", fmt.Errorf("failed to download OCR engine: %w", err)
	}

	if err := extractArchive(); err != nil {
		return "", fmt.Errorf("failed to extract OCR engine: %w", err)
	}

	fmt.Println("OCR engine successfully installed.")
	return ocrPath, nil
}

func downloadOCR(proxyURL string) error {
	if _, err := os.Stat(resDir); os.IsNotExist(err) {
		if err := os.MkdirAll(resDir, 0755); err != nil {
			return fmt.Errorf("failed to create res directory: %w", err)
		}
	}

	client := &http.Client{}
	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %w", err)
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURLParsed)}
	}

	resp, err := client.Get(ocrURL)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath.Join(resDir, ocrFileName))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Create a progress bar
	total := resp.ContentLength
	// progress := 0
	lastPercentage := 0

	reader := &ProgressReader{
		Reader: resp.Body,
		Total:  total,
		OnProgress: func(current int64) {
			if total > 0 {
				percentage := int(float64(current) / float64(total) * 100)
				if percentage > lastPercentage {
					fmt.Printf("\rDownloading... %d%%", percentage)
					lastPercentage = percentage
				}
			}
		},
	}

	_, err = io.Copy(out, reader)
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	fmt.Println("\nDownload completed successfully.")
	return nil
}

func extractArchive() error {
	archivePath := filepath.Join(resDir, ocrFileName)

	a, err := unarr.NewArchive(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer a.Close()

	content, err := a.Extract(resDir)
	fmt.Println("Extracting files...", content)
	if err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	// Remove the archive file after extraction
	if err := os.Remove(archivePath); err != nil {
		fmt.Printf("Warning: Failed to remove archive file: %v\n", err)
	}

	fmt.Println("Extraction completed successfully.")
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
