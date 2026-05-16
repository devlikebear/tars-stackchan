package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	httpbridge "github.com/devlikebear/tars-stackchan/mcp-server/internal/bridge/http"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bridge/mock"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/buildinfo"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/perception"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/tts"
)

const binaryName = "tars-stackchan-mcp"

func main() {
	os.Exit(runCommand(os.Args[1:], os.Stdout, os.Stderr))
}

func runCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "--version", "-version", "version":
			fmt.Fprintln(stdout, buildinfo.String(binaryName))
			return 0
		case "-h", "--help", "help":
			printUsage(stdout)
			return 0
		case "config":
			return runConfigCommand(args[1:], stdout, stderr)
		case "install":
			return runInstallCommand(args[1:], stdout, stderr)
		case "doctor":
			return runDoctorCommand(args[1:], stdout, stderr)
		default:
			fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
			printUsage(stderr)
			return 2
		}
	}

	bridge, err := newBridgeFromEnv()
	if err != nil {
		fmt.Fprintf(stderr, "tars-stackchan MCP server configuration failed: %v\n", err)
		return 1
	}

	firmwareRunner, err := newFirmwareRunnerFromEnv()
	if err != nil {
		fmt.Fprintf(stderr, "tars-stackchan firmware tool configuration failed: %v\n", err)
		return 1
	}
	options := []stackchan.ServerOption{}
	if firmwareRunner != nil {
		options = append(options, stackchan.WithFirmwareRunner(firmwareRunner))
	}

	server := stackchan.NewServer(bridge, options...)
	if err := server.Serve(context.Background(), os.Stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "tars-stackchan MCP server failed: %v\n", err)
		return 1
	}
	return 0
}

func newBridgeFromEnv() (stackchan.Bridge, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TARS_STACKCHAN_BRIDGE"))) {
	case "", "mock":
		return mock.New(), nil
	case "http":
		token := os.Getenv("TARS_STACKCHAN_TOKEN")
		if strings.TrimSpace(token) == "" {
			return nil, fmt.Errorf("TARS_STACKCHAN_TOKEN is required when TARS_STACKCHAN_BRIDGE=http")
		}
		return httpbridge.New(httpbridge.Config{
			BaseURL: os.Getenv("TARS_STACKCHAN_BASE_URL"),
			Token:   token,
		})
	default:
		return nil, fmt.Errorf("unsupported TARS_STACKCHAN_BRIDGE %q; use mock or http", os.Getenv("TARS_STACKCHAN_BRIDGE"))
	}
}

func newFirmwareRunnerFromEnv() (stackchan.FirmwareRunner, error) {
	if !envEnabled("TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS") {
		return nil, nil
	}
	return stackchan.NewScriptFirmwareRunner(stackchan.ScriptFirmwareRunnerConfig{
		ScriptPath: os.Getenv("TARS_STACKCHAN_UPLOAD_SCRIPT"),
		RepoRoot:   os.Getenv("TARS_STACKCHAN_REPO_ROOT"),
	})
}

type installConfig struct {
	target        string
	scope         string
	command       string
	bridge        string
	baseURL       string
	tokenEnv      string
	firmwareTools bool
	dryRun        bool
}

func defaultInstallConfig() installConfig {
	return installConfig{
		target:   "claude-code",
		scope:    "user",
		command:  defaultCommandPath(),
		bridge:   "http",
		baseURL:  envDefault("TARS_STACKCHAN_BASE_URL", "http://stackchan.local"),
		tokenEnv: "TARS_STACKCHAN_TOKEN",
	}
}

func runConfigCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	config, ok := parseInstallFlags("config", args, stderr)
	if !ok {
		return 2
	}
	if err := renderConfig(config, stdout); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	return 0
}

func runInstallCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	config, ok := parseInstallFlags("install", args, stderr)
	if !ok {
		return 2
	}
	if config.dryRun {
		if err := renderConfig(config, stdout); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		return 0
	}
	if config.target != "claude-code" {
		fmt.Fprintf(stderr, "install currently supports --target claude-code; use config --target %s for a copyable snippet\n", config.target)
		return 2
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		fmt.Fprintln(stderr, "claude CLI was not found on PATH; run config --target claude-code for the copyable command")
		return 1
	}
	if config.bridge == "http" && strings.TrimSpace(os.Getenv(config.tokenEnv)) == "" {
		fmt.Fprintf(stderr, "%s is required for HTTP install; export it first or use config --target claude-code\n", config.tokenEnv)
		return 1
	}

	command := exec.Command(claude, claudeInstallArgs(config)...)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		fmt.Fprintf(stderr, "claude mcp add failed: %v\n", err)
		return 1
	}
	return 0
}

func runDoctorCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	skipDevice := flags.Bool("skip-device", false, "skip the HTTP status probe")
	skipTTS := flags.Bool("skip-tts", false, "skip the TTS speech-path probes")
	skipPerception := flags.Bool("skip-perception", false, "skip the perception-loop / owner / TARS checks")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	ok := true
	fmt.Fprintf(stdout, "%s\n", buildinfo.String(binaryName))
	fmt.Fprintf(stdout, "command: %s\n", defaultCommandPath())

	bridgeMode := strings.ToLower(strings.TrimSpace(envDefault("TARS_STACKCHAN_BRIDGE", "mock")))
	fmt.Fprintf(stdout, "bridge: %s\n", bridgeMode)
	if bridgeMode == "mock" {
		fmt.Fprintln(stdout, "hint: mock bridge does not control real hardware; set TARS_STACKCHAN_BRIDGE=http and TARS_STACKCHAN_BASE_URL=<device-ip> for hardware checks")
	}
	if bridgeMode == "http" {
		fmt.Fprintf(stdout, "base_url: %s\n", envDefault("TARS_STACKCHAN_BASE_URL", "http://stackchan.local"))
		if strings.TrimSpace(os.Getenv("TARS_STACKCHAN_TOKEN")) == "" {
			fmt.Fprintln(stdout, "token: missing TARS_STACKCHAN_TOKEN")
			ok = false
		} else {
			fmt.Fprintln(stdout, "token: set")
			fmt.Fprintln(stdout, "hint: GET /v1/status is unauthenticated; a token that does not match the one flashed into the firmware MOD still fails mutating calls with HTTP 401")
		}
	}

	if envEnabled("TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS") {
		runner, err := newFirmwareRunnerFromEnv()
		if err != nil {
			fmt.Fprintf(stdout, "firmware_tools: not ready (%v)\n", err)
			ok = false
		} else if runner != nil {
			fmt.Fprintln(stdout, "firmware_tools: enabled")
		}
	} else {
		fmt.Fprintln(stdout, "firmware_tools: disabled")
	}

	if !*skipDevice {
		bridge, err := newBridgeFromEnv()
		if err != nil {
			fmt.Fprintf(stdout, "device: not ready (%v)\n", err)
			ok = false
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			status, err := bridge.GetStatus(ctx)
			if err != nil {
				fmt.Fprintf(stdout, "device: not ready (%v)\n", err)
				if hint := deviceProbeHint(envDefault("TARS_STACKCHAN_BASE_URL", "http://stackchan.local")); hint != "" {
					fmt.Fprintln(stdout, hint)
				}
				ok = false
			} else {
				fmt.Fprintf(stdout, "device: connected=%t firmware=%s ip=%s\n", status.Connected, status.Firmware, status.IP)
			}
		}
	}

	if !*skipPerception {
		if !runPerceptionDoctor(stdout) {
			ok = false
		}
	}

	if !*skipTTS {
		if !runTTSDoctor(stdout) {
			ok = false
		}
	}

	if !ok {
		return 1
	}
	return 0
}

