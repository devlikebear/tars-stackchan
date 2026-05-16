#!/usr/bin/env python3
"""Tiny Gemini TTS backed WAV server for Stack-chan remote TTS."""

from __future__ import annotations

import argparse
import base64
import hmac
import hashlib
import html
import http.server
import json
import os
import pathlib
import urllib.error
import urllib.request
import urllib.parse
import wave


DEFAULT_GEMINI_MODEL = "gemini-3.1-flash-tts-preview"
DEFAULT_GEMINI_ENDPOINT_TEMPLATE = "https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent"
DEFAULT_GEMINI_VOICE = "Kore"
GEMINI_TIMEOUT_SECONDS = 30

GEMINI_VOICES = {
    "zephyr": "Zephyr",
    "puck": "Puck",
    "charon": "Charon",
    "kore": "Kore",
    "fenrir": "Fenrir",
    "leda": "Leda",
    "orus": "Orus",
    "aoede": "Aoede",
    "callirrhoe": "Callirrhoe",
    "autonoe": "Autonoe",
    "enceladus": "Enceladus",
    "iapetus": "Iapetus",
    "umbriel": "Umbriel",
    "algieba": "Algieba",
    "despina": "Despina",
    "erinome": "Erinome",
    "algenib": "Algenib",
    "rasalgethi": "Rasalgethi",
    "laomedeia": "Laomedeia",
    "achernar": "Achernar",
    "alnilam": "Alnilam",
    "schedar": "Schedar",
    "gacrux": "Gacrux",
    "pulcherrima": "Pulcherrima",
    "achird": "Achird",
    "zubenelgenubi": "Zubenelgenubi",
    "vindemiatrix": "Vindemiatrix",
    "sadachbia": "Sadachbia",
    "sadaltager": "Sadaltager",
    "sulafat": "Sulafat",
}


def resolve_api_key(explicit: str | None = None) -> str:
    return (explicit or os.environ.get("TARS_STACKCHAN_GEMINI_API_KEY") or os.environ.get("GEMINI_API_KEY") or "").strip()


def resolve_tts_token(explicit: str | None = None) -> str:
    return (explicit or os.environ.get("TARS_STACKCHAN_TTS_TOKEN") or os.environ.get("TARS_STACKCHAN_TOKEN") or "").strip()


def normalize_model(model: str | None) -> str:
    return (model or DEFAULT_GEMINI_MODEL).strip() or DEFAULT_GEMINI_MODEL


def normalize_voice(voice: str | None) -> str:
    voice = (voice or DEFAULT_GEMINI_VOICE).strip()
    return GEMINI_VOICES.get(voice.lower(), voice)


def render_endpoint(endpoint_template: str, model: str) -> str:
    encoded_model = urllib.parse.quote(model, safe="")
    if "{model}" in endpoint_template:
        return endpoint_template.format(model=encoded_model)
    if "%s" in endpoint_template:
        return endpoint_template % encoded_model
    return endpoint_template


def build_gemini_payload(text: str, voice: str) -> dict[str, object]:
    return {
        "contents": [{"parts": [{"text": text}]}],
        "generationConfig": {
            "responseModalities": ["AUDIO"],
            "speechConfig": {
                "voiceConfig": {
                    "prebuiltVoiceConfig": {
                        "voiceName": voice,
                    }
                }
            },
        },
    }


def is_authorized(params: dict[str, list[str]], token: str) -> bool:
    expected = token.strip()
    provided = params.get("token", [""])[0].strip()
    return bool(expected) and hmac.compare_digest(provided, expected)


def redact_path(path: str) -> str:
    parsed = urllib.parse.urlparse(path)
    pairs = urllib.parse.parse_qsl(parsed.query, keep_blank_values=True)
    redacted = [("token", "<redacted>") if key == "token" else (key, value) for key, value in pairs]
    query = urllib.parse.urlencode(redacted)
    return urllib.parse.urlunparse(parsed._replace(query=query))


def synthesize_gemini_pcm(
    text: str,
    *,
    api_key: str,
    model: str,
    voice: str,
    endpoint_template: str,
) -> bytes:
    api_key = resolve_api_key(api_key)
    if not api_key:
        raise ValueError("GEMINI_API_KEY or TARS_STACKCHAN_GEMINI_API_KEY is required")

    model = normalize_model(model)
    voice = normalize_voice(voice)
    url = render_endpoint(endpoint_template, model)
    payload = json.dumps(build_gemini_payload(text, voice)).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=payload,
        headers={
            "Content-Type": "application/json",
            "x-goog-api-key": api_key,
        },
        method="POST",
    )

    try:
        with urllib.request.urlopen(request, timeout=GEMINI_TIMEOUT_SECONDS) as response:
            raw = response.read()
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"gemini http {error.code}: {body}") from error
    except urllib.error.URLError as error:
        raise RuntimeError(f"gemini request failed: {error.reason}") from error

    try:
        parsed = json.loads(raw.decode("utf-8"))
    except json.JSONDecodeError as error:
        raise RuntimeError("gemini response was not valid JSON") from error

    api_error = parsed.get("error")
    if isinstance(api_error, dict):
        status = api_error.get("status", "UNKNOWN")
        message = api_error.get("message", "")
        raise RuntimeError(f"gemini api error {status}: {message}")

    candidates = parsed.get("candidates") or []
    parts = (((candidates[0] or {}).get("content") or {}).get("parts") or []) if candidates else []
    inline = None
    for part in parts:
        inline = part.get("inlineData") or part.get("inline_data")
        if inline and inline.get("data"):
            break
    if not inline or not inline.get("data"):
        raise RuntimeError("gemini response did not include inline audio data")

    try:
        pcm = base64.b64decode(inline["data"])
    except (TypeError, ValueError) as error:
        raise RuntimeError("gemini inline audio data was not valid base64") from error
    if not pcm:
        raise RuntimeError("gemini inline audio data decoded to an empty payload")
    return pcm


