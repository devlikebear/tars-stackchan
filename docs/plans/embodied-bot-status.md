# Embodied Bot — Status & Continuation

> 단일 진실 소스. 다음 세션/사람이 (tars-stackchan **또는** tars repo에서)
> 이어갈 수 있도록 현재 상태·미해결·재개 방법을 정리한다.
> 최종 갱신: 2026-05-17.

## 한 줄 요약

tars-stackchan = 몸(감각 엣지+액추에이터), TARS = 뇌(역할분리 hybrid).
**오디오 퍼셉션·owner 음성식별·REST·임베디먼트·TARS 스킬/계약 = 완료(오디오는
실HW 검증).** 카메라/비전 = `esp_video` S2 host overlay로 실HW 검증 완료.
기존 Moddable `esp32-camera` 경로는 2회 실패했고, 공식 StackChan/ESP-BSP가 쓰는
`esp_video`/V4L2 경로로 교체함. TARS 자율 closed-loop = TARS Phase 1-3
PR 경로에서 self-sensory Percept 수집·cognition·BodyAction 라우팅까지 연결됨.

## 페이즈별 상태

| Phase | 상태 | 비고 |
|---|---|---|
| 1 펌웨어 퍼셉션 캡처 | 완료(실HW) | 프로토콜/Go브리지/MOD/테스트/호스트통합 ✅. `/v1/camera/snapshot`→320x240 JPEG ✅, `/v1/audio/clip`→16 kHz PCM WAV ✅ |
| 2 퍼셉션 루프 + TARS 채널 | 완료(mock) | `internal/perception`(config/loop/observe/sink), `perceive serve`, 단위테스트/계약. 오디오-온리 실HW 무크래시 검증 ✅ |
| 3 owner 식별 | 완료(mock) | `perceive enroll`, `identify`(owner/stranger/unknown 융합), 0600/0700. 음성식별 실HW 가용 |
| 4 REST+릴리스 | 완료(mock) | `/api/emotion`·`/api/perceive/status`, doctor 퍼셉션 진단, README |
| S 카메라 네이티브 브링업 | 해결됨 | Moddable `esp32-camera` 경로 수정 2회 실패 후 공식 StackChan/ESP-BSP `esp_video` V4L2 경로로 교체. 실 CoreS3 플래시/스모크 통과 |
| TARS 측 배선 | PR 경로 구현 | `../tars` Phase 1-3 PR: provider-neutral Percept/BodyAction, embodiment gate/cognition, MCP action routing. tars-stackchan Phase 4가 Stack-chan provider adapter와 payload 계약을 맞춤 |

## 동작하는 것 (실HW/검증)

- 실 CoreS3에서 `perceive serve` (`TARS_STACKCHAN_BRIDGE=http`,
  `TARS_STACKCHAN_PERCEIVE_CAMERA=off`) → 실 마이크 캡처·관측 생성·무크래시.
- `/v1/camera/snapshot` → HTTP 200, JPEG 320x240(1,831 bytes), 실 CoreS3 검증.
- `/v1/audio/clip` → HTTP 200, RIFF/WAVE PCM 16-bit mono 16000 Hz(16,044 bytes).
- `/v1/sensors`·`/v1/status`(camera/microphone capability)·
  Bearer 401 — 실 디바이스 검증.
- owner enroll/식별(음성), `/api/emotion`(표정+LED+모션), doctor 진단 — mock/단위검증.
- `cd mcp-server && go test ./...` 전체 통과, `scripts/test/*-contract.sh` 통과,
  `goreleaser check` 통과.

## 해결 기록 / 재개 지점

### 1) Spike S/S2 — CoreS3 카메라 (해결)
- 근본원인: 카메라 SCCB와 내부 I2C(AXP/AW9523/터치)가 GPIO12/11 동일 버스.
  `esp_camera_init`이 디바이스를 하드리셋.
- 시도·실패: ① config-only 동시버스공유(SCCB_Use_Port) ② M5 `In_I2C.release()`
  패턴 핸드오프(벤더링 `firmware/stackchan/host-overlay/imagein-camera-cores3/`).
