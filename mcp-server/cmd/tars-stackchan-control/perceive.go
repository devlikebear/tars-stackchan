package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/perception"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

// stderrLogger adapts an io.Writer to perception.Logger.
type stderrLogger struct{ w io.Writer }

func (l stderrLogger) Printf(format string, args ...any) {
	fmt.Fprintf(l.w, format+"\n", args...)
}

func runPerceive(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		perceiveUsage(stderr)
		return 2
	}
	switch args[0] {
	case "serve":
		return runPerceiveServe(stderr)
	case "enroll":
		return runPerceiveEnroll(args[1:], stderr)
	default:
		perceiveUsage(stderr)
		return 2
	}
}

func perceiveUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  tars-stackchan-control perceive serve              run the background sensory loop")
	fmt.Fprintln(w, "  tars-stackchan-control perceive enroll [--name N] [--faces K] [--voices K]")
	fmt.Fprintln(w, "  tars-stackchan-control perceive enroll --reset     remove the stored owner fingerprint")
}

func buildPerceptionBridge(stderr io.Writer) (stackchan.PerceptionBridge, bool) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("TARS_STACKCHAN_BRIDGE")))
	if mode == "" {
		mode = "http"
	}
	baseURL := strings.TrimSpace(os.Getenv("TARS_STACKCHAN_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	bridge, err := newBridge(mode, baseURL, os.Getenv("TARS_STACKCHAN_TOKEN"))
	if err != nil {
		fmt.Fprintf(stderr, "perceive: %v\n", err)
		return nil, false
	}
	pb, ok := bridge.(stackchan.PerceptionBridge)
	if !ok {
		fmt.Fprintf(stderr, "perceive: bridge %q does not support perception capture\n", mode)
		return nil, false
	}
	return pb, true
}

func runPerceiveServe(stderr io.Writer) int {
	pb, ok := buildPerceptionBridge(stderr)
	if !ok {
		return 1
	}
	cfg := perception.LoadConfig()
	log := stderrLogger{w: stderr}

	var summarizer perception.Summarizer = perception.StubSummarizer{}
	if cfg.GeminiEnabled() {
		summarizer = perception.GeminiSummarizer{APIKey: cfg.GeminiAPIKey, Model: cfg.GeminiModel, Endpoint: cfg.GeminiEndpoint}
		log.Printf("[perceive] multimodal summarizer: gemini model=%s", cfg.GeminiModel)
	} else {
		log.Printf("[perceive] offline stub summarizer (set TARS_STACKCHAN_PERCEIVE_MODEL + GEMINI key to enable multimodal)")
	}

	profile, err := perception.OwnerStore{Dir: cfg.OwnerDir}.Load()
	if err != nil {
		fmt.Fprintf(stderr, "perceive: load owner profile: %v\n", err)
		return 1
	}
	if profile.Enrolled() {
		log.Printf("[perceive] owner profile loaded: name=%q faces=%d voices=%d",
			profile.Name, len(profile.FaceHashes), len(profile.VoiceHashes))
	} else {
		log.Printf("[perceive] no owner enrolled; everyone is 'unknown' (run: perceive enroll)")
	}
	// Stub identifier is deterministic/offline. Gemini-based identification
	// is an opt-in follow-up gated on Spike S (real capture); the mock path
	// must stay reproducible (same rationale as the Phase 2 summarizer).
	identifier := perception.StubIdentifier{Profile: profile, Config: cfg}

	var sink perception.Sink = perception.LogSink{Log: log}
	if cfg.TARSConfigured() {
		sink = perception.TARSWebhookSink{BaseURL: cfg.TARSBaseURL, Channel: cfg.TARSWebhookChannel, Token: cfg.TARSToken, Log: log}
	} else {
		log.Printf("[perceive] TARS not configured (set TARS_STACKCHAN_TARS_BASE_URL + _WEBHOOK_CHANNEL); using log sink")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := perception.Run(ctx, perception.Deps{
		Bridge:     pb,
		Summarizer: summarizer,
		Identifier: identifier,
		Sink:       sink,
		Config:     cfg,
		Log:        log,
	}); err != nil {
		fmt.Fprintf(stderr, "perceive: %v\n", err)
		return 1
	}
	return 0
}

func runPerceiveEnroll(args []string, stderr io.Writer) int {
	name := "owner"
	faces, voices := 3, 2
	reset := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--reset":
			reset = true
		case "--name":
			if i+1 < len(args) {
				i++
				name = args[i]
			}
		case "--faces":
			if i+1 < len(args) {
				i++
				faces = atoiOr(args[i], faces)
			}
		case "--voices":
			if i+1 < len(args) {
				i++
				voices = atoiOr(args[i], voices)
			}
		default:
			fmt.Fprintf(stderr, "perceive enroll: unknown arg %q\n", args[i])
			return 2
		}
	}

	cfg := perception.LoadConfig()
	store := perception.OwnerStore{Dir: cfg.OwnerDir}

	if reset {
		if err := store.Reset(); err != nil {
			fmt.Fprintf(stderr, "perceive enroll: reset failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "owner fingerprint removed (%s)\n", cfg.OwnerDir)
		return 0
	}

	pb, ok := buildPerceptionBridge(stderr)
	if !ok {
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(stderr, "Enrolling owner %q: %d face + %d voice samples. Stay in view of the camera.\n", name, faces, voices)
	var snaps []stackchan.CameraSnapshot
	for i := 0; i < faces; i++ {
		fmt.Fprintf(stderr, "  face %d/%d...\n", i+1, faces)
		s, err := pb.CameraSnapshot(ctx, stackchan.SnapshotOptions{MaxWidth: cfg.SnapshotMaxWidth})
		if err != nil {
			fmt.Fprintf(stderr, "perceive enroll: camera snapshot failed: %v\n", err)
			return 1
		}
		snaps = append(snaps, s)
		sleepCtx(ctx, 700*time.Millisecond)
	}
	var clips []stackchan.AudioClip
	for i := 0; i < voices; i++ {
		fmt.Fprintf(stderr, "  voice %d/%d (speak now)...\n", i+1, voices)
		c, err := pb.AudioClip(ctx, stackchan.AudioOptions{DurationMs: cfg.AudioClipMs})
		if err != nil {
			fmt.Fprintf(stderr, "perceive enroll: audio clip failed: %v\n", err)
			return 1
		}
		clips = append(clips, c)
		sleepCtx(ctx, 300*time.Millisecond)
	}

	profile, err := store.Enroll(name, snaps, clips)
	if err != nil {
		fmt.Fprintf(stderr, "perceive enroll: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "enrolled %q: %d face + %d voice references stored at %s (local-only, 0600)\n",
		profile.Name, len(profile.FaceHashes), len(profile.VoiceHashes), cfg.OwnerDir)
	return 0
}

func atoiOr(s string, def int) int {
	n := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err == nil && n > 0 {
		return n
	}
	return def
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
