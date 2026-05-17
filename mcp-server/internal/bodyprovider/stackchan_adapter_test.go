package bodyprovider

import (
	"context"
	"testing"
)

func TestStackChanProviderActuateRoutesActions(t *testing.T) {
	bridge := &fakeStackChanBridge{}
	provider, err := NewStackChanProvider(bridge, StackChanProviderConfig{Name: "stackchan", CameraEnabled: true})
	if err != nil {
		t.Fatalf("NewStackChanProvider: %v", err)
	}

	actions := []BodyAction{
		{Kind: ActionSpeak, Payload: map[string]any{"text": "hello", "volume": 0.25}},
		{Kind: ActionExpress, Payload: map[string]any{"expression": "happy"}},
		{Kind: ActionMove, Payload: map[string]any{"pan_deg": 15, "tilt_deg": 35, "speed": 0.6}},
		{Kind: ActionLED, Payload: map[string]any{"pattern": "solid", "color": "#00AEEF", "brightness": 0.4}},
		{Kind: ActionMove, Payload: map[string]any{"name": "nod"}},
	}
	for _, action := range actions {
		if _, err := provider.Actuate(context.Background(), action); err != nil {
			t.Fatalf("Actuate(%s): %v", action.Kind, err)
		}
	}

	if bridge.speech.Text != "hello" || bridge.speech.Volume == nil || *bridge.speech.Volume != 0.25 {
		t.Fatalf("speech = %+v", bridge.speech)
	}
	if bridge.expression.Emotion != "happy" {
		t.Fatalf("expression = %+v", bridge.expression)
	}
	if bridge.head.PanDeg != 15 || bridge.head.TiltDeg != 35 || bridge.head.Speed != 0.6 {
		t.Fatalf("head = %+v", bridge.head)
	}
	if bridge.led.Color != "#00AEEF" || bridge.led.Brightness != 0.4 {
		t.Fatalf("led = %+v", bridge.led)
	}
	if bridge.motion.Name != "nod" {
		t.Fatalf("motion = %+v", bridge.motion)
	}
}

func TestStackChanProviderActuateRejectsMalformed(t *testing.T) {
	provider, err := NewStackChanProvider(&fakeStackChanBridge{}, StackChanProviderConfig{Name: "stackchan"})
	if err != nil {
		t.Fatalf("NewStackChanProvider: %v", err)
	}
	tests := []BodyAction{
		{Kind: ActionSpeak, Payload: map[string]any{"text": " "}},
		{Kind: ActionExpress, Payload: map[string]any{}},
		{Kind: ActionMove, Payload: map[string]any{}},
		{Kind: ActionLED, Payload: map[string]any{}},
		{Kind: "dance", Payload: map[string]any{"name": "spin"}},
	}
	for _, action := range tests {
		if _, err := provider.Actuate(context.Background(), action); err == nil {
			t.Fatalf("Actuate(%+v) succeeded, want error", action)
		}
	}
}

func TestStackChanProviderCapturePerceptUsesAudioOnlyMode(t *testing.T) {
	bridge := &fakeStackChanBridge{
		sensor: SensorState{Motion: true, SoundLevel: 0.8, TS: 1779000000000},
		snap:   CameraSnapshot{ContentType: "image/jpeg", Data: []byte{0xFF, 0xD8}},
		clip:   AudioClip{ContentType: "audio/wav", Data: []byte("RIFF"), DurationMs: 1200},
	}
	provider, err := NewStackChanProvider(bridge, StackChanProviderConfig{Name: "stackchan", CameraEnabled: false})
	if err != nil {
		t.Fatalf("NewStackChanProvider: %v", err)
	}
	capture, err := provider.CapturePercept(context.Background(), CaptureOptions{AudioClipMs: 1200, SnapshotMaxWidth: 320})
	if err != nil {
		t.Fatalf("CapturePercept: %v", err)
	}
	if !capture.Sensor.Motion || capture.Audio.DurationMs != 1200 {
		t.Fatalf("capture = %+v", capture)
	}
	if bridge.cameraCalls != 0 {
		t.Fatalf("camera calls = %d, want 0", bridge.cameraCalls)
	}
	if len(capture.Snapshot.Data) != 0 {
		t.Fatalf("snapshot should be empty in audio-only mode: %+v", capture.Snapshot)
	}
}

type fakeStackChanBridge struct {
	status      Status
	expression  ExpressionRequest
	head        HeadRequest
	led         LEDRequest
	motion      MotionRequest
	speech      SpeechRequest
	sensor      SensorState
	snap        CameraSnapshot
	clip        AudioClip
	cameraCalls int
}

func (f *fakeStackChanBridge) GetStatus(context.Context) (Status, error) {
	if f.status.Device == "" {
		f.status = Status{Connected: true, Device: "mock"}
	}
	return f.status, nil
}

func (f *fakeStackChanBridge) SetExpression(_ context.Context, req ExpressionRequest) (ActionResult, error) {
	f.expression = req
	return ActionResult{OK: true, Action: "set_expression", State: req}, nil
}

func (f *fakeStackChanBridge) MoveHead(_ context.Context, req HeadRequest) (ActionResult, error) {
	f.head = req
	return ActionResult{OK: true, Action: "move_head", State: req}, nil
}

func (f *fakeStackChanBridge) SetLED(_ context.Context, req LEDRequest) (ActionResult, error) {
	f.led = req
	return ActionResult{OK: true, Action: "set_led", State: req}, nil
}

func (f *fakeStackChanBridge) RunMotion(_ context.Context, req MotionRequest) (ActionResult, error) {
	f.motion = req
	return ActionResult{OK: true, Action: "run_motion", State: req}, nil
}

func (f *fakeStackChanBridge) Speak(_ context.Context, req SpeechRequest) (ActionResult, error) {
	f.speech = req
	return ActionResult{OK: true, Action: "speak", State: req}, nil
}

func (f *fakeStackChanBridge) CameraSnapshot(context.Context, SnapshotOptions) (CameraSnapshot, error) {
	f.cameraCalls++
	return f.snap, nil
}

func (f *fakeStackChanBridge) AudioClip(context.Context, AudioOptions) (AudioClip, error) {
	return f.clip, nil
}

func (f *fakeStackChanBridge) Sensors(context.Context) (SensorState, error) {
	return f.sensor, nil
}
