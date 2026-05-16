# Phase 3 — Owner 지문화 + 식별

> 로드맵: [`embodied-bot-roadmap.md`](./embodied-bot-roadmap.md) · 분석: [`codebase-analysis.md`](./codebase-analysis.md)

## 목표

owner를 카메라(얼굴)+음성으로 지문화(enroll)하고, 퍼셉션 관측에 `owner` / `stranger` / `unknown` 라벨을 붙여 TARS가 주인과 타인을 다르게 대하게 한다.

**끝나면 동작하는 것**: 주인이 다가오면 봇이 주인으로 인식해 친근하게, 모르는 사람/소리엔 경계하거나 중립적으로 반응한다.

## 작업

### 3.1 식별 전략 결정 — 확정 (2026-05-16)

**결정**: Phase 2 summarizer와 동일 정책 — Gemini 멀티모달 식별은 **명시적 opt-in**(`TARS_STACKCHAN_PERCEIVE_MODEL`+키), 기본은 **결정적 offline `StubIdentifier`**(sha256 해시 매칭). 근거: 실 캡처가 Spike S에 블록된 상태에서 mock 재현성 확보 + 공유 TTS GEMINI 키로 미검증 모델 호출 방지. Gemini `GeminiIdentifier` 전면 구현은 Spike S(실 캡처) 해결 시점의 opt-in 후속.
3-state 임계 env: `TARS_STACKCHAN_PERCEIVE_OWNER_MIN`(기본 0.6, `>=`→owner), `TARS_STACKCHAN_PERCEIVE_STRANGER_MAX`(기본 0.4, `<=`→stranger, 그 사이/상충→unknown). 융합: 가용 모달리티 전부 owner→owner, 전부 stranger→stranger, 상충/중간→unknown(안전 기본값). owner 미등록→항상 unknown.

### 3.2 owner enroll

- [ ] `tars-stackchan-control perceive enroll` 서브커맨드 추가 (`runCommand` switch, 기존 패턴)
  - 카메라 스냅샷 N장 + 음성 클립 M개 캡처(가이드 프롬프트) → owner 레퍼런스 지문 저장
  - 저장 위치: 절대 user data 경로(기존 TTS 캐시 경로 컨벤션 모방), 파일 권한 0600
  - `type OwnerProfile struct { Name string; FaceRefs, VoiceRefs []Ref; CreatedAt int64 }`
  - 재등록/삭제 지원 (`enroll --reset`)
- [ ] enroll 데이터는 로컬 전용. TARS로 전송하지 않음 (개인정보 정책)

### 3.3 식별기

- [ ] `mcp-server/internal/perception/identify.go`
  - `Identify(ctx, snapshot CameraSnapshot, clip AudioClip) (Identity, error)` — `Identity{ Label: owner|stranger|unknown; Confidence float64; Modality: face|voice|both }`
  - 얼굴·음성 각각 점수 → 융합 규칙(둘 다 일치=owner 강신뢰, 상충=unknown)
  - owner 미등록 상태면 항상 `unknown` 반환 (안전 기본값)
- [ ] Phase 2 `Observation`에 `Identity` 필드 추가, 루프에서 식별 호출 후 라벨 부착
- [ ] sink 페이로드에 라벨 포함 (요약 텍스트에도 자연어로: "주인으로 보이는 사람이 …")

### 3.4 [tars] 페르소나의 owner 인지 반응

> TARS repo 작업. 분리 커밋.

- [ ] Stack-chan 페르소나 프롬프트에 owner/stranger/unknown 별 행동 가이드 추가 (Phase 2에서 만든 페르소나 정의 확장 — 코어 수정 아님, 프롬프트/스타일 수준)
- [ ] (선택) owner 식별 사실을 TARS 메모리에 경험으로 남기도록 관측 요약에 명시 (Phase 4에서 메모리 축적과 함께 검증)

### 3.5 테스트

- [ ] `identify_test.go` table-driven: owner-only, voice-only, 상충, 미등록→unknown, 저신뢰→unknown
- [ ] enroll 라운드트립 테스트(저장→로드→식별), 권한 0600 확인
- [ ] Gemini 식별 호출은 인터페이스 mock
- [ ] `cd mcp-server && go test ./...` 통과

---

### 3.4 [tars] 페르소나 owner 인지 — 크로스repo 핸드오프 (이번 미실행)

> TARS repo 분리 커밋. 확정 계약: webhook 페이로드에 flatten된
> `identity`(owner|stranger|unknown), `identity_confidence`, `identity_modality`
> + `summary`/`text` 선두에 자연어 절("The owner appears to be present." 등).
> TARS 측 할 일: Stack-chan 페르소나 프롬프트에 owner/stranger/unknown 행동
> 가이드 추가(Phase 2 페르소나 확장, 프롬프트/스타일 수준). 코어 수정 불필요.

### ✅ Checkpoint: Phase 3 — mock 모드 통과 (2026-05-16)

**구현 확인:**
- [x] `perceive enroll [--name --faces --voices]` 저장 / `--reset` 삭제, 파일 0600·디렉토리 0700, 로컬 전용(TARS 미전송)
- [x] `StubIdentifier.Identify` 3-state + confidence + modality, 미등록→unknown, 상충→unknown
- [x] `Observation`/webhook 페이로드에 식별 라벨·confidence·modality + 요약 선두 자연어 절
- [~] TARS 페르소나 owner 가이드 = 크로스repo 핸드오프 문서화(§3.4), 실행은 후속 분리 커밋

**실행 확인 (하드웨어 불요 — 통과):**
- [x] `cd mcp-server && go test ./...` 전체 통과 (identify table-driven: owner/stranger/unknown/미등록/상충, enroll 라운드트립+0600/0700, step 통합)
- [x] `scripts/test/perception-loop-contract.sh` 통과
- [x] mock 스모크: enroll(jeidee, 0600/0700) → serve가 owner 인식 → 요약 "The owner appears to be present. …" → reset 정상

**보류 (Spike S 의존):** 실 CoreS3에서 본인 등장→owner 친근 반응 / 타인→stranger 반응, 오인식률 체감 — 실 카메라/오디오가 Spike S(미해결)에 블록. 스파이크 S 통과 후 `BRIDGE=http`로 검증.

Phase 3 mock 범위 완료. Phase 4는 mock 기반 진행 가능; 실HW는 Spike S 게이트.

---
