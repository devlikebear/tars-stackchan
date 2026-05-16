package mock

import (
	"context"
	"sync"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

type Bridge struct {
	mu         sync.Mutex
	status     stackchan.Status
	expression stackchan.ExpressionRequest
	head       stackchan.HeadRequest
	led        stackchan.LEDRequest
	motion     stackchan.MotionRequest
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
