# Embodied Bot — Status & Continuation

> 단일 진실 소스. 다음 세션/사람이 (tars-stackchan **또는** tars repo에서)
> 이어갈 수 있도록 현재 상태·미해결·재개 방법을 정리한다.
> 최종 갱신: 2026-05-17.

## 한 줄 요약

tars-stackchan = 몸(감각 엣지+액추에이터), TARS = 뇌(역할분리 hybrid).
**오디오 퍼셉션·owner 음성식별·REST·임베디먼트·TARS 스킬/계약 = 완료(오디오는
실HW 검증).** 카메라/비전 = 원인 확정된 OPEN(JTAG 필요). TARS 자율 closed-loop
= 스킬/MCP/계약 정합, 자율소비는 TARS 코어 갭.

## 페이즈별 상태

| Phase | 상태 | 비고 |
|---|---|---|
| 1 펌웨어 퍼셉션 캡처 | 부분완료 | 프로토콜/Go브리지/MOD/테스트/호스트통합 ✅. **오디오 `/v1/audio/clip` 실HW 검증 ✅**. 카메라 `/v1/camera/snapshot` = OPEN(스파이크 S) |
| 2 퍼셉션 루프 + TARS 채널 | 완료(mock) | `internal/perception`(config/loop/observe/sink), `perceive serve`, 단위테스트/계약. 오디오-온리 실HW 무크래시 검증 ✅ |
| 3 owner 식별 | 완료(mock) | `perceive enroll`, `identify`(owner/stranger/unknown 융합), 0600/0700. 음성식별 실HW 가용 |
| 4 REST+릴리스 | 완료(mock) | `/api/emotion`·`/api/perceive/status`, doctor 퍼셉션 진단, README |
| S 카메라 네이티브 브링업 | **OPEN** | 근본원인 확정, 수정 2회 실패. JTAG/계측 필요 |
| TARS 측 배선 | 핸드오프 완료 | `../tars` 스킬 v0.3.0 + INTEGRATION.md. 자율소비는 TARS 코어 갭 |

## 동작하는 것 (실HW/검증)

- 실 CoreS3에서 `perceive serve` (`TARS_STACKCHAN_BRIDGE=http`,
  `TARS_STACKCHAN_PERCEIVE_CAMERA=off`) → 실 마이크 캡처·관측 생성·무크래시.
- `/v1/audio/clip`·`/v1/sensors`·`/v1/status`(camera/microphone capability)·
  Bearer 401 — 실 디바이스 검증.
- owner enroll/식별(음성), `/api/emotion`(표정+LED+모션), doctor 진단 — mock/단위검증.
- `cd mcp-server && go test ./...` 전체 통과, `scripts/test/*-contract.sh` 통과,
  `goreleaser check` 통과.

## 미해결 / 재개 지점

### 1) Spike S — CoreS3 카메라 (최우선 비전 차단)
- 근본원인: 카메라 SCCB와 내부 I2C(AXP/AW9523/터치)가 GPIO12/11 동일 버스.
  `esp_camera_init`이 디바이스를 하드리셋.
- 시도·실패: ① config-only 동시버스공유(SCCB_Use_Port) ② M5 `In_I2C.release()`
  패턴 핸드오프(벤더링 `firmware/stackchan/host-overlay/imagein-camera-cores3/`).
- 다음 시도(JTAG/계측 필요): cam_hal/DVP(LCD_CAM·PSRAM DMA·XCLK LEDC),
  `i2c_del_master_bus` 실패(device 미제거), Moddable gBus 댕글링 중 식별.
  백트레이스가 CoreS3 USB-CDC로 불가 → JTAG 또는 esp32-camera/Moddable 소스
  계측이 사실상 필수. 상세: `embodied-bot-phase-1-spike.md`,
  `firmware/stackchan/patches/0002-host-camera-perception.md`.
- 대안: 외부 USB/Mac 웹캠을 비전 소스로(로드맵 out-of-scope 해제 필요).

### 2) TARS 자율 closed-loop (tars repo에서 재개)
- 현재: 관측은 `POST /v1/channels/webhook/inbound/stackchan`로 전송됨. TARS는
  이를 **영속 인박스로 저장+콘솔 노출**할 뿐 자동 에이전트 턴 없음.
- 재개(TARS repo): 스케줄/펄스 훅 또는 채널바인딩 에이전트가 inbound webhook을
  소비해 `tars-stackchan` 스킬로 세션 턴을 돌리는 코어 기능 추가. 스킬/MCP/
  페르소나/계약은 이미 정합(`../tars/workspace/skills/tars-stackchan/`).
  단 그 경로는 tars의 gitignore `workspace/`(런타임) — 영구 기여는 tars의
  추적 스킬 소스에 반영 + tars repo PR 필요.

## 이어가는 법 (Continuation)

- **tars-stackchan 측**: 이 문서 + `embodied-bot-roadmap.md` + 각 `phase-N` +
  `phase-1-spike.md` + `patches/0002`. 코드: `mcp-server/internal/perception`,
  `firmware/stackchan/host-overlay/imagein-camera-cores3` (카메라 핸드오프 벤더).
- **tars 측**: `../tars/workspace/skills/tars-stackchan/SKILL.md`(v0.3.0,
  Perception inbound 섹션) + `INTEGRATION.md`(설정/계약/갭). 자율소비 갭부터.
- 메모리(에이전트): `embodied-bot-architecture`, `cores3-camera-capture-blocked`,
  `firmware-camera-first-build`, `tars-side-wiring`.

## 빌드/검증 명령

```bash
cd mcp-server && go test ./...
sh scripts/test/firmware-bridge-contract.sh
sh scripts/test/firmware-upload-script-contract.sh
sh scripts/test/perception-loop-contract.sh
go run github.com/goreleaser/goreleaser/v2@latest check
# 실HW 오디오 루프: prepare → upload(all,DEPLOY_HOST) → perceive serve(CAMERA=off)
```

## 알려진 비스코프/메모

- 카메라 빌드는 IDF managed-component 함정 있음(첫 빌드 실패→재시도 절차는
  patch 0002). `goreleaser`는 `brews→homebrew_casks` deprecation 경고(에러 아님).
- `../tars` 변경분은 별도 repo·미커밋(gitignore workspace). 릴리스 태그는
  공개·비가역이라 버전/범위 확인 후 진행.
