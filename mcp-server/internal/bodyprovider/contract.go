package bodyprovider

import "context"

type Capability string

const (
	CapabilityVision     Capability = "vision"
	CapabilityHearing    Capability = "hearing"
	CapabilitySpeech     Capability = "speech"
	CapabilityExpression Capability = "expression"
	CapabilityMotion     Capability = "motion"
	CapabilityLED        Capability = "led"
)

type ActionKind string

const (
	ActionSpeak   ActionKind = "speak"
	ActionExpress ActionKind = "express"
	ActionMove    ActionKind = "move"
	ActionLED     ActionKind = "led"
)

type BodyAction struct {
	Kind    ActionKind     `json:"kind"`
	Payload map[string]any `json:"payload,omitempty"`
}

type Status struct {
	Connected      bool     `json:"connected"`
	Device         string   `json:"device"`
	Firmware       string   `json:"firmware"`
	BatteryPercent int      `json:"battery_percent,omitempty"`
	IP             string   `json:"ip,omitempty"`
	Capabilities   []string `json:"capabilities"`
}

type ExpressionRequest struct {
	Emotion string `json:"emotion"`
}

type HeadRequest struct {
	PanDeg  int     `json:"pan_deg"`
	TiltDeg int     `json:"tilt_deg"`
	Speed   float64 `json:"speed"`
}

type LEDRequest struct {
	Pattern    string  `json:"pattern"`
	Color      string  `json:"color"`
	Brightness float64 `json:"brightness"`
}

type MotionRequest struct {
	Name string `json:"name"`
}

type SpeechRequest struct {
	Text   string   `json:"text"`
	Volume *float64 `json:"volume,omitempty"`
}

type ActionResult struct {
	OK     bool   `json:"ok"`
	Action string `json:"action"`
	State  any    `json:"state,omitempty"`
}

type StackChanBridge interface {
	GetStatus(context.Context) (Status, error)
	SetExpression(context.Context, ExpressionRequest) (ActionResult, error)
	MoveHead(context.Context, HeadRequest) (ActionResult, error)
	SetLED(context.Context, LEDRequest) (ActionResult, error)
	RunMotion(context.Context, MotionRequest) (ActionResult, error)
	Speak(context.Context, SpeechRequest) (ActionResult, error)
}

type SnapshotOptions struct {
	MaxWidth int
}

type AudioOptions struct {
	DurationMs int
}

type CameraSnapshot struct {
	ContentType string
	Data        []byte
}

type AudioClip struct {
	ContentType string
	Data        []byte
	DurationMs  int
}

type SensorState struct {
	Motion     bool    `json:"motion"`
	SoundLevel float64 `json:"sound_level"`
	TS         int64   `json:"ts"`
}

type PerceptionBridge interface {
	CameraSnapshot(context.Context, SnapshotOptions) (CameraSnapshot, error)
	AudioClip(context.Context, AudioOptions) (AudioClip, error)
	Sensors(context.Context) (SensorState, error)
}

type CaptureOptions struct {
	CameraEnabled    bool
	SnapshotMaxWidth int
	AudioClipMs      int
}

type CapturedPercept struct {
	Sensor   SensorState
	Snapshot CameraSnapshot
	Audio    AudioClip
}

type Provider interface {
	Name() string
	Capabilities() []Capability
	Actuate(context.Context, BodyAction) (ActionResult, error)
	CapturePercept(context.Context, CaptureOptions) (CapturedPercept, error)
}