- 기존 `esp32-camera` 경로를 계속 팔 경우(JTAG/계측 필요):
  cam_hal/DVP(LCD_CAM·PSRAM DMA·XCLK LEDC), `i2c_del_master_bus`
  실패(device 미제거), Moddable gBus 댕글링 중 식별.
  백트레이스가 CoreS3 USB-CDC로 불가 → JTAG 또는 esp32-camera/Moddable 소스
  계측이 사실상 필수. 상세: `embodied-bot-phase-1-spike.md`,
  `firmware/stackchan/patches/0002-host-camera-perception.md`.

#### 웹 리서치 업데이트 (2026-05-17)

조사 소스:
- M5Stack 공식 CoreS3 Camera 예제:
  https://docs.m5stack.com/en/arduino/m5cores3/camera
- upstream `stack-chan/stack-chan` Moddable 카메라 스텁:
  https://raw.githubusercontent.com/stack-chan/stack-chan/677224032e9ca25ac5c327b2eacd0034804b756f/firmware/stackchan/camera.ts
- M5Stack 공식 StackChan 펌웨어:
  https://github.com/m5stack/StackChan/tree/da156e1fa0e1c2a5e00b78fbf69b1f7e7bca0483
- Espressif `m5stack_core_s3` BSP:
  https://github.com/espressif/esp-bsp/tree/0789f79ab745659e6006c02a3884d39c357c7551/bsp/m5stack_core_s3

확인한 사실:
- `stack-chan/stack-chan`의 Moddable 디바이스 `camera.ts`는 여전히 `capture()`
  가 `undefined`를 반환하는 스텁이다. 따라서 현재 tars-stackchan의
  `embedded:io/image/in/camera` 통합은 upstream Stack-chan 기능 재사용이 아니라
  Moddable SDK 카메라 모듈을 호스트에 직접 붙인 별도 경로다.
- M5Stack 공식 Arduino 예제는 CoreS3 GC0308 핀맵을 `SDA=12`, `SCL=11`,
  `D0..D7=39,40,41,42,15,16,48,47`, `VSYNC=46`, `HREF=38`, `PCLK=45`로
  쓰고, `M5.In_I2C.release()` 뒤 `esp_camera_init()`을 호출한다. 이 예제는
  `RGB565/QVGA`, `fb_count=2`, `PSRAM`, `sccb_i2c_port=-1`, `xclk=-1`로
  동작시키는 패턴이다.
- 더 중요한 새 단서: M5Stack 공식 StackChan 펌웨어와 Espressif `m5stack_core_s3`
  BSP는 `esp32-camera` 직접 사용이 아니라 **`esp_video` + V4L2 DVP 경로**를
  쓴다. 둘 다 SCCB를 새로 만들지 않고 기존 I2C 핸들을 주입한다:
  `init_sccb=false`, `i2c_handle=<shared bus>`, `freq=100000`.
- Espressif BSP는 카메라 시작 전에 `bsp_feature_enable(BSP_FEATURE_CAMERA, true)`
  로 IO expander `BSP_CAMERA_EN`과 PMU 카메라 3.3V 레일을 켠 뒤 `esp_video_init`
  을 호출한다. M5Stack StackChan 펌웨어도 `AW9523(0x58)`/AXP2101을 초기화한
  뒤 카메라를 초기화하고, `StackChanCamera`가 `/dev/video*`를 열어 V4L2
  `DQBUF/QBUF` 프레임을 읽고 JPEG로 변환해 웹소켓으로 보낸다.

해석:
- 기존 2회 실패는 모두 **Moddable `embedded:io/image/in/camera` =
  `esp32-camera` 경로** 안에서 해결하려 한 것이다. 공식 StackChan/ESP-BSP가
  실제 CoreS3에서 택한 안정 경로는 `esp_video`이므로, 다음 스파이크는
  `esp32-camera`/SCCB 핸드오프를 더 파는 것이 아니라 `esp_video` 기반
  CoreS3 전용 host camera binding으로 갈 가치가 가장 높다.
