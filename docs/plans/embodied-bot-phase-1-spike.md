# Phase 1.0 Spike — CoreS3 캡처 가능성 검증 결과

> Phase 1: [`embodied-bot-phase-1-firmware-perception.md`](./embodied-bot-phase-1-firmware-perception.md)

조사일: 2026-05-16. 대상: `.work/stack-chan/firmware` (upstream Stack-chan, Moddable SDK, CoreS3 타깃).

## 결론 요약

| 센서 | 가능 여부 | 비고 |
|---|---|---|
| 마이크 | ✅ 가능 (upstream 구현 존재) | `/v1/audio/clip` 그대로 진행 가능 |
| 카메라 | ✅ **가능 (Moddable 카메라 모듈 호스트 통합)** | upstream stack-chan 스텁이지만 Moddable SDK가 CoreS3 카메라를 완전 지원. 폴백 불필요 |

## 2차 조사 (카메라 네이티브 스파이크) — 결과: GREEN

사용자 선택("카메라 네이티브 스파이크 먼저")에 따라 Moddable SDK 통합 가능성 조사. 결과 **통합 가능, 폴백 불필요**:

- Moddable SDK에 ECMA-419 카메라 모듈 존재: `~/.local/share/moddable/modules/io/imagein/camera/esp32/camera.c` (`embedded:io/image/in/camera`). JPEG 지원(`MODDEF_CAMERA_JPEG_QUALITY`, `isJPEG`), QVGA 등 framesize 테이블 보유.
- IDF 의존성은 매니페스트가 선언: `esp32-camera ^2.0.10` + `esp_jpeg ^1.3.1` (IDF component manager가 빌드 시 해결).
- **CoreS3 타깃 매니페스트가 카메라 HW를 완전 정의**: `~/.local/share/moddable/build/devices/esp32/targets/m5stack_cores3/manifest.json`에 `config.camera` 전체 핀맵(`powerdown/reset/xclk:2/pclk:45/href:38/vsync:46/scl:11/sda:12/i2c_port:1/d0..d7`) + `{"frameSize":"QVGA"}`. = M5Stack CoreS3 GC0308 DVP 배선. esp32-camera가 SCCB로 센서 오토디텍트.
- 즉시 쓸 수 있는 템플릿: `~/.local/share/moddable/examples/io/imagein/camera/camera-server-jpeg` (HTTP로 JPEG 응답 = 우리 `/v1/camera/snapshot`과 동형).

해석: 카메라 드라이버 신규 작성이 아니라 **호스트 펌웨어에 기존 Moddable 카메라 모듈을 include + 스텁 `camera.ts`를 실제 바인딩으로 교체**하는 범위 한정 작업.

### 잔여 리스크 (한정적 — 플래시 시점 검증, 차단 아님)

- `esp32-camera`/`esp_jpeg` IDF 컴포넌트가 빌드 환경에서 해결돼야 함(IDF component manager, 네트워크). Moddable 카메라 예제의 표준 경로.
- 카메라 프레임버퍼는 PSRAM 필요 — CoreS3는 8MB PSRAM, JPEG QVGA 소형, 여유 충분.
- **호스트 펌웨어 변경**이므로 런타임 MOD 플래시(`npm run mod`)가 아니라 **호스트 디플로이 플래시**(`scripts/dev/upload-firmware.sh all` / `TARS_STACKCHAN_DEPLOY_HOST=1`) 필요. 로드맵이 예고한 재플래시가 host deploy로 구체화됨.
- stack-chan 호스트 빌드에 카메라 매니페스트는 CoreS3 플랫폼 경로에서만 include — 기존 그래픽/오디오 모듈과 공존, 빌드 검증 필요(저위험).

## 최종 권장 (확정)

오디오-우선 재범위 **불필요**. 원래 4페이즈 계획 유지. owner 식별도 얼굴+음성 계획대로. Phase 1에 "호스트 펌웨어 카메라 통합" 작업을 추가(아래 §개정).

---

