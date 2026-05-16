package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	httpbridge "github.com/devlikebear/tars-stackchan/mcp-server/internal/bridge/http"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bridge/mock"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/buildinfo"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/controlui"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

const (
	defaultAddr    = "127.0.0.1:8787"
	defaultBaseURL = "http://stackchan.local"
)

const binaryName = "tars-stackchan-control"

type appConfig struct {
	addr       string
	bridge     stackchan.Bridge
	bridgeMode string
	baseURL    string
	tokenSet   bool
}

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
		default:
			fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
			printUsage(stderr)
			return 2
		}
	}

	config, err := loadConfig()
	if err != nil {
		fmt.Fprintf(stderr, "tars-stackchan control configuration failed: %v\n", err)
		return 1
	}

	handler := controlui.NewServer(controlui.ServerConfig{
		Bridge:     config.bridge,
		BridgeMode: config.bridgeMode,
		BaseURL:    config.baseURL,
		TokenSet:   config.tokenSet,
	})

	fmt.Fprintf(stderr, "TARS Stack-chan Control: http://%s\n", config.addr)
	if err := http.ListenAndServe(config.addr, handler); err != nil {
		fmt.Fprintf(stderr, "tars-stackchan control failed: %v\n", err)
		return 1
	}
	return 0
}

func loadConfig() (appConfig, error) {
	addr := strings.TrimSpace(os.Getenv("TARS_STACKCHAN_CONTROL_ADDR"))
	if addr == "" {
		addr = defaultAddr
	}

	bridgeMode := strings.ToLower(strings.TrimSpace(os.Getenv("TARS_STACKCHAN_BRIDGE")))
	if bridgeMode == "" {
		bridgeMode = "http"
	}

	baseURL := strings.TrimSpace(os.Getenv("TARS_STACKCHAN_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	token := os.Getenv("TARS_STACKCHAN_TOKEN")

	bridge, err := newBridge(bridgeMode, baseURL, token)
	if err != nil {
		return appConfig{}, err
	}

	return appConfig{
		addr:       addr,
		bridge:     bridge,
		bridgeMode: bridgeMode,
		baseURL:    baseURL,
		tokenSet:   strings.TrimSpace(token) != "",
	}, nil
}

func newBridge(mode, baseURL, token string) (stackchan.Bridge, error) {
	switch mode {
	case "mock":
		return mock.New(), nil
	case "http":
		return httpbridge.New(httpbridge.Config{
			BaseURL: baseURL,
			Token:   token,
		})
	default:
		return nil, fmt.Errorf("unsupported TARS_STACKCHAN_BRIDGE %q; use mock or http", mode)
	}
}

func printUsage(stdout io.Writer) {
	fmt.Fprintf(stdout, `Usage:
  %[1]s             run the local web control console
  %[1]s --version   print version

Environment:
  TARS_STACKCHAN_CONTROL_ADDR
  TARS_STACKCHAN_BRIDGE
  TARS_STACKCHAN_BASE_URL
  TARS_STACKCHAN_TOKEN
`, binaryName)
}
