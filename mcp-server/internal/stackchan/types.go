package stackchan

import (
	"context"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bodyprovider"
)

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

type Status = bodyprovider.Status
type ExpressionRequest = bodyprovider.ExpressionRequest
type HeadRequest = bodyprovider.HeadRequest
type LEDRequest = bodyprovider.LEDRequest
type MotionRequest = bodyprovider.MotionRequest
type SpeechRequest = bodyprovider.SpeechRequest

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

type ActionResult = bodyprovider.ActionResult
type Bridge = bodyprovider.StackChanBridge

// SnapshotOptions and AudioOptions are perception capture inputs. Zero values
// mean "let the firmware use its configured default".
type SnapshotOptions = bodyprovider.SnapshotOptions

type AudioOptions = bodyprovider.AudioOptions

// CameraSnapshot is a single still frame. Data is the raw image body exactly
// as the firmware returned it (JPEG); no decoding is performed here, so
// dimensions are intentionally not surfaced (we do not have them without
// decoding and must not fabricate them).
type CameraSnapshot = bodyprovider.CameraSnapshot

// AudioClip is a short recorded clip. Data is the raw WAV body; DurationMs is
// the duration that was requested (the firmware framing matches it).
type AudioClip = bodyprovider.AudioClip

// SensorState is the lightweight, pollable trigger state. Field tags match the
// /v1/sensors protocol body.
type SensorState = bodyprovider.SensorState

// PerceptionBridge is an OPTIONAL capability, kept separate from Bridge so the
// MCP tool path and existing implementations/test doubles are unaffected.
// Only firmware/hardware that advertises the camera/microphone capabilities
// implements it; consumers type-assert for it.
type PerceptionBridge = bodyprovider.PerceptionBridge

type FirmwareRunner interface {
	UploadFirmware(context.Context, FirmwareUploadRequest) (FirmwareUploadResult, error)
}
