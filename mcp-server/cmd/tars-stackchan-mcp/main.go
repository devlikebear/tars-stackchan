package main

import (
	"context"
	"fmt"
	"os"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bridge/mock"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

func main() {
	server := stackchan.NewServer(mock.New())
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "tars-stackchan MCP server failed: %v\n", err)
		os.Exit(1)
	}
}
