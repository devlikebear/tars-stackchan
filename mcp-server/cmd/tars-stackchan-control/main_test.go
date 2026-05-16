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
