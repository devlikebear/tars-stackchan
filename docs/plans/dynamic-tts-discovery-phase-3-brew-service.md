# Phase 3 — Homebrew Service + CLI install/status

**목표:** `brew services start tars-stackchan` 한 번이면 릴레이가 로그인 시
자동 기동·재시작·재부팅 생존. 수동 `run-local-tts.sh` 의존 제거.
끝나면 "사용자가 아무것도 안 해도 떠 있는 릴레이" 가 완성된다.

**예상:** Medium (8-12h). GoReleaser/Homebrew 설정 + CLI 보조 커맨드.

## 컨벤션 제약

- `.goreleaser.yaml` `brews:` 블록(현재 lines 65-92) 확장. GoReleaser는
  `brews[].service` 스탠자로 Homebrew `service do ... end` 블록을 생성한다.
- 서비스가 구동할 실행체는 **Phase 1의 Go 바이너리** (`tars-stackchan-control tts serve`).
  formula `install:` 은 이미 `libexec`로 두 바이너리를 깐다 — 그대로 재사용.
- 토큰/키는 서비스 환경에 안전히 주입돼야 함. 평문을 plist에 굽지 않는다 →
  `tts serve` 가 env(`TARS_STACKCHAN_TOKEN`, `*_GEMINI_API_KEY`)에서 읽는
  Phase 1 동작에 의존. 서비스 env 설정 방법을 caveats에 명시.

## 작업

- [ ] `cmd/tars-stackchan-control/main.go` — `tts status` / `tts install` 구현
  - `tts status`: 로컬 릴레이 `GET http://127.0.0.1:<port>/health` 프로브 +
    `dns-sd -G v4 <hostname>` 해석 결과를 사람이 읽을 형태로 출력.
    `brew services list` 파싱은 하지 않음(취약) — health/mDNS 사실만 보고
  - `tts install`: env 가이드 출력기 (실제 설치는 brew가 함). `brew services` 사용법,
    `TARS_STACKCHAN_TOKEN`/`GEMINI_API_KEY` 설정법을 그대로 안내. doctor와 톤 일치
  - 테스트: `main_test.go` — `tts status` 릴레이 down 시 비0 종료+안내,
    up 시 0 종료. `tts install` 출력에 필수 env 키 포함

- [ ] `.goreleaser.yaml` — `brews[0]` 에 `service:` 추가
  - `service: |` 블록: `run [opt_libexec/"tars-stackchan-control", "tts", "serve"]`,
    `keep_alive true`, `log_path var/"log/tars-stackchan-tts.log"`,
    `error_log_path` 동, `environment_variables` 는 굽지 않고 caveats로 안내
  - 기존 `install:` 에 `(bin/"tars-stackchan-control")...write_env_script` 유지 확인
  - `caveats:` 갱신: 기존 Claude Code 연결 안내 아래에 TTS 서비스 절 추가:
    ```
    Start the Gemini TTS relay as a background service:
      export TARS_STACKCHAN_TOKEN=...      # firmware MOD 토큰과 동일
      export GEMINI_API_KEY=...            # 또는 TARS_STACKCHAN_GEMINI_API_KEY
      brew services start tars-stackchan
    The relay advertises tars-stackchan-tts.local over mDNS.
    음성이 안 들리면 펌웨어를 IP로 재플래시: TARS_STACKCHAN_TTS_HOST=<mac-ip>
    ```

- [ ] `scripts/test/release-config-contract.sh` — `service` 스탠자 존재 +
      `keep_alive` + run 타깃이 `tars-stackchan-control tts serve` 인지 단언
      (기존 release-config 계약 패턴 모방, CI에서 이미 실행됨)

- [ ] `scripts/dev/run-local-tts.sh` — **deprecation shim 으로 축소**
  - 내부에서 `tars-stackchan-control tts serve "$@"` 를 exec 하도록 교체 +
    stderr 에 "deprecated: use `brew services start tars-stackchan`" 1줄
  - (완전 삭제는 Phase 4. 여기선 Python 경로 끊고 Go로 리다이렉트만)

- [ ] `docs/hardware-smoke.md` / `README.md` — 릴레이 기동을
      `brew services start tars-stackchan` 기준으로 갱신, 수동 스크립트는 "개발용" 격하

## ✅ Checkpoint: Phase 3 완료 확인

**구현 확인:**
- [ ] `cd mcp-server && go test ./... && go vet ./...` 통과
- [ ] `go run github.com/goreleaser/goreleaser/v2@latest check` 통과
      (README lines 218-222 의 기존 검증 명령)
- [ ] `scripts/test/release-config-contract.sh` 통과 (service 스탠자 단언)

**실행 확인:**
- [ ] `go run .../goreleaser release --snapshot --clean --skip=publish` →
      생성된 Formula 에 `service do ... keep_alive true ... end` 포함
- [ ] `tars-stackchan-control tts status` — 릴레이 up 시 health/mDNS OK 출력 0종료,
      down 시 안내 + 비0 종료

**수동 확인:**
- [ ] 로컬에서 스냅샷 Formula 설치 → `export TARS_STACKCHAN_TOKEN/GEMINI_API_KEY` →
      `brew services start tars-stackchan` → `tts status` OK
- [ ] **Mac 재부팅(또는 로그아웃/로그인)** 후 사용자 수동 조작 없이 `stackchan_speak`
      → 음성 정상 (= 무인 자동 기동 입증, 사고 2번째 축 해소 검증)
- [ ] `brew services stop tars-stackchan` → 릴레이/ mDNS 광고 모두 정리됨

**통과 시 사용자 확인 후 Phase 4로. 실패 시 실패 항목 보고 → 원인 파악 → 수정 → 재검증.**
