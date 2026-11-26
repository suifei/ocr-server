"""End-to-end check with installed PaddleOCR and a generated (not user) PNG."""
import argparse
import base64
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import tempfile
import time
import urllib.request


def port_for_test():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def request(url, payload=None):
    data = json.dumps(payload).encode("utf-8") if payload is not None else None
    req = urllib.request.Request(url, data, {"Content-Type": "application/json"})
    # This is a local smoke test; proxy settings should never route it remotely.
    with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(req, timeout=25) as response:
        return json.load(response)


def main():
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("--binary", type=Path, required=True)
    cli.add_argument("--python", type=Path, required=True)
    cli.add_argument("--det-model", type=Path)
    cli.add_argument("--rec-model", type=Path)
    args = cli.parse_args()
    binary, python = args.binary.absolute(), args.python.absolute()
    image = Path(__file__).with_name("ocr_fixture.png").absolute()
    image_payload = {"image_base64": base64.b64encode(image.read_bytes()).decode("ascii")}
    port = port_for_test()
    url = f"http://127.0.0.1:{port}"
    command = [
        str(binary), "-addr", "127.0.0.1", "-port", str(port),
        "-ocr-backend", "paddleocr", "-ocr-python", str(python),
        "-min-processors", "1", "-max-processors", "1",
    ]
    for flag, path in (("-ocr-det-model", args.det_model), ("-ocr-rec-model", args.rec_model)):
        if path is not None:
            command += [flag, str(path.absolute())]
    with tempfile.TemporaryDirectory(prefix="ocr-smoke-") as working:
        old = subprocess.run([
            str(binary), "-ocr-exe", "res/not-installed/PaddleOCR-json.exe",
            "-min-processors", "1", "-max-processors", "1",
        ], cwd=working, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=15)
        diagnostics = old.stdout.decode("utf-8", errors="replace")
        assert old.returncode != 0 and "已检查" in diagnostics and "-ocr-exe" in diagnostics, diagnostics
        print("PASS: legacy missing path gives checked locations and -ocr-exe guidance")
        with tempfile.TemporaryFile() as logs:
            options = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP} if os.name == "nt" else {}
            with subprocess.Popen(command, cwd=working, stdout=logs, stderr=subprocess.STDOUT, **options) as proc:
                try:
                    deadline = time.monotonic() + 185
                    while True:
                        if proc.poll() is not None:
                            raise RuntimeError(f"server exited: {proc.returncode}")
                        try:
                            request(url + "/stats")
                            break
                        except OSError:
                            if time.monotonic() >= deadline:
                                raise
                            time.sleep(0.25)
                    bad = request(url, {"image_base64": base64.b64encode(b"not a picture").decode("ascii")})
                    assert "error" in bad, bad
                    # With one worker, concurrent callers must queue and reuse it.
                    with ThreadPoolExecutor(max_workers=2) as pool:
                        responses = list(pool.map(
                            lambda body: request(url, body), [image_payload, {"image_path": str(image)}]
                        ))
                    for result in responses:
                        hit = next(item for item in result["data"] if item.get("text") == "OCR TEST 123")
                        assert 0 < hit["score"] <= 1 and len(hit["box"]) == 4, result
                    stats = request(url + "/stats")
                    assert stats["total_requests"] == 3 and stats["successful_requests"] == 2, stats
                    print("PASS: real local OCR via base64 and path, preserved box/score/text")
                    print(json.dumps({"responses": responses, "stats": stats}, ensure_ascii=False))
                except Exception:
                    logs.seek(0)
                    print(logs.read().decode("utf-8", errors="replace"))
                    raise
                finally:
                    if proc.poll() is None:
                        try:
                            proc.send_signal(signal.CTRL_BREAK_EVENT if os.name == "nt" else signal.SIGTERM)
                            proc.wait(timeout=10)
                        except (OSError, subprocess.TimeoutExpired):
                            proc.kill()
                            proc.wait(timeout=5)


if __name__ == "__main__":
    main()
