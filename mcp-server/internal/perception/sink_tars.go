package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Sink delivers an observation to the brain (TARS). Implementations must not
// block the loop indefinitely; transient failures are retried then dropped.
type Sink interface {
	Post(ctx context.Context, obs Observation) error
}

// Logger is the minimal logging surface (so tests can capture).
type Logger interface {
	Printf(format string, args ...any)
}

// LogSink is the fallback when TARS is not configured: it records that an
// observation would have been sent. Useful in mock/dev.
type LogSink struct{ Log Logger }

func (s LogSink) Post(_ context.Context, obs Observation) error {
	if s.Log != nil {
		s.Log.Printf("[perceive] (no TARS) observation trigger=%s salience=%.2f summary=%q",
			obs.Trigger, obs.Salience, obs.Summary)
	}
	return nil
}

// webhookPayload is the agreed contract posted to TARS's existing inbound
// webhook channel (internal/tarsserver/handler_agentruntime_channels.go:
// /v1/channels/webhook/inbound/<channel>). Documented in
// docs/plans/embodied-bot-phase-2-perception-loop.md.
type webhookPayload struct {
	XEmbodiment bool    `json:"x-embodiment"`
	Source      string  `json:"source"` // always "stackchan"
	TS          int64   `json:"ts"`
	Trigger     string  `json:"trigger"`
	Salience    float64 `json:"salience"`
	Summary     string  `json:"summary"`
	Owner       string  `json:"owner"`
	Modality    string  `json:"modality"`
	MediaRef    string  `json:"media_ref,omitempty"`
	// Flattened identity so TARS's generic webhook channel can branch the
	// persona without Stack-chan-specific parsing.
	Identity           string  `json:"identity"` // owner | stranger | unknown
	IdentityConfidence float64 `json:"identity_confidence"`
	IdentityModality   string  `json:"identity_modality"`
	ImageRef           string  `json:"image_ref,omitempty"`
	AudioRef           string  `json:"audio_ref,omitempty"`
	// Text mirrors Summary so TARS's generic webhook channel (which routes a
	// message body into the session) needs no Stack-chan-specific parsing.
	Text string `json:"text"`
}

// TARSWebhookSink POSTs observations to TARS. It retries transient failures
// with bounded backoff, then drops (returns the last error) so the loop is
// never wedged by a down brain.
type TARSWebhookSink struct {
	BaseURL    string
	Channel    string
	Token      string
	Client     *http.Client
	MaxRetries int
	Backoff    time.Duration
	Log        Logger
}

func (s TARSWebhookSink) Post(ctx context.Context, obs Observation) error {
	body, err := json.Marshal(webhookPayload{
		XEmbodiment:        true,
		Source:             "stackchan",
		TS:                 obs.TS,
		Trigger:            obs.Trigger,
		Salience:           obs.Salience,
		Summary:            obs.Summary,
		Owner:              obs.Identity.Label,
		Modality:           modalityFromObservation(obs),
		MediaRef:           mediaRefFromObservation(obs),
		Identity:           obs.Identity.Label,
		IdentityConfidence: obs.Identity.Confidence,
		IdentityModality:   obs.Identity.Modality,
		ImageRef:           obs.ImageRef,
		AudioRef:           obs.AudioRef,
		Text:               obs.Summary,
	})
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/v1/channels/webhook/inbound/%s",
		strings.TrimRight(s.BaseURL, "/"), s.Channel)

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	retries := s.MaxRetries
	if retries <= 0 {
		retries = 3
	}
	backoff := s.Backoff
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff * time.Duration(attempt)):
			}
		}
		lastErr = s.postOnce(ctx, client, endpoint, body)
		if lastErr == nil {
			return nil
		}
	}
	if s.Log != nil {
		s.Log.Printf("[perceive] dropping observation after %d attempts: %v", retries+1, lastErr)
	}
	return lastErr
}

func (s TARSWebhookSink) postOnce(ctx context.Context, client *http.Client, endpoint string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(s.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+s.Token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("tars webhook returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

func modalityFromObservation(obs Observation) string {
	switch strings.ToLower(strings.TrimSpace(obs.Identity.Modality)) {
	case "voice":
		return "audio"
	case "face":
		return "vision"
	case "both":
		if strings.TrimSpace(obs.AudioRef) != "" {
			return "audio"
		}
		if strings.TrimSpace(obs.ImageRef) != "" {
			return "vision"
		}
	}
	if strings.TrimSpace(obs.AudioRef) != "" {
		return "audio"
	}
	if strings.TrimSpace(obs.ImageRef) != "" {
		return "vision"
	}
	return "sensor"
}

func mediaRefFromObservation(obs Observation) string {
	if modalityFromObservation(obs) == "audio" {
		if ref := strings.TrimSpace(obs.AudioRef); ref != "" {
			return ref
		}
	}
	if ref := strings.TrimSpace(obs.ImageRef); ref != "" {
		return ref
	}
	return strings.TrimSpace(obs.AudioRef)
}
