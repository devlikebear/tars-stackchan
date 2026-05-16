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

func TestHostOf(t *testing.T) {
	cases := map[string]string{
		"http://stackchan.local":      "stackchan.local",
		"http://stackchan.local/v1":   "stackchan.local",
		"https://192.168.219.113:443": "192.168.219.113",
		"192.168.219.113":             "192.168.219.113",
		"stackchan.local:80/path":     "stackchan.local",
		"":                            "",
	}
	for in, want := range cases {
		if got := hostOf(in); got != want {
			t.Fatalf("hostOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeviceProbeHint(t *testing.T) {
	mdns := deviceProbeHint("http://stackchan.local")
	if !strings.Contains(mdns, "mDNS") || !strings.Contains(mdns, "stackchan.local") {
		t.Fatalf("mDNS hint = %q, want mDNS guidance mentioning the host", mdns)
	}

	ip := deviceProbeHint("http://192.168.219.113")
	if !strings.Contains(ip, "DHCP") {
		t.Fatalf("IP hint = %q, want DHCP guidance", ip)
	}
	if strings.Contains(ip, "mDNS") {
		t.Fatalf("IP hint should not mention mDNS: %q", ip)
	}
}

func TestDoctorMockBridgeWarnsNotRealHardware(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_BRIDGE", "mock")
	t.Setenv("TARS_STACKCHAN_BASE_URL", "")
	t.Setenv("TARS_STACKCHAN_TOKEN", "")

	var stdout, stderr bytes.Buffer
	if code := runCommand([]string{"doctor", "--skip-device"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "bridge: mock") {
		t.Fatalf("doctor output missing bridge line:\n%s", out)
	}
	if !strings.Contains(out, "mock bridge does not control real hardware") {
		t.Fatalf("doctor output missing mock warning:\n%s", out)
	}
}

func TestDoctorHTTPTokenNote(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_BRIDGE", "http")
	t.Setenv("TARS_STACKCHAN_BASE_URL", "http://stackchan.local")
	t.Setenv("TARS_STACKCHAN_TOKEN", "secret")

	var stdout, stderr bytes.Buffer
	if code := runCommand([]string{"doctor", "--skip-device"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "token: set") {
		t.Fatalf("doctor output missing token line:\n%s", out)
	}
	if !strings.Contains(out, "fails mutating calls with HTTP 401") {
		t.Fatalf("doctor output missing firmware-token note:\n%s", out)
	}
}
