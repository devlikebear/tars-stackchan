package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestNewBridgeFromEnvDefaultsToMock(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_BRIDGE", "")
	t.Setenv("TARS_STACKCHAN_BASE_URL", "")
	t.Setenv("TARS_STACKCHAN_TOKEN", "")

	bridge, err := newBridgeFromEnv()
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}
	if got := fmt.Sprintf("%T", bridge); got != "*mock.Bridge" {
		t.Fatalf("bridge type = %s, want *mock.Bridge", got)
	}
}

func TestNewBridgeFromEnvCreatesHTTPBridge(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_BRIDGE", "http")
	t.Setenv("TARS_STACKCHAN_BASE_URL", "http://stackchan.local")
	t.Setenv("TARS_STACKCHAN_TOKEN", "secret")

	bridge, err := newBridgeFromEnv()
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}
	if got := fmt.Sprintf("%T", bridge); got != "*httpbridge.Bridge" {
		t.Fatalf("bridge type = %s, want *httpbridge.Bridge", got)
	}
}

func TestNewBridgeFromEnvRejectsUnknownBridge(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_BRIDGE", "serial")
	t.Setenv("TARS_STACKCHAN_BASE_URL", "")
	t.Setenv("TARS_STACKCHAN_TOKEN", "")

	_, err := newBridgeFromEnv()
	if err == nil {
		t.Fatal("expected unknown bridge error")
	}
}

func TestRunCommandPrintsClaudeCodeConfigWithFirmwareTools(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCommand([]string{
		"config",
		"--target", "claude-code",
		"--command", "/opt/homebrew/bin/tars-stackchan-mcp",
		"--base-url", "http://stackchan.local",
		"--firmware-tools",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %s", exitCode, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"claude mcp add --transport stdio --scope user",
		"--env TARS_STACKCHAN_BRIDGE=http",
		"--env TARS_STACKCHAN_BASE_URL=http://stackchan.local",
		"--env TARS_STACKCHAN_TOKEN=\"$TARS_STACKCHAN_TOKEN\"",
		"--env TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS=1",
		"tars-stackchan -- /opt/homebrew/bin/tars-stackchan-mcp",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("config output missing %q:\n%s", want, output)
		}
	}
}

func TestRunCommandPrintsClaudeDesktopConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCommand([]string{
		"config",
		"--target", "claude-desktop",
		"--command", "/tmp/tars-stackchan-mcp",
		"--firmware-tools",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %s", exitCode, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		`"command": "/tmp/tars-stackchan-mcp"`,
		`"TARS_STACKCHAN_BRIDGE": "http"`,
		`"TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS": "1"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("config output missing %q:\n%s", want, output)
		}
	}
}

func TestRunCommandVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCommand([]string{"--version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "tars-stackchan-mcp") {
		t.Fatalf("version output = %q, want binary name", stdout.String())
	}
}
