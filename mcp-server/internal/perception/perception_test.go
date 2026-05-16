package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

func baseCfg() Config {
	return Config{
		SensorPollInterval:   time.Second,
		IdleSnapshotInterval: 30 * time.Second,
		MinTriggerInterval:   5 * time.Second,
		MaxCapturesPerHour:   60,
		SoundLevelThreshold:  0.2,
		SnapshotMaxWidth:     320,
		AudioClipMs:          1500,
		CameraEnabled:        true,
	}
}

func TestDecideTrigger(t *testing.T) {
	t0 := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	cfg := baseCfg()

	tests := []struct {
		name       string
		state      triggerState
		sensor     stackchan.SensorState
		now        time.Time
		wantFire   bool
		wantReason string
	}{
		{
			name:       "first tick idle fires (zero state)",
			sensor:     stackchan.SensorState{},
			now:        t0,
			wantFire:   true,
			wantReason: "idle",
		},
		{
			name:       "motion fires event",
			state:      triggerState{lastTrigger: t0.Add(-10 * time.Second), lastIdle: t0},
			sensor:     stackchan.SensorState{Motion: true},
			now:        t0.Add(10 * time.Second),
			wantFire:   true,
			wantReason: "event",
		},
		{
			name:       "sound at threshold fires event",
			state:      triggerState{lastTrigger: t0.Add(-10 * time.Second), lastIdle: t0},
			sensor:     stackchan.SensorState{SoundLevel: 0.2},
			now:        t0.Add(10 * time.Second),
			wantFire:   true,
			wantReason: "event",
		},
		{
			name:       "event debounced within MinTriggerInterval, idle not due -> no fire",
			state:      triggerState{lastTrigger: t0, lastIdle: t0},
			sensor:     stackchan.SensorState{Motion: true},
			now:        t0.Add(2 * time.Second),
			wantFire:   false,
			wantReason: "",
		},
		{
			name:       "quiet but idle interval elapsed -> idle",
			state:      triggerState{lastTrigger: t0, lastIdle: t0},
			sensor:     stackchan.SensorState{SoundLevel: 0.05},
			now:        t0.Add(31 * time.Second),
			wantFire:   true,
			wantReason: "idle",
		},
		{
			name:       "quiet and idle not elapsed -> no fire",
			state:      triggerState{lastTrigger: t0, lastIdle: t0},
			sensor:     stackchan.SensorState{},
			now:        t0.Add(5 * time.Second),
			wantFire:   false,
			wantReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := tt.state
			fire, reason := decideTrigger(cfg, &st, tt.sensor, tt.now)
			if fire != tt.wantFire || reason != tt.wantReason {
				t.Fatalf("decideTrigger = (%v,%q), want (%v,%q)", fire, reason, tt.wantFire, tt.wantReason)
			}
		})
	}
}

func TestRateLimitBlocksAndTrims(t *testing.T) {
	t0 := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	cfg := baseCfg()
	cfg.MaxCapturesPerHour = 2
	st := &triggerState{}

	// Two captures fill the hourly budget.
	st.record(t0)
	st.record(t0.Add(time.Minute))
	if fire, _ := decideTrigger(cfg, st, stackchan.SensorState{Motion: true}, t0.Add(2*time.Minute)); fire {
		t.Fatal("expected rate limit to block the third capture within the hour")
	}
	// An hour later the window has slid; capture allowed again.
	if fire, reason := decideTrigger(cfg, st, stackchan.SensorState{Motion: true}, t0.Add(61*time.Minute)); !fire || reason != "event" {
		t.Fatalf("expected event after window slid, got (%v,%q)", fire, reason)
	}
	if len(st.recent) != 0 {
		t.Fatalf("stale timestamps not trimmed: %d remain", len(st.recent))
	}
}

// fakeBridge is a controllable PerceptionBridge for loop tests.
type fakeBridge struct {
	sensor   stackchan.SensorState
	snap     stackchan.CameraSnapshot
	clip     stackchan.AudioClip
	camCalls int
}

func (f *fakeBridge) CameraSnapshot(context.Context, stackchan.SnapshotOptions) (stackchan.CameraSnapshot, error) {
	f.camCalls++
	return f.snap, nil
}
func (f *fakeBridge) AudioClip(context.Context, stackchan.AudioOptions) (stackchan.AudioClip, error) {
	return f.clip, nil
}
func (f *fakeBridge) Sensors(context.Context) (stackchan.SensorState, error) { return f.sensor, nil }

