// Package perception runs the background sensory loop: it polls the
// Stack-chan bridge for cheap trigger state, captures camera/audio on a
// trigger or on an idle interval, summarizes the moment, and posts a compact
// observation to TARS. It is isolated from the stdlib-only MCP/bridge core on
// purpose (see docs/plans/codebase-analysis.md §4).
package perception

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the resolved perception-loop settings. Every field has a safe
// default so `perceive` runs with zero env in mock mode.
type Config struct {
	SensorPollInterval   time.Duration // how often to GET /v1/sensors
	IdleSnapshotInterval time.Duration // capture at least this often with no trigger
	MinTriggerInterval   time.Duration // debounce between event-triggered captures
	MaxCapturesPerHour   int           // hard rate limit (LLM cost / noise guard)
	SoundLevelThreshold  float64       // /v1/sensors sound_level >= this fires
	SnapshotMaxWidth     int           // passed to CameraSnapshot
	AudioClipMs          int           // passed to AudioClip

	// CameraEnabled gates the camera-capture step. Default true. Set
	// TARS_STACKCHAN_PERCEIVE_CAMERA=off for real-HW audio-only operation
	// while the CoreS3 SCCB I2C-bus conflict (Spike S) is unresolved — the
	// camera capture would otherwise hard-reset the device.
	CameraEnabled bool

	CacheDir string // absolute dir for raw media (refs only leave the device)

	// Owner identification (Phase 3). OwnerDir holds the durable, local-only
	// fingerprint. Thresholds drive the 3-state owner/stranger/unknown
	// decision (per-modality "same as owner" probability 0..1).
	OwnerDir              string
	OwnerConfidenceMin    float64 // >= this => owner
	StrangerConfidenceMax float64 // <= this => stranger; in-between => unknown

	// TARS sink. When BaseURL or WebhookChannel is empty the loop uses a
	// log-only sink (still useful in mock/dev).
	TARSBaseURL        string
	TARSWebhookChannel string
	TARSToken          string

	// Multimodal summary (Gemini). Opt-in: enabled only when the API key is
	// present AND a model is explicitly chosen via
	// TARS_STACKCHAN_PERCEIVE_MODEL. Otherwise a deterministic offline stub
	// summarizer is used — this keeps the mock/Phase-2 path reproducible and
	// avoids accidentally spending the shared TTS GEMINI key on a model that
	// may not support multimodal generateContent.
	GeminiAPIKey   string
	GeminiModel    string
	GeminiEndpoint string
	GeminiExplicit bool // TARS_STACKCHAN_PERCEIVE_MODEL was set
}

// GeminiEnabled reports whether real multimodal summarization should be used.
func (c Config) GeminiEnabled() bool {
	return c.GeminiExplicit && strings.TrimSpace(c.GeminiAPIKey) != "" && strings.TrimSpace(c.GeminiModel) != ""
}

const (
	defaultSensorPoll     = 1 * time.Second
	defaultIdleSnapshot   = 30 * time.Second
	defaultMinTrigger     = 5 * time.Second
	defaultMaxPerHour     = 60
	defaultSoundThreshold = 0.2
	defaultSnapshotWidth  = 320
	defaultAudioClipMs    = 1500
	defaultGeminiModel    = "gemini-3.1-flash"
	defaultGeminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent"
	relCacheDirFallback   = ".work/perception-cache"
)

// DefaultCacheDir mirrors tts.DefaultCacheDir: an absolute, writable location
// so the loop works when launched as a background service (cwd may be "/").
func DefaultCacheDir() string {
	if base, err := os.UserCacheDir(); err == nil && base != "" {
		return filepath.Join(base, "tars-stackchan", "perception")
	}
	return relCacheDirFallback
}

// LoadConfig resolves Config from TARS_STACKCHAN_PERCEIVE_* (and reuses the
// shared GEMINI / TARS_STACKCHAN_GEMINI_API_KEY keys like the TTS relay).
func LoadConfig() Config {
	return Config{
		SensorPollInterval:    envDuration("TARS_STACKCHAN_PERCEIVE_SENSOR_POLL", defaultSensorPoll),
		IdleSnapshotInterval:  envDuration("TARS_STACKCHAN_PERCEIVE_IDLE_INTERVAL", defaultIdleSnapshot),
		MinTriggerInterval:    envDuration("TARS_STACKCHAN_PERCEIVE_MIN_TRIGGER_INTERVAL", defaultMinTrigger),
		MaxCapturesPerHour:    envInt("TARS_STACKCHAN_PERCEIVE_MAX_PER_HOUR", defaultMaxPerHour),
		SoundLevelThreshold:   envFloat("TARS_STACKCHAN_PERCEIVE_SOUND_THRESHOLD", defaultSoundThreshold),
		SnapshotMaxWidth:      envInt("TARS_STACKCHAN_PERCEIVE_SNAPSHOT_MAX_WIDTH", defaultSnapshotWidth),
		AudioClipMs:           envInt("TARS_STACKCHAN_PERCEIVE_AUDIO_MS", defaultAudioClipMs),
		CameraEnabled:         !envDisabled("TARS_STACKCHAN_PERCEIVE_CAMERA"),
		CacheDir:              firstNonEmpty(os.Getenv("TARS_STACKCHAN_PERCEIVE_CACHE_DIR"), DefaultCacheDir()),
		OwnerDir:              firstNonEmpty(os.Getenv("TARS_STACKCHAN_PERCEIVE_OWNER_DIR"), DefaultOwnerDir()),
		OwnerConfidenceMin:    envFloat("TARS_STACKCHAN_PERCEIVE_OWNER_MIN", 0.6),
		StrangerConfidenceMax: envFloat("TARS_STACKCHAN_PERCEIVE_STRANGER_MAX", 0.4),
		TARSBaseURL:           strings.TrimRight(strings.TrimSpace(os.Getenv("TARS_STACKCHAN_TARS_BASE_URL")), "/"),
		TARSWebhookChannel:    strings.TrimSpace(os.Getenv("TARS_STACKCHAN_TARS_WEBHOOK_CHANNEL")),
		TARSToken:             firstNonEmpty(os.Getenv("TARS_STACKCHAN_TARS_TOKEN"), os.Getenv("TARS_STACKCHAN_TOKEN")),
		GeminiAPIKey:          firstNonEmpty(os.Getenv("TARS_STACKCHAN_GEMINI_API_KEY"), os.Getenv("GEMINI_API_KEY")),
		GeminiModel:           firstNonEmpty(os.Getenv("TARS_STACKCHAN_PERCEIVE_MODEL"), defaultGeminiModel),
		GeminiEndpoint:        firstNonEmpty(os.Getenv("TARS_STACKCHAN_PERCEIVE_ENDPOINT"), defaultGeminiEndpoint),
		GeminiExplicit:        strings.TrimSpace(os.Getenv("TARS_STACKCHAN_PERCEIVE_MODEL")) != "",
	}
}

// TARSConfigured reports whether observations can be POSTed to TARS.
func (c Config) TARSConfigured() bool {
	return c.TARSBaseURL != "" && c.TARSWebhookChannel != ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// envDisabled reports whether key is explicitly set to a falsy value
// (off/0/false/no/disable). Unset => not disabled.
func envDisabled(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "off", "0", "false", "no", "disable", "disabled":
		return true
	default:
		return false
	}
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return def
}
