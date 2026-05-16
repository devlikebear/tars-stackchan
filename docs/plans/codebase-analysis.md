# Codebase Analysis — tars-stackchan ↔ tars (Embodied Bot)

이 문서는 "Embodied Bot" 기획/개발계획서의 기반 참조 자료다. 이후 모든 페이즈 문서는 이 분석을 전제로 한다.

분석일: 2026-05-16. 대상 커밋: tars-stackchan `8213ddf`, tars `f94e0fb5`.

## 1. tars-stackchan 현재 정체

**AI 봇이 아니라 "AI 에이전트가 Stack-chan을 원격 조종하는 출력 전용 브리지"**.

```
AI agent / TARS / Claude Code  →  stdio MCP server  →  로컬 HTTP /v1 (WiFi)  →  Stack-chan 펌웨어 MOD  →  표정/머리/LED/모션/음성
```

| 항목 | 상태 |
|---|---|
| 언어/구조 | Go 1.22, 단일 모듈 `mcp-server/`, `internal/{stackchan,tts,controlui,buildinfo}` |
| 의존성 | **stdlib only, 외부 의존성 0** (roadmap에 "유지 필수"로 명시) — Embodied Bot에서 이 컨벤션은 **의도적으로 완화**된다 (§4 참조) |
| 데이터 흐름 | **단방향 출력만**. 입력(센서) 경로 없음 |
| MCP 툴 | `stackchan_get_status/set_expression/move_head/set_led/run_motion/speak`(+옵션 `upload_firmware`) |
| 프로토콜 | 버전드 HTTP `/v1`, Bearer 토큰. `GET /v1/sensors`, `GET /v1/camera/snapshot`가 *Future endpoints*로 이미 예고됨 |
| 펌웨어 | upstream Stack-chan(Moddable SDK) 체크아웃 + bridge MOD 패치. `HttpServerService` + `Robot` facade 사용 |
| 음성 | Go TTS 릴레이 → Gemini TTS, `dns-sd` mDNS 광고 |

### 핵심 파일 (모방/확장 대상)

| 역할 | 경로 | Embodied Bot에서 |
|---|---|---|
| Bridge 인터페이스 + 타입 + 툴 상수 | `mcp-server/internal/stackchan/types.go` | 퍼셉션 메서드/타입 추가 지점 |
| MCP 툴 등록 | `mcp-server/internal/stackchan/mcp.go` | (필요 시) 퍼셉션 조회 툴 추가 |
| HTTP 브리지(디바이스측 /v1 클라이언트) | `mcp-server/internal/stackchan/server.go` | `/v1/camera/snapshot` 등 클라이언트 호출 추가 |
| 로컬 컨트롤 REST 서버 | `mcp-server/internal/controlui/server.go` (`http.ServeMux`, `/api/*`) | 퍼셉션 루프 호스트 + REST 표면 확장의 모방 대상 |
| 컨트롤 CLI 디스패치 | `mcp-server/cmd/tars-stackchan-control/main.go` (`os.Args` switch `runCommand`) | 신규 서브커맨드(`perceive serve` 등) 추가 패턴 |
| TTS 릴레이 (장기 구동 서비스 모방) | `mcp-server/internal/tts/serve.go`, `mdns.go` | 백그라운드 루프 + Homebrew service 패턴 |
| 공유 프로토콜 명세 | `docs/protocol/local-control-api.md` | `/v1` 확장 명세 갱신 지점 |
| 펌웨어 MOD | `firmware/stackchan/mods/tars_stackchan_bridge` (패치: `firmware/stackchan/patches/0001-*.md`) | CoreS3 카메라/마이크 캡처 추가 지점 |

### 빌드/검증 컨벤션

- Go 모듈: `mcp-server/` (`go 1.22`)
- CLI 디스패치: `os.Args[1:]` switch (`cmd/tars-stackchan-control/main.go runCommand`)
- 테스트: 동 디렉토리 `_test.go`, table-driven, `cd mcp-server && go test ./...`
- 계약 테스트: `scripts/test/*-contract.sh` (CI `.github/workflows/ci.yml`)
- 릴리스: `v*` 태그 → `.goreleaser.yaml` → `devlikebear/homebrew-tap` 자동
- 디바이스 호스트네임 관례: `stackchan.local` / TTS `tars-stackchan-tts.local`

## 2. tars 현재 정체 (아키텍처 결정의 핵심)

