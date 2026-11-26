# OCR 服务器

Go 编写的本地 OCR HTTP 服务。默认使用 **PaddleOCR 3.7.0 + PP-OCRv6 small**（CPU），仍支持 Windows 下的旧版 PaddleOCR-json。原有 HTTP 接口不变。新后端保留原图色彩，每个工作进程加载一次模型并重复使用；图片不发送给云端推理。

## 安装与启动

需要 Go 1.22.3+（仅构建时）和对应平台的 CPython 3.12。Python 依赖安装在项目独立虚拟环境，Go 可执行文件不包含 Python 或模型。

| 平台 | 支持范围 | 启动方式 |
|---|---|---|
| Windows | x86-64、64 位 Python | PowerShell |
| Linux | x86-64、glibc；若 OpenCV 缺少动态库，安装发行版提供的 libGL/libglib | Shell |
| macOS | Apple Silicon、原生 arm64 Python | Shell |

[PaddlePaddle 3.3.1 官方 PyPI 文件](https://pypi.org/project/paddlepaddle/3.3.1/#files)提供上述 CPython 3.12 轮子。未把 Intel Mac、Windows ARM 或其他架构纳入这一版本的 Python 运行时支持；仅 Go 可执行文件能编译不等于可推理。

### Linux / Apple Silicon macOS

```sh
python3.12 -m venv .venv-ocr
.venv-ocr/bin/python -m pip install -r requirements-cpu.txt
go build -o ocr-server ./cmd/server
./ocr-server -ocr-python "$PWD/.venv-ocr/bin/python"
```

### Windows PowerShell

```powershell
py -3.12 -m venv .venv-ocr
& .\.venv-ocr\Scripts\python.exe -m pip install -r requirements-cpu.txt
go build -o ocr-server.exe ./cmd/server
.\ocr-server.exe -ocr-python (Resolve-Path .\.venv-ocr\Scripts\python.exe).Path
```

首次使用默认模型会下载官方检测/识别模型。网络受限时可在 Python 环境设置 `PADDLE_PDX_MODEL_SOURCE=bos` 使用飞桨模型源；不要将环境变量直接写到个人 shell 配置。对于已有、解压后的官方模型，可通过 `-ocr-det-model` 和 `-ocr-rec-model` 指定对应目录。源连通性检查与模型下载是两件事：已备齐本地模型且要求离线启动时，可另设 `PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True`。

## 接口

默认监听 `localhost:1111`。`POST /` 接受下面两种 JSON 输入（路径是**服务器本地**文件，建议用绝对路径）：

```json
{"image_path": "/path/to/image.png"}
```

```json
{"image_base64": "<base64>"}
```

成功返回 `{"data":[{"box":[[0,0],[10,0],[10,8],[0,8]],"score":0.98,"text":"示例"}]}`。无文字或识别失败仍通过 `error` 字段报告。`GET /stats` 返回统计。此 API 没有访问鉴权，默认只绑回环；不要直接向不受信任客户端暴露，尤其是支持服务器本地读取的 `image_path` 接口。

## 配置与旧版兼容

配置文件是启动目录下的 `.ocr-server/config.yaml`，启动参数可覆盖。换工作目录时推荐显式传绝对的 Python、模型或旧引擎路径。

| 键 | 对应参数 | 默认值／说明 |
|---|---|---|
| `ocr_backend` | `-ocr-backend` | `paddleocr`，或 `legacy` |
| `ocr_python` | `-ocr-python` | 解释器路径；未指定时从 PATH 查找 |
| `ocr_det_model` / `ocr_rec_model` | `-ocr-det-model` / `-ocr-rec-model` | 可选，本地官方模型目录 |
| `ocr_exe_path` | `-ocr-exe` | 仅 legacy：缺省 `res/PaddleOCR-json_v1.4.0/PaddleOCR-json.exe` |
| `min_processors` / `max_processors` | `-min-processors` / `-max-processors` | 新生成配置 1 / 至多 2；每个工作进程占用单独模型内存 |
| `queue_size` / `warm_up_count` | `-queue-size` / `-warm-up-count` | 100 / 0 |
| `threshold_mode` / `threshold_value` | `-threshold-mode` / `-threshold-value` | 仅 legacy，0（固定阈值）/ 100；模式 1 为 Otsu |

老 YAML 无 `ocr_backend` 但非空 `ocr_exe_path` 时继续选择 legacy。升级该类配置请加 `ocr_backend: paddleocr`，填写 Python 路径，并从 `min_processors: 1`、`max_processors: 2`、`warm_up_count: 0` 起用。显式 `-ocr-exe` 仍选择 legacy，除非同时明确指定后端。legacy 需自行完整解压 [PaddleOCR-json Windows 引擎包](https://github.com/hiroi-sora/PaddleOCR-json/releases/tag/v1.4.0)，而不是只复制 exe；没有默认自动下载旧 exe 的流程。查找相对路径先用启动目录，再用服务器可执行文件目录；未找到时日志列出检查过的路径及 `-ocr-exe` 绝对路径解决办法。

## 开发与验证

```sh
go test ./...
go vet ./...
```

测试包含旧引擎路径定位、工作进程协议与超时，以及单 worker 池复用。Windows/macOS/Linux 的 Go 构建和每个平台的真实模型推理应分别验证。已安装运行时后，可用 `python test/smoke_local.py --binary /path/to/ocr-server --python /path/to/python` 启动一次隔离的回环服务，验证上传、本地路径输入及旧版缺失路径提示。推理失败请先核对解释器架构、依赖版本、模型目录和启动日志。

## 致谢

OCR 服务器项目的开发离不开众多优秀的开源项目和社区的支持。我们在此特别感谢以下项目和贡献者：

- [Go 语言](https://golang.org/)：感谢 Go 团队开发了这门高效、简洁的编程语言，为我们提供了强大的开发工具和丰富的标准库。

- [PaddleOCR](https://github.com/PaddlePaddle/PaddleOCR)：感谢百度开源的这个强大的 OCR 工具箱，为我们提供了高精度的文字识别能力。

- [PaddleOCR-json](https://github.com/hiroi-sora/PaddleOCR-json)：特别感谢 hiroi-sora 开发的这个项目，它为 PaddleOCR 提供了便捷的 JSON 接口，大大简化了我们的集成工作。

感谢 [@ximengxuan](https://github.com/ximengxuan) 提交 Windows 引擎启动报告 [#4](https://github.com/suifei/ocr-server/issues/4)，以及社区对 Linux 与 Apple Silicon 的支持需求。

我们还要感谢以下 Go 语言库的作者和维护者：

- [viper](https://github.com/spf13/viper)：优秀的配置解决方案。
- [validator](https://github.com/go-playground/validator)：强大的数据验证库。
- [lumberjack](https://github.com/natefinch/lumberjack)：可靠的日志轮转工具。
- [paddleocr](https://github.com/doraemonkeys/paddleocr)：一个简单易用的 PaddleOCR-json golang 客户端。
- [yaml](gopkg.in/yaml.v2)：YAML 配置文件的解析库。

这些开源项目为我们的开发工作提供了宝贵的支持，使得 OCR 服务器的开发成为可能。我们深深感谢这些项目的贡献者们付出的努力和分享精神。

同时，我们也要感谢所有为这个项目提供反馈、建议和贡献的用户和开发者。你们的参与让这个项目不断进步和完善。

如果在使用过程中发现任何问题或有任何改进建议，欢迎在 GitHub 上提出 issue 或 pull request。让我们一起努力，使这个项目变得更好！

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。