## Spike S 결과 (2026-05-16) — 근본 원인 확정: 공유 I2C 버스 소유권 충돌

camera snapshot이 CoreS3를 하드 리셋하는 원인을 코드로 규명(백트레이스는 USB-CDC라 불가).

**확정 근거:**
- M5Stack CoreS3는 카메라 SCCB와 내부 주변장치(AXP2101 `0x34`, AW9523 `0x58`, FT6206 터치)가 **동일 물리 I2C 버스 = GPIO12(SDA)/GPIO11(SCL)**. Moddable CoreS3 타깃 매니페스트가 `ft6206 sda:12 scl:11` 및 카메라 `scl:11 sda:12 i2c_port:1`로 확인.
- Moddable I2C(`modules/io/i2c/esp32/_i2c.c`)는 부팅 시 그 내부 버스를 **IDF 신규 `i2c_master` 드라이버**(`i2c_new_master_bus`)로 점유(AXP/AW9523/터치 구동).
- `esp_camera_init`→SCCB가 같은 버스를 **별도로** 잡으려 함. Moddable `camera.c`는 `.sccb_i2c_port`만 전달하고 **Moddable의 `i2c_master_bus_handle_t`를 esp32-camera에 공유/주입하지 않음**. esp32-camera SCCB(2.1.6, Kconfig 기본 NEW 드라이버)는 자체 버스를 생성 → 같은 포트/핀에 두 번째 버스 소유 시도 → IDF v6에서 충돌/abort → 리셋. (status/sensors는 I2C 미사용이라 정상; snapshot만 리셋; JS 예외 아님 — 모두 일치.)

**왜 작은 수정으로 안 되는가:** CoreS3에서 카메라 SCCB는 하드웨어상 내부 버스를 **반드시 공유**해야 한다(전용 포트로 분리 불가 — 핀이 물리적으로 한 버스). 따라서 포트/드라이버 config 토글로 해결 불가. 해결은 **버스 핸들 공유 통합** 수준:

- 옵션 A: Moddable `camera.c`/esp32-camera를 패치해 esp32-camera SCCB가 Moddable의 기존 `i2c_master` 버스 핸들을 재사용하도록 브리지 (esp32-camera 2.1.6의 외부 버스 주입 가능 여부 확인 필요 — 현재 미지원으로 보임 → esp32-camera SCCB 자체 패치 필요). **중간~큰 펌웨어-네이티브 작업, 불확실성 있음.**
- 옵션 B: 외부 카메라 소스(Mac/USB 웹캠)로 비전 입력, Stack-chan은 오디오+출력 (로드맵 out-of-scope였음, 해제 시).
- 옵션 C: 오디오-우선(마이크는 동일 SCCB 무관 — `embedded:io/audio/in` I2S, 충돌 없음 가능성 높음. **오디오 캡처는 별도 검증 가치 있음**).

**스파이크 S 판정**: 근본 원인 확정·문서화 완료. 실제 수정(옵션 A)은 독립적 펌웨어-네이티브 작업으로 별도 결정 필요.

### 오디오-우선 실HW 검증 결과 (2026-05-17) — ✅ PASS

debug 펌웨어 실디바이스(192.168.219.113)에서 **`/v1/audio/clip?ms=1500` 단독 검증**:
- `HTTP 200`, `audio/wav`, **48044 B = 16kHz·16bit·mono·1.5s 정확**, `time≈2.2s`
- 시리얼 리셋/패닉 전무, 디바이스 정상 유지 → **마이크(I2S)는 SCCB I2C 버스 충돌과 완전 무관**. Phase 1의 audio 000은 직전 카메라 크래시 리부팅의 巻添え였음(단독 검증된 적 없었음).

**실HW 오디오-온리 루프 스모크** (`BRIDGE=http` + `TARS_STACKCHAN_PERCEIVE_CAMERA=off`): 루프가 실 마이크로 관측 생성(`obs-*.wav`, 카메라 스킵), 시리얼 0바이트(무크래시), 종료 후 status 200. → **퍼셉션 루프는 실 CoreS3에서 오디오-온리로 지금 동작**(owner 음성식별 경로 포함). 카메라/비전만 Spike S 블록.

