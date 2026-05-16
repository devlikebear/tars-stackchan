# Phase 2 — 퍼셉션 루프 + TARS 감각 채널 (Closed Loop)

> 로드맵: [`embodied-bot-roadmap.md`](./embodied-bot-roadmap.md) · 분석: [`codebase-analysis.md`](./codebase-analysis.md)

## 목표

tars-stackchan에 백그라운드 퍼셉션 루프를 추가하고, 관측을 TARS의 기존 webhook inbound 채널로 보내 TARS가 페르소나로 반응(말/표정)하게 한다. **owner 식별은 아직 없음** — "누군가/무언가 있다" 수준.

**끝나면 동작하는 것**: 봇 앞에서 움직이거나 소리내면, 잠시 후 Stack-chan이 TARS의 판단대로 표정·음성으로 반응한다 (예: 쳐다보고 "어, 누구 왔나?").

## 작업

### 2.1 퍼셉션 루프 서브커맨드

- [ ] `mcp-server/cmd/tars-stackchan-control/main.go`의 `runCommand` switch에 `perceive` 서브커맨드 추가 (기존 `tts` 서브커맨드 디스패치 패턴 모방)
- [ ] `mcp-server/internal/perception/` 신규 패키지 (퍼셉션 코드 격리 — codebase-analysis §4)
  - `loop.go`: `Run(ctx, Config) error` — 이벤트 트리거 + 주기 스냅샷 루프
    - `GET /v1/sensors` 폴링(예: 1s), 모션/사운드 임계 초과 시 트리거
    - 트리거 없어도 `idle_interval`(예: 30s)마다 1회 스냅샷 (주기 스냅샷)
    - 트리거 시: `CameraSnapshot` + `AudioClip` 캡처 → 관측 1건 구성
    - 디바운스/레이트리밋(예: 최소 트리거 간격, 시간당 상한)으로 LLM 비용·소음 억제
  - `config.go`: env 기반 설정 (`TARS_STACKCHAN_PERCEIVE_*`), 기존 tts config 패턴 모방
- [ ] Phase 1의 `Bridge` 캡처 메서드만 사용 (펌웨어 직접 호출 금지 — 레이어 유지)

### 2.2 관측 구성 + 멀티모달 요약

- [ ] `mcp-server/internal/perception/observe.go`
  - `type Observation struct { TS int64; Trigger string; Summary string; Salience float64; ImageRef, AudioRef string }`
  - Gemini 멀티모달 호출로 스냅샷+오디오를 **짧은 자연어 관측 요약**으로 변환 (기존 `internal/tts/gemini.go`의 Gemini 호출·키 관리 패턴 모방, `GEMINI_API_KEY` 재사용)
  - 원시 미디어는 로컬 캐시에만 저장(경로만 ref). TARS로는 **요약 텍스트 위주** 전송 (개인정보 정책, codebase-analysis §5)
- [ ] 캐시 디렉토리 컨벤션은 기존 TTS relay 캐시 패턴 모방 (절대 user cache 경로)

### 2.3 TARS 감각 채널 송신

- [ ] `mcp-server/internal/perception/sink_tars.go`
  - `PostObservation(ctx, Observation) error` — TARS `/v1/channels/webhook/inbound/<channel>`로 POST
  - 페이로드: 관측 요약 + 메타(트리거 종류, 시각, salience)를 webhook 채널이 받는 형식으로 매핑
  - 엔드포인트/토큰/채널명은 env 설정 (`TARS_STACKCHAN_TARS_BASE_URL`, `..._WEBHOOK_CHANNEL`, 인증 토큰)
  - 실패 시 백오프 재시도 + 드롭(루프가 막히지 않게)

### 2.4 [tars] 페르소나 + 채널 배선 — 크로스repo 핸드오프 (이번 미실행)

> TARS repo (`/Users/changheonshin/workspace/myworks/tars`) 작업. tars-stackchan과 **분리 커밋**. 이번 세션 범위 아님 — 아래 **확정 계약**으로 TARS 측에서 후속 진행.

**확정 webhook 페이로드 계약** (tars-stackchan이 POST하는 형식, `internal/perception/sink_tars.go` 구현 일치):

```
POST {TARS_STACKCHAN_TARS_BASE_URL}/v1/channels/webhook/inbound/{TARS_STACKCHAN_TARS_WEBHOOK_CHANNEL}
Authorization: Bearer {TARS_STACKCHAN_TARS_TOKEN}   # 설정 시
Content-Type: application/json

{ "source":"stackchan", "ts":<unix_ms>, "trigger":"event|idle",
  "salience":0..1, "summary":"<자연어>", "text":"<summary 동일>",
  "image_ref":"obs-<ts>.jpg", "audio_ref":"obs-<ts>.wav" }
```

