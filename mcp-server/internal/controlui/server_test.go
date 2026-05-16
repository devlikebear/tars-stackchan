package controlui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

type recordingBridge struct {
	status     stackchan.Status
	expression stackchan.ExpressionRequest
	head       stackchan.HeadRequest
	led        stackchan.LEDRequest
	motion     stackchan.MotionRequest
	speech     stackchan.SpeechRequest
}

func (b *recordingBridge) GetStatus(context.Context) (stackchan.Status, error) {
	if b.status.Device == "" {
		b.status = stackchan.Status{
			Connected: true,
			Device:    "stackchan-k151",
			Firmware:  "test",
			IP:        "127.0.0.1",
			Capabilities: []string{
				"expression",
				"head",
				"leds",
				"motion",
				"speech",
			},
		}
	}
	return b.status, nil
}

func (b *recordingBridge) SetExpression(_ context.Context, req stackchan.ExpressionRequest) (stackchan.ActionResult, error) {
	b.expression = req
	return stackchan.ActionResult{OK: true, Action: "set_expression", State: req}, nil
}

func (b *recordingBridge) MoveHead(_ context.Context, req stackchan.HeadRequest) (stackchan.ActionResult, error) {
	b.head = req
	return stackchan.ActionResult{OK: true, Action: "move_head", State: req}, nil
}

func (b *recordingBridge) SetLED(_ context.Context, req stackchan.LEDRequest) (stackchan.ActionResult, error) {
	b.led = req
	return stackchan.ActionResult{OK: true, Action: "set_led", State: req}, nil
}

func (b *recordingBridge) RunMotion(_ context.Context, req stackchan.MotionRequest) (stackchan.ActionResult, error) {
	b.motion = req
	return stackchan.ActionResult{OK: true, Action: "run_motion", State: req}, nil
}

func (b *recordingBridge) Speak(_ context.Context, req stackchan.SpeechRequest) (stackchan.ActionResult, error) {
	b.speech = req
	return stackchan.ActionResult{OK: true, Action: "speak", State: req}, nil
}

func TestHandlerServesControlPanel(t *testing.T) {
	handler := NewServer(ServerConfig{
		Bridge:     &recordingBridge{},
		BridgeMode: "mock",
		BaseURL:    "http://stackchan.local",
		TokenSet:   true,
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), "TARS Stack-chan Control") {
		t.Fatalf("body did not contain control panel title")
	}
	if !strings.Contains(response.Body.String(), "stackchan_speak") {
		t.Fatalf("body did not expose speech control")
	}
	if !strings.Contains(response.Body.String(), `id="speechVolume"`) {
		t.Fatalf("body did not expose speech volume control")
	}
	if !strings.Contains(response.Body.String(), `volume: Number($('speechVolume').value)`) {
		t.Fatalf("body did not send speech volume payload")
	}
}

func TestStatusAPIUsesBridge(t *testing.T) {
	handler := NewServer(ServerConfig{Bridge: &recordingBridge{}})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}

	var status stackchan.Status
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !status.Connected || status.Device != "stackchan-k151" {
		t.Fatalf("status = %#v", status)
	}
}

func TestActionAPIReusesStackchanValidation(t *testing.T) {
	bridge := &recordingBridge{}
	handler := NewServer(ServerConfig{Bridge: bridge})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/head", bytes.NewBufferString(`{"pan_deg":45,"tilt_deg":120,"speed":0.6}`))
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if bridge.head.TiltDeg != stackchan.SafeTiltMax {
		t.Fatalf("tilt = %d, want %d", bridge.head.TiltDeg, stackchan.SafeTiltMax)
	}

	var result stackchan.ActionResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !result.OK || result.Action != "move_head" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSpeechAPIRecordsText(t *testing.T) {
	bridge := &recordingBridge{}
	handler := NewServer(ServerConfig{Bridge: bridge})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/speech", bytes.NewBufferString(`{"text":"hello stack-chan","volume":0.15}`))
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if bridge.speech.Text != "hello stack-chan" {
		t.Fatalf("speech text = %q, want hello stack-chan", bridge.speech.Text)
	}
	if bridge.speech.Volume == nil || *bridge.speech.Volume != 0.15 {
		t.Fatalf("speech volume = %#v, want 0.15", bridge.speech.Volume)
	}
}

func TestActionAPIRejectsInvalidPayload(t *testing.T) {
	handler := NewServer(ServerConfig{Bridge: &recordingBridge{}})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/speech", bytes.NewBufferString(`{"text":"   "}`))
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if !strings.Contains(response.Body.String(), "speech text is required") {
		t.Fatalf("body = %q, want speech text error", response.Body.String())
	}
}
