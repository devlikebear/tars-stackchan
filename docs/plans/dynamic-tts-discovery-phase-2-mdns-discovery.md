# Phase 2 — mDNS 광고 + 펌웨어 hostname bake

**목표:** Go 릴레이가 `tars-stackchan-tts.local` 을 mDNS로 광고하고,
`prepare-firmware-upload.sh`가 IP 대신 그 hostname을 펌웨어에 굽는다. 끝나면
**Mac IP가 바뀌어도** 디바이스가 이름으로 릴레이를 찾는다 (재플래시 불필요).

**예상:** Medium (8-12h). 펌웨어 재플래시 1회 필요(USB).

## 핵심 설계 (확정)

- mDNS 광고 = macOS 내장 `dns-sd -R` shell-out (제로 의존성 유지).
  서비스: `dns-sd -R "tars-stackchan-tts" _http._tcp local <port>` +
  hostname A 레코드는 `dns-sd -P` 또는 `tars-stackchan-tts.local` 직접 등록 방식 검증.
  > 구현 시 먼저 `dns-sd` 옵션 실측: 디바이스 upstream `RemoteTTS`가 요구하는 건
  > **hostname resolution**(`tars-stackchan-tts.local` → A 레코드)이지 서비스 브라우징이
  > 아니다. `dns-sd -P <name> <type> <domain> <port> <host>.local <ipv4>` 로
  > 프록시 등록하거나, 동적 IP면 `dns-sd -R` + 호스트의 자기 `.local` 활용.
  > Phase 2 작업 0에서 이 방식을 1차 실측 확정한다.
- IP 폴백 유지: `prepare-firmware-upload.sh`는 `TARS_STACKCHAN_TTS_HOST`가 설정되면
  그 값(IP든 hostname이든)을 그대로 굽고, **미설정 시 기본값 = `tars-stackchan-tts.local`**.
  (기존: 미설정 시 `ipconfig getifaddr en0`. 이 기본값만 교체, override 경로는 보존.)

## 알려진 리스크 (계획에 내장된 안전장치)

`cmd/tars-stackchan-mcp/main.go:240-245`에 작성자가 이미
"mDNS (`*.local`) frequently fails to resolve" 를 디바이스 호스트네임에 대해 문서화함.
릴레이 `.local` 도 ESP32에서 같은 불안정을 겪을 수 있다. 따라서:

- mDNS 는 **기본 자동 경로**, `TARS_STACKCHAN_TTS_HOST=<IP>` 는 **확실한 수동 폴백**.
  둘 다 1급 지원. 문서에 "안 들리면 IP를 박아라" 명시.
- Phase 4 doctor 가 mDNS 해석 성공 여부를 사전 진단(아래 Phase 4 참조).

## 작업

- [ ] **작업 0 (실측 스파이크):** macOS에서 `dns-sd`로 `tars-stackchan-tts.local`을
      광고하고 ESP32 upstream `RemoteTTS`가 해석하는지 1차 검증.
  - 수동으로 `dns-sd -P tars-stackchan-tts _http._tcp local 18080 tars-stackchan-tts.local <en0-ip>`
    띄운 뒤, `dns-sd -G v4 tars-stackchan-tts.local` 로 해석 확인
  - 별도 LAN 기기(또는 디바이스 시리얼 로그)에서 `tars-stackchan-tts.local` ping/resolve
  - 결과를 `docs/plans/dynamic-tts-discovery-phase-2-mdns-discovery.md` 하단
    "실측 결과" 섹션에 기록. **여기서 dns-sd 정확한 명령형을 확정**하고 이후 작업에 반영
  - 검증: `tars-stackchan-tts.local` 이 현재 en0 IP로 resolve 됨

- [ ] `mcp-server/internal/tts/mdns.go` — `func Advertise(ctx, serviceName, hostname string, port int) (stop func(), err error)`
  - `os/exec`로 작업 0에서 확정한 `dns-sd` 명령 실행, `exec.CommandContext`로
    릴레이 종료 시 자동 kill (Python의 수동성을 제거하는 핵심)
  - `dns-sd` 미존재(비 macOS) 시 에러 대신 경고 로그 + no-op (`runtime.GOOS != "darwin"`)
  - `serviceName`/`hostname` 기본값 상수 `DefaultTTSHostname = "tars-stackchan-tts.local"`,
    `DefaultServiceName = "tars-stackchan-tts"` (한 곳에 정의, 펌웨어 스크립트와 값 일치)
  - 테스트: `mdns_test.go` — 비 darwin no-op, 명령 인자 조립 검증(exec는 fake로 주입)

- [ ] `mcp-server/internal/tts/serve.go` — `Serve` 진입 시 `Advertise` 호출
  - `--mdns`(기본 true) / `--mdns-hostname`(기본 `DefaultTTSHostname`) 플래그
  - 컨텍스트 취소 시 `stop()` 보장 (defer)
  - 시작 로그: `advertising mDNS tars-stackchan-tts.local -> :<port>`

- [ ] `cmd/tars-stackchan-control/main.go` — `tts serve` 플래그에 `--mdns`,
      `--mdns-hostname` 추가, `printUsage` 갱신

