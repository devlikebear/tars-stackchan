package hostbody

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultProviderName = "host"
	defaultAudioClip    = 1500 * time.Millisecond
	defaultIdleInterval = 30 * time.Second
)

type Config struct {
	ProviderName  string
	TARSBaseURL   string
	TARSToken     string
	SessionID     string
	Owner         string
	CameraEnabled bool
	AudioClip     time.Duration
	IdleInterval  time.Duration
	CacheDir      string
	TTSBaseURL    string
	TTSToken      string
	SayVoice      string
}

func LoadConfig() Config {
	return Config{
		ProviderName:  firstNonEmpty(os.Getenv("TARS_STACKCHAN_HOST_PROVIDER"), DefaultProviderName),
		TARSBaseURL:   strings.TrimRight(firstNonEmpty(os.Getenv("TARS_STACKCHAN_HOST_TARS_BASE_URL"), os.Getenv("TARS_STACKCHAN_TARS_BASE_URL")), "/"),
		TARSToken:     firstNonEmpty(os.Getenv("TARS_STACKCHAN_HOST_TARS_TOKEN"), os.Getenv("TARS_STACKCHAN_TARS_TOKEN"), os.Getenv("TARS_STACKCHAN_TOKEN")),
		SessionID:     strings.TrimSpace(os.Getenv("TARS_STACKCHAN_HOST_SESSION_ID")),
		Owner:         firstNonEmpty(os.Getenv("TARS_STACKCHAN_HOST_OWNER"), "unknown"),
		CameraEnabled: !envDisabled("TARS_STACKCHAN_HOST_CAMERA"),
		AudioClip:     envDuration("TARS_STACKCHAN_HOST_AUDIO_MS", defaultAudioClip),
		IdleInterval:  envDuration("TARS_STACKCHAN_HOST_IDLE_INTERVAL", defaultIdleInterval),
		CacheDir:      firstNonEmpty(os.Getenv("TARS_STACKCHAN_HOST_CACHE_DIR"), DefaultCacheDir()),
		TTSBaseURL:    strings.TrimRight(strings.TrimSpace(os.Getenv("TARS_STACKCHAN_HOST_TTS_BASE_URL")), "/"),
		TTSToken:      firstNonEmpty(os.Getenv("TARS_STACKCHAN_HOST_TTS_TOKEN"), os.Getenv("TARS_STACKCHAN_TTS_TOKEN"), os.Getenv("TARS_STACKCHAN_TOKEN")),
		SayVoice:      strings.TrimSpace(os.Getenv("TARS_STACKCHAN_HOST_SAY_VOICE")),
	}
}

func DefaultCacheDir() string {
	if base, err := os.UserCacheDir(); err == nil && base != "" {
		return filepath.Join(base, "tars-stackchan", "hostbody")
	}
	return ".work/hostbody-cache"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func envDisabled(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "off", "0", "false", "no", "disable", "disabled":
		return true
	default:
		return false
	}
}

func envDuration(key string, def time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}
	if strings.HasSuffix(value, "ms") || strings.HasSuffix(value, "s") || strings.HasSuffix(value, "m") {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		return time.Duration(n) * time.Millisecond
	}
	return def
}
