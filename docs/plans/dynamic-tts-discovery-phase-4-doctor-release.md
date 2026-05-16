# Phase 4 — doctor end-to-end 진단 + Python 폐기 + 릴리스

**목표:** `tars-stackchan-mcp doctor` 가 음성 경로 전체(릴레이 health → mDNS 해석 →
디바이스 reachability)를 사전 진단해 이번 같은 무음 사고를 **사용자가 말 시키기 전에**
잡아낸다. Python 릴레이 잔재 제거 후 `v0.2.0` 릴리스.

**예상:** Medium (8-12h).

## 컨벤션 제약

- `doctor` 는 기존 `cmd/tars-stackchan-mcp/main.go:173` `runDoctor` 확장.
  기존 출력 포맷(`key: value`, `hint: ...`) 과 `deviceProbeHint`(line 240) 패턴 유지.
- 기존 mDNS 경고 로직(line 244-245)과 **충돌 없이 보강** — 릴레이용 새 진단 추가이지
  기존 디바이스 hint 삭제 아님.

## 작업

- [ ] `cmd/tars-stackchan-mcp/main.go` — `runDoctor` 에 TTS 음성 경로 진단 추가
  - `tts_relay_health`: `GET http://127.0.0.1:<port>/health` → ok/실패 + hint
    (실패 시 `hint: relay down; run 'brew services start tars-stackchan'`)
  - `tts_mdns`: `dns-sd -G v4 tars-stackchan-tts.local` 타임아웃 프로브 →
    해석 IP 또는 `hint: mDNS unresolved; set TARS_STACKCHAN_TTS_HOST=<mac-ip>
    and re-flash` (Phase 2 알려진 리스크의 사용자향 출구)
  - `tts_token`: `TARS_STACKCHAN_TOKEN`/`*_TTS_TOKEN` 존재 여부 (값 미출력)
  - `gemini_key`: `GEMINI_API_KEY`/`TARS_STACKCHAN_GEMINI_API_KEY` 존재 여부
  - 포트/호스트네임 상수는 Phase 2의 `internal/tts.DefaultTTSHostname` 재사용 (값 단일 출처)
  - 테스트: `main_test.go` — 각 진단 항목 up/down 분기, hint 문자열 단언

- [ ] `mcp-server/internal/tts` — doctor가 쓰는 프로브를 패키지 함수로 노출
  - `func ProbeHealth(ctx, baseURL string) error`, `func ProbeMDNS(ctx, hostname string) (string, error)`
  - Phase 3 `tts status` 와 동일 함수 공유 (중복 구현 금지 — 분석 함정 #4 회피)

- [ ] **Python 릴레이 폐기**
  - `scripts/dev/tts-remote-server.py` 삭제
  - `scripts/dev/run-local-tts.sh` 삭제 (Phase 3에서 shim화 → 여기서 제거)
  - `scripts/test/tts-remote-server-contract.sh` 삭제,
    `scripts/test/tts-remote-server-contract.py` 삭제
  - `.github/workflows/ci.yml` — "Test TTS relay contract" step을 Phase 1의
    `tts-relay-go-contract.sh` 만 남기도록 정리, `astral-sh/setup-uv` step이
    다른 용도로 안 쓰이면 제거 (firmware upload helper가 uv 쓰면 유지 — 확인 후 결정)
  - `firmware/README.md` 등 문서의 Python 릴레이 언급 일괄 갱신

- [ ] `docs/hardware-smoke.md` — 스모크 절차를 v0.2.0 경로로 재작성:
      brew service 기동 → mDNS hostname 플래시 → IP 변경 후 음성 유지 시나리오 포함.
      상단 Status/Date 갱신

- [ ] **릴리스 v0.2.0**
  - `git switch main && git pull --rebase`
  - 모든 페이즈 머지 완료 + `cd mcp-server && go test ./...` 그린 확인
  - `git tag -a v0.2.0 -m "v0.2.0 — dynamic mDNS TTS discovery + brew service relay"`
  - `git push origin v0.2.0` → `.github/workflows/release.yml` → GoReleaser →
    `devlikebear/homebrew-tap` 자동 갱신 (현재 v0.1.2 → v0.2.0)
  - 릴리스 노트: 사고 배경 + mDNS/brew service 전환 + **마이그레이션 안내**
    (`brew upgrade tars-stackchan && brew services start tars-stackchan`,
    펌웨어 1회 재플래시 권장)

## ✅ Checkpoint: Phase 4 완료 확인

**구현 확인:**
- [ ] `cd mcp-server && go test ./... && go vet ./...` 통과, 의존성 0 유지
- [ ] `rg -n "tts-remote-server|run-local-tts" .` → 코드/CI/문서에 잔재 없음
- [ ] `go run github.com/goreleaser/goreleaser/v2@latest check` 통과

**실행 확인:**
- [ ] 릴레이 down 상태에서 `tars-stackchan-mcp doctor` →
      `tts_relay_health` 실패 + `brew services start` hint 출력
- [ ] 릴레이 up + mDNS OK 에서 `doctor` → 음성 경로 전 항목 OK
- [ ] `TARS_STACKCHAN_TOKEN` 미설정 시 `doctor` 가 정확히 그 항목만 결함 지적

**수동 확인:**
- [ ] v0.2.0 태그 push 후 `release.yml` 성공, homebrew-tap formula 가 0.2.0 +
      service 스탠자 포함으로 갱신됨
- [ ] 클린 환경: `brew upgrade tars-stackchan` →
      `brew services start tars-stackchan` → `doctor` 그린 → `stackchan_speak` 음성 정상
- [ ] **사고 재현 시도 실패 확인**: Mac IP 변경 + 재부팅 후에도 무조작 음성 정상

**통과 시 프로젝트 완료. 실패 시 실패 항목 보고 → 원인 파악 → 수정 → 재검증.**

---

## 완료 후 메모

- 마케팅/문서 사이트가 있다면 음성 설정 안내 갱신 필요 여부 점검
- mDNS 가 특정 네트워크(게스트 WiFi, AP isolation)에서 실패 보고가 잦으면
  IP 폴백을 doctor가 더 적극 권유하도록 후속 튜닝 (Phase 2 알려진 리스크 추적)
