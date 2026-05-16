package httpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

const maxErrorBodyBytes = 4096

// maxMediaBytes bounds a perception capture body so a misbehaving or hostile
// firmware response cannot exhaust memory. QVGA JPEG and a few-second 16 kHz
// mono WAV are well under this.
const maxMediaBytes = 4 << 20

type Config struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

type Bridge struct {
	baseURL string
	token   string
	client  *http.Client
}

var (
	_ stackchan.Bridge           = (*Bridge)(nil)
	_ stackchan.PerceptionBridge = (*Bridge)(nil)
)

func New(config Config) (*Bridge, error) {
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		return nil, errors.New("TARS_STACKCHAN_BASE_URL is required for HTTP bridge")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("base URL scheme %q is not supported", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, errors.New("base URL host is required")
	}

	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	return &Bridge{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   config.Token,
		client:  client,
	}, nil
}

func (b *Bridge) GetStatus(ctx context.Context) (stackchan.Status, error) {
	var status stackchan.Status
	if err := b.do(ctx, http.MethodGet, "/v1/status", nil, false, &status); err != nil {
		return stackchan.Status{}, err
	}
	return status, nil
}

func (b *Bridge) SetExpression(ctx context.Context, req stackchan.ExpressionRequest) (stackchan.ActionResult, error) {
	return b.postAction(ctx, "/v1/expression", req)
}

func (b *Bridge) MoveHead(ctx context.Context, req stackchan.HeadRequest) (stackchan.ActionResult, error) {
	return b.postAction(ctx, "/v1/head", req)
}

func (b *Bridge) SetLED(ctx context.Context, req stackchan.LEDRequest) (stackchan.ActionResult, error) {
	return b.postAction(ctx, "/v1/leds", req)
}

func (b *Bridge) RunMotion(ctx context.Context, req stackchan.MotionRequest) (stackchan.ActionResult, error) {
	return b.postAction(ctx, "/v1/motion", req)
}

func (b *Bridge) Speak(ctx context.Context, req stackchan.SpeechRequest) (stackchan.ActionResult, error) {
	return b.postAction(ctx, "/v1/speech", req)
}

func (b *Bridge) CameraSnapshot(ctx context.Context, opts stackchan.SnapshotOptions) (stackchan.CameraSnapshot, error) {
	path := "/v1/camera/snapshot"
	if opts.MaxWidth > 0 {
		path += "?" + url.Values{"max_width": {strconv.Itoa(opts.MaxWidth)}}.Encode()
	}
	data, contentType, err := b.getMedia(ctx, path, "image/jpeg")
	if err != nil {
		return stackchan.CameraSnapshot{}, err
	}
	return stackchan.CameraSnapshot{ContentType: contentType, Data: data}, nil
}

func (b *Bridge) AudioClip(ctx context.Context, opts stackchan.AudioOptions) (stackchan.AudioClip, error) {
	path := "/v1/audio/clip"
	if opts.DurationMs > 0 {
		path += "?" + url.Values{"ms": {strconv.Itoa(opts.DurationMs)}}.Encode()
	}
	data, contentType, err := b.getMedia(ctx, path, "audio/wav")
	if err != nil {
		return stackchan.AudioClip{}, err
	}
	return stackchan.AudioClip{ContentType: contentType, Data: data, DurationMs: opts.DurationMs}, nil
}

func (b *Bridge) Sensors(ctx context.Context) (stackchan.SensorState, error) {
	var state stackchan.SensorState
	if err := b.do(ctx, http.MethodGet, "/v1/sensors", nil, true, &state); err != nil {
		return stackchan.SensorState{}, err
	}
	return state, nil
}

// getMedia performs an authenticated GET that returns a binary body. Capture
// endpoints require the bearer token (only /v1/status is unauthenticated).
func (b *Bridge) getMedia(ctx context.Context, path, accept string) ([]byte, string, error) {
	if strings.TrimSpace(b.token) == "" {
		return nil, "", errors.New("TARS_STACKCHAN_TOKEN is required for perception capture requests")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+path, nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Authorization", "Bearer "+b.token)
	if accept != "" {
		request.Header.Set("Accept", accept)
	}

	response, err := b.client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = response.Status
		}
		return nil, "", fmt.Errorf("firmware GET %s returned %d: %s", path, response.StatusCode, message)
	}

	// Read one extra byte so an over-limit body is detected rather than
	// silently truncated.
	body, err := io.ReadAll(io.LimitReader(response.Body, maxMediaBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read firmware media response: %w", err)
	}
	if len(body) > maxMediaBytes {
		return nil, "", fmt.Errorf("firmware GET %s body exceeds %d byte limit", path, maxMediaBytes)
	}
	if len(body) == 0 {
		return nil, "", fmt.Errorf("firmware GET %s returned an empty body", path)
	}
	return body, response.Header.Get("Content-Type"), nil
}

func (b *Bridge) postAction(ctx context.Context, path string, payload any) (stackchan.ActionResult, error) {
	var result stackchan.ActionResult
	if err := b.do(ctx, http.MethodPost, path, payload, true, &result); err != nil {
		return stackchan.ActionResult{}, err
	}
	return result, nil
}

func (b *Bridge) do(ctx context.Context, method, path string, payload any, auth bool, target any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, b.baseURL+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if auth {
		if strings.TrimSpace(b.token) == "" {
			return errors.New("TARS_STACKCHAN_TOKEN is required for mutating HTTP bridge requests")
		}
		request.Header.Set("Authorization", "Bearer "+b.token)
	}

	response, err := b.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("firmware %s %s returned %d: %s", method, path, response.StatusCode, message)
	}

	if target == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode firmware response: %w", err)
	}
	return nil
}