- 이는 JTAG 없이도 검증 가능한 경로다. 먼저 순정 M5Stack StackChan firmware
  또는 ESP-BSP `display_camera_video` 예제를 CoreS3에 올려 카메라 자체와 보드
  레일/IO expander를 sanity-check 한 뒤, tars-stackchan host overlay에 같은
  `esp_video` 초기화 코드를 이식한다.

권장 S2 스파이크:
1. **보드 sanity**: M5Stack StackChan firmware 또는 ESP-BSP
   `examples/display_camera_video`를 CoreS3에 빌드/플래시해 화면에 카메라
   프레임이 뜨는지 확인한다. 실패하면 하드웨어/보드 설정 문제.
2. **host binding 교체**: 현재 `imagein-camera-cores3`의 `esp_camera_init`
   경로 대신 CoreS3 전용 `esp_video`/V4L2 binding을 만든다. manifest에는
   `esp_video`, GC0308 DVP `320x240` 설정, 필요 시 `esp_image_effects`/`esp_jpeg`
   를 추가한다.
3. **I2C/전원 순서**: Moddable 내부 I2C 버스를 삭제하지 말고, 공식 StackChan처럼
   기존 버스 핸들을 `esp_video_init_sccb_config_t.i2c_handle`에 주입한다. 카메라
   enable은 AW9523 `CAMERA_EN`/AXP2101 CAM 3.3V 레일을 공식 BSP와 같은 순서로
   보장한다.
4. **snapshot API**: `/v1/camera/snapshot`은 V4L2에서 프레임 1장을 읽고 JPEG로
   변환해 반환한다. MVP 비전 목적은 1-2Hz 스냅샷이면 충분하므로 연속 스트리밍
   구현은 계속 제외한다.
5. **실패 시 fallback**: S2도 리셋이면 그때 JTAG/계측으로 이동한다. 사용자
   인식 기능을 먼저 체감해야 하면 Mac/USB 웹캠을 임시 비전 소스로 두는 선택지가
   있으나, 로드맵의 "CoreS3 내장 카메라" 전제를 해제해야 한다.

#### S2 적용 내용 (2026-05-17)

- `firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c`를 CoreS3 전용
  `esp_video` binding으로 교체했다. 흐름은 `i2c_master_get_bus_handle`
  → `esp_video_init_sccb_config_t{init_sccb=false, i2c_handle=<shared bus>}`
  → `/dev/video2` V4L2 `VIDIOC_DQBUF` → `fmt2jpg` 변환 → 기존
  `embedded:io/image/in/camera` JS API(`onReadable`, `read`, disposable buffer)다.
- overlay manifest에 `esp_video` 의존성을 추가했고, `sdkconfig/sdkconfig.defaults`
  에 `CONFIG_ESP_VIDEO_ENABLE_DVP_VIDEO_DEVICE=y`,
  `CONFIG_CAMERA_GC0308_DVP_RGB565_BE_320X240_20FPS=y`를 추가했다.
- `prepare-firmware-upload.sh`는 CoreS3 host manifest에 overlay manifest와
  overlay `SDKCONFIGPATH`를 주입하고, 공식 BSP와 맞추기 위해 `xclk: -1`
  (외부 20 MHz 클럭)로 설정한다.
- 현재 검증: `scripts/test/firmware-upload-script-contract.sh` 통과,
  `git diff --check` 통과, `prepare-firmware-upload.sh` 실제 실행 통과,
  CoreS3 host firmware build/flash 통과.
- 실HW 검증: `scripts/dev/upload-firmware.sh all`로 host+MOD 플래시 성공
  (`/dev/cu.usbmodem1101`, MAC `44:1b:f6:e1:e4:38`, IP `192.168.219.111`).
  이후 `scripts/test/hardware-smoke.sh` 통과, `/v1/camera/snapshot?max_width=320`
  은 HTTP 200 JPEG 320x240, `/v1/audio/clip?ms=500`은 HTTP 200 WAV 16 kHz.
