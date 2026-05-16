#!/usr/bin/env python3
from __future__ import annotations

import base64
import http.server
import importlib.util
import json
import pathlib
import tempfile
import threading
import unittest
import wave


REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]
SERVER_PATH = REPO_ROOT / "scripts" / "dev" / "tts-remote-server.py"


def load_server_module():
    spec = importlib.util.spec_from_file_location("tts_remote_server", SERVER_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load {SERVER_PATH}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class FakeGemini:
    def __init__(self, pcm: bytes):
        self.pcm = pcm
        self.requests: list[dict[str, object]] = []
        outer = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_POST(self) -> None:
                raw = self.rfile.read(int(self.headers.get("Content-Length", "0")))
                outer.requests.append(
                    {
                        "path": self.path,
                        "api_key": self.headers.get("x-goog-api-key"),
                        "body": json.loads(raw.decode("utf-8")),
                    }
                )
                payload = {
                    "candidates": [
                        {
                            "content": {
                                "parts": [
                                    {
                                        "inlineData": {
                                            "mimeType": "audio/L16;rate=24000",
                                            "data": base64.b64encode(outer.pcm).decode("ascii"),
                                        }
                                    }
                                ]
                            }
                        }
                    ]
                }
                data = json.dumps(payload).encode("utf-8")
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def log_message(self, fmt: str, *args: object) -> None:
                return

        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    @property
    def endpoint_template(self) -> str:
        host, port = self.server.server_address
        return f"http://{host}:{port}/v1beta/models/{{model}}:generateContent"

    def __enter__(self) -> "FakeGemini":
        self.thread.start()
        return self

    def __exit__(self, *unused: object) -> None:
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)


class GeminiTTSServerContract(unittest.TestCase):
    def setUp(self) -> None:
        self.module = load_server_module()

    def test_generates_wav_from_gemini_pcm_and_caches_it(self) -> None:
        pcm = b"\x10\x00\x20\x00\x30\x00\x40\x00"
        with tempfile.TemporaryDirectory() as tmp, FakeGemini(pcm) as gemini:
            wav_path = self.module.generate_wav(
                "안녕하세요",
                pathlib.Path(tmp),
                api_key="test-key",
                model="",
                voice="kore",
                endpoint_template=gemini.endpoint_template,
                sample_rate=24000,
            )
            cached_path = self.module.generate_wav(
                "안녕하세요",
                pathlib.Path(tmp),
                api_key="test-key",
                model="",
                voice="kore",
                endpoint_template=gemini.endpoint_template,
                sample_rate=24000,
            )

            self.assertEqual(wav_path, cached_path)
            self.assertEqual(len(gemini.requests), 1)
            request = gemini.requests[0]
            self.assertIn("gemini-3.1-flash-tts-preview:generateContent", request["path"])
            self.assertEqual(request["api_key"], "test-key")

            body = request["body"]
            self.assertEqual(body["contents"][0]["parts"][0]["text"], "안녕하세요")
            self.assertEqual(body["generationConfig"]["responseModalities"], ["AUDIO"])
            voice_name = body["generationConfig"]["speechConfig"]["voiceConfig"]["prebuiltVoiceConfig"]["voiceName"]
            self.assertEqual(voice_name, "Kore")

            with wave.open(str(wav_path), "rb") as wav:
                self.assertEqual(wav.getnchannels(), 1)
                self.assertEqual(wav.getsampwidth(), 2)
                self.assertEqual(wav.getframerate(), 24000)
                self.assertEqual(wav.readframes(wav.getnframes()), pcm)

    def test_requires_gemini_api_key(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(ValueError, "GEMINI_API_KEY"):
                self.module.generate_wav(
                    "hello",
                    pathlib.Path(tmp),
                    api_key=" ",
                    model="gemini-3.1-flash-tts-preview",
                    voice="Kore",
                    endpoint_template="http://127.0.0.1/{model}",
                    sample_rate=24000,
                )


if __name__ == "__main__":
    unittest.main()
