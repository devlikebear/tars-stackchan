package stackchan

import "context"

const (
	ToolGetStatus      = "stackchan_get_status"
	ToolSetExpression  = "stackchan_set_expression"
	ToolMoveHead       = "stackchan_move_head"
	ToolSetLED         = "stackchan_set_led"
	ToolRunMotion      = "stackchan_run_motion"
	ToolSpeak          = "stackchan_speak"
	ToolUploadFirmware = "stackchan_upload_firmware"

	SafeTiltMin = 5
	SafeTiltMax = 85
)

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

type FirmwareUploadRequest struct {
	Mode       string `json:"mode,omitempty"`
	DeployHost *bool  `json:"deploy_host,omitempty"`
	SkipSmoke  *bool  `json:"skip_smoke,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
	UploadPort string `json:"upload_port,omitempty"`
}

type FirmwareUploadResult struct {
	OK       bool   `json:"ok"`
	Mode     string `json:"mode"`
	Script   string `json:"script,omitempty"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

type ActionResult struct {
	OK     bool   `json:"ok"`
	Action string `json:"action"`
	State  any    `json:"state,omitempty"`
}

type Bridge interface {
	GetStatus(context.Context) (Status, error)
	SetExpression(context.Context, ExpressionRequest) (ActionResult, error)
	MoveHead(context.Context, HeadRequest) (ActionResult, error)
	SetLED(context.Context, LEDRequest) (ActionResult, error)
	RunMotion(context.Context, MotionRequest) (ActionResult, error)
	Speak(context.Context, SpeechRequest) (ActionResult, error)
}

// SnapshotOptions and AudioOptions are perception capture inputs. Zero values
// mean "let the firmware use its configured default".
type SnapshotOptions struct {
	// MaxWidth is an upper bound; firmware picks the nearest supported
	// framesize. 0 means use the firmware default (QVGA).
	MaxWidth int
}

type AudioOptions struct {
	// DurationMs is the requested clip length. 0 means the firmware default
	// (1500ms). Firmware clamps to its safe maximum.
	DurationMs int
}

// CameraSnapshot is a single still frame. Data is the raw image body exactly
// as the firmware returned it (JPEG); no decoding is performed here, so
// dimensions are intentionally not surfaced (we do not have them without
// decoding and must not fabricate them).
type CameraSnapshot struct {
	ContentType string
	Data        []byte
}

// AudioClip is a short recorded clip. Data is the raw WAV body; DurationMs is
// the duration that was requested (the firmware framing matches it).
type AudioClip struct {
	ContentType string
	Data        []byte
	DurationMs  int
}

// SensorState is the lightweight, pollable trigger state. Field tags match the
// /v1/sensors protocol body.
type SensorState struct {
	Motion     bool    `json:"motion"`
	SoundLevel float64 `json:"sound_level"`
	TS         int64   `json:"ts"`
}

// PerceptionBridge is an OPTIONAL capability, kept separate from Bridge so the
// MCP tool path and existing implementations/test doubles are unaffected.
// Only firmware/hardware that advertises the camera/microphone capabilities
// implements it; consumers type-assert for it.
type PerceptionBridge interface {
	CameraSnapshot(context.Context, SnapshotOptions) (CameraSnapshot, error)
	AudioClip(context.Context, AudioOptions) (AudioClip, error)
	Sensors(context.Context) (SensorState, error)
}

type FirmwareRunner interface {
	UploadFirmware(context.Context, FirmwareUploadRequest) (FirmwareUploadResult, error)
}
