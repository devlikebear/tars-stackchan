package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestLoadConfigDefaultsToHTTPControl(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_CONTROL_ADDR", "")
	t.Setenv("TARS_STACKCHAN_BRIDGE", "")
	t.Setenv("TARS_STACKCHAN_BASE_URL", "")
	t.Setenv("TARS_STACKCHAN_TOKEN", "")

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.addr != defaultAddr {
		t.Fatalf("addr = %q, want %q", config.addr, defaultAddr)
	}
	if config.bridgeMode != "http" {
		t.Fatalf("bridge mode = %q, want http", config.bridgeMode)
	}
	if config.baseURL != defaultBaseURL {
		t.Fatalf("base URL = %q, want %q", config.baseURL, defaultBaseURL)
	}
	if got := fmt.Sprintf("%T", config.bridge); got != "*httpbridge.Bridge" {
		t.Fatalf("bridge type = %s, want *httpbridge.Bridge", got)
	}
}

func TestLoadConfigSupportsMockMode(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_CONTROL_ADDR", "127.0.0.1:9999")
	t.Setenv("TARS_STACKCHAN_BRIDGE", "mock")
	t.Setenv("TARS_STACKCHAN_BASE_URL", "")
	t.Setenv("TARS_STACKCHAN_TOKEN", "secret")

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.addr != "127.0.0.1:9999" {
		t.Fatalf("addr = %q, want override", config.addr)
	}
	if !config.tokenSet {
		t.Fatal("tokenSet = false, want true")
	}
	if got := fmt.Sprintf("%T", config.bridge); got != "*mock.Bridge" {
		t.Fatalf("bridge type = %s, want *mock.Bridge", got)
	}
}

func TestLoadConfigRejectsUnknownBridge(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_BRIDGE", "serial")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected unknown bridge error")
	}
}

func TestRunCommandTTSUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runCommand([]string{"tts"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "tts serve") {
		t.Fatalf("stderr = %q, want tts serve usage", stderr.String())
	}
}

func TestRunCommandTTSUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runCommand([]string{"tts", "bogus"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown tts subcommand") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunCommandTTSServeMissingToken(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_TTS_TOKEN", "")
	t.Setenv("TARS_STACKCHAN_TOKEN", "")
	var stdout, stderr bytes.Buffer
	// --port 0 binds an ephemeral port; serve must fail fast on missing token
	// before it ever listens.
	if code := runCommand([]string{"tts", "serve", "--port", "0"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, want 1 (missing token)", code)
	}
	if !strings.Contains(stderr.String(), "TOKEN") {
		t.Fatalf("stderr = %q, want token requirement", stderr.String())
	}
}

func TestRunCommandTTSInstallGuide(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runCommand([]string{"tts", "install"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	out := stderr.String()
	if !strings.Contains(out, "brew services start tars-stackchan") {
		t.Fatalf("install guide missing brew services line: %q", out)
	}
	if !strings.Contains(out, "tars-stackchan-tts.local") {
		t.Fatalf("install guide missing mDNS hostname: %q", out)
	}
}

func TestRunCommandTTSStatusRelayDown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Port 1 is unbindable/unreachable, so the health probe fails fast.
	code := runCommand([]string{"tts", "status", "--port", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (relay down)", code)
	}
	if !strings.Contains(stderr.String(), "relay: DOWN") {
		t.Fatalf("stderr = %q, want relay DOWN", stderr.String())
	}
}

func TestRunCommandVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCommand([]string{"--version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "tars-stackchan-control") {
		t.Fatalf("version output = %q, want binary name", stdout.String())
	}
}
