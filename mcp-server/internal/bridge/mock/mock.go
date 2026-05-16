package mock

import (
	"context"
	_ "embed"
	"sync"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

var (
	_ stackchan.Bridge           = (*Bridge)(nil)
	_ stackchan.PerceptionBridge = (*Bridge)(nil)
)

//go:embed testdata/sample.jpg
var sampleJPEG []byte

//go:embed testdata/sample.wav
var sampleWAV []byte

type Bridge struct {
	mu         sync.Mutex
	status     stackchan.Status
	expression stackchan.ExpressionRequest
	head       stackchan.HeadRequest
	led        stackchan.LEDRequest
	motion     stackchan.MotionRequest
	speech     stackchan.SpeechRequest
}

func New() *Bridge {
	return &Bridge{
		status: stackchan.Status{
			Connected:      true,
			Device:         "stackchan-k151",
			Firmware:       "tars-stackchan-mock",
			BatteryPercent: 100,
			IP:             "127.0.0.1",
			Capabilities: []string{
				"expression",
				"head",
				"leds",
				"motion",
				"speech",
				"camera",
				"microphone",
			},
		},
		expression: stackchan.ExpressionRequest{Emotion: "neutral"},
		head:       stackchan.HeadRequest{PanDeg: 0, TiltDeg: 30, Speed: 0.5},
		led:        stackchan.LEDRequest{Pattern: "solid", Color: "#00AEEF", Brightness: 0.5},
	}
}

func (b *Bridge) GetStatus(context.Context) (stackchan.Status, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.status, nil
}

func (b *Bridge) SetExpression(_ context.Context, req stackchan.ExpressionRequest) (stackchan.ActionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expression = req
	return stackchan.ActionResult{OK: true, Action: "set_expression", State: req}, nil
}

func (b *Bridge) MoveHead(_ context.Context, req stackchan.HeadRequest) (stackchan.ActionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.head = req
	return stackchan.ActionResult{OK: true, Action: "move_head", State: req}, nil
}

func (b *Bridge) SetLED(_ context.Context, req stackchan.LEDRequest) (stackchan.ActionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.led = req
	return stackchan.ActionResult{OK: true, Action: "set_led", State: req}, nil
}

func (b *Bridge) RunMotion(_ context.Context, req stackchan.MotionRequest) (stackchan.ActionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.motion = req
	return stackchan.ActionResult{OK: true, Action: "run_motion", State: req}, nil
}

func (b *Bridge) Speak(_ context.Context, req stackchan.SpeechRequest) (stackchan.ActionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.speech = req
	return stackchan.ActionResult{OK: true, Action: "speak", State: req}, nil
}

// CameraSnapshot returns a deterministic, valid JPEG fixture so UI/CI flows
// exercise the perception path without hardware.
func (b *Bridge) CameraSnapshot(_ context.Context, _ stackchan.SnapshotOptions) (stackchan.CameraSnapshot, error) {
	return stackchan.CameraSnapshot{ContentType: "image/jpeg", Data: sampleJPEG}, nil
}

// AudioClip returns a deterministic, valid 16 kHz mono WAV fixture.
func (b *Bridge) AudioClip(_ context.Context, opts stackchan.AudioOptions) (stackchan.AudioClip, error) {
	return stackchan.AudioClip{ContentType: "audio/wav", Data: sampleWAV, DurationMs: opts.DurationMs}, nil
}

// Sensors returns a stable, media-free trigger state with a real timestamp.
func (b *Bridge) Sensors(_ context.Context) (stackchan.SensorState, error) {
	return stackchan.SensorState{Motion: false, SoundLevel: 0, TS: time.Now().UnixMilli()}, nil
}
