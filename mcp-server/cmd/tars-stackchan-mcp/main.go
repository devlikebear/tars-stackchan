package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	httpbridge "github.com/devlikebear/tars-stackchan/mcp-server/internal/bridge/http"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bridge/mock"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

func main() {
	bridge, err := newBridgeFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tars-stackchan MCP server configuration failed: %v\n", err)
		os.Exit(1)
	}

	server := stackchan.NewServer(bridge)
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "tars-stackchan MCP server failed: %v\n", err)
		os.Exit(1)
	}
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
