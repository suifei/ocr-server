.PHONY: all build run test clean

GO ?= go
GOOS := $(shell $(GO) env GOOS)
BIN := ocr-server
ifeq ($(GOOS),windows)
BIN := ocr-server.exe
endif

all: test build

build:
	$(GO) build -o $(BIN) ./cmd/server

run:
	$(GO) run ./cmd/server

test:
	$(GO) test ./...

clean:
	rm -f ocr-server ocr-server.exe
