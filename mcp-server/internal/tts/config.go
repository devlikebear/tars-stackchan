package tts

import (
	"os"
	"strings"
)

// Config holds the resolved relay settings. It mirrors the Handler class
// attributes plus argparse defaults in tts-remote-server.py.
type Config struct {
	APIKey           string
	Token            string
	Model            string
	Voice            string
	EndpointTemplate string
	SampleRate       int
	PromptPrefix     string
	CacheDir         string
}

// DefaultCacheDir mirrors the argparse default for --cache-dir.
const DefaultCacheDir = ".work/tts-cache"

// DefaultSampleRate mirrors the argparse default for --sample-rate.
const DefaultSampleRate = 24000

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// ResolveAPIKey mirrors resolve_api_key: explicit, then
// TARS_STACKCHAN_GEMINI_API_KEY, then GEMINI_API_KEY.
func ResolveAPIKey(explicit string) string {
	return firstNonEmpty(
		explicit,
		os.Getenv("TARS_STACKCHAN_GEMINI_API_KEY"),
		os.Getenv("GEMINI_API_KEY"),
	)
}

// ResolveToken mirrors resolve_tts_token: explicit, then
// TARS_STACKCHAN_TTS_TOKEN, then TARS_STACKCHAN_TOKEN.
func ResolveToken(explicit string) string {
	return firstNonEmpty(
		explicit,
		os.Getenv("TARS_STACKCHAN_TTS_TOKEN"),
		os.Getenv("TARS_STACKCHAN_TOKEN"),
	)
}

// Flags carries raw command-line values before env fallback is applied.
type Flags struct {
	APIKey           string
	Token            string
	Model            string
	Voice            string
	EndpointTemplate string
	SampleRate       int
	PromptPrefix     string
	CacheDir         string
}

// Resolve folds flags and environment defaults into a Config, matching the
// precedence in tts-remote-server.py main(): flag value wins, otherwise the
// matching env var, otherwise the documented default.
func Resolve(f Flags) Config {
	model := f.Model
	if strings.TrimSpace(model) == "" {
		model = os.Getenv("TARS_STACKCHAN_TTS_MODEL")
	}

	voice := f.Voice
	if strings.TrimSpace(voice) == "" {
		voice = os.Getenv("TARS_STACKCHAN_TTS_VOICE")
	}
	if strings.TrimSpace(voice) == "" {
		voice = DefaultGeminiVoice
	}

	endpoint := f.EndpointTemplate
	if strings.TrimSpace(endpoint) == "" {
		endpoint = os.Getenv("TARS_STACKCHAN_GEMINI_ENDPOINT")
	}
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultGeminiEndpointTemplate
	}

	promptPrefix := f.PromptPrefix
	if promptPrefix == "" {
		promptPrefix = os.Getenv("TARS_STACKCHAN_TTS_PROMPT_PREFIX")
	}

	cacheDir := firstNonEmpty(f.CacheDir, os.Getenv("TARS_STACKCHAN_TTS_CACHE"), DefaultCacheDir)

	sampleRate := f.SampleRate
	if sampleRate <= 0 {
		sampleRate = DefaultSampleRate
	}

	return Config{
		APIKey:           ResolveAPIKey(f.APIKey),
		Token:            ResolveToken(f.Token),
		Model:            NormalizeModel(model),
		Voice:            voice,
		EndpointTemplate: endpoint,
		SampleRate:       sampleRate,
		PromptPrefix:     promptPrefix,
		CacheDir:         cacheDir,
	}
}
