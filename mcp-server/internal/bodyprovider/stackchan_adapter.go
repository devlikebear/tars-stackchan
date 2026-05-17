package bodyprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type StackChanProviderConfig struct {
	Name          string
	CameraEnabled bool
}

type StackChanProvider struct {
	name          string
	bridge        StackChanBridge
	perception    PerceptionBridge
	cameraEnabled bool
}

var _ Provider = (*StackChanProvider)(nil)

func NewStackChanProvider(bridge StackChanBridge, cfg StackChanProviderConfig) (*StackChanProvider, error) {
	if bridge == nil {
		return nil, errors.New("stackchan bridge is required")
	}
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "stackchan"
	}
	provider := &StackChanProvider{
		name:          name,
		bridge:        bridge,
		cameraEnabled: cfg.CameraEnabled,
	}
	if perception, ok := bridge.(PerceptionBridge); ok {
		provider.perception = perception
	}
	return provider, nil
}

func (p *StackChanProvider) Name() string {
	if p == nil {
		return ""
	}
	return p.name
}

func (p *StackChanProvider) Capabilities() []Capability {
	if p == nil {
		return nil
	}
	out := make([]Capability, 0, 6)
	if p.cameraEnabled {
		out = append(out, CapabilityVision)
	}
	out = append(out,
		CapabilityHearing,
		CapabilitySpeech,
		CapabilityExpression,
		CapabilityMotion,
		CapabilityLED,
	)
	return out
}

func (p *StackChanProvider) Actuate(ctx context.Context, action BodyAction) (ActionResult, error) {
	if p == nil || p.bridge == nil {
		return ActionResult{}, errors.New("stackchan provider is not configured")
	}
	normalized, err := normalizeAction(action)
	if err != nil {
		return ActionResult{}, err
	}
	switch normalized.Kind {
	case ActionSpeak:
		req, err := decodePayload[SpeechRequest](normalized.Payload)
		if err != nil {
			return ActionResult{}, err
		}
		return p.bridge.Speak(ctx, req)
	case ActionExpress:
		emotion := firstString(normalized.Payload, "emotion", "expression")
		if emotion == "" {
			return ActionResult{}, errors.New("express action requires emotion")
		}
		return p.bridge.SetExpression(ctx, ExpressionRequest{Emotion: emotion})
	case ActionMove:
		if motion := firstString(normalized.Payload, "name", "motion", "preset"); motion != "" {
			return p.bridge.RunMotion(ctx, MotionRequest{Name: motion})
		}
		req, err := decodePayload[HeadRequest](normalized.Payload)
		if err != nil {
			return ActionResult{}, err
		}
		return p.bridge.MoveHead(ctx, req)
	case ActionLED:
		req, err := decodePayload[LEDRequest](normalized.Payload)
		if err != nil {
			return ActionResult{}, err
		}
		return p.bridge.SetLED(ctx, req)
	default:
		return ActionResult{}, fmt.Errorf("unsupported body action kind %q", normalized.Kind)
	}
}

func (p *StackChanProvider) CapturePercept(ctx context.Context, opts CaptureOptions) (CapturedPercept, error) {
	if p == nil || p.perception == nil {
		return CapturedPercept{}, errors.New("stackchan perception bridge is not configured")
	}
	sensor, err := p.perception.Sensors(ctx)
	if err != nil {
		return CapturedPercept{}, err
	}
	var snapshot CameraSnapshot
	cameraEnabled := opts.CameraEnabled && p.cameraEnabled
	if cameraEnabled {
		snapshot, err = p.perception.CameraSnapshot(ctx, SnapshotOptions{MaxWidth: opts.SnapshotMaxWidth})
		if err != nil {
			return CapturedPercept{}, err
		}
	}
	audio, err := p.perception.AudioClip(ctx, AudioOptions{DurationMs: opts.AudioClipMs})
	if err != nil {
		return CapturedPercept{}, err
	}
	return CapturedPercept{Sensor: sensor, Snapshot: snapshot, Audio: audio}, nil
}

func normalizeAction(action BodyAction) (BodyAction, error) {
	kind := ActionKind(strings.ToLower(strings.TrimSpace(string(action.Kind))))
	payload := clonePayload(action.Payload)
	switch kind {
	case ActionSpeak:
		text := firstString(payload, "text", "message", "summary")
		if text == "" {
			return BodyAction{}, errors.New("speak action requires text")
		}
		payload["text"] = text
	case ActionExpress:
		if firstString(payload, "emotion", "expression") == "" {
			return BodyAction{}, errors.New("express action requires emotion")
		}
	case ActionMove, ActionLED:
		if len(payload) == 0 {
			return BodyAction{}, fmt.Errorf("%s action requires payload", kind)
		}
	default:
		return BodyAction{}, fmt.Errorf("unsupported body action kind %q", action.Kind)
	}
	return BodyAction{Kind: kind, Payload: payload}, nil
}

func decodePayload[T any](payload map[string]any) (T, error) {
	var out T
	data, err := json.Marshal(payload)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func clonePayload(payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload))
	for key, value := range payload {
		out[key] = value
	}
	return out
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fmt.Sprint(payload[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}
