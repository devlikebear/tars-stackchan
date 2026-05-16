# Phase 1 — 펌웨어 퍼셉션 캡처

> 로드맵: [`embodied-bot-roadmap.md`](./embodied-bot-roadmap.md) · 분석: [`codebase-analysis.md`](./codebase-analysis.md)

## 목표

CoreS3의 카메라 스냅샷과 마이크 오디오 클립을 `/v1`로 노출하고, tars-stackchan 브리지가 이를 읽을 수 있게 한다.

**끝나면 동작하는 것**: 디바이스에 `curl`하면 JPEG 한 장과 짧은 WAV가 돌아온다. `tars-stackchan-control` mock 모드에서도 동일 인터페이스가 동작한다.

## 작업

### 1.0 스파이크 — 캡처 가능성 검증 (먼저, 차단성)

- [ ] CoreS3 + Moddable SDK에서 카메라 프레임 1장 캡처 가능 여부 PoC
  - upstream Stack-chan 펌웨어가 노출하는 카메라/오디오 API 조사 (`.work/stack-chan/firmware/` 문서·소스)
  - 최소 MOD 코드로 프레임 1장 JPEG 인코딩 + 시리얼 출력까지 확인
  - 마이크 PCM N초 캡처 → WAV 가능 여부 확인
- [ ] 결과를 `docs/plans/embodied-bot-phase-1-spike.md`로 기록 (가능/제약/대안)
- [ ] **불가 시**: 즉시 사용자 보고 → 외장 캡처 등 폴백 재논의 (Phase 1 이후 중단)

> **스파이크 결과(2026-05-16, [spike 문서](./embodied-bot-phase-1-spike.md)): GREEN.** 마이크는 upstream `microphone.ts`로 즉시 가능. 카메라는 Moddable SDK `embedded:io/image/in/camera`(esp32-camera+esp_jpeg) 호스트 통합으로 가능 — CoreS3 타깃 매니페스트가 핀맵+QVGA를 이미 정의. 1.0b로 진행.

### 1.0b 호스트 펌웨어 카메라 통합 (스파이크 GREEN 후속) — 완료

> 구현 방식 정정: upstream `camera.ts` 교체 불필요. prepare 스크립트가 호스트 매니페스트를 프로그래밍 패치하는 기존 컨벤션을 확장 → **upstream 소스 무수정**, MOD가 `embedded:io/image/in/camera`를 직접 import.

- [x] `scripts/dev/prepare-firmware-upload.sh` CoreS3 호스트 매니페스트 패치에 `$(MODDABLE)/modules/io/imagein/camera/manifest.json` include + `config.camera`(QVGA/jpeg) 추가. 핀맵은 SDK 타깃 매니페스트에서 자동 전파(체인 확인됨)
- [x] 스텁 `camera.ts`는 손대지 않음(미사용). 스냅샷은 1.2에서 MOD가 `embedded:io/image/in/camera` 직접 import (템플릿: `$(MODDABLE)/examples/io/imagein/camera/camera-server-jpeg`)
- [x] 패치 노트 신규: `firmware/stackchan/patches/0002-host-camera-perception.md` (호스트 변경·IDF 의존·host deploy 명문화)
- [x] 계약 테스트: `scripts/test/firmware-upload-script-contract.sh`에 camera include/config 어서션 추가
- [ ] 빌드 검증(CoreS3 호스트 빌드 성공, IDF 컴포넌트 해결, PSRAM 예산) → **하드웨어 필요, Phase 1 Checkpoint에서**
- [ ] **런타임 MOD 아닌 호스트 디플로이 플래시 필요** → Checkpoint 수동 확인 host deploy 경로 (반영됨)

### 1.1 프로토콜 명세 확장

- [ ] `docs/protocol/local-control-api.md`의 *Future endpoints*를 정식 명세로 승격
  - `GET /v1/camera/snapshot` → `image/jpeg` (쿼리: `max_width` 선택), 인증: Bearer
  - `GET /v1/audio/clip?ms=1500` → `audio/wav` (16kHz mono 권장), Bearer
  - `GET /v1/sensors` → JSON: `{ "motion": bool, "sound_level": float, "ts": <unix_ms> }` (이벤트 트리거용 경량 폴링)
  - 실패 응답은 기존 `{ "error": ... }` non-2xx 컨벤션 유지