**이미 성숙한 로컬 AI 에이전트 런타임** (Go 1.25.6, 활발 개발 중, 커밋 #874). 외부 의존성 사용(cobra, zerolog, gorilla/websocket, robfig/cron 등).

| TARS가 이미 가진 것 | 모듈 | Embodied Bot 요청과의 관계 |
|---|---|---|
| LLM 에이전트 루프 + 멀티모달 | `internal/llm` (`gemini_native_convert.go`, provider 이미지 plumbing) | "LLM 두뇌" — **재사용** |
| **MCP 클라이언트** | `internal/mcp` (`client.go`, `remote_transport.go`) | tars-stackchan 출력 툴 소비 — **이미 가능** (`config --target tars`) |
| 영속 메모리 + 시맨틱 검색 + 야간 reflection | `internal/memory`, `internal/reflection` | "경험 기억/발전" — **재사용** |
| 페르소나/스타일 | `internal/prompt`, `tarsserver/session_style.go`, `internal/sysprompt` | "페르소나" — **재사용** |
| **인바운드 webhook 채널** | `internal/tarsserver/handler_agentruntime_channels.go:35` (`/v1/channels/webhook/inbound/`) | **퍼셉션 관측 수신 경로 — 이미 존재** |
| REST/콘솔 서버 + 채널 | `internal/tarsserver`, Telegram/webhook | "REST 제어" — **재사용** |
| 스케줄러·watchdog | `internal/cron`, `internal/pulse` | 백그라운드 상시 구동 — **재사용** |

TARS에 **없는** 것: 펌웨어/하드웨어 코드, Stack-chan 프로토콜, 출력 액추에이션, 센서 캡처. 이것이 tars-stackchan의 고유 자산.

## 3. 확정된 아키텍처 — 역할 분리 (Hybrid)

사용자 확인 완료: hybrid 분리, owner 식별은 tars-stackchan 쪽.

```
[CoreS3 카메라/마이크]
   │ 이벤트 트리거 + 주기 스냅샷
   ▼
tars-stackchan  =  "몸 (Sensory Edge + Actuator)"          [신규 코드 대부분 여기]
   - 신규: 펌웨어 캡처(/v1/camera/snapshot, /v1/audio/clip)
   - 신규: 퍼셉션 루프 + owner 식별(임베딩 매칭)
   - 신규: 라벨링된 관측 → TARS webhook inbound POST
   - 기존: /v1 브리지 + TTS 릴레이 + 출력 액션
   │  ▲
   │  │ 관측 이벤트 POST                출력 (기존 MCP 툴)
   ▼  │ /v1/channels/webhook/inbound/   stackchan_* tools
TARS  =  "뇌 (Cognition)"  =  요청의 "백그라운드 AI 서버"   [신규 코드 거의 0]
   - 기존 재사용: LLM 멀티모달 / 메모리 / reflection / 페르소나 / 스케줄러 / REST
   - 신규: stackchan 페르소나 + webhook 채널→세션 배선 (설정/스킬 수준)
```

**왜 이 분리인가**: 요청 1·3·4(LLM 두뇌, 메모리, REST)는 TARS가 이미 보유 → tars-stackchan에 재구현하면 통째 중복. 펌웨어/하드웨어는 TARS에 넣으면 TARS가 특정 하드웨어에 오염. 양쪽 경계(관측 POST, MCP 출력)는 **둘 다 이미 존재하는 인터페이스** → 신규 결합 최소.

## 4. 컨벤션 충돌 및 처리 (중요)

| 충돌 | 처리 |
|---|---|
| tars-stackchan "stdlib only, 외부 의존성 0" vs 비전/오디오/임베딩 처리 | **의도적 완화**. owner 식별 임베딩·이미지 디코딩 등은 외부 의존성 또는 외부 API 필요. 단 (a) 신규 의존성은 페이즈 문서에서 명시·정당화, (b) 멀티모달 추론은 **클라우드 Gemini로 위임**해 로컬 의존성 최소화(이미 TTS에서 Gemini 사용 중), (c) `mcp-server/` 핵심 브리지/툴은 stdlib 유지, 퍼셉션은 분리된 패키지/서브커맨드로 격리 |
| 실시간성 | **이벤트 트리거 + 주기 스냅샷** (사용자 확정). 연속 스트리밍 아님 → 펌웨어 변경·대역폭·LLM 비용 최소 |
| LLM 위치 | **클라우드 멀티모달(Gemini)** (사용자 확정) |
| 하드웨어 | **M5Stack CoreS3** (카메라+마이크 내장, 사용자 확정) |

## 5. 미해결/리스크

- CoreS3 + Moddable SDK에서 카메라 프레임/마이크 PCM 캡처 가능 여부는 **Phase 1에서 펌웨어 PoC로 먼저 검증**(스파이크). 불가 시 외장 캡처 폴백 검토.
- owner 식별 정확도(조명/각도/마이크 잡음)는 MVP에서 "완벽" 목표 아님 — owner/stranger/unknown 3-state로 설계.
- 펌웨어 재플래시 1회 필요(Phase 1, USB) — 사용자 협조 시점 명시.
- 개인정보: 카메라/음성 원본은 로컬에서만 처리, TARS로는 **요약/라벨 위주** 전송(원시 미디어 최소화)을 기본 정책으로.
