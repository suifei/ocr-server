"""Embedded stdin/stdout transport for local PaddleOCR 3.7."""
import base64
import json
import os
import sys

# Keep third-party diagnostics, including native stdout, out of the JSON stream.
protocol = os.fdopen(os.dup(sys.stdout.fileno()), "w", encoding="utf-8", newline="\n")
os.dup2(sys.stderr.fileno(), sys.stdout.fileno())


def send(**response):
    protocol.write(json.dumps(response, ensure_ascii=False, allow_nan=False) + "\n")
    protocol.flush()


try:
    import cv2
    import numpy as np
    from paddleocr import PaddleOCR

    ocr = PaddleOCR(
        text_detection_model_name="PP-OCRv6_small_det",
        text_recognition_model_name="PP-OCRv6_small_rec",
        text_detection_model_dir=sys.argv[1] or None,
        text_recognition_model_dir=sys.argv[2] or None,
        use_doc_orientation_classify=False,
        use_doc_unwarping=False,
        use_textline_orientation=False,
        device="cpu",
        enable_mkldnn=False,
        cpu_threads=2,
    )
except Exception as exc:
    send(error=f"{type(exc).__name__}: {exc}")
    raise SystemExit(1)

send(ready=True)
for line in sys.stdin:
    try:
        request = json.loads(line)
        image_bytes = base64.b64decode(request["image"], validate=True)
        image = cv2.imdecode(np.frombuffer(image_bytes, dtype=np.uint8), cv2.IMREAD_COLOR)
        if image is None:
            send(code=400, message="无法解码图像", data=[])
            continue
        lines = []
        for result in ocr.predict(image):
            for text, score, poly in zip(
                result["rec_texts"], result["rec_scores"], result["rec_polys"], strict=True
            ):
                lines.append(
                    dict(text=text, score=float(score), box=[[int(x), int(y)] for x, y in poly])
                )
        if lines:
            send(code=100, message="", data=lines)
        else:
            send(code=101, message="未识别到文字", data=[])
    except Exception as exc:
        print(f"OCR failed: {type(exc).__name__}: {exc}", file=sys.stderr)
        send(code=500, message="PaddleOCR 本地推理失败", data=[])
