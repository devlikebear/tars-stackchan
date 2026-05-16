package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	httpbridge "github.com/devlikebear/tars-stackchan/mcp-server/internal/bridge/http"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bridge/mock"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/controlui"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

const (
	defaultAddr    = "127.0.0.1:8787"
	defaultBaseURL = "http://stackchan.local"
)

type appConfig struct {
	addr       string
	bridge     stackchan.Bridge
	bridgeMode string
	baseURL    string
	tokenSet   bool
}

func main() {
	config, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tars-stackchan control configuration failed: %v\n", err)
		os.Exit(1)
	}

	handler := controlui.NewServer(controlui.ServerConfig{
		Bridge:     config.bridge,
		BridgeMode: config.bridgeMode,
		BaseURL:    config.baseURL,
		TokenSet:   config.tokenSet,
	})

	fmt.Fprintf(os.Stderr, "TARS Stack-chan Control: http://%s\n", config.addr)
	if err := http.ListenAndServe(config.addr, handler); err != nil {
		fmt.Fprintf(os.Stderr, "tars-stackchan control failed: %v\n", err)
		os.Exit(1)
	}
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