- 구현상 주의: `esp_video` DVP 드라이버의 `VIDIOC_S_FMT`는 현재 센서 기본
  포맷 외 조합을 `EINVAL`로 거부한다. overlay는 실패 시 `VIDIOC_G_FMT`의 현재
  포맷으로 폴백해 320x240 JPEG 캡처를 안정화했다.

### 2) TARS 자율 closed-loop (TARS Phase 1-3 PR 경로)
- 현재 구현 경로: 관측은 기존
  `POST /v1/channels/webhook/inbound/stackchan` 또는 전용
  `/v1/embodiment/percept/{provider}`로 들어오고, payload의 `x-embodiment`,
  `owner`, `modality`, `media_ref`를 TARS의 provider-neutral Percept로 정규화한다.
- TARS Phase 1-3 PR은 self-sensory Percept를 채널 인박스에 저장한 뒤
  embodiment gate/cognition으로 넘긴다. owner 음성/지시성 관측은 session-bound
  autonomous turn을 만들고, 응답의 `tars-body-action` 블록은 provider capability에
  맞을 때만 `speak`/`express`/`move`/`led` 액션으로 되돌아간다.
- tars-stackchan Phase 4는 위 경로에 맞춰 Stack-chan MCP provider adapter,
  `bodyprovider` 계약, capability 선언, legacy webhook 호환 payload를 제공한다.
- **아키텍처 결정 (2026-05-17)**: TARS 공용 로직을 신규 프로젝트용 pkg로
  export하는 안은 **기각**. `tool` 패키지가 internal 전반에 결합돼 22k LOC가
  영구 공개 API + `config` 전염이 되고, 이미 `pkg/tarsclient`+스킬/계약 기반
  "서버 위임" 모델이 작동 중(중복코드 0)이며 신규 에이전트는 TARS 서버 상주
  가정 OK로 확인됨. 코드로 확인: `InboundWebhook`/`InboundTelegram` 모두
  `appendChannelMessage`로 저장만 함 → 갭은 스택짱 한정이 아닌 채널 전반
  일반 갭. **tars#875** 로 추적: `feat(channels): 채널 inbound webhook
  자율 소비 (channel-bound autonomous turn)`.

## 이어가는 법 (Continuation)

- **tars-stackchan 측**: 이 문서 + `embodied-bot-roadmap.md` + 각 `phase-N` +
  `phase-1-spike.md` + `patches/0002`. 코드: `mcp-server/internal/perception`,
  `firmware/stackchan/host-overlay/imagein-camera-cores3` (S2 `esp_video` overlay).
  다음 작업은 이 카메라/오디오 실HW 캡처를 perception loop의 실제 비전 입력으로
  켜서 owner 얼굴/시선 인식 루프로 연결하는 것.
- **tars 측**: Phase 1-3 PR의 `internal/embodiment`, `/v1/embodiment/percept/*`,
  기존 inbound webhook의 embodiment autodetect, MCP action transport를 확인한다.
  설정 예시는 `config/tars.config.example.yaml`의 `embodiment.providers`와 이 repo의
  `mcp-server/examples/tars/tars.config.yaml`을 함께 본다.
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
# 실HW 카메라 스냅샷:
# TARS_STACKCHAN_BASE_URL=http://192.168.219.111 scripts/test/hardware-smoke.sh
# curl -H "Authorization: Bearer $TARS_STACKCHAN_TOKEN" \
#   "http://192.168.219.111/v1/camera/snapshot?max_width=320" -o /tmp/tars-stackchan-camera.jpg
```

## 알려진 비스코프/메모

- 카메라 빌드는 IDF managed-component 함정 있음(첫 빌드 실패→재시도 절차는
  patch 0002). S2에서는 overlay manifest가 target manifest 뒤에서
  `SDKCONFIGPATH=./sdkconfig-combined`를 적용해야 GC0308 Kconfig가 실제 빌드에
  반영된다. `goreleaser`는 `brews→homebrew_casks` deprecation 경고(에러 아님).
- `../tars` 변경분은 별도 repo·미커밋(gitignore workspace). 릴리스 태그는
  공개·비가역이라 버전/범위 확인 후 진행.
