package stackchan

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type recordingBridge struct {
	status     Status
	expression ExpressionRequest
	head       HeadRequest
	led        LEDRequest
	motion     MotionRequest
}

func (b *recordingBridge) GetStatus(context.Context) (Status, error) {
	if b.status.Device == "" {
		b.status = Status{
			Connected: true,
			Device:    "stackchan-k151",
			Firmware:  "mock",
			IP:        "127.0.0.1",
			Capabilities: []string{
				"expression",
				"head",
				"leds",
				"motion",
			},
		}
	}
	return b.status, nil
}

func (b *recordingBridge) SetExpression(_ context.Context, req ExpressionRequest) (ActionResult, error) {
	b.expression = req
	return ActionResult{OK: true, Action: "set_expression"}, nil
}

func (b *recordingBridge) MoveHead(_ context.Context, req HeadRequest) (ActionResult, error) {
	b.head = req
	return ActionResult{OK: true, Action: "move_head"}, nil
}

func (b *recordingBridge) SetLED(_ context.Context, req LEDRequest) (ActionResult, error) {
	b.led = req
	return ActionResult{OK: true, Action: "set_led"}, nil
}

func (b *recordingBridge) RunMotion(_ context.Context, req MotionRequest) (ActionResult, error) {
	b.motion = req
	return ActionResult{OK: true, Action: "run_motion"}, nil
}

func TestListToolsExposesStackchanTools(t *testing.T) {
	want := []string{
		"stackchan_get_status",
		"stackchan_set_expression",
		"stackchan_move_head",
		"stackchan_set_led",
		"stackchan_run_motion",
	}

	gotTools := ListTools()
	if len(gotTools) != len(want) {
		t.Fatalf("tool count = %d, want %d", len(gotTools), len(want))
	}

	seen := map[string]bool{}
	for _, tool := range gotTools {
		seen[tool.Name] = true
		if tool.InputSchema["type"] != "object" {
			t.Fatalf("%s input schema type = %v, want object", tool.Name, tool.InputSchema["type"])
		}
	}

	for _, name := range want {
		if !seen[name] {
			t.Fatalf("missing tool %q in %#v", name, gotTools)
		}
	}
}

func TestCallToolRejectsUnknownJSONFields(t *testing.T) {
	bridge := &recordingBridge{}
	_, err := CallTool(context.Background(), bridge, "stackchan_set_expression", json.RawMessage(`{"emotion":"happy","extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %q, want unknown field", err)
	}
}

func TestMoveHeadClampsTiltToSafeRange(t *testing.T) {
	bridge := &recordingBridge{}

	_, err := CallTool(context.Background(), bridge, "stackchan_move_head", json.RawMessage(`{"pan_deg":45,"tilt_deg":120,"speed":0.6}`))
	if err != nil {
		t.Fatalf("move head high tilt: %v", err)
	}
	if bridge.head.TiltDeg != 85 {
		t.Fatalf("high tilt = %d, want 85", bridge.head.TiltDeg)
	}

	_, err = CallTool(context.Background(), bridge, "stackchan_move_head", json.RawMessage(`{"pan_deg":-20,"tilt_deg":-40,"speed":0.6}`))
	if err != nil {
		t.Fatalf("move head low tilt: %v", err)
	}
	if bridge.head.TiltDeg != 5 {
		t.Fatalf("low tilt = %d, want 5", bridge.head.TiltDeg)
	}
}

func TestExpressionAllowlist(t *testing.T) {
	bridge := &recordingBridge{}

	_, err := CallTool(context.Background(), bridge, "stackchan_set_expression", json.RawMessage(`{"emotion":"happy"}`))
	if err != nil {
		t.Fatalf("valid expression: %v", err)
	}
	if bridge.expression.Emotion != "happy" {
		t.Fatalf("emotion = %q, want happy", bridge.expression.Emotion)
	}

	_, err = CallTool(context.Background(), bridge, "stackchan_set_expression", json.RawMessage(`{"emotion":"furious"}`))
	if err == nil {
		t.Fatal("expected invalid expression error")
	}
	if !strings.Contains(err.Error(), "unsupported expression") {
		t.Fatalf("error = %q, want unsupported expression", err)
	}
}

func TestLEDColorValidation(t *testing.T) {
	bridge := &recordingBridge{}

	_, err := CallTool(context.Background(), bridge, "stackchan_set_led", json.RawMessage(`{"pattern":"solid","color":"#00AEEF","brightness":0.5}`))
	if err != nil {
		t.Fatalf("valid led: %v", err)
	}
	if bridge.led.Color != "#00AEEF" {
		t.Fatalf("color = %q, want #00AEEF", bridge.led.Color)
	}

	_, err = CallTool(context.Background(), bridge, "stackchan_set_led", json.RawMessage(`{"pattern":"solid","color":"blue","brightness":0.5}`))
	if err == nil {
		t.Fatal("expected invalid color error")
	}
	if !strings.Contains(err.Error(), "invalid LED color") {
		t.Fatalf("error = %q, want invalid LED color", err)
	}
}