// runPerceptionDoctor reports the Embodied Bot perception state. It never
// probes /v1/camera/snapshot — that hard-resets a real CoreS3 (Spike S:
// shared I2C bus) — so vision is reported from config/known limitation only.
func runPerceptionDoctor(stdout io.Writer) bool {
	ok := true
	cfg := perception.LoadConfig()

	profile, err := perception.OwnerStore{Dir: cfg.OwnerDir}.Load()
	switch {
	case err != nil:
		fmt.Fprintf(stdout, "owner: error (%v)\n", err)
		ok = false
	case profile.Enrolled():
		fmt.Fprintf(stdout, "owner: enrolled name=%q faces=%d voices=%d\n",
			profile.Name, len(profile.FaceHashes), len(profile.VoiceHashes))
	default:
		fmt.Fprintln(stdout, "owner: not enrolled")
		fmt.Fprintln(stdout, "hint: run `tars-stackchan-control perceive enroll` so the bot can tell owner from stranger (otherwise everyone is 'unknown')")
	}

	if cfg.CameraEnabled {
		fmt.Fprintln(stdout, "perception_camera: enabled")
		fmt.Fprintln(stdout, "hint: real M5Stack CoreS3 camera capture resets the device (Spike S: camera SCCB shares the internal I2C bus). Set TARS_STACKCHAN_PERCEIVE_CAMERA=off for audio-only operation until the bus-handle bridge fix lands")
	} else {
		fmt.Fprintln(stdout, "perception_camera: disabled (audio-only mode)")
	}

	if !cfg.TARSConfigured() {
		fmt.Fprintln(stdout, "tars_webhook: not configured")
		fmt.Fprintln(stdout, "hint: set TARS_STACKCHAN_TARS_BASE_URL and TARS_STACKCHAN_TARS_WEBHOOK_CHANNEL so observations reach the brain; without it the loop uses a log-only sink")
	} else {
		fmt.Fprintf(stdout, "tars_webhook: configured channel=%s\n", cfg.TARSWebhookChannel)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.TARSBaseURL, nil)
		resp, perr := http.DefaultClient.Do(req)
		if perr != nil {
			fmt.Fprintf(stdout, "tars_webhook: unreachable (%v)\n", perr)
			fmt.Fprintf(stdout, "hint: is TARS running and reachable at %s ?\n", cfg.TARSBaseURL)
			ok = false
		} else {
			resp.Body.Close()
			fmt.Fprintf(stdout, "tars_webhook: TARS reachable (HTTP %d at %s)\n", resp.StatusCode, cfg.TARSBaseURL)
		}
	}

	return ok
}

