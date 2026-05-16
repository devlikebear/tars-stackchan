package stackchan

import (
	"bytes"
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
	speech     SpeechRequest
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
				"speech",
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

func (b *recordingBridge) Speak(_ context.Context, req SpeechRequest) (ActionResult, error) {
	b.speech = req
	return ActionResult{OK: true, Action: "speak"}, nil
}

type recordingFirmwareRunner struct {
	request FirmwareUploadRequest
}

func (r *recordingFirmwareRunner) UploadFirmware(_ context.Context, req FirmwareUploadRequest) (FirmwareUploadResult, error) {
	r.request = req
	return FirmwareUploadResult{OK: true, Mode: req.Mode, ExitCode: 0, Stdout: "uploaded"}, nil
}

func TestListToolsExposesStackchanTools(t *testing.T) {
	want := []string{
		"stackchan_get_status",
		"stackchan_set_expression",
		"stackchan_move_head",
		"stackchan_set_led",
		"stackchan_run_motion",
		"stackchan_speak",
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

func TestServerListsFirmwareToolOnlyWhenRunnerConfigured(t *testing.T) {
	baseServer := NewServer(&recordingBridge{})
	baseResponse, ok := baseServer.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if !ok {
		t.Fatal("expected tools/list response")
	}
	if responseHasTool(t, baseResponse, ToolUploadFirmware) {
		t.Fatalf("%s should be hidden until firmware tools are explicitly enabled", ToolUploadFirmware)
	}

	server := NewServer(&recordingBridge{}, WithFirmwareRunner(&recordingFirmwareRunner{}))
	response, ok := server.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if !ok {
		t.Fatal("expected tools/list response")
	}
	if !responseHasTool(t, response, ToolUploadFirmware) {
		t.Fatalf("missing tool %q when firmware runner is configured", ToolUploadFirmware)
	}
}

func TestFirmwareUploadToolPassesValidatedRequest(t *testing.T) {
	runner := &recordingFirmwareRunner{}
	server := NewServer(&recordingBridge{}, WithFirmwareRunner(runner))

	response, ok := server.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"stackchan_upload_firmware","arguments":{"mode":"mod","base_url":"http://stackchan.local","upload_port":"/dev/cu.usbmodem101"}}}`))
	if !ok {
		t.Fatal("expected tools/call response")
	}
	if response.Error != nil {
		t.Fatalf("firmware upload tool error: %s", response.Error.Message)
	}
	if runner.request.Mode != "mod" {
		t.Fatalf("mode = %q, want mod", runner.request.Mode)
	}
	if runner.request.BaseURL != "http://stackchan.local" {
		t.Fatalf("base URL = %q, want hardware URL", runner.request.BaseURL)
	}
	if runner.request.UploadPort != "/dev/cu.usbmodem101" {
		t.Fatalf("upload port = %q, want explicit USB port", runner.request.UploadPort)
	}
}

func TestFirmwareUploadToolRejectsUnknownMode(t *testing.T) {
	server := NewServer(&recordingBridge{}, WithFirmwareRunner(&recordingFirmwareRunner{}))

	response, ok := server.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"stackchan_upload_firmware","arguments":{"mode":"factory-reset"}}}`))
	if !ok {
		t.Fatal("expected tools/call response")
	}
	if response.Error == nil {
		t.Fatal("expected firmware mode validation error")
	}
	if !strings.Contains(response.Error.Message, "unsupported firmware upload mode") {
		t.Fatalf("error = %q, want unsupported mode", response.Error.Message)
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

func TestCallToolIgnoresReservedMetaField(t *testing.T) {
	bridge := &recordingBridge{}
	_, err := CallTool(context.Background(), bridge, "stackchan_set_expression",
		json.RawMessage(`{"emotion":"happy","_meta":{"progressToken":"abc"}}`))
	if err != nil {
		t.Fatalf("unexpected error with reserved _meta field: %v", err)
	}
}

func TestDecodeToolCallParamsIgnoresReservedMetaField(t *testing.T) {
	params, err := decodeToolCallParams(json.RawMessage(
		`{"name":"stackchan_get_status","arguments":{},"_meta":{"progressToken":1}}`))
	if err != nil {
		t.Fatalf("unexpected error decoding params with _meta: %v", err)
	}
	if params.Name != "stackchan_get_status" {
		t.Fatalf("name = %q, want stackchan_get_status", params.Name)
	}
}

func responseHasTool(t *testing.T, response rpcResponse, toolName string) bool {
	t.Helper()

	payload, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("decode tools/list result: %v", err)
	}
	for _, tool := range result.Tools {
		if tool.Name == toolName {
			return true
		}
	}
	return false
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

func TestSpeakRequiresText(t *testing.T) {
	bridge := &recordingBridge{}

	_, err := CallTool(context.Background(), bridge, "stackchan_speak", json.RawMessage(`{"text":"hello stack-chan"}`))
	if err != nil {
		t.Fatalf("valid speech: %v", err)
	}
	if bridge.speech.Text != "hello stack-chan" {
		t.Fatalf("text = %q, want hello stack-chan", bridge.speech.Text)
	}

	_, err = CallTool(context.Background(), bridge, "stackchan_speak", json.RawMessage(`{"text":"quiet hello","volume":0.15}`))
	if err != nil {
		t.Fatalf("valid speech volume: %v", err)
	}
	if bridge.speech.Volume == nil || *bridge.speech.Volume != 0.15 {
		t.Fatalf("volume = %#v, want 0.15", bridge.speech.Volume)
	}

	_, err = CallTool(context.Background(), bridge, "stackchan_speak", json.RawMessage(`{"text":"too loud","volume":1.1}`))
	if err == nil {
		t.Fatal("expected invalid volume error")
	}
	if !strings.Contains(err.Error(), "speech volume") {
		t.Fatalf("error = %q, want speech volume error", err)
	}

	_, err = CallTool(context.Background(), bridge, "stackchan_speak", json.RawMessage(`{"text":"   "}`))
	if err == nil {
		t.Fatal("expected missing text error")
	}
	if !strings.Contains(err.Error(), "speech text is required") {
		t.Fatalf("error = %q, want speech text required", err)
	}

	_, err = CallTool(context.Background(), bridge, "stackchan_speak", json.RawMessage(`{"text":"`+strings.Repeat("x", 241)+`"}`))
	if err == nil {
		t.Fatal("expected long text error")
	}
	if !strings.Contains(err.Error(), "speech text must be 240 characters or fewer") {
		t.Fatalf("error = %q, want speech text length error", err)
	}
}