- [ ] `Status.Capabilities`에 `camera`, `microphone` 추가 정의 (펌웨어가 실제 지원 시 반환)

### 1.2 펌웨어 MOD 확장

- [ ] `firmware/stackchan/mods/tars_stackchan_bridge`에 핸들러 추가 (기존 `HttpServerService` 라우팅 패턴 모방)
  - `GET /v1/camera/snapshot` 핸들러 — 프레임 캡처 → JPEG 응답
  - `GET /v1/audio/clip` 핸들러 — `ms` 파싱(상한 클램프, 예: ≤3000), PCM→WAV 응답
  - `GET /v1/sensors` 핸들러 — 모션/사운드 경량 상태
  - 기존 Bearer 토큰 검증을 신규 핸들러에도 적용 (기존 출력 핸들러와 동일 미들웨어/체크)
- [ ] `firmware/stackchan/patches/0001-install-tars-stackchan-bridge-mod.md` 갱신 (신규 엔드포인트·CoreS3 카메라 설정 반영)
- [ ] 캡처 해상도/클립 길이 상한을 manifest config로 노출 (대역폭 안전장치)

### 1.3 브리지 클라이언트 + 타입 — 완료

> 설계 정정(개선): 원안은 "`Bridge` 인터페이스에 추가"였으나, 그러면 모든 구현체·테스트 stub이 깨지고 MCP 툴 경로까지 영향. 프로토콜의 "옵셔널 capability" framing에 맞춰 **별도 `PerceptionBridge` 인터페이스**로 분리 → 기존 `Bridge`/MCP/테스트 무영향, 컨벤션 부합. 실제 HTTP 브리지는 `internal/bridge/http`, mock은 `internal/bridge/mock`(server.go는 MCP/JSON-RPC 서버라 무관).

- [x] `mcp-server/internal/stackchan/types.go`: `SnapshotOptions`/`AudioOptions`, `CameraSnapshot{ContentType,Data}`(디코딩 안 하므로 Width/Height 미노출 — 없는 값 날조 금지), `AudioClip{ContentType,Data,DurationMs}`, `SensorState{motion,sound_level,ts}` json 태그, 별도 `PerceptionBridge` 인터페이스
- [x] HTTP 브리지(`internal/bridge/http/http.go`): `CameraSnapshot`/`AudioClip`/`Sensors`. 캡처는 인증 필수 GET 바이너리(`getMedia`, maxMediaBytes 4MB 상한+초과 감지), Sensors는 기존 `do()` JSON 재사용, 에러 포맷 기존 패턴 동일
- [x] mock 브리지(`internal/bridge/mock/mock.go`): 유효 픽스처 `testdata/sample.jpg`(2x2 JPEG)·`sample.wav`(16kHz mono) `//go:embed`, status capabilities에 camera/microphone 추가
- [x] 컴파일타임 보증 `var _ stackchan.PerceptionBridge = (*Bridge)(nil)` (http·mock 양쪽), `go build/vet/test ./...` 통과

### 1.4 테스트 + 계약 — 완료

> 위치 정정: HTTP 브리지 테스트는 `internal/bridge/http/http_test.go`(server_test.go 아님 — 그건 MCP 서버). mock 테스트는 `internal/bridge/mock/mock_test.go`(신규). 계약은 `firmware-bridge-contract.sh`가 MOD `*.test.mjs`를 node --test로 돌리므로 거기에 추가.

- [x] `internal/bridge/http/http_test.go`: camera/audio/sensors — auth+쿼리 전파, 기본값 쿼리 생략, 펌웨어 에러(503+body), 빈 바디 거부, maxMediaBytes 초과 거부, 토큰 누락 거부 (httptest, 기존 패턴/`writeJSON` 재사용)
- [x] `internal/bridge/mock/mock_test.go`(신규): JPEG SOI/RIFF·WAVE 픽스처 유효성, capabilities camera/microphone, sensors 미디어프리+ts (`slices.Contains` 사용)
- [x] `bridge-core.test.mjs`: `parseSnapshotOptions`/`parseAudioClipOptions` 기본값·클램프·비숫자 throw
- [x] `mod-manifest.test.mjs`: 퍼셉션 라우트(GET+withAuth)·바이너리 응답·`./perception` 모듈·bridge-core 네이티브 무import 격리 회귀 잠금
- [x] 검증: `go test ./...` 전체 통과, `firmware-bridge-contract.sh`(bridge-core 12 + mod-manifest 7) 통과, `firmware-upload-script-contract.sh` 통과

