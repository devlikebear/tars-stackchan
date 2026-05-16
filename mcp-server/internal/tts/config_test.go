package tts

import (
	"path/filepath"
	"testing"
)

func TestResolveAPIKeyPrecedence(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_GEMINI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	if got := ResolveAPIKey("explicit"); got != "explicit" {
		t.Fatalf("explicit should win, got %q", got)
	}

	t.Setenv("TARS_STACKCHAN_GEMINI_API_KEY", "tars-key")
	t.Setenv("GEMINI_API_KEY", "generic-key")
	if got := ResolveAPIKey(""); got != "tars-key" {
		t.Fatalf("TARS_STACKCHAN_GEMINI_API_KEY should win over GEMINI_API_KEY, got %q", got)
	}

	t.Setenv("TARS_STACKCHAN_GEMINI_API_KEY", "")
	if got := ResolveAPIKey(""); got != "generic-key" {
		t.Fatalf("GEMINI_API_KEY fallback expected, got %q", got)
	}
}

func TestResolveTokenPrecedence(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_TTS_TOKEN", "tts-token")
	t.Setenv("TARS_STACKCHAN_TOKEN", "device-token")
	if got := ResolveToken(""); got != "tts-token" {
		t.Fatalf("TARS_STACKCHAN_TTS_TOKEN should win, got %q", got)
	}
	t.Setenv("TARS_STACKCHAN_TTS_TOKEN", "  ")
	if got := ResolveToken(""); got != "device-token" {
		t.Fatalf("TARS_STACKCHAN_TOKEN fallback expected, got %q", got)
	}
	if got := ResolveToken("flag"); got != "flag" {
		t.Fatalf("explicit token should win, got %q", got)
	}
}

func TestResolveDefaults(t *testing.T) {
	for _, k := range []string{
		"TARS_STACKCHAN_TTS_MODEL", "TARS_STACKCHAN_TTS_VOICE",
		"TARS_STACKCHAN_GEMINI_ENDPOINT", "TARS_STACKCHAN_TTS_PROMPT_PREFIX",
		"TARS_STACKCHAN_TTS_CACHE", "TARS_STACKCHAN_GEMINI_API_KEY", "GEMINI_API_KEY",
		"TARS_STACKCHAN_TTS_TOKEN", "TARS_STACKCHAN_TOKEN",
	} {
		t.Setenv(k, "")
	}

	cfg := Resolve(Flags{})
	if cfg.Model != DefaultGeminiModel {
		t.Fatalf("Model = %q, want default", cfg.Model)
	}
	if cfg.Voice != DefaultGeminiVoice {
		t.Fatalf("Voice = %q, want default", cfg.Voice)
	}
	if cfg.EndpointTemplate != DefaultGeminiEndpointTemplate {
		t.Fatalf("EndpointTemplate = %q, want default", cfg.EndpointTemplate)
	}
	if cfg.SampleRate != DefaultSampleRate {
		t.Fatalf("SampleRate = %d, want %d", cfg.SampleRate, DefaultSampleRate)
	}
	if cfg.CacheDir != DefaultCacheDir() {
		t.Fatalf("CacheDir = %q, want %q", cfg.CacheDir, DefaultCacheDir())
	}
	if !filepath.IsAbs(cfg.CacheDir) {
		t.Fatalf("default CacheDir %q must be absolute so the relay works as a launchd/brew service (CWD=/ read-only)", cfg.CacheDir)
	}
}

func TestResolveEnvAndFlagOverride(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_TTS_MODEL", "env-model")
	t.Setenv("TARS_STACKCHAN_TTS_CACHE", "/env/cache")
	cfg := Resolve(Flags{})
	if cfg.Model != "env-model" {
		t.Fatalf("env model expected, got %q", cfg.Model)
	}
	if cfg.CacheDir != "/env/cache" {
		t.Fatalf("env cache dir expected, got %q", cfg.CacheDir)
	}

	cfg = Resolve(Flags{Model: "flag-model", CacheDir: "/flag/cache", SampleRate: 16000})
	if cfg.Model != "flag-model" {
		t.Fatalf("flag model should win, got %q", cfg.Model)
	}
	if cfg.CacheDir != "/flag/cache" {
		t.Fatalf("flag cache dir should win, got %q", cfg.CacheDir)
	}
	if cfg.SampleRate != 16000 {
		t.Fatalf("flag sample rate should win, got %d", cfg.SampleRate)
	}
}
