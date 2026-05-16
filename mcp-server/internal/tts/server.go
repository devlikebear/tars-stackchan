package tts

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// maxTextRunes mirrors the 240-codepoint cap in tts-remote-server.py.
const maxTextRunes = 240

// authorized mirrors is_authorized: the configured token must be non-empty and
// constant-time equal to the provided token.
func authorized(q url.Values, token string) bool {
	expected := strings.TrimSpace(token)
	provided := strings.TrimSpace(q.Get("token"))
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// redactPath mirrors redact_path: replace the token query value with
// "<redacted>" for safe logging.
func redactPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Has("token") {
		q.Set("token", "<redacted>")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// cacheKey mirrors generate_wav's hashing: sha256 over
// model\0voice\0sampleRate\0prompt, all normalized.
func cacheKey(model, voice string, sampleRate int, prompt string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		model, voice, strconv.Itoa(sampleRate), prompt,
	}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// generateWAV mirrors generate_wav: read-through cache, atomic temp+rename.
func generateWAV(ctx context.Context, client *http.Client, cfg Config, text string) (string, error) {
	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		return "", err
	}
	model := NormalizeModel(cfg.Model)
	voice := NormalizeVoice(cfg.Voice)
	prompt := text
	if cfg.PromptPrefix != "" {
		prompt = cfg.PromptPrefix + text
	}
	key := cacheKey(model, voice, cfg.SampleRate, prompt)
	wavPath := filepath.Join(cfg.CacheDir, key+".wav")
	if _, err := os.Stat(wavPath); err == nil {
		return wavPath, nil
	}

	synthCfg := cfg
	synthCfg.Model = model
	synthCfg.Voice = voice
	pcm, err := SynthesizePCM(ctx, client, synthCfg, prompt)
	if err != nil {
		return "", err
	}

	tmpPath := filepath.Join(cfg.CacheDir, "."+key+".tmp.wav")
	tmp, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	if err := WritePCMAsWAV(tmp, pcm, cfg.SampleRate); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	if err := os.Rename(tmpPath, wavPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return wavPath, nil
}

func logRequest(r *http.Request, code, size int) {
	fmt.Printf("%s - \"%s %s %s\" %d %d\n",
		r.RemoteAddr, r.Method, redactPath(r.URL.RequestURI()), r.Proto, code, size)
}

// NewHandler returns the relay HTTP handler. It mirrors the Handler.do_GET
// routing and status codes in tts-remote-server.py.
func NewHandler(cfg Config) http.Handler {
	client := &http.Client{Timeout: geminiTimeout}
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
		logRequest(r, http.StatusOK, 3)
	})

	mux.HandleFunc("/api/tts", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !authorized(q, cfg.Token) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("invalid token\n"))
			logRequest(r, http.StatusUnauthorized, 14)
			return
		}

		text := strings.TrimSpace(q.Get("text"))
		if text == "" {
			http.Error(w, "text is required", http.StatusBadRequest)
			logRequest(r, http.StatusBadRequest, 0)
			return
		}
		if utf8.RuneCountInString(text) > maxTextRunes {
			http.Error(w, "text must be 240 characters or fewer", http.StatusBadRequest)
			logRequest(r, http.StatusBadRequest, 0)
			return
		}

		wavPath, err := generateWAV(r.Context(), client, cfg, text)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logRequest(r, http.StatusInternalServerError, 0)
			return
		}
		payload, err := os.ReadFile(wavPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logRequest(r, http.StatusInternalServerError, 0)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
		logRequest(r, http.StatusOK, len(payload))
	})

	// Any other path: 404 "not found", matching the Python relay.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
		logRequest(r, http.StatusNotFound, 0)
	})

	return mux
}

// readinessTimeout bounds the graceful shutdown drain.
const readinessTimeout = 5 * time.Second
