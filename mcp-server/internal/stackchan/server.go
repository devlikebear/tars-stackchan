package stackchan

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/buildinfo"
)

const protocolVersion = "2024-11-05"

type Server struct {
	bridge         Bridge
	firmwareRunner FirmwareRunner
}

type ServerOption func(*Server)

func WithFirmwareRunner(runner FirmwareRunner) ServerOption {
	return func(server *Server) {
		server.firmwareRunner = runner
	}
}

func NewServer(bridge Bridge, options ...ServerOption) *Server {
	server := &Server{bridge: bridge}
	for _, option := range options {
		option(server)
	}
	return server
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(out)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		response, ok := s.HandleMessage(ctx, []byte(line))
		if !ok {
			continue
		}
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func (s *Server) HandleMessage(ctx context.Context, payload []byte) (rpcResponse, bool) {
	var req rpcRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return errorResponse(json.RawMessage("null"), -32700, fmt.Sprintf("parse error: %v", err)), true
	}

	hasID := len(bytes.TrimSpace(req.ID)) > 0
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		if !hasID {
			return rpcResponse{}, false
		}
		return errorResponse(req.ID, -32600, "invalid JSON-RPC version"), true
	}

	switch req.Method {
	case "initialize":
		if !hasID {
			return rpcResponse{}, false
		}
		return resultResponse(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "tars-stackchan",
				"version": buildinfo.Version,
			},
		}), true

	case "notifications/initialized":
		return rpcResponse{}, false

	case "ping":
		if !hasID {
			return rpcResponse{}, false
		}
		return resultResponse(req.ID, map[string]any{}), true

	case "tools/list":
		if !hasID {
			return rpcResponse{}, false
		}
		return resultResponse(req.ID, map[string]any{"tools": s.listTools()}), true

	case "tools/call":
		if !hasID {
			return rpcResponse{}, false
		}
		params, err := decodeToolCallParams(req.Params)
		if err != nil {
			return errorResponse(req.ID, -32602, err.Error()), true
		}
		var result ToolCallResult
		if params.Name == ToolUploadFirmware {
			result, err = CallFirmwareTool(ctx, s.firmwareRunner, params.Arguments)
		} else {
			result, err = CallTool(ctx, s.bridge, params.Name, params.Arguments)
		}
		if err != nil {
			return errorResponse(req.ID, -32000, err.Error()), true
		}
		return resultResponse(req.ID, result), true

	default:
		if !hasID {
			return rpcResponse{}, false
		}
		return errorResponse(req.ID, -32601, fmt.Sprintf("method %q not found", req.Method)), true
	}
}

func (s *Server) listTools() []Tool {
	if s.firmwareRunner != nil {
		return ListToolsWithFirmware()
	}
	return ListTools()
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func decodeToolCallParams(raw json.RawMessage) (toolCallParams, error) {
	params, err := decodeStrict[toolCallParams](raw)
	if err != nil {
		return toolCallParams{}, err
	}
	if strings.TrimSpace(params.Name) == "" {
		return toolCallParams{}, fmt.Errorf("tool name is required")
	}
	if len(bytes.TrimSpace(params.Arguments)) == 0 {
		params.Arguments = []byte(`{}`)
	}
	return params, nil
}

func resultResponse(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func errorResponse(id json.RawMessage, code int, message string) rpcResponse {
	return rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	}
}