type captureSink struct {
	mu  sync.Mutex
	obs []Observation
}

func (s *captureSink) Post(_ context.Context, o Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.obs = append(s.obs, o)
	return nil
}

func TestStepCapturesSummarizesAndSinks(t *testing.T) {
	cfg := baseCfg()
	cfg.CacheDir = t.TempDir()
	fb := &fakeBridge{
		sensor: stackchan.SensorState{Motion: true},
		snap:   stackchan.CameraSnapshot{ContentType: "image/jpeg", Data: []byte{0xFF, 0xD8, 0xFF}},
		clip:   stackchan.AudioClip{ContentType: "audio/wav", Data: []byte("RIFFxxxxWAVE"), DurationMs: 1500},
	}
	sink := &captureSink{}
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	d := Deps{
		Bridge:     fb,
		Summarizer: StubSummarizer{},
		Identifier: StubIdentifier{Profile: &OwnerProfile{}, Config: cfg},
		Sink:       sink,
		Config:     cfg,
		Now:        func() time.Time { return now },
	}
	st := &triggerState{}
	if !d.step(context.Background(), st) {
		t.Fatal("expected step to fire on motion")
	}
	if len(sink.obs) != 1 {
		t.Fatalf("sink received %d observations, want 1", len(sink.obs))
	}
	o := sink.obs[0]
	if o.Trigger != "event" || o.Summary == "" || o.ImageRef == "" || o.AudioRef == "" {
		t.Fatalf("unexpected observation: %#v", o)
	}
	// Debounced immediately after.
	if d.step(context.Background(), st) {
		t.Fatal("expected debounce to suppress the immediate next step")
	}
}

func TestCameraDisabledAudioOnlyMode(t *testing.T) {
	cfg := baseCfg()
	cfg.CacheDir = t.TempDir()
	cfg.CameraEnabled = false
	fb := &fakeBridge{
		sensor: stackchan.SensorState{Motion: true},
		snap:   stackchan.CameraSnapshot{Data: []byte{0xFF, 0xD8, 0xFF}}, // must NOT be used
		clip:   stackchan.AudioClip{ContentType: "audio/wav", Data: []byte("RIFFxxxxWAVE"), DurationMs: 1500},
	}
	sink := &captureSink{}
	d := Deps{
		Bridge:     fb,
		Summarizer: StubSummarizer{},
		Identifier: StubIdentifier{Profile: &OwnerProfile{}, Config: cfg},
		Sink:       sink,
		Config:     cfg,
		Now:        func() time.Time { return time.Unix(1700000000, 0) },
	}
	if !d.step(context.Background(), &triggerState{}) {
		t.Fatal("expected step to fire")
	}
	if fb.camCalls != 0 {
		t.Fatalf("camera was called %d times with CameraEnabled=false", fb.camCalls)
	}
	o := sink.obs[0]
	if o.ImageRef != "" {
		t.Fatalf("audio-only observation must have no image ref, got %q", o.ImageRef)
	}
	if o.AudioRef == "" {
		t.Fatal("audio-only observation should still have an audio ref")
	}
}

func TestTARSWebhookSinkPostsAgreedPayload(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sink := TARSWebhookSink{BaseURL: srv.URL, Channel: "stackchan", Token: "secret"}
	obs := Observation{TS: 123, Trigger: "event", Summary: "someone waved", Salience: 0.7, ImageRef: "obs-123.jpg"}
	if err := sink.Post(context.Background(), obs); err != nil {
		t.Fatalf("post: %v", err)
	}
	if gotPath != "/v1/channels/webhook/inbound/stackchan" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotBody.Source != "stackchan" || gotBody.Summary != "someone waved" || gotBody.Text != "someone waved" || gotBody.Salience != 0.7 {
		t.Fatalf("payload = %#v", gotBody)
	}
}

func TestTARSWebhookSinkRetriesThenDrops(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	sink := TARSWebhookSink{BaseURL: srv.URL, Channel: "c", MaxRetries: 2, Backoff: time.Millisecond}
	err := sink.Post(context.Background(), Observation{Summary: "x"})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if attempts != 3 { // initial + 2 retries
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestLogSinkIsNoOp(t *testing.T) {
	if err := (LogSink{}).Post(context.Background(), Observation{}); err != nil {
		t.Fatalf("log sink should never error: %v", err)
	}
}