def write_wav(path: pathlib.Path, pcm: bytes, sample_rate: int) -> None:
    with wave.open(str(path), "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(sample_rate)
        wav.writeframes(pcm)


def generate_wav(
    text: str,
    cache_dir: pathlib.Path,
    *,
    api_key: str,
    model: str,
    voice: str,
    endpoint_template: str,
    sample_rate: int,
    prompt_prefix: str = "",
) -> pathlib.Path:
    cache_dir.mkdir(parents=True, exist_ok=True)
    model = normalize_model(model)
    voice = normalize_voice(voice)
    prompt = f"{prompt_prefix}{text}" if prompt_prefix else text
    key = hashlib.sha256(f"{model}\0{voice}\0{sample_rate}\0{prompt}".encode("utf-8")).hexdigest()
    wav_path = cache_dir / f"{key}.wav"
    if wav_path.exists():
        return wav_path

    pcm = synthesize_gemini_pcm(
        prompt,
        api_key=api_key,
        model=model,
        voice=voice,
        endpoint_template=endpoint_template,
    )
    tmp_wav_path = cache_dir / f".{key}.tmp.wav"
    try:
        write_wav(tmp_wav_path, pcm, sample_rate)
        tmp_wav_path.replace(wav_path)
    finally:
        tmp_wav_path.unlink(missing_ok=True)
    return wav_path


class Handler(http.server.BaseHTTPRequestHandler):
    cache_dir: pathlib.Path
    api_key: str
    token: str
    model: str
    voice: str
    endpoint_template: str
    sample_rate: int
    prompt_prefix: str

    def do_GET(self) -> None:
        parsed = urllib.parse.urlparse(self.path)
        if parsed.path == "/health":
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.end_headers()
            self.wfile.write(b"ok\n")
            return

        if parsed.path != "/api/tts":
            self.send_error(404, "not found")
            return

        params = urllib.parse.parse_qs(parsed.query)
        if not is_authorized(params, self.token):
            self.send_response(401)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.end_headers()
            self.wfile.write(b"invalid token\n")
            return

        text = params.get("text", [""])[0].strip()
        if not text:
            self.send_error(400, "text is required")
            return
        if len(text) > 240:
            self.send_error(400, "text must be 240 characters or fewer")
            return

        try:
            wav_path = generate_wav(
                text,
                self.cache_dir,
                api_key=self.api_key,
                model=self.model,
                voice=self.voice,
                endpoint_template=self.endpoint_template,
                sample_rate=self.sample_rate,
                prompt_prefix=self.prompt_prefix,
            )
        except (OSError, RuntimeError, ValueError) as error:
            self.send_error(500, html.escape(str(error)))
            return

        payload = wav_path.read_bytes()
        self.send_response(200)
        self.send_header("Content-Type", "audio/wav")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, fmt: str, *args: object) -> None:
        print(f"{self.address_string()} - {fmt % args}")

    def log_request(self, code: int | str = "-", size: int | str = "-") -> None:
        print(f'{self.address_string()} - "{self.command} {redact_path(self.path)} {self.request_version}" {code} {size}')


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="0.0.0.0")
    parser.add_argument("--port", type=int, default=int(os.environ.get("TARS_STACKCHAN_TTS_PORT", "18080")))
    parser.add_argument("--api-key", default=resolve_api_key())
    parser.add_argument("--token", default=resolve_tts_token())
    parser.add_argument("--model", default=os.environ.get("TARS_STACKCHAN_TTS_MODEL", DEFAULT_GEMINI_MODEL))
    parser.add_argument("--voice", default=os.environ.get("TARS_STACKCHAN_TTS_VOICE", DEFAULT_GEMINI_VOICE))
    parser.add_argument("--gemini-endpoint-template", default=os.environ.get("TARS_STACKCHAN_GEMINI_ENDPOINT", DEFAULT_GEMINI_ENDPOINT_TEMPLATE))
    parser.add_argument("--sample-rate", type=int, default=24000)
    parser.add_argument("--prompt-prefix", default=os.environ.get("TARS_STACKCHAN_TTS_PROMPT_PREFIX", ""))
    parser.add_argument(
        "--cache-dir",
        type=pathlib.Path,
        default=pathlib.Path(os.environ.get("TARS_STACKCHAN_TTS_CACHE", ".work/tts-cache")),
    )
    args = parser.parse_args()
    if not resolve_tts_token(args.token):
        parser.error("TARS_STACKCHAN_TTS_TOKEN or TARS_STACKCHAN_TOKEN is required")

    Handler.cache_dir = args.cache_dir
    Handler.api_key = resolve_api_key(args.api_key)
    Handler.token = resolve_tts_token(args.token)
    Handler.model = normalize_model(args.model)
    Handler.voice = args.voice
    Handler.endpoint_template = args.gemini_endpoint_template
    Handler.sample_rate = args.sample_rate
    Handler.prompt_prefix = args.prompt_prefix

    server = http.server.ThreadingHTTPServer((args.host, args.port), Handler)
    print(f"TARS Stack-chan Gemini TTS: http://{args.host}:{args.port}/api/tts?text=hello")
    print(f"Gemini model: {Handler.model}, voice: {normalize_voice(Handler.voice)}")
    server.serve_forever()


if __name__ == "__main__":
    main()