신규: `TARS_STACKCHAN_PERCEIVE_CAMERA=off`로 카메라 캡처 스킵(실HW 오디오-온리 운용). Spike S 해결 시 다시 켠다.

## (참고) 카메라가 막혔다고 본 1차 조사 — 무효화됨

> 아래는 upstream stack-chan 코드만 봤을 때의 1차 결론. Moddable SDK 레이어 조사로 **뒤집힘**. 기록 보존용.

## 마이크 (가능)

- `stackchan/microphone.ts`: `record(durationMilliSec=3000)` → 44바이트 WAV 헤더 포함 `ArrayBuffer` 반환. 실제 ESP32 `import AudioIn from 'embedded:io/audio/in'` 사용.
- `stackchan/manifest_microphone.json`: `audioIn` `sampleRate:16000`, `bitsPerSample:16` 정의. 우리 `/v1/audio/clip` 권장 포맷(16kHz mono)과 일치.
- → MOD에서 `Microphone.record(ms)` 호출 후 응답 = `/v1/audio/clip` 직접 구현 가능. 추가 네이티브 작업 없음.

## 카메라 (현 펌웨어로 불가)

근거:

- `stackchan/camera.ts`의 디바이스 `Camera`는 **스텁**: `start()/stop()` no-op, `capture()` → `undefined`.
- 유일한 실제 구현은 `stackchan/wasm/camera.ts` — 브라우저/WASM 시뮬레이터 경로(`useBrowserCamera:true`, `wasmBridge.capture`). ESP32 디바이스용 아님.
- `stackchan/main.ts:138`은 스텁 `Camera`를 사용. `robot.ts`는 타입만 import, `NULL_CAMERA` 널오브젝트 존재(카메라 옵셔널 전제).
- on-device 카메라 소비처는 프리뷰 페이스(`default-mods/on-robot-created.ts:128`)뿐이며 명시적으로 `useBrowserCamera:true` + "camera unavailable" 폴백.
- CoreS3 플랫폼(`platforms/m5stackchan_cores3/manifest.json`, `host/provider.js`)에 카메라 항목 없음 — IMU(BMI270)+오디오만 배선.
- 저장소 전체에 `esp32-camera`/`esp_camera`/`io/camera`/`gc0308`/`ov2640` 등 네이티브 카메라 모듈 참조 없음.
- WASM 카메라조차 `imageType:'jpeg'`는 `undefined` 반환(`tests/unit/wasm-stubs.test.ts:439`).

해석: CoreS3에 카메라 HW는 존재하나, 이 Moddable 펌웨어 코드베이스에 on-device 캡처 드라이버/바인딩이 없음. 캡처하려면 MOD가 아니라 **호스트 펌웨어에 네이티브 ESP32 카메라 모듈 통합**이 필요 — 검증된 바 없고 별도 펌웨어-네이티브 스파이크가 선행돼야 함.

## 권장 폴백 (사용자 결정 대기)

1. **오디오-우선 MVP로 재범위 (권장)**: Phase 1~4를 마이크 기반으로 진행. owner 식별은 음성 지문 우선. 카메라/비전은 별도 트랙으로 분리(아래 2). 견고한 기반 위에서 전체 계획 계속 진행.
2. **카메라 = 별도 펌웨어-네이티브 스파이크 트랙 (병렬, 보류)**: esp32-camera 네이티브 모듈을 Moddable+CoreS3 빌드에 통합 가능한지 별도 PoC. 성공 시 비전 페이즈 추가.
3. **외부 카메라 소스 (선택지)**: Mac/USB 웹캠을 비전 입력으로, Stack-chan은 오디오+출력 담당. (로드맵 out-of-scope였음 — 채택 시 명시적 해제 필요.)

권장: 1 + 2(보류). 3은 사용자가 "스택짱 자체의 눈" 의미를 일부 양보할 의향이 있을 때만.
