package ocrengine

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/doraemonkeys/paddleocr"
)

//go:embed paddle_worker.py
var paddleScript string

// Engine is used serially by one pool slot. Close may interrupt a call.
type Engine interface {
	Recognize(context.Context, []byte) (paddleocr.Result, error)
	Close() error
}

type WorkerOptions struct {
	Python   string
	DetModel string
	RecModel string
}

type PaddleWorker struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	output *bufio.Scanner
	closed sync.Once
}

type response struct {
	Ready   bool             `json:"ready"`
	Error   string           `json:"error"`
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    []paddleocr.Data `json:"data"`
}

func NewPaddleWorker(ctx context.Context, opts WorkerOptions) (*PaddleWorker, error) {
	python, err := findPython(opts.Python)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	worker, err := startWorker(ctx, python, paddleScript, opts.DetModel, opts.RecModel)
	if err != nil {
		return nil, fmt.Errorf("启动本地 PaddleOCR 失败 (Python %q；请核对虚拟环境、PaddleOCR 3.7.0 与模型): %w", python, err)
	}
	return worker, nil
}

func findPython(configured string) (string, error) {
	candidates := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3"}
	}
	if configured != "" {
		candidates = []string{configured}
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("找不到可用的 Python (%v)，请用 -ocr-python 指定虚拟环境解释器的完整路径", candidates)
}

func startWorker(ctx context.Context, python, program string, args ...string) (*PaddleWorker, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.Command(python, append([]string{"-u", "-X", "utf8", "-c", program}, args...)...)
	cmd.Stderr = os.Stderr
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		input.Close()
		output.Close()
		return nil, err
	}
	worker := &PaddleWorker{cmd: cmd, stdin: input, output: bufio.NewScanner(output)}
	worker.output.Buffer(make([]byte, 64*1024), 16*1024*1024)
	ready, err := worker.exchange(ctx, nil)
	if err != nil || !ready.Ready {
		worker.Close()
		if err == nil {
			err = fmt.Errorf("worker 初始化错误: %s", ready.Error)
		}
		return nil, err
	}
	return worker, nil
}

func (p *PaddleWorker) exchange(ctx context.Context, image []byte) (response, error) {
	type outcome struct {
		value response
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		if image != nil {
			err := json.NewEncoder(p.stdin).Encode(struct {
				Image []byte `json:"image"`
			}{Image: image})
			if err != nil {
				done <- outcome{err: err}
				return
			}
		}
		if !p.output.Scan() {
			err := p.output.Err()
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			done <- outcome{err: err}
			return
		}
		var value response
		err := json.Unmarshal(p.output.Bytes(), &value)
		done <- outcome{value, err}
	}()
	select {
	case result := <-done:
		return result.value, result.err
	case <-ctx.Done():
		p.Close()
		<-done // The killed worker cannot leave a stale response in the stream.
		return response{}, ctx.Err()
	}
}

func (p *PaddleWorker) Recognize(ctx context.Context, image []byte) (paddleocr.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	value, err := p.exchange(ctx, image)
	if err != nil {
		return paddleocr.Result{}, err
	}
	if value.Code == 0 {
		return paddleocr.Result{}, fmt.Errorf("无效的 PaddleOCR 响应")
	}
	return paddleocr.Result{Code: value.Code, Msg: value.Message, Data: value.Data}, nil
}

func (p *PaddleWorker) Close() error {
	p.closed.Do(func() {
		_ = p.cmd.Process.Kill()
		_ = p.cmd.Wait()
	})
	return nil
}
