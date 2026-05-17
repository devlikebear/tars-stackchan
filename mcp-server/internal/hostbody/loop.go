package hostbody

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bodyprovider"
)

type Summarizer interface {
	Summarize(context.Context, bodyprovider.CameraSnapshot, bodyprovider.AudioClip, string) (summary string, salience float64, err error)
}

type CaptureDeps struct {
	Capturer   Capturer
	Sink       Sink
	Summarizer Summarizer
	Config     Config
	Log        Logger
	Now        func() time.Time
}

func CaptureOnce(ctx context.Context, deps CaptureDeps, trigger string) error {
	if deps.Sink == nil {
		return fmt.Errorf("hostbody sink is required")
	}
	if deps.Summarizer == nil {
		deps.Summarizer = StubSummarizer{}
	}
	now := time.Now().UTC()
	if deps.Now != nil {
		now = deps.Now().UTC()
	}
	trigger = firstNonEmpty(trigger, "idle")

	var snap bodyprovider.CameraSnapshot
	var imageRef string
	if deps.Config.CameraEnabled && deps.Capturer.Tools.HasCamera() {
		captured, err := deps.Capturer.CaptureCamera(ctx)
		if err != nil {
			logf(deps.Log, "[hostbody] camera capture skipped: %v", err)
		} else {
			snap = captured
			imageRef = writeMedia(deps.Config.CacheDir, now, "jpg", snap.Data)
		}
	}

	var clip bodyprovider.AudioClip
	var audioRef string
	if deps.Capturer.Tools.HasAudio() {
		captured, err := deps.Capturer.CaptureAudio(ctx, deps.Config.AudioClip)
		if err != nil {
			return err
		}
		clip = captured
		audioRef = writeMedia(deps.Config.CacheDir, now, "wav", clip.Data)
	}
	if len(clip.Data) == 0 && len(snap.Data) == 0 {
		return fmt.Errorf("hostbody has no captured media")
	}

	summary, salience, err := deps.Summarizer.Summarize(ctx, snap, clip, trigger)
	if err != nil {
		return err
	}
	modality := "audio"
	mediaRef := audioRef
	if audioRef == "" && imageRef != "" {
		modality = "vision"
		mediaRef = imageRef
	}
	return deps.Sink.Post(ctx, Percept{
		Provider:   firstNonEmpty(deps.Config.ProviderName, DefaultProviderName),
		Owner:      firstNonEmpty(deps.Config.Owner, "unknown"),
		SessionID:  deps.Config.SessionID,
		Modality:   modality,
		Summary:    summary,
		Salience:   salience,
		Trigger:    trigger,
		MediaRef:   mediaRef,
		ImageRef:   imageRef,
		AudioRef:   audioRef,
		CapturedAt: now,
	})
}

func Run(ctx context.Context, deps CaptureDeps) error {
	interval := deps.Config.IdleInterval
	if interval <= 0 {
		interval = defaultIdleInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	logf(deps.Log, "[hostbody] loop started provider=%s interval=%s", firstNonEmpty(deps.Config.ProviderName, DefaultProviderName), interval)
	for {
		select {
		case <-ctx.Done():
			logf(deps.Log, "[hostbody] loop stopped: %v", ctx.Err())
			return nil
		case <-ticker.C:
			if err := CaptureOnce(ctx, deps, "idle"); err != nil {
				logf(deps.Log, "[hostbody] capture skipped: %v", err)
			}
		}
	}
}

type StubSummarizer struct{}

func (StubSummarizer) Summarize(_ context.Context, snap bodyprovider.CameraSnapshot, clip bodyprovider.AudioClip, trigger string) (string, float64, error) {
	parts := []string{"Host percept captured"}
	if len(clip.Data) > 0 {
		parts = append(parts, "audio")
	}
	if len(snap.Data) > 0 {
		parts = append(parts, "vision")
	}
	return strings.Join(parts, " + ") + " (" + trigger + ").", 0.7, nil
}

func writeMedia(cacheDir string, ts time.Time, ext string, data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if strings.TrimSpace(cacheDir) == "" {
		cacheDir = DefaultCacheDir()
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return ""
	}
	name := fmt.Sprintf("host-%d.%s", ts.UnixMilli(), ext)
	path := filepath.Join(cacheDir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return ""
	}
	return path
}

func logf(log Logger, format string, args ...any) {
	if log != nil {
		log.Printf(format, args...)
	}
}