- `text`는 `summary`와 동일 — TARS 범용 webhook 채널이 Stack-chan 전용 파싱 없이 메시지 본문으로 라우팅하도록.
- `image_ref/audio_ref`는 **로컬 캐시 파일명만** (원시 미디어 비전송, 개인정보 정책).
- TARS 측 할 일(분리 커밋): inbound webhook 채널 1개 → "Stack-chan 페르소나" 세션 라우팅, 페르소나 정의(`internal/prompt`/`sysprompt`/`session_style`), 출력은 기존 `stackchan_*` MCP 툴(`config --target tars`). 코어 수정 불가피 시 phase 문서 하단 기록 후 사용자 확인.

- [ ] TARS에 webhook inbound 채널 1개 설정 — 들어온 관측이 "Stack-chan 페르소나" 세션으로 라우팅되도록 (`internal/tarsserver/handler_agentruntime_channels.go`의 inbound 채널 설정·매핑 방식 확인 후 설정/스킬 수준으로 배선, 코어 수정 최소화)
- [ ] Stack-chan 페르소나 정의 — TARS 페르소나/스타일/시스템 프롬프트 메커니즘(`internal/prompt`, `tarsserver/session_style.go`, `internal/sysprompt`) 활용. "실체를 가진 데스크탑 동거 로봇, 감각 입력에 짧고 자연스럽게 반응" 성격
- [ ] 출력 경로 확인 — TARS가 그 세션에서 tars-stackchan MCP 툴(`stackchan_set_expression`/`stackchan_speak`/`stackchan_run_motion`)을 호출하도록 MCP 연결(`config --target tars`)·툴 정책 점검
- [ ] (코어 수정이 불가피하면) 변경 범위를 phase 문서 하단에 기록하고 사용자 확인 후 진행

### 2.5 테스트

- [ ] `internal/perception/*_test.go` table-driven: 트리거 판정, 디바운스/레이트리밋, sink POST 페이로드 형식, Gemini 호출은 인터페이스로 추상화해 mock
- [ ] `cd mcp-server && go test ./...` 통과
- [ ] (선택) `scripts/test/perception-loop-contract.sh` — 루프 기동/종료·sink 페이로드 계약 (기존 contract 스크립트 패턴)

---

### ✅ Checkpoint: Phase 2 — mock 모드 통과 (2026-05-16)

**구현 확인:**
- [x] `tars-stackchan-control perceive serve` 서브커맨드, mock 브리지로 루프 기동/정상 종료(SIGINT/ctx)
- [x] 관측 → `TARSWebhookSink`가 확정 페이로드(§2.4)로 POST, 백오프 재시도+드롭
- [x] `internal/perception` 격리 패키지: config(env)/loop(트리거·디바운스·레이트리밋·idle)/observe(Stub+Gemini opt-in)/sink
- [~] TARS 페르소나+채널 배선 = 크로스repo 핸드오프 문서화(§2.4), 실행은 후속 분리 커밋

**실행 확인 (하드웨어 불요 — 통과):**
- [x] `cd mcp-server && go test ./...` 전체 통과 (perception 단위테스트: decideTrigger/rate-limit/step/sink payload/retry-drop)
- [x] `scripts/test/perception-loop-contract.sh` 통과
- [x] mock 스모크: idle/event 트리거 → mock JPEG/WAV 캡처·캐시 → stub 요약 → LogSink 관측 출력 → ctx 취소 클린 종료

**설계 정정:** Gemini 멀티모달 요약은 **명시적 opt-in**(`TARS_STACKCHAN_PERCEIVE_MODEL`+키)으로 변경. 기본은 결정적 offline `StubSummarizer` — mock/Phase2 재현성 확보 및 공유 TTS GEMINI 키로 미검증 모델 호출 방지. 실 멀티모달 모델명은 Spike S(실 캡처) 해결 시점에 사용자가 명시 설정.

**보류 (Spike S 의존):**
- [ ] 실 CoreS3 + TARS 구동 end-to-end (봇 앞 동작→표정/음성 반응) — **실 카메라/오디오 캡처가 Spike S(미해결)에 블록**. 스파이크 S 통과 후 `TARS_STACKCHAN_BRIDGE=http` + TARS 배선으로 검증.

Phase 2 mock 범위 완료. Phase 3는 mock 기반으로 진행 가능; 실HW closed-loop는 Spike S 게이트.

---