// runTTSDoctor diagnoses the speech path end-to-end (token, Gemini key, relay
// health, mDNS resolution) so a silent-audio regression is caught before the
// user ever asks Stack-chan to speak. It reuses internal/tts probes so the
// checks never drift from `tars-stackchan-control tts status`.
func runTTSDoctor(stdout io.Writer) bool {
	ok := true

	if tts.ResolveToken("") == "" {
		fmt.Fprintln(stdout, "tts_token: missing TARS_STACKCHAN_TTS_TOKEN / TARS_STACKCHAN_TOKEN")
		ok = false
	} else {
		fmt.Fprintln(stdout, "tts_token: set")
	}

	if tts.ResolveAPIKey("") == "" {
		fmt.Fprintln(stdout, "gemini_key: missing GEMINI_API_KEY / TARS_STACKCHAN_GEMINI_API_KEY")
		ok = false
	} else {
		fmt.Fprintln(stdout, "gemini_key: set")
	}

	port := 18080
	if v := strings.TrimSpace(os.Getenv("TARS_STACKCHAN_TTS_PORT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			port = n
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := tts.ProbeHealth(ctx, baseURL); err != nil {
		fmt.Fprintf(stdout, "tts_relay_health: down (%v)\n", err)
		fmt.Fprintln(stdout, "hint: start the relay with 'brew services start tars-stackchan'")
		ok = false
	} else {
		fmt.Fprintf(stdout, "tts_relay_health: ok (%s/health)\n", baseURL)
	}

	if ip, err := tts.ProbeMDNS(ctx, tts.DefaultTTSHostname); err != nil {
		// mDNS failure is not fatal: the IP-override bake is a valid fallback.
		fmt.Fprintf(stdout, "tts_mdns: unresolved (%v)\n", err)
		fmt.Fprintf(stdout, "hint: %s did not resolve; re-flash firmware with TARS_STACKCHAN_TTS_HOST=<mac-ip> as a fixed fallback\n", tts.DefaultTTSHostname)
	} else {
		fmt.Fprintf(stdout, "tts_mdns: ok (%s -> %s)\n", tts.DefaultTTSHostname, ip)
	}

	return ok
}

// deviceProbeHint returns an actionable hint when the device status probe
// fails. mDNS (`*.local`) frequently does not resolve, and DHCP-assigned IPs
// change, so point users at the IP from the firmware boot log / status JSON.
func deviceProbeHint(baseURL string) string {
	host := hostOf(baseURL)
	if host != "" && strings.HasSuffix(strings.ToLower(host), ".local") {
		return "hint: " + host + " relies on mDNS which often fails to resolve; set TARS_STACKCHAN_BASE_URL to the device IP from the firmware boot log or GET /v1/status"
	}
	return "hint: verify the device joined Wi-Fi and TARS_STACKCHAN_BASE_URL points to its current IP (DHCP addresses can change between boots)"
}

// hostOf extracts the hostname from a base URL, tolerating bare host:port and
// unparsable input.
func hostOf(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}
	if parsed, err := url.Parse(baseURL); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	host := baseURL
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	if i := strings.IndexAny(host, "/:"); i >= 0 {
		host = host[:i]
	}
	return host
}

func parseInstallFlags(name string, args []string, stderr io.Writer) (installConfig, bool) {
	config := defaultInstallConfig()
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&config.target, "target", config.target, "target client: claude-code, claude-desktop, or tars")
	flags.StringVar(&config.scope, "scope", config.scope, "Claude Code MCP scope")
	flags.StringVar(&config.command, "command", config.command, "path to tars-stackchan-mcp")
	flags.StringVar(&config.bridge, "bridge", config.bridge, "bridge mode: http or mock")
	flags.StringVar(&config.baseURL, "base-url", config.baseURL, "Stack-chan local HTTP base URL")
	flags.StringVar(&config.tokenEnv, "token-env", config.tokenEnv, "environment variable that stores the Stack-chan token")
	flags.BoolVar(&config.firmwareTools, "firmware-tools", config.firmwareTools, "enable firmware build/upload MCP tools")
	flags.BoolVar(&config.dryRun, "dry-run", config.dryRun, "print the install command without changing client config")
	if err := flags.Parse(args); err != nil {
		return installConfig{}, false
	}
	config.target = strings.ToLower(strings.TrimSpace(config.target))
	config.scope = strings.ToLower(strings.TrimSpace(config.scope))
	config.bridge = strings.ToLower(strings.TrimSpace(config.bridge))
	return config, true
}

func renderConfig(config installConfig, stdout io.Writer) error {
	switch config.target {
	case "claude-code":
		renderClaudeCodeCommand(config, stdout)
	case "claude-desktop":
		return renderClaudeDesktopConfig(config, stdout)
	case "tars":
		renderTARSConfig(config, stdout)
	default:
		return fmt.Errorf("unsupported target %q; use claude-code, claude-desktop, or tars", config.target)
	}
	return nil
}

func renderClaudeCodeCommand(config installConfig, stdout io.Writer) {
	fmt.Fprintf(stdout, "claude mcp add --transport stdio --scope %s \\\n", shellValue(config.scope))
	for _, env := range printableClaudeEnv(config) {
		fmt.Fprintf(stdout, "  --env %s \\\n", env)
	}
	fmt.Fprintf(stdout, "  tars-stackchan -- %s\n", shellValue(config.command))
}

func renderClaudeDesktopConfig(config installConfig, stdout io.Writer) error {
	payload := map[string]any{
		"mcpServers": map[string]any{
			"tars-stackchan": map[string]any{
				"command": config.command,
				"args":    []string{},
				"env":     desktopEnv(config),
			},
		},
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, string(encoded))
	return nil
}

