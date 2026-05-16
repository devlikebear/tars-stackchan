package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// geminiTimeout mirrors GEMINI_TIMEOUT_SECONDS in tts-remote-server.py.
const geminiTimeout = 30 * time.Second

// RenderEndpoint mirrors render_endpoint: substitute the URL-encoded model into
// a {model} or %s template, otherwise return the template unchanged.
func RenderEndpoint(endpointTemplate, model string) string {
	encoded := url.QueryEscape(model)
	if strings.Contains(endpointTemplate, "{model}") {
		return strings.ReplaceAll(endpointTemplate, "{model}", encoded)
	}
	if strings.Contains(endpointTemplate, "%s") {
		return strings.Replace(endpointTemplate, "%s", encoded, 1)
	}
	return endpointTemplate
}

// BuildPayload mirrors build_gemini_payload.
func BuildPayload(text, voice string) ([]byte, error) {
	payload := map[string]any{
		"contents": []any{
			map[string]any{"parts": []any{map[string]any{"text": text}}},
		},
		"generationConfig": map[string]any{
			"responseModalities": []any{"AUDIO"},
			"speechConfig": map[string]any{
				"voiceConfig": map[string]any{
					"prebuiltVoiceConfig": map[string]any{
						"voiceName": voice,
					},
				},
			},
		},
	}
	return json.Marshal(payload)
}

type geminiInlineData struct {
	Data string `json:"data"`
}

type geminiResponse struct {
	Error *struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
	Candidates []struct {
		Content struct {
			Parts []struct {
				InlineDataCamel *geminiInlineData `json:"inlineData"`
				InlineDataSnake *geminiInlineData `json:"inline_data"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// SynthesizePCM mirrors synthesize_gemini_pcm: POST to the Gemini endpoint and
// return decoded PCM bytes. Error strings match the Python relay so existing
// operator runbooks and the contract tests stay valid.
func SynthesizePCM(ctx context.Context, client *http.Client, cfg Config, text string) ([]byte, error) {
	apiKey := ResolveAPIKey(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY or TARS_STACKCHAN_GEMINI_API_KEY is required")
	}

	model := NormalizeModel(cfg.Model)
	voice := NormalizeVoice(cfg.Voice)
	endpoint := RenderEndpoint(cfg.EndpointTemplate, model)

	payload, err := BuildPayload(text, voice)
	if err != nil {
		return nil, fmt.Errorf("gemini payload encode failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	if client == nil {
		client = &http.Client{Timeout: geminiTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, errors.New("gemini response was not valid JSON")
	}
	if parsed.Error != nil {
		status := parsed.Error.Status
		if status == "" {
			status = "UNKNOWN"
		}
		return nil, fmt.Errorf("gemini api error %s: %s", status, parsed.Error.Message)
	}

	var encoded string
	if len(parsed.Candidates) > 0 {
		for _, part := range parsed.Candidates[0].Content.Parts {
			inline := part.InlineDataCamel
			if inline == nil {
				inline = part.InlineDataSnake
			}
			if inline != nil && inline.Data != "" {
				encoded = inline.Data
				break
			}
		}
	}
	if encoded == "" {
		return nil, errors.New("gemini response did not include inline audio data")
	}

	pcm, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("gemini inline audio data was not valid base64")
	}
	if len(pcm) == 0 {
		return nil, errors.New("gemini inline audio data decoded to an empty payload")
	}
	return pcm, nil
}
