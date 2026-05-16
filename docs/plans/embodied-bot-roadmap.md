# Roadmap — Embodied Bot (Stack-chan + TARS)

> 기반 분석: [`codebase-analysis.md`](./codebase-analysis.md) — 모든 페이즈는 이 분석을 전제로 한다.

## 가치 제안

> "Stack-chan이 리모컨이 아니라, **보고 듣고 기억하며 주인을 알아보는 실체**가 된다."

tars-stackchan을 "출력 전용 브리지"에서 **감각 엣지 + 액추에이터**로 확장하고, TARS를 그 봇의 **뇌(인지·기억·페르소나)**로 연결한다.

## 핵심 설계 결정 (확정됨)

| 축 | 결정 | 근거 |
|---|---|---|
| 아키텍처 | **역할 분리 (Hybrid)** — tars-stackchan=몸, TARS=뇌 | 요청 1·3·4(LLM·메모리·REST)는 TARS가 이미 보유. 재구현=중복. 양쪽 경계 모두 기존 인터페이스 |
| 두뇌 위치 | **TARS 그대로 재사용** (신규 AI 서버를 안 만든다) | TARS = 요청의 "Go 백그라운드 AI 서버" 그 자체. LLM·메모리·페르소나·스케줄러 보유 |
| 관측 → 뇌 경로 | TARS 기존 `/v1/channels/webhook/inbound/` 재사용 | `handler_agentruntime_channels.go:35`에 이미 존재. TARS 신규 코드 거의 0 |
| 뇌 → 몸 경로 | TARS 기존 MCP 클라이언트 → tars-stackchan MCP 툴 | `config --target tars` 이미 지원. 출력은 기존 `stackchan_*` 재사용 |
| owner 식별 | **tars-stackchan 쪽**(감각 엣지) | 캡처 직후 라벨링, TARS는 제공자-중립 유지, 원시 미디어 전송 최소화 |
| 두뇌 모달리티 | **클라우드 Gemini 멀티모달** | 이미 TTS에서 Gemini 사용. 로컬 의존성 최소 |
| 실시간성 | **이벤트 트리거 + 주기 스냅샷** | 펌웨어 변경·대역폭·LLM 비용 최소. 연속 스트리밍 아님 |
| 하드웨어 | **M5Stack CoreS3** | 카메라+마이크 내장 |
| 의존성 컨벤션 | tars-stackchan stdlib-only를 **퍼셉션 한정 완화** | 핵심 브리지는 stdlib 유지, 퍼셉션은 격리 패키지 (codebase-analysis §4) |

## 범위 (MVP)

- 펌웨어 MOD: CoreS3 카메라 스냅샷 + 마이크 오디오 클립 캡처 → `/v1` 노출
- tars-stackchan 퍼셉션 루프: 이벤트 트리거 + 주기 스냅샷
- owner enroll(얼굴/음성 지문) + owner/stranger/unknown 3-state 식별
- 라벨링된 관측 → TARS webhook inbound POST → TARS 페르소나 반응 → 기존 MCP 출력 (closed loop)
- 경험의 메모리 축적: TARS 기존 영속 메모리/reflection에 위임 (배선만)
- 외부 제어 REST: tars-stackchan `/v1` + 컨트롤 REST 표면 정비 (음성/피드백/감정 출력)
- doctor 확장, 문서/프로토콜 갱신, 릴리스

## Out of Scope (명시적 제외)

- **연속 비디오/오디오 스트리밍** — 이벤트+스냅샷으로 충분. WebSocket/MJPEG 제외
- **TARS 코어에 신규 LLM/메모리 구현** — 기존 재사용. 신규 두뇌 서버 안 만듦
- **로컬 멀티모달 모델** — 클라우드 Gemini 위임 (하이브리드 제외)
- **다중 owner / 가족 프로필 관리 UI** — 단일 owner enroll로 시작
- **외장 카메라 폴백** — CoreS3 내장 전제. 캡처 불가 시 Phase 1 스파이크 결과로 재논의
- **Linux/Windows 릴레이/호스트** — macOS 데스크탑 전용 (기존 전제 유지)
- **원시 미디어의 TARS 영속 저장** — 라벨/요약 위주 전송, 원본은 로컬 한정

## 페이즈 (수직 슬라이스)