func renderTARSConfig(config installConfig, stdout io.Writer) {
	fmt.Fprintln(stdout, "mcp:")
	fmt.Fprintln(stdout, "  servers:")
	fmt.Fprintln(stdout, "    tars-stackchan:")
	fmt.Fprintf(stdout, "      command: %s\n", config.command)
	fmt.Fprintln(stdout, "      args: []")
	fmt.Fprintln(stdout, "      env:")
	fmt.Fprintf(stdout, "        TARS_STACKCHAN_BRIDGE: %s\n", config.bridge)
	if config.bridge == "http" {
		fmt.Fprintf(stdout, "        TARS_STACKCHAN_BASE_URL: %s\n", config.baseURL)
		fmt.Fprintf(stdout, "        TARS_STACKCHAN_TOKEN: ${%s}\n", config.tokenEnv)
	}
	if config.firmwareTools {
		fmt.Fprintln(stdout, "        TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS: \"1\"")
	}
}

func printableClaudeEnv(config installConfig) []string {
	env := []string{"TARS_STACKCHAN_BRIDGE=" + config.bridge}
	if config.bridge == "http" {
		env = append(env,
			"TARS_STACKCHAN_BASE_URL="+config.baseURL,
			"TARS_STACKCHAN_TOKEN=\"$"+config.tokenEnv+"\"",
		)
	}
	if config.firmwareTools {
		env = append(env, "TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS=1")
	}
	return env
}

func desktopEnv(config installConfig) map[string]string {
	env := map[string]string{
		"TARS_STACKCHAN_BRIDGE": config.bridge,
	}
	if config.bridge == "http" {
		env["TARS_STACKCHAN_BASE_URL"] = config.baseURL
		env["TARS_STACKCHAN_TOKEN"] = "${" + config.tokenEnv + "}"
	}
	if config.firmwareTools {
		env["TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS"] = "1"
	}
	return env
}

func claudeInstallArgs(config installConfig) []string {
	args := []string{
		"mcp", "add",
		"--transport", "stdio",
		"--scope", config.scope,
	}
	for _, env := range installEnv(config) {
		args = append(args, "--env", env)
	}
	args = append(args, "tars-stackchan", "--", config.command)
	return args
}

func installEnv(config installConfig) []string {
	env := []string{"TARS_STACKCHAN_BRIDGE=" + config.bridge}
	if config.bridge == "http" {
		env = append(env,
			"TARS_STACKCHAN_BASE_URL="+config.baseURL,
			"TARS_STACKCHAN_TOKEN="+os.Getenv(config.tokenEnv),
		)
	}
	if config.firmwareTools {
		env = append(env, "TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS=1")
	}
	return env
}

func defaultCommandPath() string {
	if len(os.Args) > 0 && strings.TrimSpace(os.Args[0]) != "" {
		if path, err := exec.LookPath(os.Args[0]); err == nil {
			if absolute, absErr := filepath.Abs(path); absErr == nil {
				return absolute
			}
			return path
		}
		if strings.Contains(os.Args[0], string(os.PathSeparator)) {
			if absolute, err := filepath.Abs(os.Args[0]); err == nil {
				return absolute
			}
			return os.Args[0]
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return binaryName
	}
	return executable
}

func envDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envEnabled(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func shellValue(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"\\$") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func printUsage(stdout io.Writer) {
	fmt.Fprintf(stdout, `Usage:
  %[1]s                         run the stdio MCP server
  %[1]s --version               print version
  %[1]s config [flags]          print MCP client config snippets
  %[1]s install [flags]         install into Claude Code
  %[1]s doctor [flags]          check local configuration

Common flags for config/install:
  --target claude-code|claude-desktop|tars
  --base-url http://stackchan.local
  --token-env TARS_STACKCHAN_TOKEN
  --firmware-tools

Firmware MCP tools are local flashing tools and are hidden unless
TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS=1 is configured.
`, binaryName)
}
