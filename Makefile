.PHONY: all clean
build:
	go build -o ocr-server.exe ./cmd/server/main.go

run:
	go run ./cmd/server/main.go

test:
	go build -o ocr-server.exe ./cmd/server/main.go && ./ocr-server.exe
	
clean:
	rm -rf ocr-server.exe

all: clean build test