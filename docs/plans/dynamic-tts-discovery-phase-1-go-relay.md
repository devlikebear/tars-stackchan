# Phase 1 — Go TTS Relay (Python 파리티)

**목표:** `scripts/dev/tts-remote-server.py`와 동등한 Go 릴레이를
`tars-stackchan-control tts serve` 서브커맨드로 제공. Python 릴레이는 이 페이즈에서는
**그대로 둔다**(폴백). 끝나면 Go 릴레이만으로 디바이스 음성이 난다.

**예상:** Medium~Hard (12-16h). 외부 API 연동 + 오디오 포맷, 단 전부 stdlib.

## 컨벤션 제약 (반드시 준수)

- `mcp-server/go.mod` 외부 의존성 **0개 유지**. stdlib만: `net/http`, `crypto/hmac`,
  `crypto/subtle`, `encoding/binary`, `encoding/json`, `os/exec`(Phase 2), `flag`.
- CLI 디스패치는 `cmd/tars-stackchan-control/main.go`의 `runCommand` switch 패턴 확장.
  기존 `case "mock"`, `case "http"` 와 같은 결로 `case "tts"` 추가.
- 비즈니스 로직은 `mcp-server/internal/tts/` 신규 패키지에 둔다 (기존 `internal/bridge`,
  `internal/stackchan` 와 동일한 internal 모듈 경계 관례).
- 에러: `fmt.Errorf("...: %w", err)` 래핑 (기존 `internal/bridge/http/http.go` 패턴).
- 테스트: 동 디렉토리 `_test.go`, table-driven.

## Python 릴레이 파리티 체크리스트 (출처: `scripts/dev/tts-remote-server.py`)

이식해야 할 동작:
- `resolve_api_key`: `--api-key` → `TARS_STACKCHAN_GEMINI_API_KEY` → `GEMINI_API_KEY`
- `resolve_tts_token`: `--token` → `TARS_STACKCHAN_TTS_TOKEN` → `TARS_STACKCHAN_TOKEN`
- `normalize_voice`: `GEMINI_VOICES` 맵(소문자 키 → 정규명), 미존재 시 입력 그대로
- `render_endpoint`: `{model}`/`%s` 템플릿에 URL-encoded model 치환
- `build_gemini_payload`: `responseModalities:["AUDIO"]` + `prebuiltVoiceConfig.voiceName`
- `is_authorized`: `hmac.compare_digest`(상수시간) — Go는 `crypto/subtle.ConstantTimeCompare`
- `redact_path`: 로그에서 `token` 쿼리값을 `<redacted>`로 치환
- `synthesize_gemini_pcm` → `write_wav`: PCM → WAV(24000Hz, mono, 16-bit) 헤더 생성
- 라우트: `GET /health` → `ok`, `GET /api/tts?token=&text=` → WAV body
- 캐시: `TARS_STACKCHAN_TTS_CACHE`(기본 `.work/tts-cache`) 디렉토리 재사용
- 서버: ThreadingHTTPServer 동등 → Go `http.Server`(기본 동시성으로 충족)
- 플래그/env 기본값: `--host 0.0.0.0`, `--port 18080`(`TARS_STACKCHAN_TTS_PORT`),
  `--model`(`TARS_STACKCHAN_TTS_MODEL`), `--voice`(`TARS_STACKCHAN_TTS_VOICE`),
  `--sample-rate 24000`, `--prompt-prefix`(`TARS_STACKCHAN_TTS_PROMPT_PREFIX`)

## 작업

- [ ] `mcp-server/internal/tts/voices.go` — `var geminiVoices = map[string]string{...}`
  - `tts-remote-server.py`의 `GEMINI_VOICES` 전체를 그대로 옮긴다 (대소문자 키 동일)
  - `func NormalizeVoice(v string) string` — 소문자 lookup, 미존재 시 trim 후 원본
  - 테스트: `voices_test.go` — known voice / unknown passthrough / 공백 trim 3케이스

- [ ] `mcp-server/internal/tts/config.go` — `type Config struct{...}` + `func Resolve(...)`
  - api key / token / model / voice / port / sampleRate / promptPrefix /
    endpointTemplate / cacheDir 해석. 우선순위는 파리티 체크리스트대로
  - `func ResolveAPIKey(explicit string) string`, `ResolveToken(explicit string) string`
  - 테스트: env 우선순위 table-driven (`config_test.go`)