---

### ✅ Checkpoint: Phase 1 완료 확인

**구현 확인:**
- [x] `docs/protocol/local-control-api.md`에 3개 신규 엔드포인트 정식 명세
- [x] 별도 `PerceptionBridge` 인터페이스 + HTTP·mock 양쪽 구현 + 컴파일타임 보증
- [x] 스파이크 문서에 캡처 가능성 결론 기록 (GREEN)

**실행 확인 (하드웨어 불요 — 전부 통과):**
- [x] `cd mcp-server && go test ./...` 통과 (http 퍼셉션 9 + mock 4 신규 포함)
- [x] `scripts/test/firmware-bridge-contract.sh` 통과 (bridge-core 12 + mod-manifest 7)
- [x] `scripts/test/firmware-upload-script-contract.sh` 통과 (camera/audioin 어서션)
- [x] mock 검증은 `internal/bridge/mock/mock_test.go`가 유효 JPEG/WAV 픽스처로 수행 (controlui `/api` 퍼셉션 라우트는 Phase 4 REST 범위라 Phase 1 제외 — 원안의 `localhost:8787/api` 항목 정정)

**수동 확인 (실디바이스 M5Stack CoreS3, 2026-05-16):**
- [x] 호스트 디플로이 플래시 성공 — IDF managed-component(esp32-camera/esp_jpeg) 통합, 호스트 펌웨어+MOD 플래시, 부팅, WiFi 접속(192.168.219.113) 확인. (빌드 함정·해결법은 patch 0002)
- [x] `GET /v1/status` → 200, `capabilities`에 `camera`,`microphone` 포함 (실디바이스 반영)
- [x] `GET /v1/sensors` → `{"motion":false,"sound_level":0,"ts":...}` 정상
- [x] 인증: 토큰 없는 `/v1/camera/snapshot` → **401** (withAuth 정상)
- [x] `GET /v1/audio/clip?ms=1500` → **200, 16kHz/16bit/mono WAV, 리셋 없음** (2026-05-17 실HW 검증). 마이크는 I2S라 SCCB 충돌 무관
- [ ] ❌ **OPEN DEFECT(원인 확정, Spike S)**: `GET /v1/camera/snapshot` → 디바이스 리셋. CoreS3 카메라 SCCB ↔ 내부 I2C(AXP/AW9523/터치) 동일 버스 소유권 충돌. 수정=버스핸들 브리지(별도 과제). 우회: `TARS_STACKCHAN_PERCEIVE_CAMERA=off` 오디오-온리

**Checkpoint 판정: 부분 통과 — Phase 1 종결(2026-05-16).** 퍼셉션 transport/프로토콜/Go·MOD 구현/테스트/계약/sensors/auth/호스트통합 = 완료·실HW 검증. 온디바이스 네이티브 camera(추정상 audio) 캡처가 디바이스 리셋 = **OPEN DEFECT, 별도 스파이크 S로 분리**(roadmap 참조).

근본 원인(코드 규명, 백트레이스는 CoreS3 USB-CDC 단독이라 확보 불가): Moddable 비공식 보드의 esp32-camera↔GC0308 네이티브 브링업. **반증된 저위험 가설**: 전원레일(AXP2101 `0x90=0xbf`가 ALDO1-4/BLDO1-2/DLDO1 이미 인가) / perception.js Camera 옵션(`dimensionsToFrameSize(320,·)`→유효 QVGA; 옵션 오류면 catch→HTTP 400이지 리셋 아님). 미검토 용의자(스파이크 S): AW9523 게이팅 카메라 reset/enable, LEDC 충돌(camera.c `LEDC_TIMER_0`/`CHANNEL_0` 하드코딩 vs CoreS3 백라이트/부저), GC0308 클럭/DMA.

**조치 완료**: 디바이스를 정상 debug 펌웨어로 원복(192.168.219.113 온라인·WiFi 정상 확인). **Phase 2~4는 mock 브리지로 진행**, 실 캡처 의존 지점은 스파이크 S 통과 후 연결. 상세: [`embodied-bot-phase-1-spike.md`](./embodied-bot-phase-1-spike.md), `firmware/stackchan/patches/0002-host-camera-perception.md`.

---
