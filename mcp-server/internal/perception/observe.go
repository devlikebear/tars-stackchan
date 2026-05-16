package perception

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

// Observation is the compact, privacy-conscious record that leaves the
// device. Raw media stays local; only refs (paths) + a natural-language
// summary are sent to TARS (codebase-analysis §5).
type Observation struct {
	TS       int64    `json:"ts"`
	Trigger  string   `json:"trigger"` // "event" | "idle"
	Summary  string   `json:"summary"`
	Salience float64  `json:"salience"` // 0..1, summarizer's "how notable"
	Identity Identity `json:"identity"`
	ImageRef string   `json:"image_ref,omitempty"`
	AudioRef string   `json:"audio_ref,omitempty"`
}

// Summarizer turns a captured moment into a short natural-language summary
// and a salience score. Abstracted so tests/offline use a deterministic stub.
type Summarizer interface {
	Summarize(ctx context.Context, snap stackchan.CameraSnapshot, clip stackchan.AudioClip, trigger string) (summary string, salience float64, err error)
}

// StubSummarizer is deterministic and offline. Used when no Gemini key is
// configured and in tests.
type StubSummarizer struct{}

func (StubSummarizer) Summarize(_ context.Context, snap stackchan.CameraSnapshot, clip stackchan.AudioClip, trigger string) (string, float64, error) {
	salience := 0.3
	if trigger == "event" {
		salience = 0.6
	}
	return fmt.Sprintf("(%s) captured %d-byte image and %dms audio; no scene model available offline.",
		trigger, len(snap.Data), clip.DurationMs), salience, nil
}

const geminiTimeout = 30 * time.Second

// GeminiSummarizer calls Gemini multimodal generateContent with the JPEG +
// WAV inline, mirroring internal/tts/gemini.go's REST/key conventions.
type GeminiSummarizer struct {
	APIKey   string
	Model    string
	Endpoint string // template containing {model}
	Client   *http.Client
}

func (g GeminiSummarizer) Summarize(ctx context.Context, snap stackchan.CameraSnapshot, clip stackchan.AudioClip, trigger string) (string, float64, error) {
	if strings.TrimSpace(g.APIKey) == "" {
		return "", 0, errors.New("GEMINI_API_KEY or TARS_STACKCHAN_GEMINI_API_KEY is required")
	}
	endpoint := strings.ReplaceAll(g.Endpoint, "{model}", url.QueryEscape(g.Model))

	parts := []map[string]any{{"text": summaryPrompt(trigger)}}
	if len(snap.Data) > 0 {
		parts = append(parts, inlinePart(orDefault(snap.ContentType, "image/jpeg"), snap.Data))
	}
	if len(clip.Data) > 0 {
		parts = append(parts, inlinePart(orDefault(clip.ContentType, "audio/wav"), clip.Data))
	}
	payload, err := json.Marshal(map[string]any{
		"contents": []map[string]any{{"role": "user", "parts": parts}},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"temperature":      0.4,
		},
	})
	if err != nil {
		return "", 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.APIKey)

	client := g.Client
	if client == nil {
		client = &http.Client{Timeout: geminiTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("gemini summarize returned %d", resp.StatusCode)
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", 0, fmt.Errorf("decode gemini response: %w", err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", 0, errors.New("gemini returned no content")
	}

	var out struct {
		Summary  string  `json:"summary"`
		Salience float64 `json:"salience"`
	}
	raw := strings.TrimSpace(parsed.Candidates[0].Content.Parts[0].Text)
	if err := json.Unmarshal([]byte(raw), &out); err != nil || strings.TrimSpace(out.Summary) == "" {
		// Model didn't honor JSON; fall back to the raw text as the summary.
		return raw, 0.5, nil
	}
	return out.Summary, clamp01(out.Salience), nil
}

func summaryPrompt(trigger string) string {
	return "You are the sensory cortex of a small desk robot. Given a camera " +
		"frame and a short audio clip, reply ONLY with compact JSON " +
		`{"summary": string, "salience": number}. "summary" is one or two ` +
		"plain sentences describing who/what is present and notable sounds " +
		"(no preamble). \"salience\" is 0..1 for how much this warrants the " +
		"robot's attention. Trigger reason: " + trigger + "."
}

func inlinePart(mime string, data []byte) map[string]any {
	return map[string]any{"inline_data": map[string]any{
		"mime_type": mime,
		"data":      base64.StdEncoding.EncodeToString(data),
	}}
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// cacheMedia writes the raw snapshot/clip under cacheDir and returns relative
// refs. Raw bytes never leave the device; only these refs travel in the
// Observation. A write failure is non-fatal (ref left empty).
func cacheMedia(cacheDir string, ts int64, snap stackchan.CameraSnapshot, clip stackchan.AudioClip) (imageRef, audioRef string) {
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", ""
	}
	if len(snap.Data) > 0 {
		name := fmt.Sprintf("obs-%d.jpg", ts)
		if os.WriteFile(filepath.Join(cacheDir, name), snap.Data, 0o600) == nil {
			imageRef = name
		}
	}
	if len(clip.Data) > 0 {
		name := fmt.Sprintf("obs-%d.wav", ts)
		if os.WriteFile(filepath.Join(cacheDir, name), clip.Data, 0o600) == nil {
			audioRef = name
		}
	}
	return imageRef, audioRef
}
