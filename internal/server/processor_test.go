package server

import (
	"context"
	"testing"
	"time"

	"github.com/doraemonkeys/paddleocr"
	"github.com/suifei/ocr-server/internal/config"
)

func TestPreloadedSingleWorkerCanBeAcquiredAndReused(t *testing.T) {
	s, err := NewServer(config.Config{MinProcessors: 1, MaxProcessors: 1, QueueSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	proc := &OCRProcessor{processor: testEngine{}}
	s.idleProcessors = append(s.idleProcessors, proc)
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		got, err := s.getAvailableProcessor(ctx)
		cancel()
		if err != nil || got != proc || len(s.activeProcessors) != 1 || len(s.idleProcessors) != 0 {
			t.Fatalf("unexpected checkout %p: %v", got, err)
		}
		s.releaseProcessor(proc)
		if len(s.activeProcessors) != 0 || len(s.idleProcessors) != 1 {
			t.Fatal("single worker was not returned to idle pool")
		}
	}
}

type testEngine struct{}

func (testEngine) Recognize(context.Context, []byte) (paddleocr.Result, error) {
	return paddleocr.Result{}, nil
}
func (testEngine) Close() error { return nil }
