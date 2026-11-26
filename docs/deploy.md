# 从 Release 安装与部署

下面针对 CPU 版：Linux x86-64（glibc，不是 Alpine/musl）和 Apple Silicon
macOS（macOS 13 或更新版本；13 是 Go 可执行文件的最低版本，Python 轮子也需要与
系统兼容）。使用原生 CPython 3.12。Release 内已有可执行文件，安装时**不需要 Go**；
Python 依赖和模型不随 ZIP 分发。Windows 的源码安装步骤仍见主 README。

## 下载、校验与运行

在 [Releases](https://github.com/suifei/ocr-server/releases/latest) 同一版本下下载
`SHA256SUMS.txt` 和你所用的平台包（文件名以 `linux-amd64.zip` 或
`macos-arm64.zip` 结尾）。`archive` 填入下载的文件名，再检查对应的 SHA-256：

```sh
archive=ocr-server-v1.1.1-linux-amd64.zip  # Mac 改为 ocr-server-v1.1.1-macos-arm64.zip
awk -v archive="$archive" '$2 == archive { print; matched=1 } END { if (!matched) exit 1 }' \
  SHA256SUMS.txt > ocr-archive.sha256 && shasum -a 256 -c ocr-archive.sha256
```

Linux 未安装 `shasum` 时可用 `sha256sum -c ocr-archive.sha256`。校验成功后：

```sh
mkdir ocr-server
unzip "$archive" -d ocr-server
cd ocr-server
python3.12 -c 'import platform; print(platform.machine())'
python3.12 -m venv .venv-ocr
.venv-ocr/bin/python -m pip install -r requirements-cpu.txt
./ocr-server -version
.venv-ocr/bin/python test/smoke_local.py --binary ./ocr-server --python .venv-ocr/bin/python
./ocr-server -addr 127.0.0.1 -ocr-python "$PWD/.venv-ocr/bin/python"
```

Python 在 Mac 上应显示 `arm64`，在 Linux x86-64 上应显示 `x86_64`。Mac 上如果
显示 `x86_64`，换用原生 Python 重新创建虚拟环境，不要复用 Rosetta 环境。
若 Linux 提示缺少 `libGL.so.1` 或 `libgthread-2.0.so.0`，安装发行版对应的
OpenGL/GLib 运行库（例如 Debian 或 Ubuntu 的 `libgl1` 与 `libglib2.0-0`
或其 `t64` 后继包）。已装好且仍报错时，先用虚拟环境的 Python 运行
`python -c 'import cv2, paddle; print(paddle.__version__)'` 检查缺的是哪一层。

首次启动默认模型需要联网下载。若已解压至本地，给冒烟测试传
`--det-model`、`--rec-model`，给服务传 `-ocr-det-model`、`-ocr-rec-model`，
均使用绝对路径；纯离线时另设 `PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True`。
冒烟测试用包内合成图实际跑两种 HTTP 输入，并检查坏图不影响后续请求。

## Linux：可选的 systemd 用户服务

先按上面的命令跑通。在普通用户的 `~/.config/systemd/user/ocr-server.service`
保存以下单元，将三处目录改为实际解压位置的绝对路径：

```ini
[Unit]
Description=Local OCR Server
StartLimitIntervalSec=600
StartLimitBurst=3

[Service]
Type=simple
WorkingDirectory=/home/your-user/ocr-server
ExecStart=/home/your-user/ocr-server/ocr-server -addr 127.0.0.1 -ocr-python /home/your-user/ocr-server/.venv-ocr/bin/python
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
```

```sh
systemctl --user daemon-reload
systemctl --user enable --now ocr-server.service
journalctl --user -u ocr-server.service -n 50 --no-pager
curl -fsS http://127.0.0.1:1111/stats
```

默认随用户会话运行；如需无人登录时开机运行，由管理员按该服务器策略配置 user
lingering 或专用非 root 服务账户。若要远程使用，优先通过 SSH 转发或在前面加鉴权；
此服务没有访问鉴权，且 `image_path` 指向服务器本地文件，不要直接公开端口。
