# Phase 4 — REST 정비 + 메모리/페르소나 안정화 + 릴리스

> 로드맵: [`embodied-bot-roadmap.md`](./embodied-bot-roadmap.md) · 분석: [`codebase-analysis.md`](./codebase-analysis.md)

## 목표

요청 4(외부에서 REST로 음성/피드백/모니터 감정 제어)를 깔끔한 표면으로 정비하고, 봇이 경험을 기억(요청 3)하는지 검증한 뒤, 문서/doctor/릴리스로 마무리한다.

**끝나면 동작하는 것**: claude code/tars/스크립트가 REST 한 번으로 음성·표정·감정·모션을 제어하고, 봇이 며칠에 걸친 경험을 TARS 메모리에 축적·참조한다.

## 작업

### 4.1 REST 표면 정비 (대부분 기존 확장)

- [ ] 현황 점검: 외부 제어는 이미 (a) tars-stackchan MCP 툴 (b) `controlui` `/api/*` (c) 디바이스 `/v1`로 가능. **신규 서버를 만들지 않는다** — `internal/controlui/server.go` 표면을 정비/문서화
- [ ] 누락분만 추가 (기존 `handleTool` 패턴 모방):
  - `POST /api/emotion` — 모니터 감정/이모션 표현 (표정+LED+모션을 한 번에, 감정 프리셋 매핑)
  - `POST /api/speak` 표면 일관화 (volume 등 기존 안전 규칙 유지)
  - `GET /api/perceive/status` — 퍼셉션 루프·owner 등록·TARS 연결 상태 (운영 가시성)
- [ ] `docs/protocol/local-control-api.md` + README "Local Control Console" 섹션 갱신: 외부(claude code/tars)에서의 호출 예시 포함
- [ ] 인증/입력 검증은 기존 컨벤션(Bearer, allowlist, 클램프, unknown 필드 거부) 그대로 적용

### 4.2 메모리/경험 축적 검증 (요청 3 — TARS 위임)

- [ ] TARS 기존 영속 메모리/reflection이 Stack-chan 관측 경험을 축적하는지 end-to-end 검증
  - 관측 요약이 TARS 세션→메모리 추출 후보로 흐르는지 확인 (`internal/memory`, `internal/reflection`)
  - [tars] 필요 시 Stack-chan 페르소나 세션의 메모리 추출 정책만 조정 (코어 수정 아님)
- [ ] 결과를 짧은 검증 노트로 기록 (며칠 운영 후 메모리에 owner/사건이 남고 이후 반응에 반영되는지)

### 4.3 doctor 확장

- [ ] `tars-stackchan-mcp doctor`(기존 진단 패턴)에 추가 진단:
  - 펌웨어 캡처 엔드포인트 도달성(`/v1/camera/snapshot` 등)
  - 퍼셉션 루프 health, owner enroll 존재 여부
  - TARS webhook inbound 도달성(설정된 경우)
  - 실패 시 actionable 힌트 (기존 doctor 힌트 스타일 모방)

### 4.4 문서 + 릴리스

- [ ] README에 "Embodied Bot" 섹션 (아키텍처 다이어그램, 구동 순서: TARS 실행 → `perceive enroll` → `perceive serve`)
- [ ] (선택) `perceive serve`를 Homebrew service로 — 기존 TTS relay의 `brews.service` 패턴 모방
- [ ] `scripts/test/*-contract.sh` 신규/갱신분 CI 반영 확인
- [ ] 버전 범프 + `v*` 태그 → GoReleaser → homebrew-tap (기존 릴리스 플로우)
- [ ] [tars] TARS 측 변경(페르소나/채널 배선)도 해당 repo 컨벤션으로 커밋·릴리스

---

### 4.2 메모리/경험 축적 — 크로스repo 핸드오프 (이번 미실행)

> TARS repo 작업이라 분리. TARS는 이미 영속 메모리/시맨틱검색/야간 reflection
> 보유(codebase-analysis §2). Phase 2의 확정 webhook 계약으로 관측 요약이 TARS
> 세션에 들어가면, 메모리 추출 후보로 흐르는지 TARS 측에서 end-to-end 검증 후
> 짧은 노트 기록(필요 시 Stack-chan 페르소나 세션의 메모리 추출 정책만 조정 —
> 코어 수정 아님). tars-stackchan 측 코드 작업 없음.

### ✅ Checkpoint: Phase 4 — mock 모드 완료 (2026-05-17)

**구현 확인:**
- [x] `/api/emotion`(표정+LED+모션 프리셋 합성, unknown emotion/field 400, GET 405, `{"motion":false}` 억제) + `/api/perceive/status`(owner/camera/TARS 가시성, 시크릿 미노출, Spike S 노트) — 기존 안전규칙·Bearer/allowlist 유지
- [x] doctor 확장: owner enroll 존재·camera 모드·TARS webhook 설정/도달성 + actionable 힌트. **카메라 스냅샷 프로브는 의도적으로 안 함**(실HW 리셋, Spike S)
- [~] 메모리 축적 검증 = 크로스repo 핸드오프(§4.2), tars-stackchan 측 무코드

**실행 확인 (하드웨어 불요 — 통과):**
- [x] `cd mcp-server && go test ./...` 전체 통과 (controlui emotion/perceive-status 테스트 포함)
- [x] `scripts/test/*-contract.sh` 통과
- [x] mock 스모크: `/api/emotion surprised` → expression+LED+motion 3-leg 적용, `/api/perceive/status` JSON, doctor 퍼셉션 블록 출력
- [x] `goreleaser check` 통과 (릴리스 설정 유효) — **태깅/배포는 외부·비가역이라 사용자 명시 요청 시에만 수행**

**보류 (Spike S / 크로스repo):** `/api/emotion` 실HW 표정·LED 변화, 외부 REST 실제 제어, 며칠 운영 메모리 체감 — 실HW closed-loop·TARS 배선 후. 카메라는 Spike S.

Phase 4 mock 범위 완료. 릴리스는 사용자 결정 사항으로 분리.

---

## 전체 완료 시 상태

요청 1~5가 모두 충족된다:
1. 백그라운드 AI 서버(=TARS) + WiFi 스택짱 비전/오디오 감각 입력 → 페르소나 가진 실체 ✔ (Phase 1·2)
2. owner 지문화 + owner/타인 식별 ✔ (Phase 3)
3. 경험 기억/발전 ✔ (TARS 메모리 위임, Phase 2·4 검증)
4. REST로 음성/피드백/모니터 감정 외부 제어 ✔ (Phase 4, 기존 확장)
5. 아키텍처: tars 빌트인도 직접구현도 아닌 **역할 분리(hybrid)** — 중복 제거 ✔ (전 페이즈 관통)
