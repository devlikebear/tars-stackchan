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
	"strings"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

const maxErrorBodyBytes = 4096

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
