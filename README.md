

# OCR Server

This project implements a high-performance OCR (Optical Character Recognition) server using Go and PaddleOCR.

## Features

- Dynamic scaling of OCR processors based on load
- Concurrent processing of OCR tasks
- Health checking and automatic recovery of OCR processors
- RESTful API for OCR requests
- Server statistics endpoint

## Prerequisites

- Go 1.16 or higher
- PaddleOCR executable

## OCR服务器的代码结构

```
ocr-server/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── server/
│   │   ├── server.go
│   │   ├── handlers.go
│   │   ├── processor.go
│   │   └── stats.go
│   └── utils/
│       └── logger.go
├── pkg/
│   └── ocrengine/
│       └── ocrengine.go
├── go.mod
├── go.sum
└── README.md
```


每个文件的主要内容:

1. cmd/server/main.go:
   这个文件将包含主函数,负责解析命令行参数,初始化配置,并启动服务器。

2. internal/config/config.go:
   这里将定义Config结构体和相关的配置加载函数。

3. internal/server/server.go:
   这个文件将包含Server结构体的定义和核心服务器逻辑,如Initialize(), Start(), waitForShutdown()等方法。

4. internal/server/handlers.go:
   这里将定义HTTP请求处理函数,如handleOCR()。

5. internal/server/processor.go:
   这个文件将包含OCRProcessor结构体和相关的处理方法,如createOCRProcessor(), getAvailableProcessor(), releaseProcessor()等。

6. internal/server/stats.go:
   这里将定义与服务器统计相关的函数,如GetStats()。

7. internal/utils/logger.go:
   这个文件将包含日志相关的设置和工具函数。

8. pkg/ocrengine/ocrengine.go:
   这里将封装与PaddleOCR交互的逻辑,包括OCR处理和错误处理。
   


## Installation

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/ocr-server.git
   ```

2. Change to the project directory:
   ```
   cd ocr-server
   ```

3. Install dependencies:
   ```
   go mod download
   ```

## Configuration

The server can be configured using command-line flags. Run the server with `-help` to see all available options.

## Usage

To start the server:

```
go run cmd/server/main.go
```

To perform OCR on an image, send a POST request to the server:

```
curl -X POST -H "Content-Type: application/json" -d '{"image_path":"/path/to/image.jpg"}' http://localhost:1111
```

To get server statistics:

```
curl http://localhost:1111/stats
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.