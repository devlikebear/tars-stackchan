package controlui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

// emotionPreset composes a monitor "emotion" into the existing safe
// primitives: a face expression, an LED look, and an optional motion. Values
// reuse the firmware allowlists (expression / led pattern / motion) so the
// existing safety contract (clamps, allowlists, #RRGGBB) is unchanged.
type emotionPreset struct {
	Expression string
	LEDPattern string
	LEDColor   string
	Brightness float64
	Motion     string // optional; "" = none
}

// emotionPresets is intentionally keyed by the expression allowlist plus a
// couple of common monitor moods, so callers (claude code / tars / scripts)
// drive a single REST call instead of three.
var emotionPresets = map[string]emotionPreset{
	"happy":     {"happy", "solid", "#00FF66", 0.6, "nod"},
	"sad":       {"sad", "solid", "#2244AA", 0.4, ""},
	"angry":     {"angry", "solid", "#FF2200", 0.7, "shake"},
	"surprised": {"surprised", "blink", "#FFAA00", 0.8, "look_around"},
	"sleepy":    {"sleepy", "pulse", "#332211", 0.3, ""},
	"neutral":   {"neutral", "solid", "#00AEEF", 0.5, ""},
	"blink":     {"blink", "blink", "#00AEEF", 0.5, ""},
	"excited":   {"happy", "blink", "#00FF66", 0.9, "look_around"},
	"calm":      {"neutral", "pulse", "#0066AA", 0.35, ""},
}

type emotionRequest struct {
	Emotion string `json:"emotion"`
	Motion  *bool  `json:"motion,omitempty"` // optional: false suppresses the preset motion
}

type emotionResponse struct {
	OK      bool   `json:"ok"`
	Action  string `json:"action"`
	Emotion string `json:"emotion"`
	Applied []any  `json:"applied"`
}

func knownEmotions() []string {
	names := make([]string, 0, len(emotionPresets))
	for k := range emotionPresets {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// handleEmotion maps one emotion to expression + LED (+ optional motion) via
// the bridge. It does not bypass any firmware-side safety: each leg goes
// through the same bridge calls the individual /api/* endpoints use.
func (s *Server) handleEmotion(w http.ResponseWriter, r *http.Request) {
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
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var req emotionRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid emotion request: "+err.Error())
		return
	}

	preset, ok := emotionPresets[strings.ToLower(strings.TrimSpace(req.Emotion))]
	if !ok {
		writeError(w, http.StatusBadRequest,
			"unsupported emotion; allowed: "+strings.Join(knownEmotions(), ", "))
		return
	}

	ctx := r.Context()
	applied := make([]any, 0, 3)

	exprRes, err := s.bridge.SetExpression(ctx, stackchan.ExpressionRequest{Emotion: preset.Expression})
	if err != nil {
		writeError(w, http.StatusBadGateway, "expression: "+err.Error())
		return
	}
	applied = append(applied, exprRes)

	ledRes, err := s.bridge.SetLED(ctx, stackchan.LEDRequest{
		Pattern:    preset.LEDPattern,
		Color:      preset.LEDColor,
		Brightness: preset.Brightness,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "led: "+err.Error())
		return
	}
	applied = append(applied, ledRes)

	wantMotion := preset.Motion != "" && (req.Motion == nil || *req.Motion)
	if wantMotion {
		motionRes, err := s.bridge.RunMotion(ctx, stackchan.MotionRequest{Name: preset.Motion})
		if err != nil {
			writeError(w, http.StatusBadGateway, "motion: "+err.Error())
			return
		}
		applied = append(applied, motionRes)
	}

	writeJSON(w, http.StatusOK, emotionResponse{
		OK:      true,
		Action:  "set_emotion",
		Emotion: strings.ToLower(strings.TrimSpace(req.Emotion)),
		Applied: applied,
	})
}
