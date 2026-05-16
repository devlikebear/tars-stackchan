package httpbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

func TestGetStatusUsesV1Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/status" {
			t.Fatalf("path = %s, want /v1/status", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("status request authorization = %q, want empty", r.Header.Get("Authorization"))
		}
		writeJSON(t, w, stackchan.Status{
			Connected:      true,
			Device:         "stackchan-k151",
			Firmware:       "tars-stackchan-dev",
			BatteryPercent: 87,
			IP:             "192.168.1.42",
			Capabilities:   []string{"expression", "head", "leds", "motion", "speech"},
		})
	}))
	defer server.Close()

	bridge, err := New(Config{BaseURL: server.URL, Token: "secret"})
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}

	status, err := bridge.GetStatus(context.Background())
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if !status.Connected || status.Device != "stackchan-k151" || status.IP != "192.168.1.42" {
		t.Fatalf("status = %#v", status)
	}
}

func TestMutatingRequestsSendBearerTokenAndJSON(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		call       func(context.Context, *Bridge) (stackchan.ActionResult, error)
		wantFields map[string]any
	}{
		{
			name: "expression",
			path: "/v1/expression",
			call: func(ctx context.Context, b *Bridge) (stackchan.ActionResult, error) {
				return b.SetExpression(ctx, stackchan.ExpressionRequest{Emotion: "happy"})
			},
			wantFields: map[string]any{"emotion": "happy"},
		},
		{
			name: "head",
			path: "/v1/head",
			call: func(ctx context.Context, b *Bridge) (stackchan.ActionResult, error) {
				return b.MoveHead(ctx, stackchan.HeadRequest{PanDeg: 45, TiltDeg: 85, Speed: 0.6})
			},
			wantFields: map[string]any{"pan_deg": float64(45), "tilt_deg": float64(85), "speed": 0.6},
		},
		{
			name: "leds",
			path: "/v1/leds",
			call: func(ctx context.Context, b *Bridge) (stackchan.ActionResult, error) {
				return b.SetLED(ctx, stackchan.LEDRequest{Pattern: "solid", Color: "#00AEEF", Brightness: 0.5})
			},
			wantFields: map[string]any{"pattern": "solid", "color": "#00AEEF", "brightness": 0.5},
		},
		{
			name: "motion",
			path: "/v1/motion",
			call: func(ctx context.Context, b *Bridge) (stackchan.ActionResult, error) {
				return b.RunMotion(ctx, stackchan.MotionRequest{Name: "nod"})
			},
			wantFields: map[string]any{"name": "nod"},
		},
		{
			name: "speech",
			path: "/v1/speech",
			call: func(ctx context.Context, b *Bridge) (stackchan.ActionResult, error) {
				volume := 0.15
				return b.Speak(ctx, stackchan.SpeechRequest{Text: "hello stack-chan", Volume: &volume})
			},
			wantFields: map[string]any{"text": "hello stack-chan", "volume": 0.15},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Fatalf("method = %s, want POST", r.Method)
				}
				if r.URL.Path != tt.path {
					t.Fatalf("path = %s, want %s", r.URL.Path, tt.path)
				}
				if r.Header.Get("Authorization") != "Bearer secret" {
					t.Fatalf("authorization = %q, want Bearer secret", r.Header.Get("Authorization"))
				}
				if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
					t.Fatalf("content type = %q, want application/json", got)
				}

				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				for key, want := range tt.wantFields {
					if got := body[key]; got != want {
						t.Fatalf("body[%s] = %#v, want %#v; body=%#v", key, got, want, body)
					}
				}
				writeJSON(t, w, stackchan.ActionResult{OK: true, Action: tt.name})
			}))
			defer server.Close()

			bridge, err := New(Config{BaseURL: server.URL, Token: "secret"})
			if err != nil {
				t.Fatalf("new bridge: %v", err)
			}
			result, err := tt.call(context.Background(), bridge)
			if err != nil {
				t.Fatalf("call: %v", err)
			}
			if !result.OK || result.Action != tt.name {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestFirmwareErrorIsSurfaced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"servo offline"}`, http.StatusTeapot)
	}))
	defer server.Close()

	bridge, err := New(Config{BaseURL: server.URL, Token: "secret"})
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}
	_, err = bridge.SetExpression(context.Background(), stackchan.ExpressionRequest{Emotion: "happy"})
	if err == nil {
		t.Fatal("expected firmware error")
	}
	if !strings.Contains(err.Error(), "418") || !strings.Contains(err.Error(), "servo offline") {
		t.Fatalf("error = %q, want status and firmware body", err)
	}
}

func TestNewRejectsMissingBaseURL(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected missing base URL error")
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
