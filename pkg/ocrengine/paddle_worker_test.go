package ocrengine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doraemonkeys/paddleocr"
)

func TestLegacyPathUsesBinaryDirectoryAfterLaunchDirectory(t *testing.T) {
	launch, binary := t.TempDir(), t.TempDir()
	writeEngine := func(root string) string {
		file := filepath.Join(root, LegacyRelativePath)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, nil, 0644); err != nil {
			t.Fatal(err)
		}
		return file
	}
	exe := writeEngine(binary)
	got, err := resolveLegacyPath("", launch, binary)
	if err != nil || got != exe {
		t.Fatalf("expected %q, got %q (%v)", exe, got, err)
	}
	cwd := writeEngine(launch)
	got, err = resolveLegacyPath("", launch, binary)
	if err != nil || got != cwd {
		t.Fatalf("expected cwd path %q, got %q (%v)", cwd, got, err)
	}
	_, err = resolveLegacyPath("missing.exe", launch, binary)
	if err == nil || !strings.Contains(err.Error(), "-ocr-exe") || !strings.Contains(err.Error(), "missing.exe") {
		t.Fatalf("expected actionable error, got %v", err)
	}
}

func TestWorkerProtocolAndReuse(t *testing.T) {
	python, err := findPython("")
	if err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	program := `
import base64, json, sys
print(json.dumps({"ready": True}), flush=True)
for line in sys.stdin:
    data = base64.b64decode(json.loads(line)["image"])
    entry = {"text": data.decode(), "score": 0.875, "box": [[0,0],[10,0],[10,6],[0,6]]}
    print(json.dumps({"code": 100, "data": [entry]}), flush=True)
`
	worker, err := startWorker(ctx, python, program)
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()
	for _, word := range []string{"PaddleOCR", "中文"} {
		result, err := worker.Recognize(ctx, []byte(word))
		if err != nil || result.Code != paddleocr.CodeSuccess || len(result.Data) != 1 {
			t.Fatalf("unexpected result %+v: %v", result, err)
		}
		item := result.Data[0]
		if item.Text != word || item.Score != 0.875 || len(item.Rect) != 4 || item.Rect[2][1] != 6 {
			t.Fatalf("unexpected detection payload %+v", item)
		}
	}
}

func TestWorkerTimeoutCannotLeakStaleResponse(t *testing.T) {
	python, err := findPython("")
	if err != nil {
		t.Skip(err)
	}
	start, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	worker, err := startWorker(start, python,
		"import sys,time\nprint('{\"ready\":true}', flush=True)\nsys.stdin.readline()\ntime.sleep(30)")
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = worker.Recognize(ctx, []byte("input"))
	if err != context.DeadlineExceeded {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	_, err = worker.Recognize(start, []byte("next"))
	if err == nil {
		t.Fatal("timed-out process remained available")
	}
}
