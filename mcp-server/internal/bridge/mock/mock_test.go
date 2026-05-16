package mock

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

func TestStatusAdvertisesPerceptionCapabilities(t *testing.T) {
	status, err := New().GetStatus(context.Background())
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	for _, want := range []string{"camera", "microphone"} {
		if !slices.Contains(status.Capabilities, want) {
			t.Fatalf("capabilities %v missing %q", status.Capabilities, want)
		}
	}
}

func TestCameraSnapshotReturnsValidJPEGFixture(t *testing.T) {
	snap, err := New().CameraSnapshot(context.Background(), stackchan.SnapshotOptions{MaxWidth: 320})
	if err != nil {
		t.Fatalf("camera snapshot: %v", err)
	}
	if snap.ContentType != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", snap.ContentType)
	}
	// JPEG SOI marker.
	if len(snap.Data) < 3 || snap.Data[0] != 0xFF || snap.Data[1] != 0xD8 {
		t.Fatalf("data is not a JPEG: % x", snap.Data[:min(4, len(snap.Data))])
	}
}

func TestAudioClipReturnsValidWAVFixtureAndDuration(t *testing.T) {
	clip, err := New().AudioClip(context.Background(), stackchan.AudioOptions{DurationMs: 1500})
	if err != nil {
		t.Fatalf("audio clip: %v", err)
	}
	if clip.ContentType != "audio/wav" {
		t.Fatalf("content type = %q, want audio/wav", clip.ContentType)
	}
	if clip.DurationMs != 1500 {
		t.Fatalf("duration = %d, want 1500", clip.DurationMs)
	}
	if len(clip.Data) < 12 || !bytes.Equal(clip.Data[0:4], []byte("RIFF")) || !bytes.Equal(clip.Data[8:12], []byte("WAVE")) {
		t.Fatalf("data is not a RIFF/WAVE container")
	}
}

func TestSensorsReturnsMediaFreeStateWithTimestamp(t *testing.T) {
	state, err := New().Sensors(context.Background())
	if err != nil {
		t.Fatalf("sensors: %v", err)
	}
	if state.Motion || state.SoundLevel != 0 {
		t.Fatalf("state = %#v, want media-free defaults", state)
	}
	if state.TS <= 0 {
		t.Fatalf("ts = %d, want a positive unix-ms timestamp", state.TS)
	}
}