- [ ] `mcp-server/internal/tts/gemini.go` — Gemini 합성
  - `func RenderEndpoint(tmpl, model string) string` ({model}/%s 치환, url.QueryEscape)
  - `func BuildPayload(text, voice string) ([]byte, error)` (JSON, py와 동일 구조)
  - `func SynthesizePCM(ctx, cfg, text string) ([]byte, error)` — `http.Client` POST,
    base64 audio 추출, `fmt.Errorf` 래핑. 비-200은 본문 일부 포함 에러
  - 테스트: `gemini_test.go` — `httptest.Server`로 mock Gemini, 정상/4xx/파싱실패

- [ ] `mcp-server/internal/tts/wav.go` — `func WritePCMAsWAV(w io.Writer, pcm []byte, sampleRate int) error`
  - 44-byte RIFF/WAVE 헤더 + PCM. `encoding/binary` LittleEndian. mono, 16-bit
  - 테스트: `wav_test.go` — 헤더 매직(`RIFF`/`WAVE`/`fmt `/`data`), 길이 필드 정확성

- [ ] `mcp-server/internal/tts/server.go` — HTTP 핸들러
  - `func NewHandler(cfg Config) http.Handler` — `/health`, `/api/tts`
  - `func authorized(q url.Values, token string) bool` — `subtle.ConstantTimeCompare`
  - `func redactPath(raw string) string` — token 쿼리값 `<redacted>`
  - 캐시: `cacheDir`에 `sha256(model|voice|text)` 키로 WAV read-through
  - 요청 로깅 포맷은 `tts-remote-server.py`의 `'%s - "%s %s" %d %d'` 와 동등하게
  - 테스트: `server_test.go` — 무토큰 401, 정상 200+WAV, /health, redact, 캐시 히트

- [ ] `mcp-server/internal/tts/serve.go` — `func Serve(ctx, cfg Config, host string, port int) error`
  - `http.Server` 구성, graceful shutdown(SIGINT/SIGTERM), 시작 시
    `tts serve listening http://host:port/api/tts` stderr 안내(py `main` 동등)

- [ ] `cmd/tars-stackchan-control/main.go` — `case "tts":` 추가
  - 기존 `runCommand` switch에 `case "tts": return runTTS(rest, stdout, stderr)`
  - `runTTS`: 1st arg `serve` 디스패치(`install`/`status`는 Phase 3 placeholder 에러)
  - `flag.NewFlagSet("tts serve", ...)` — `--host --port --token --api-key --model
    --voice --prompt-prefix --cache-dir`. 미지정 시 `internal/tts.Resolve`
  - `printUsage`(main.go:118)에 `tts serve` 라인 추가
  - 테스트: `main_test.go` — `tts` 없는 인자 usage, `tts serve --help` 정상 종료

- [ ] `scripts/test/tts-relay-go-contract.sh` 신규 (기존 `tts-remote-server-contract.sh` 패턴 모방)
  - Go 바이너리 빌드 → mock Gemini(또는 `--api-key` 가짜 + 네트워크 차단 경로) 없이
    `/health` 200 확인 + 무토큰 401 확인까지(외부망 불요 범위)
  - `.github/workflows/ci.yml`의 "Test TTS relay contract" step 다음에 동 step 추가

## ✅ Checkpoint: Phase 1 완료 확인

**구현 확인:**
- [ ] `cd mcp-server && go build ./... && go vet ./...` 무에러
- [ ] `cd mcp-server && go test ./...` 전체 통과 (신규 `internal/tts` 포함)
- [ ] `go.mod` 에 require 블록 없음 (외부 의존성 0 유지)
- [ ] `scripts/test/tts-relay-go-contract.sh` 통과

**실행 확인:**
- [ ] `tars-stackchan-control tts serve --port 18080` 기동 →
      `curl -s localhost:18080/health` → `ok`
- [ ] `curl -s "localhost:18080/api/tts?token=$TOKEN&text=test" -o /tmp/go.wav -w '%{http_code}'`
      → `200`, `file /tmp/go.wav` → `WAVE audio ... 24000 Hz`
- [ ] 무토큰 요청 → `401`
- [ ] 로그에 토큰 평문 미노출(`<redacted>` 확인)

**수동 확인:**
- [ ] Python 릴레이 정지 + Go 릴레이만 기동 상태에서 `stackchan_speak` →
      Stack-chan이 실제로 말한다 (디바이스는 아직 IP/기존 설정 사용 — 같은 포트면 동작)

**통과 시 사용자 확인 후 Phase 2로. 실패 시 실패 항목 보고 → 원인 파악 → 수정 → 재검증.**
