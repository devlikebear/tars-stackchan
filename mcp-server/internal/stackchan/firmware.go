package stackchan

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type ScriptFirmwareRunnerConfig struct {
	ScriptPath string
	RepoRoot   string
	Env        []string
}

type ScriptFirmwareRunner struct {
	scriptPath string
	repoRoot   string
	env        []string
}

func NewScriptFirmwareRunner(config ScriptFirmwareRunnerConfig) (*ScriptFirmwareRunner, error) {
	scriptPath, err := resolveUploadScript(config.ScriptPath, config.RepoRoot)
	if err != nil {
		return nil, err
	}
	repoRoot := strings.TrimSpace(config.RepoRoot)
	if repoRoot == "" {
		repoRoot = filepath.Clean(filepath.Join(filepath.Dir(scriptPath), "..", ".."))
	}
	return &ScriptFirmwareRunner{
		scriptPath: scriptPath,
		repoRoot:   repoRoot,
		env:        append([]string{}, config.Env...),
	}, nil
}

func (r *ScriptFirmwareRunner) UploadFirmware(ctx context.Context, req FirmwareUploadRequest) (FirmwareUploadResult, error) {
	if err := validateFirmwareUpload(&req); err != nil {
		return FirmwareUploadResult{}, err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, r.scriptPath, req.Mode)
	cmd.Dir = r.repoRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = firmwareEnv(os.Environ(), r.env, req)

	err := cmd.Run()
	result := FirmwareUploadResult{
		OK:       err == nil,
		Mode:     req.Mode,
		Script:   r.scriptPath,
		ExitCode: exitCode(err),
		Stdout:   redactKnownSecrets(stdout.String(), cmd.Env),
		Stderr:   redactKnownSecrets(stderr.String(), cmd.Env),
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return result, nil
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, fmt.Errorf("run firmware upload helper: %w", err)
}

func resolveUploadScript(scriptPath, repoRoot string) (string, error) {
	if strings.TrimSpace(scriptPath) != "" {
		return requireUploadScript(scriptPath)
	}

	if strings.TrimSpace(repoRoot) != "" {
		return requireUploadScript(filepath.Join(repoRoot, "scripts", "dev", "upload-firmware.sh"))
	}

	for _, root := range candidateRoots() {
		if root == "" {
			continue
		}
		if path, err := requireUploadScript(filepath.Join(root, "scripts", "dev", "upload-firmware.sh")); err == nil {
			return path, nil
		}
	}

	return "", errors.New("firmware upload helper not found; set TARS_STACKCHAN_UPLOAD_SCRIPT or TARS_STACKCHAN_REPO_ROOT")
}

func requireUploadScript(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("firmware upload helper is a directory: %s", absolute)
	}
	if info.Mode()&0111 == 0 {
		return "", fmt.Errorf("firmware upload helper is not executable: %s", absolute)
	}
	return absolute, nil
}

func candidateRoots() []string {
	roots := []string{}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, ancestors(cwd)...)
	}
	if executable, err := os.Executable(); err == nil {
		roots = append(roots, ancestors(filepath.Dir(executable))...)
	}
	return dedupeStrings(roots)
}

func ancestors(path string) []string {
	path = filepath.Clean(path)
	roots := []string{}
	for {
		roots = append(roots, path)
		parent := filepath.Dir(path)
		if parent == path {
			return roots
		}
		path = parent
	}
}

func firmwareEnv(base []string, extra []string, req FirmwareUploadRequest) []string {
	env := append([]string{}, base...)
	env = append(env, extra...)
	if req.DeployHost != nil {
		env = append(env, "TARS_STACKCHAN_DEPLOY_HOST="+boolEnv(*req.DeployHost))
	}
	if req.SkipSmoke != nil {
		env = append(env, "TARS_STACKCHAN_SKIP_SMOKE="+boolEnv(*req.SkipSmoke))
	}
	if req.BaseURL != "" {
		env = append(env, "TARS_STACKCHAN_BASE_URL="+req.BaseURL)
	}
	if req.UploadPort != "" {
		env = append(env, "TARS_STACKCHAN_UPLOAD_PORT="+req.UploadPort)
	}
	return env
}

func boolEnv(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func redactKnownSecrets(text string, env []string) string {
	for _, key := range []string{"TARS_STACKCHAN_TOKEN", "TARS_STACKCHAN_TTS_TOKEN", "GEMINI_API_KEY"} {
		if value := envValue(env, key); value != "" {
			text = strings.ReplaceAll(text, value, "<redacted>")
		}
	}
	return text
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return strings.TrimPrefix(env[i], prefix)
		}
	}
	return ""
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func parseBoolEnv(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err == nil {
		return parsed
	}
	return strings.TrimSpace(value) == "1"
}
