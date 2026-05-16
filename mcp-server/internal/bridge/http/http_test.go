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

func TestCameraSnapshotSendsAuthAndMaxWidthQuery(t *testing.T) {
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/camera/snapshot" {
			t.Fatalf("path = %s, want /v1/camera/snapshot", r.URL.Path)
		}
		if got := r.URL.Query().Get("max_width"); got != "320" {
			t.Fatalf("max_width = %q, want 320", got)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q, want Bearer secret", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(jpeg)
	}))
	defer server.Close()

	bridge, err := New(Config{BaseURL: server.URL, Token: "secret"})
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}
	snap, err := bridge.CameraSnapshot(context.Background(), stackchan.SnapshotOptions{MaxWidth: 320})
	if err != nil {
		t.Fatalf("camera snapshot: %v", err)
	}
	if snap.ContentType != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", snap.ContentType)
	}
	if string(snap.Data) != string(jpeg) {
		t.Fatalf("data = %x, want %x", snap.Data, jpeg)
	}
}

func TestCameraSnapshotOmitsQueryWhenDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Fatalf("raw query = %q, want empty", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte{0xFF, 0xD8, 0xFF})
	}))
	defer server.Close()

	bridge, _ := New(Config{BaseURL: server.URL, Token: "secret"})
	if _, err := bridge.CameraSnapshot(context.Background(), stackchan.SnapshotOptions{}); err != nil {
		t.Fatalf("camera snapshot: %v", err)
	}
}

func TestAudioClipSendsMsQueryAndPropagatesDuration(t *testing.T) {
	wav := []byte("RIFF\x24\x00\x00\x00WAVE")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/clip" {
			t.Fatalf("path = %s, want /v1/audio/clip", r.URL.Path)
		}
		if got := r.URL.Query().Get("ms"); got != "1500" {
			t.Fatalf("ms = %q, want 1500", got)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q, want Bearer secret", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(wav)
	}))
	defer server.Close()

	bridge, _ := New(Config{BaseURL: server.URL, Token: "secret"})
	clip, err := bridge.AudioClip(context.Background(), stackchan.AudioOptions{DurationMs: 1500})
	if err != nil {
		t.Fatalf("audio clip: %v", err)
	}
	if clip.ContentType != "audio/wav" || string(clip.Data) != string(wav) || clip.DurationMs != 1500 {
		t.Fatalf("clip = %#v", clip)
	}
}

func TestSensorsParsesV1SensorsWithAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/sensors" {
			t.Fatalf("%s %s, want GET /v1/sensors", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q, want Bearer secret", r.Header.Get("Authorization"))
		}
		writeJSON(t, w, stackchan.SensorState{Motion: true, SoundLevel: 0.42, TS: 1747396800000})
	}))
	defer server.Close()

	bridge, _ := New(Config{BaseURL: server.URL, Token: "secret"})
	state, err := bridge.Sensors(context.Background())
	if err != nil {
		t.Fatalf("sensors: %v", err)
	}
	if !state.Motion || state.SoundLevel != 0.42 || state.TS != 1747396800000 {
		t.Fatalf("state = %#v", state)
	}
}

func TestPerceptionRequiresToken(t *testing.T) {
	bridge, err := New(Config{BaseURL: "http://stackchan.local"})
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}
	if _, err := bridge.CameraSnapshot(context.Background(), stackchan.SnapshotOptions{}); err == nil ||
		!strings.Contains(err.Error(), "TARS_STACKCHAN_TOKEN") {
		t.Fatalf("error = %v, want missing token error", err)
	}
}

func TestCameraSnapshotSurfacesFirmwareError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"camera unavailable"}`, http.StatusServiceUnavailable)
	}))
	defer server.Close()

	bridge, _ := New(Config{BaseURL: server.URL, Token: "secret"})
	_, err := bridge.CameraSnapshot(context.Background(), stackchan.SnapshotOptions{})
	if err == nil || !strings.Contains(err.Error(), "503") || !strings.Contains(err.Error(), "camera unavailable") {
		t.Fatalf("error = %v, want 503 + firmware body", err)
	}
}

func TestCameraSnapshotRejectsEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
	}))
	defer server.Close()

	bridge, _ := New(Config{BaseURL: server.URL, Token: "secret"})
	_, err := bridge.CameraSnapshot(context.Background(), stackchan.SnapshotOptions{})
	if err == nil || !strings.Contains(err.Error(), "empty body") {
		t.Fatalf("error = %v, want empty body error", err)
	}
}

func TestCameraSnapshotRejectsOverLimitBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(make([]byte, maxMediaBytes+1))
	}))
	defer server.Close()

	bridge, _ := New(Config{BaseURL: server.URL, Token: "secret"})
	_, err := bridge.CameraSnapshot(context.Background(), stackchan.SnapshotOptions{})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("error = %v, want over-limit error", err)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
