package hostbody

import (
	"context"
	"testing"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bodyprovider"
)

func TestCaptureOncePostsAudioPercept(t *testing.T) {
	runner := &recordingRunner{writePayload: []byte("RIFF-host-audio")}
	sink := &capturePerceptSink{}
	deps := CaptureDeps{
		Capturer: Capturer{Tools: Tools{Sox: "/bin/sox"}, Runner: runner, WorkDir: t.TempDir()},
		Sink:     sink,
		Config: Config{
			ProviderName:  "host",
			Owner:         "owner",
			SessionID:     "sess_main",
			AudioClip:     time.Second,
			CameraEnabled: false,
			CacheDir:      t.TempDir(),
		},
		Now: func() time.Time { return time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC) },
		Summarizer: staticSummarizer{
			summary:  "owner spoke near the Mac",
			salience: 0.9,
		},
	}

	if err := CaptureOnce(context.Background(), deps, "event"); err != nil {
		t.Fatalf("CaptureOnce: %v", err)
	}
	if len(sink.percepts) != 1 {
		t.Fatalf("percepts = %d, want 1", len(sink.percepts))
	}
	got := sink.percepts[0]
	if got.Provider != "host" || got.Owner != "owner" || got.Modality != "audio" || got.AudioRef == "" || got.MediaRef != got.AudioRef {
		t.Fatalf("percept = %+v", got)
	}
}

type staticSummarizer struct {
	summary  string
	salience float64
}

func (s staticSummarizer) Summarize(context.Context, bodyprovider.CameraSnapshot, bodyprovider.AudioClip, string) (string, float64, error) {
	return s.summary, s.salience, nil
}

type capturePerceptSink struct {
	percepts []Percept
}

func (s *capturePerceptSink) Post(_ context.Context, p Percept) error {
	s.percepts = append(s.percepts, p)
	return nil
}
