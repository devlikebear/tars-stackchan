package hostbody

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

type Percept struct {
	Provider   string
	Modality   string
	Owner      string
	Summary    string
	MediaRef   string
	ImageRef   string
	AudioRef   string
	Trigger    string
	Salience   float64
	SessionID  string
	CapturedAt time.Time
}

type Sink interface {
	Post(context.Context, Percept) error
}

type TARSSink struct {
	BaseURL  string
	Provider string
	Token    string
	Client   *http.Client
}

func (s TARSSink) Post(ctx context.Context, percept Percept) error {
	provider := firstNonEmpty(percept.Provider, s.Provider, DefaultProviderName)
	body, err := json.Marshal(map[string]any{
		"x-embodiment": true,
		"source":       provider,
		"provider":     provider,
		"summary":      percept.Summary,
		"text":         percept.Summary,
		"owner":        firstNonEmpty(percept.Owner, "unknown"),
		"modality":     firstNonEmpty(percept.Modality, "audio"),
		"media_ref":    percept.MediaRef,
		"image_ref":    percept.ImageRef,
		"audio_ref":    percept.AudioRef,
		"trigger":      firstNonEmpty(percept.Trigger, "idle"),
		"salience":     percept.Salience,
		"session_id":   percept.SessionID,
		"captured_at":  percept.CapturedAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/v1/embodiment/percept/%s", strings.TrimRight(s.BaseURL, "/"), provider)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(s.Token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("tars embodiment endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

type LogSink struct {
	Log Logger
}

func (s LogSink) Post(_ context.Context, p Percept) error {
	if s.Log != nil {
		s.Log.Printf("[hostbody] percept provider=%s modality=%s owner=%s summary=%q", p.Provider, p.Modality, p.Owner, p.Summary)
	}
	return nil
}

type Logger interface {
	Printf(format string, args ...any)
}
