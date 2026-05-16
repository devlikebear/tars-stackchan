package stackchan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

var (
	allowedExpressions = map[string]bool{
		"neutral":   true,
		"happy":     true,
		"sad":       true,
		"angry":     true,
		"surprised": true,
		"sleepy":    true,
		"blink":     true,
	}

	allowedLEDPatterns = map[string]bool{
		"solid": true,
		"blink": true,
		"pulse": true,
		"off":   true,
	}

	hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

const maxSpeechTextRunes = 240

func ListTools() []Tool {
	return controlTools()
}

func ListToolsWithFirmware() []Tool {
	tools := controlTools()
	return append(tools, firmwareUploadTool())
}

func controlTools() []Tool {
	return []Tool{
		{
			Name:        ToolGetStatus,
			Description: "Read Stack-chan connection, firmware, battery, IP, and capability status.",
			InputSchema: objectSchema(map[string]any{}, []string{}),
		},
		{
			Name:        ToolSetExpression,
			Description: "Set Stack-chan's face expression.",
			InputSchema: objectSchema(map[string]any{
				"emotion": enumStringSchema(expressionValues()),
			}, []string{"emotion"}),
		},
		{
			Name:        ToolMoveHead,
			Description: "Move Stack-chan's head. Tilt is clamped to 5..85 degrees for servo safety.",
			InputSchema: objectSchema(map[string]any{
				"pan_deg":  numberSchema("Horizontal pan angle in degrees."),
				"tilt_deg": numberSchema("Vertical tilt angle in degrees. Safe server clamp: 5..85."),
				"speed":    numberSchema("Normalized servo speed from 0.0 to 1.0."),
			}, []string{"pan_deg", "tilt_deg", "speed"}),
		},
		{
			Name:        ToolSetLED,
			Description: "Set Stack-chan LED pattern, color, and brightness.",
			InputSchema: objectSchema(map[string]any{
				"pattern":    enumStringSchema(ledPatternValues()),
				"color":      map[string]any{"type": "string", "pattern": "^#[0-9A-Fa-f]{6}$"},
				"brightness": numberSchema("Normalized brightness from 0.0 to 1.0."),
			}, []string{"pattern", "color", "brightness"}),
		},
		{
			Name:        ToolRunMotion,
			Description: "Run a named Stack-chan motion such as nod or shake.",
			InputSchema: objectSchema(map[string]any{
				"name": map[string]any{"type": "string"},
			}, []string{"name"}),
		},
		{
			Name:        ToolSpeak,
			Description: "Speak text through Stack-chan's configured speech voice.",
			InputSchema: objectSchema(map[string]any{
				"text": map[string]any{
					"type":        "string",
					"description": "Text to speak. Maximum 240 characters.",
					"maxLength":   maxSpeechTextRunes,
				},
				"volume": numberSchema("Optional normalized speech volume from 0.0 to 1.0."),
			}, []string{"text"}),
		},
	}
}

func firmwareUploadTool() Tool {
	return Tool{
		Name:        ToolUploadFirmware,
		Description: "Prepare, build, upload, or smoke-test the TARS Stack-chan firmware through the local upload helper. This tool is only available when firmware tools are explicitly enabled.",
		InputSchema: objectSchema(map[string]any{
			"mode": map[string]any{
				"type":        "string",
				"description": "Upload helper mode. Defaults to mod, which builds and flashes only the bridge MOD.",
				"enum":        []string{"prepare", "deps", "host", "mod", "smoke", "all"},
				"default":     "mod",
			},
			"deploy_host": map[string]any{
				"type":        "boolean",
				"description": "Set TARS_STACKCHAN_DEPLOY_HOST=1 for mode=all.",
			},
			"skip_smoke": map[string]any{
				"type":        "boolean",
				"description": "Set TARS_STACKCHAN_SKIP_SMOKE=1 for mode=all.",
			},
			"base_url": map[string]any{
				"type":        "string",
				"description": "Override TARS_STACKCHAN_BASE_URL for smoke checks.",
			},
			"upload_port": map[string]any{
				"type":        "string",
				"description": "Override TARS_STACKCHAN_UPLOAD_PORT, for example /dev/cu.usbmodem101.",
			},
		}, []string{}),
	}
}

func CallTool(ctx context.Context, bridge Bridge, name string, args json.RawMessage) (ToolCallResult, error) {
	if bridge == nil {
		return ToolCallResult{}, errors.New("stackchan bridge is nil")
	}

	switch name {
	case ToolGetStatus:
		if err := decodeNoArgs(args); err != nil {
			return ToolCallResult{}, err
		}
		status, err := bridge.GetStatus(ctx)
		if err != nil {
			return ToolCallResult{}, err
		}
		return jsonTextResult(status)

	case ToolSetExpression:
		req, err := decodeStrict[ExpressionRequest](args)
		if err != nil {
			return ToolCallResult{}, err
		}
		if err := validateExpression(req); err != nil {
			return ToolCallResult{}, err
		}
		result, err := bridge.SetExpression(ctx, req)
		if err != nil {
			return ToolCallResult{}, err
		}
		return jsonTextResult(result)

	case ToolMoveHead:
		req, err := decodeStrict[HeadRequest](args)
		if err != nil {
			return ToolCallResult{}, err
		}
		if err := validateHead(req); err != nil {
			return ToolCallResult{}, err
		}
		req.TiltDeg = clamp(req.TiltDeg, SafeTiltMin, SafeTiltMax)
		result, err := bridge.MoveHead(ctx, req)
		if err != nil {
			return ToolCallResult{}, err
		}
		return jsonTextResult(result)

	case ToolSetLED:
		req, err := decodeStrict[LEDRequest](args)
		if err != nil {
			return ToolCallResult{}, err
		}
		if err := validateLED(req); err != nil {
			return ToolCallResult{}, err
		}
		result, err := bridge.SetLED(ctx, req)
		if err != nil {
			return ToolCallResult{}, err
		}
		return jsonTextResult(result)

	case ToolRunMotion:
		req, err := decodeStrict[MotionRequest](args)
		if err != nil {
			return ToolCallResult{}, err
		}
		if strings.TrimSpace(req.Name) == "" {
			return ToolCallResult{}, errors.New("motion name is required")
		}
		result, err := bridge.RunMotion(ctx, req)
		if err != nil {
			return ToolCallResult{}, err
		}
		return jsonTextResult(result)

	case ToolSpeak:
		req, err := decodeStrict[SpeechRequest](args)
		if err != nil {
			return ToolCallResult{}, err
		}
		if err := validateSpeech(req); err != nil {
			return ToolCallResult{}, err
		}
		result, err := bridge.Speak(ctx, req)
		if err != nil {
			return ToolCallResult{}, err
		}
		return jsonTextResult(result)

	default:
		return ToolCallResult{}, fmt.Errorf("unknown tool %q", name)
	}
}

func CallFirmwareTool(ctx context.Context, runner FirmwareRunner, args json.RawMessage) (ToolCallResult, error) {
	if runner == nil {
		return ToolCallResult{}, errors.New("firmware tools are disabled; set TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS=1")
	}

	req, err := decodeStrict[FirmwareUploadRequest](args)
	if err != nil {
		return ToolCallResult{}, err
	}
	if err := validateFirmwareUpload(&req); err != nil {
		return ToolCallResult{}, err
	}
	result, err := runner.UploadFirmware(ctx, req)
	if err != nil {
		return ToolCallResult{}, err
	}
	return jsonTextResult(result)
}

func decodeNoArgs(raw json.RawMessage) error {
	var req struct{}
	_, err := decodeStrictInto(raw, &req)
	return err
}

func decodeStrict[T any](raw json.RawMessage) (T, error) {
	var target T
	_, err := decodeStrictInto(raw, &target)
	return target, err
}

func decodeStrictInto(raw json.RawMessage, target any) (any, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		raw = []byte(`{}`)
	}

	raw = stripReservedMeta(raw)

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, errors.New("invalid trailing JSON value")
	}
	return target, nil
}

