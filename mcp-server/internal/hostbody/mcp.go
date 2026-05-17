package hostbody

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bodyprovider"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/buildinfo"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

const (
	ToolHostGetStatus = "host_get_status"
	ToolHostSpeak     = "host_speak"
)

type Host struct {
	Tools      Tools
	Runner     Runner
	WorkDir    string
	TTSBaseURL string
	TTSToken   string
	SayVoice   string
}

func (h Host) Capabilities(cameraEnabled bool) []bodyprovider.Capability {
	return h.Tools.Capabilities(CapabilityOptions{
		CameraEnabled: cameraEnabled,
		TTSConfigured: strings.TrimSpace(h.TTSBaseURL) != "" && strings.TrimSpace(h.TTSToken) != "",
	})
}

func (h Host) Speak(ctx context.Context, req bodyprovider.SpeechRequest) (bodyprovider.ActionResult, error) {
	return Speaker{
		Tools:      h.Tools,
		Runner:     h.Runner,
		WorkDir:    h.WorkDir,
		TTSBaseURL: h.TTSBaseURL,
		TTSToken:   h.TTSToken,
		SayVoice:   h.SayVoice,
	}.SpeakRequest(ctx, req)
}

func ListTools(tools Tools) []stackchan.Tool {
	out := []stackchan.Tool{
		{
			Name:        ToolHostGetStatus,
			Description: "Read Mac host embodiment provider capabilities and tool availability.",
			InputSchema: objectSchema(map[string]any{}, []string{}),
		},
	}
	if tools.HasSpeech(true) {
		out = append(out, stackchan.Tool{
			Name:        ToolHostSpeak,
			Description: "Speak text through the Mac host speaker.",
			InputSchema: objectSchema(map[string]any{
				"text": map[string]any{
					"type":        "string",
					"description": "Text to speak. Maximum 240 characters.",
					"maxLength":   240,
				},
				"volume": map[string]any{"type": "number"},
			}, []string{"text"}),
		})
	}
	return out
}

func CallTool(ctx context.Context, host Host, name string, args json.RawMessage) (stackchan.ToolCallResult, error) {
	switch name {
	case ToolHostGetStatus:
		var empty struct{}
		if err := json.Unmarshal(defaultRaw(args), &empty); err != nil {
			return stackchan.ToolCallResult{}, err
		}
		return jsonTextResult(map[string]any{
			"provider":     DefaultProviderName,
			"capabilities": host.Capabilities(true),
			"tools":        host.Tools,
		})
	case ToolHostSpeak:
		var req bodyprovider.SpeechRequest
		if err := json.Unmarshal(defaultRaw(args), &req); err != nil {
			return stackchan.ToolCallResult{}, err
		}
		if strings.TrimSpace(req.Text) == "" {
			return stackchan.ToolCallResult{}, fmt.Errorf("text is required")
		}
		if len([]rune(req.Text)) > 240 {
			return stackchan.ToolCallResult{}, fmt.Errorf("text must be 240 characters or fewer")
		}
		result, err := host.Speak(ctx, req)
		if err != nil {
			return stackchan.ToolCallResult{}, err
		}
		return jsonTextResult(result)
	default:
		return stackchan.ToolCallResult{}, fmt.Errorf("unknown host tool %q", name)
	}
}

type Server struct {
	host Host
}

func NewServer(host Host) *Server {
	return &Server{host: host}
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
	switch req.Method {
	case "initialize":
		if !hasID {
			return rpcResponse{}, false
		}
		return resultResponse(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "tars-stackchan-host", "version": buildinfo.Version},
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
		return resultResponse(req.ID, map[string]any{"tools": ListTools(s.host.Tools)}), true
	case "tools/call":
		if !hasID {
			return rpcResponse{}, false
		}
		params, err := decodeToolCallParams(req.Params)
		if err != nil {
			return errorResponse(req.ID, -32602, err.Error()), true
		}
		result, err := CallTool(ctx, s.host, params.Name, params.Arguments)
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

func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func jsonTextResult(value any) (stackchan.ToolCallResult, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return stackchan.ToolCallResult{}, err
	}
	return stackchan.ToolCallResult{Content: []stackchan.ContentBlock{{Type: "text", Text: string(data)}}}, nil
}

func defaultRaw(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return []byte(`{}`)
	}
	return raw
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
	var params toolCallParams
	if err := json.Unmarshal(defaultRaw(raw), &params); err != nil {
		return toolCallParams{}, err
	}
	if strings.TrimSpace(params.Name) == "" {
		return toolCallParams{}, fmt.Errorf("tool name is required")
	}
	params.Arguments = defaultRaw(params.Arguments)
	return params, nil
}

func resultResponse(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id json.RawMessage, code int, message string) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
}
