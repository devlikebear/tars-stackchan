# Roadmap — Dynamic TTS Discovery & Relay Automation

## 배경 / 사고 교훈

Stack-chan 음성 무음 사고. 표정·모션은 정상인데 소리만 안 났다.

근본 원인 2가지:

1. **하드코딩된 IP** — `scripts/dev/prepare-firmware-upload.sh`가 펌웨어 플래시 시
   호스트 `en0` IP를 `config.tts.host`(raw IP)로 host manifest에 굽는다. DHCP로
   Mac IP가 바뀌면 디바이스가 죽은 IP로 TTS를 요청 → 영구 무음. 런타임 오버라이드 없음.
2. **수동 릴레이** — Gemini TTS 릴레이(`scripts/dev/tts-remote-server.py`)가
   `run-local-tts.sh`로 수동 기동. 떠 있지 않으면 무음. 자동 기동/감시 없음.

## 가치 제안

> "Mac IP가 바뀌든 재부팅을 하든, 사용자가 아무것도 안 해도 Stack-chan이 말한다."

음성 경로를 **재플래시 불필요(IP 독립) + 무인 자동 기동** 상태로 만든다.

## 핵심 설계 결정 (확정됨)

| 축 | 결정 | 근거 |
|---|---|---|
| IP 동적 해결 | **mDNS `.local`** (`tars-stackchan-tts.local`) | upstream `RemoteTTS`(`tts-remote`)가 이미 `.local` 호스트네임 resolve 지원 (`.work/stack-chan/firmware/docs/text-to-speech_ja.md:45`). 펌웨어 신규 코드 거의 없음 |
| 폴백 | `TARS_STACKCHAN_TTS_HOST`(IP) 가 `.local` 기본값을 **항상 이긴다** | 작성자가 `cmd/tars-stackchan-mcp/main.go:240-245`에 mDNS 불안정을 이미 문서화. mDNS=기본 자동, IP=확실한 수동 폴백 2단 구조 |
| 릴레이 | **Go 포팅** + Homebrew `service` 블록 | 기존 Go 바이너리 컨벤션 통일, uv/python 런타임 의존 제거, `brew services`로 무인 기동 |
| mDNS 광고 구현 | macOS 내장 **`dns-sd -R` shell-out** | `mcp-server/go.mod` 제로 외부 의존성 컨벤션 유지. 릴레이는 macOS 데스크탑 전용 |

## 범위 (MVP)

- Go TTS 릴레이 (`tars-stackchan-control tts serve`) — Python 파리티
- `dns-sd -R` 기반 mDNS 광고 (`tars-stackchan-tts.local`)
- `prepare-firmware-upload.sh` — hostname 기본값 bake (IP override 유지)
- Homebrew `service` 블록 + `tts install`/`tts status` CLI
- `doctor` — mDNS 해석 + 릴레이 health end-to-end 검증
- Python 릴레이 / `run-local-tts.sh` 폐기, docs 갱신, v0.2.0 릴리스

## Out of Scope (명시적 제외)

- **BLE 디스커버리** — 같은 WiFi LAN이고 오디오는 어차피 WiFi 필요. mDNS가 네이티브로 해결. 폐기.
- mDNS 자체 구현(stdlib multicast) — `dns-sd` shell-out으로 충분
- Linux/Windows 릴레이 — macOS 데스크탑 전용 (기존 전제 유지)
- 릴레이 멀티테넌시/원격 노출 — 로컬 전용 토큰 모델 유지
- 디바이스 펌웨어의 새 디스커버리 기능 — upstream `.local` resolve 재사용으로 불필요

## 페이즈 (수직 슬라이스)

| Phase | 산출물 | 끝나면 동작하는 것 |
|---|---|---|
| **1. Go 릴레이** | `tts serve` 서브커맨드, Gemini 파리티 | `curl :18080/api/tts?...` → WAV. Python과 동등 |
| **2. mDNS + 펌웨어 bake** | `dns-sd -R` 광고 + hostname bake | IP 바뀌어도 디바이스가 `.local`로 릴레이 도달 |
| **3. Homebrew service** | `brews.service` + `tts install/status` | `brew services start tars-stackchan` → 로그인 시 자동 기동 |
| **4. doctor + 릴리스** | doctor end-to-end 검증, Python 폐기, v0.2.0 | `doctor`가 음성 경로 전체 진단, 태그 릴리스 |

각 페이즈는 `docs/plans/dynamic-tts-discovery-phase-N-*.md` 참조.

## 진행 규칙 (HITL)

- Claude Code는 페이즈 파일을 순서대로 연다. 작업은 체크박스 단위 순차 실행.
- 각 페이즈 끝 **Checkpoint** 통과 → 사용자 확인 → 다음 페이즈.
- Checkpoint 실패 시: 실패 항목 보고 → 사용자와 원인 파악 → 수정 → 재검증.
- Phase 2는 한 번의 펌웨어 재플래시가 필요(USB 연결). 그 시점에 사용자 협조 요청.

## 빌드/검증 컨벤션 (코드베이스 분석)

- Go 모듈: `mcp-server/` (`go 1.22`, **외부 의존성 0**, stdlib only — 유지 필수)
- CLI 디스패치: `os.Args[1:]` switch (`cmd/tars-stackchan-control/main.go:36 runCommand`)
- 테스트: 동 디렉토리 `_test.go`, table-driven, `cd mcp-server && go test ./...`
- 계약 테스트: `scripts/test/*-contract.sh` (CI `.github/workflows/ci.yml`에서 실행)
- 릴리스: `v*` 태그 → `.goreleaser.yaml` → `devlikebear/homebrew-tap` 자동 (현재 v0.1.2)
- 디바이스 기본 호스트네임 관례: 이미 `stackchan.local` 사용 (`main.go:19 defaultBaseURL`)