// stripReservedMeta removes the MCP-reserved top-level "_meta" key from a JSON
// object before strict decoding. MCP clients (e.g. Claude Code) attach "_meta"
// to request params and tool arguments; it carries protocol metadata, not tool
// input, so it must be ignored rather than rejected as an unknown field.
// Strictness is preserved for every other key.
func stripReservedMeta(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return raw
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return raw
	}
	if _, ok := fields["_meta"]; !ok {
		return raw
	}
	delete(fields, "_meta")

	cleaned, err := json.Marshal(fields)
	if err != nil {
		return raw
	}
	return cleaned
}

func validateExpression(req ExpressionRequest) error {
	if strings.TrimSpace(req.Emotion) == "" {
		return errors.New("expression emotion is required")
	}
	if !allowedExpressions[req.Emotion] {
		return fmt.Errorf("unsupported expression %q", req.Emotion)
	}
	return nil
}

func validateHead(req HeadRequest) error {
	if req.Speed < 0 || req.Speed > 1 {
		return fmt.Errorf("head speed %.2f must be between 0.0 and 1.0", req.Speed)
	}
	return nil
}

func validateLED(req LEDRequest) error {
	if strings.TrimSpace(req.Pattern) == "" {
		return errors.New("LED pattern is required")
	}
	if !allowedLEDPatterns[req.Pattern] {
		return fmt.Errorf("unsupported LED pattern %q", req.Pattern)
	}
	if req.Pattern != "off" && !hexColorPattern.MatchString(req.Color) {
		return fmt.Errorf("invalid LED color %q; expected #RRGGBB", req.Color)
	}
	if req.Brightness < 0 || req.Brightness > 1 {
		return fmt.Errorf("LED brightness %.2f must be between 0.0 and 1.0", req.Brightness)
	}
	return nil
}

func validateSpeech(req SpeechRequest) error {
	if strings.TrimSpace(req.Text) == "" {
		return errors.New("speech text is required")
	}
	if utf8.RuneCountInString(req.Text) > maxSpeechTextRunes {
		return fmt.Errorf("speech text must be %d characters or fewer", maxSpeechTextRunes)
	}
	if req.Volume != nil && (*req.Volume < 0 || *req.Volume > 1) {
		return fmt.Errorf("speech volume %.2f must be between 0.0 and 1.0", *req.Volume)
	}
	return nil
}

func validateFirmwareUpload(req *FirmwareUploadRequest) error {
	req.Mode = strings.TrimSpace(req.Mode)
	if req.Mode == "" {
		req.Mode = "mod"
	}
	switch req.Mode {
	case "prepare", "deps", "host", "mod", "smoke", "all":
	default:
		return fmt.Errorf("unsupported firmware upload mode %q", req.Mode)
	}
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.UploadPort = strings.TrimSpace(req.UploadPort)
	return nil
}

func jsonTextResult(value any) (ToolCallResult, error) {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ToolCallResult{}, err
	}
	return ToolCallResult{
		Content: []ContentBlock{
			{Type: "text", Text: string(payload)},
		},
	}, nil
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func enumStringSchema(values []string) map[string]any {
	return map[string]any{
		"type": "string",
		"enum": values,
	}
}

func numberSchema(description string) map[string]any {
	return map[string]any{
		"type":        "number",
		"description": description,
	}
}

func expressionValues() []string {
	return sortedKeys(allowedExpressions)
}

func ledPatternValues() []string {
	return sortedKeys(allowedLEDPatterns)
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
