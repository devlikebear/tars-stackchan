package controlui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

const maxRequestBodyBytes = 1 << 20

type ServerConfig struct {
	Bridge     stackchan.Bridge
	BridgeMode string
	BaseURL    string
	TokenSet   bool
}

type Server struct {
	bridge stackchan.Bridge
	config configResponse
	mux    *http.ServeMux
}

type configResponse struct {
	BridgeMode string `json:"bridge_mode"`
	BaseURL    string `json:"base_url,omitempty"`
	TokenSet   bool   `json:"token_set"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewServer(config ServerConfig) *Server {
	server := &Server{
		bridge: config.Bridge,
		config: configResponse{
			BridgeMode: config.BridgeMode,
			BaseURL:    config.BaseURL,
			TokenSet:   config.TokenSet,
		},
		mux: http.NewServeMux(),
	}
	if strings.TrimSpace(server.config.BridgeMode) == "" {
		server.config.BridgeMode = "unknown"
	}

	server.mux.HandleFunc("/", server.handleIndex)
	server.mux.HandleFunc("/api/config", server.handleConfig)
	server.mux.HandleFunc("/api/status", server.handleStatus)
	server.mux.HandleFunc("/api/expression", server.handleTool(stackchan.ToolSetExpression))
	server.mux.HandleFunc("/api/head", server.handleTool(stackchan.ToolMoveHead))
	server.mux.HandleFunc("/api/leds", server.handleTool(stackchan.ToolSetLED))
	server.mux.HandleFunc("/api/motion", server.handleTool(stackchan.ToolRunMotion))
	server.mux.HandleFunc("/api/speech", server.handleTool(stackchan.ToolSpeak))
	server.mux.HandleFunc("/api/emotion", server.handleEmotion)
	server.mux.HandleFunc("/api/perceive/status", server.handlePerceiveStatus)

	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, indexHTML)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.config)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.bridge == nil {
		writeError(w, http.StatusServiceUnavailable, "stackchan bridge is nil")
		return
	}

	status, err := s.bridge.GetStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleTool(toolName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if s.bridge == nil {
			writeError(w, http.StatusServiceUnavailable, "stackchan bridge is nil")
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			body = []byte(`{}`)
		}

		result, err := stackchan.CallTool(r.Context(), s.bridge, toolName, body)
		if err != nil {
			writeError(w, statusForToolError(err), err.Error())
			return
		}
		value, err := resultJSON(result)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, value)
	}
}

func resultJSON(result stackchan.ToolCallResult) (any, error) {
	if len(result.Content) == 0 {
		return nil, errors.New("empty tool result")
	}
	var value any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &value); err != nil {
		return nil, fmt.Errorf("decode tool result: %w", err)
	}
	return value, nil
}

func statusForToolError(err error) int {
	message := err.Error()
	clientErrorFragments := []string{
		"unknown field",
		"required",
		"unsupported",
		"invalid",
		"must be",
		"between",
	}
	for _, fragment := range clientErrorFragments {
		if strings.Contains(message, fragment) {
			return http.StatusBadRequest
		}
	}
	return http.StatusBadGateway
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
