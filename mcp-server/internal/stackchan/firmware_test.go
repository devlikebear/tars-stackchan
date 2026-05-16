package stackchan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptFirmwareRunnerPassesSafeEnvironmentAndRedactsSecrets(t *testing.T) {
	repoRoot := t.TempDir()
	scriptPath := filepath.Join(repoRoot, "scripts", "dev", "upload-firmware.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatalf("mkdir script dir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte(`#!/bin/sh
echo "mode=$1"
echo "deploy=$TARS_STACKCHAN_DEPLOY_HOST"
echo "skip=$TARS_STACKCHAN_SKIP_SMOKE"
echo "base=$TARS_STACKCHAN_BASE_URL"
echo "port=$TARS_STACKCHAN_UPLOAD_PORT"
echo "token=$TARS_STACKCHAN_TOKEN"
`), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	runner, err := NewScriptFirmwareRunner(ScriptFirmwareRunnerConfig{
		RepoRoot: repoRoot,
		Env:      []string{"TARS_STACKCHAN_TOKEN=secret-token"},
	})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	deployHost := true
	skipSmoke := true
	result, err := runner.UploadFirmware(context.Background(), FirmwareUploadRequest{
		Mode:       "all",
		DeployHost: &deployHost,
		SkipSmoke:  &skipSmoke,
		BaseURL:    "http://stackchan.local",
		UploadPort: "/dev/cu.usbmodem101",
	})
	if err != nil {
		t.Fatalf("upload firmware: %v", err)
	}
	if !result.OK || result.ExitCode != 0 {
		t.Fatalf("result = %#v, want success", result)
	}
	for _, want := range []string{
		"mode=all",
		"deploy=1",
		"skip=1",
		"base=http://stackchan.local",
		"port=/dev/cu.usbmodem101",
		"token=<redacted>",
	} {
		if !strings.Contains(result.Stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, result.Stdout)
		}
	}
	if strings.Contains(result.Stdout, "secret-token") {
		t.Fatalf("stdout leaked token:\n%s", result.Stdout)
	}
}

func TestScriptFirmwareRunnerReturnsNonzeroExitAsToolResult(t *testing.T) {
	repoRoot := t.TempDir()
	scriptPath := filepath.Join(repoRoot, "scripts", "dev", "upload-firmware.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatalf("mkdir script dir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte(`#!/bin/sh
echo "failed" >&2
exit 42
`), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	runner, err := NewScriptFirmwareRunner(ScriptFirmwareRunnerConfig{RepoRoot: repoRoot})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	result, err := runner.UploadFirmware(context.Background(), FirmwareUploadRequest{Mode: "mod"})
	if err != nil {
		t.Fatalf("upload firmware: %v", err)
	}
	if result.OK {
		t.Fatal("OK = true, want failed result")
	}
	if result.ExitCode != 42 {
		t.Fatalf("exit code = %d, want 42", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "failed") {
		t.Fatalf("stderr = %q, want script output", result.Stderr)
	}
}