- [ ] `scripts/dev/prepare-firmware-upload.sh` — 기본값 교체 (override 보존)
  - 현재 `lines 13-23`: `tts_host="${TARS_STACKCHAN_TTS_HOST:-}"` 미설정 시
    `ipconfig getifaddr en0`. → 미설정 시 기본값을 `tars-stackchan-tts.local` 로 변경
  - `TARS_STACKCHAN_TTS_HOST` 가 설정되면 그대로 사용 (IP/hostname 무관) — 변경 없음
  - 안내 로그: `ready: patched host TTS remote server to <host>:<port>` 유지 + host가
    `.local`이면 "mDNS hostname (relay must advertise it)" 한 줄 추가
  - `scripts/test/firmware-upload-script-contract.sh` 갱신: 기본값이
    `tars-stackchan-tts.local`로 구워지는지 + `TARS_STACKCHAN_TTS_HOST` override가
    여전히 우선하는지 두 케이스 단언

- [ ] `firmware/README.md` / `docs/hardware-smoke.md` — TTS 설정 절을
      "기본 mDNS hostname, 안 들리면 `TARS_STACKCHAN_TTS_HOST=<IP>` 폴백" 으로 갱신

## ✅ Checkpoint: Phase 2 완료 확인

**구현 확인:**
- [ ] `cd mcp-server && go test ./... && go vet ./...` 통과, `go.mod` 의존성 0 유지
- [ ] `scripts/test/firmware-upload-script-contract.sh` 통과 (기본값 + override 2케이스)
- [ ] 작업 0 "실측 결과" 섹션이 dns-sd 확정 명령과 함께 채워짐

**실행 확인:**
- [ ] `tars-stackchan-control tts serve` 기동 중 다른 터미널에서
      `dns-sd -G v4 tars-stackchan-tts.local` → 현재 en0 IP 반환
- [ ] 릴레이 프로세스 kill → `dns-sd -G v4 tars-stackchan-tts.local` 더 이상 해석 안 됨
      (광고가 릴레이 수명에 묶여 자동 정리되는지)

**수동 확인 (펌웨어 재플래시 1회 — 사용자 USB 협조 필요):**
- [ ] USB 연결 후 `TARS_STACKCHAN_TTS_HOST` 미설정 상태로 `upload-firmware.sh all`
      (`DEPLOY_HOST=1`) → host manifest에 `tts.host = "tars-stackchan-tts.local"` 구워짐
- [ ] 디바이스 부팅 후 `stackchan_speak` → 음성 정상
- [ ] **Mac Wi-Fi 재접속해 en0 IP를 바꾼 뒤** 재플래시 없이 `stackchan_speak` →
      여전히 음성 정상 (= IP 독립성 입증, 사고 재발 방지 핵심 검증)
- [ ] 폴백 검증: `TARS_STACKCHAN_TTS_HOST=<IP>` 로 재플래시 → 그 IP로 동작

**통과 시 사용자 확인 후 Phase 3로. 실패(특히 mDNS 미해석) 시: IP 폴백으로 음성
복구 가능함을 먼저 확인 → mDNS 실패 원인(라우터 mDNS 차단/AP isolation 등) 보고 →
사용자와 대응 결정.**

---

## 실측 결과 (작업 0, 2026-05-16 확정)

**확정 명령:**
```
dns-sd -P tars-stackchan-tts _http._tcp local <port> tars-stackchan-tts.local <primary-ipv4>
```

검증 (macOS 호스트, en0=192.168.219.115):
- `dns-sd -G v4 tars-stackchan-tts.local` → `192.168.219.115` 해석됨 (TTL 240)
- `dscacheutil -q host -a name tars-stackchan-tts.local` → IP 반환
- `ping tars-stackchan-tts.local` → 응답. 즉 표준 mDNS 리졸버(ESP32 포함) 해석 가능
- 등록 로그: `Name now registered and active` (레코드 + 서비스 양쪽)

**핵심 제약 / 설계 반영:**
- `dns-sd -P` 는 **IP를 인자로 고정**한다. 릴레이가 기동 시 현재 primary IPv4 를
  스스로 계산해 넘겨야 한다. (Go 무의존 방법: `net.Dial("udp","8.8.8.8:80")` 후
  `LocalAddr().IP` — 실제 패킷 안 보냄, outbound source IP 획득.)
- `-P` 등록은 프로세스 수명에 묶인다(스파이크에서 kill 시 해석 중단 확인). →
  릴레이 종료 시 mDNS 광고 자동 정리됨 (Python 수동성 제거 목표 충족).
- 실행 중 Mac IP 변경 시 `-P` 레코드는 stale. 완화책: brew service(Phase 3)가
  관리, doctor(Phase 4)가 mDNS 해석 사전 진단, `TARS_STACKCHAN_TTS_HOST=<IP>`
  1급 폴백 유지(알려진 리스크 섹션). Phase 2 범위에서는 기동 시점 IP 고정으로 충분.
- 참고: macOS는 `<LocalHostName>.local`(예 `chshin-macbook.local`)을 자동 Bonjour
  광고하며 IP 변경을 네이티브 추적하나, 머신명 종속이라 프로젝트 고정명으로 부적합 →
  `-P` 프록시 방식 채택.