> **상태(2026-05-16): Phase 2·3 mock 범위 완료.** `internal/perception`
> (config/loop/observe/sink/owner/identify) + `perceive serve|enroll` 서브커맨드
> + 단위테스트/계약 통과, mock end-to-end 스모크 OK(트리거→캡처→식별→요약→sink,
> enroll 0600/0700→owner 인식). TARS 측 배선(2.4/3.4)은 확정 webhook 계약으로
> 크로스repo 핸드오프 **수행됨(2026-05-17)**: `../tars` repo의 기존
> `workspace/skills/tars-stackchan/SKILL.md`에 "Perception inbound" 섹션 +
> owner/stranger/unknown 반응 가이드 추가(v0.3.0), `INTEGRATION.md`(webhook
> 채널·MCP 등록·실행순서·계약) 신규. **정직한 갭**: TARS webhook inbound은
> 영속 인박스(콘솔 노출)이고 자율 "관측→에이전트 턴" 소비는 TARS 코어 기능이
> 필요 — 별도 repo 결정사항, 우회 안 함. 스킬/MCP/페르소나/계약은 정합. TARS
> 미구동이라 end-to-end 미검증. (tars repo 변경 미커밋.)
>
> **Phase 4 mock 완료(2026-05-17).** `/api/emotion`(표정+LED+모션 프리셋) +
> `/api/perceive/status` + doctor 퍼셉션 진단 추가, README "Embodied Bot" 섹션,
> go test/계약/`goreleaser check` 통과. 릴리스 태깅은 사용자 결정으로 분리.
> 메모리 검증(4.2)은 크로스repo 핸드오프.
>
> **Spike S 완료(2026-05-16) + 오디오 실HW 검증(2026-05-17).** 근본원인: CoreS3
> 카메라 SCCB와 내부 I2C(AXP/AW9523/터치)가 GPIO12/11 동일 버스 — Moddable
> i2c_master 점유 버스를 esp_camera_init이 별도로 잡아 IDF v6 충돌→리셋. 수정은
> 버스핸들 브리지(중간~큰 작업, 별도 결정). **마이크는 I2S라 무관 — 실HW 검증
> 완료**: `TARS_STACKCHAN_PERCEIVE_CAMERA=off`로 실 CoreS3 오디오-온리 퍼셉션
> 루프 무크래시 동작(owner 음성식별 포함). **카메라/비전만 블록**, 오디오
> closed-loop는 실HW 가용(TARS 배선만 추가하면 됨).
>
> **Phase 1 마감 — 부분 통과 + 분리된 OPEN DEFECT.**
> 퍼셉션 transport/프로토콜/Go·MOD 구현/테스트/계약/호스트통합 = 완료·실HW 검증.
> 실 CoreS3 camera(추정상 audio) 캡처는 디바이스 리셋 — Moddable 비공식 보드의
> esp32-camera↔GC0308 네이티브 브링업 문제, 백트레이스 HW상 불가. **별도 스파이크
> "S. CoreS3 카메라 네이티브 브링업"으로 분리.** Phase 2~4는 **mock 브리지로 진행**
> (실 캡처 의존 지점은 스파이크 해결 후 연결). 상세: `embodied-bot-phase-1-spike.md`,
> `firmware/stackchan/patches/0002-host-camera-perception.md`.

| Phase | 산출물 | 끝나면 동작하는 것 |
|---|---|---|
| **1. 펌웨어 퍼셉션 캡처** | `/v1/camera/snapshot`·`/v1/audio/clip`·`/v1/sensors` 프로토콜+MOD+Go 브리지+mock+테스트, CoreS3 호스트 카메라/오디오 모듈 통합 | ✅ status/sensors/auth/호스트통합 실HW 검증. ❌ 실 카메라 캡처는 디바이스 리셋(스파이크 S로 분리) |
| **S. CoreS3 카메라 네이티브 브링업 (스파이크, 분리)** | esp32-camera↔GC0308 on CoreS3 원인규명·수정 (AW9523 카메라 리셋/enable, LEDC 충돌, GC0308 클럭/DMA 후보) | 실 디바이스에서 `/v1/camera/snapshot`→JPEG, `/v1/audio/clip`→WAV 안정 동작 |
| **2. 퍼셉션 루프 + TARS 감각 채널** | 이벤트+주기 캡처 루프, 관측→TARS webhook POST, TARS 페르소나 반응 | 봇 앞에서 움직이면 TARS가 보고 말/표정으로 반응 (owner 식별 없는 "누군가 있다") |
| **3. Owner 지문화 + 식별** | owner enroll CLI, 얼굴/음성 임베딩 매칭, 관측에 owner/stranger/unknown 라벨 | 봇이 주인과 타인을 구분해 다르게 반응 |
| **4. REST 정비 + 메모리/페르소나 안정화 + 릴리스** | REST 표면 정비, 메모리 축적 검증, doctor 확장, 문서/릴리스 | 외부(claude code/tars)에서 REST로 음성·감정·피드백 제어, 봇이 경험을 기억 |

각 페이즈는 `docs/plans/embodied-bot-phase-N-*.md` 참조.

## 진행 규칙 (HITL)

- Claude Code는 페이즈 파일을 순서대로 연다. 작업은 체크박스 단위 순차 실행.
- 각 페이즈 끝 **Checkpoint** 통과 → 사용자 확인 → 다음 페이즈.
- Checkpoint 실패 시: 실패 항목 보고 → 사용자와 원인 파악 → 수정 → 재검증.
- **Phase 1은 펌웨어 재플래시(USB 연결) 필요** — 그 시점에 사용자 협조 요청.
- **Phase 1 캡처 스파이크 결과(종결)**: 마이크=upstream OK, 카메라=Moddable 모듈 호스트 통합으로 빌드/플래시까지 성공했으나 **실HW 캡처가 디바이스 리셋** → 스파이크 S로 분리. **Phase 2~4는 mock 브리지로 진행**; 실 캡처 의존 작업은 스파이크 S 통과 후에만 실HW 연결.
- TARS repo 수정이 필요한 작업은 페이즈 문서에서 `[tars]` 태그로 명시. tars-stackchan repo 작업과 섞이지 않게 분리 커밋.

## 크로스 repo 메모

- tars-stackchan: `/Users/changheonshin/workspace/myworks/tars-stackchan`
- tars: `/Users/changheonshin/workspace/myworks/tars`
- 신규 코드의 ~90%는 tars-stackchan. TARS 측은 설정/스킬/페르소나 + webhook 채널 배선 수준(기존 기능 재사용).

## 빌드/검증 컨벤션 (codebase-analysis 발췌)

- Go 모듈: `mcp-server/` (`go 1.22`). 퍼셉션 신규 의존성은 페이즈 문서에서 명시·격리.
- CLI: `os.Args[1:]` switch (`cmd/tars-stackchan-control/main.go runCommand`)
- 테스트: 동 디렉토리 `_test.go`, table-driven, `cd mcp-server && go test ./...`
- 계약 테스트: `scripts/test/*-contract.sh` (CI에서 실행)
- 릴리스: `v*` 태그 → `.goreleaser.yaml` → homebrew-tap 자동
