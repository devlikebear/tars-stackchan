package hostbody

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCaptureAudioUsesSoxAndReturnsWAV(t *testing.T) {
	runner := &recordingRunner{writePayload: []byte("RIFF-host-audio")}
	capturer := Capturer{
		Tools:   Tools{Sox: "/bin/sox"},
		Runner:  runner,
		WorkDir: t.TempDir(),
	}

	clip, err := capturer.CaptureAudio(context.Background(), 1200*time.Millisecond)
	if err != nil {
		t.Fatalf("CaptureAudio: %v", err)
	}
	if clip.ContentType != "audio/wav" || string(clip.Data) != "RIFF-host-audio" || clip.DurationMs != 1200 {
		t.Fatalf("clip = %+v", clip)
	}
	if runner.command != "/bin/sox" || !containsArg(runner.args, "trim") {
		t.Fatalf("runner call = %s %v", runner.command, runner.args)
	}
}

func TestCaptureCameraUsesImageSnapAndReturnsJPEG(t *testing.T) {
	runner := &recordingRunner{writePayload: []byte{0xFF, 0xD8, 0xFF}}
	capturer := Capturer{
		Tools:   Tools{ImageSnap: "/bin/imagesnap"},
		Runner:  runner,
		WorkDir: t.TempDir(),
	}

	snap, err := capturer.CaptureCamera(context.Background())
	if err != nil {
		t.Fatalf("CaptureCamera: %v", err)
	}
	if snap.ContentType != "image/jpeg" || len(snap.Data) != 3 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if runner.command != "/bin/imagesnap" {
		t.Fatalf("runner command = %q", runner.command)
	}
}

func TestSpeakerUsesSayForText(t *testing.T) {
	runner := &recordingRunner{}
	speaker := Speaker{
		Tools:  Tools{Say: "/usr/bin/say"},
		Runner: runner,
	}

	if err := speaker.Speak(context.Background(), "hello host", nil); err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if runner.command != "/usr/bin/say" || !containsArg(runner.args, "hello host") {
		t.Fatalf("runner call = %s %v", runner.command, runner.args)
	}
}

func TestSpeakerUsesTTSRelayAndAfplayWhenSayMissing(t *testing.T) {
	runner := &recordingRunner{}
	server := newTTSServer(t, []byte("RIFF-host-tts"))
	speaker := Speaker{
		Tools:      Tools{AFPlay: "/usr/bin/afplay"},
		Runner:     runner,
		WorkDir:    t.TempDir(),
		TTSBaseURL: server.URL,
		TTSToken:   "secret",
	}

	if err := speaker.Speak(context.Background(), "relay speech", nil); err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if runner.command != "/usr/bin/afplay" {
		t.Fatalf("runner command = %q", runner.command)
	}
	if len(runner.args) != 1 {
		t.Fatalf("afplay args = %v", runner.args)
	}
	payload, err := os.ReadFile(runner.args[0])
	if err != nil {
		t.Fatalf("read generated wav: %v", err)
	}
	if string(payload) != "RIFF-host-tts" {
		t.Fatalf("generated wav = %q", payload)
	}
}

type recordingRunner struct {
	command      string
	args         []string
	writePayload []byte
}

func (r *recordingRunner) Run(_ context.Context, command string, args ...string) error {
	r.command = command
	r.args = append([]string(nil), args...)
	if r.writePayload == nil {
		return nil
	}
	for i := len(args) - 1; i >= 0; i-- {
		if strings.HasPrefix(args[i], "-") || (!strings.HasSuffix(args[i], ".wav") && !strings.HasSuffix(args[i], ".jpg")) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(args[i]), 0o755); err != nil {
			return err
		}
		return os.WriteFile(args[i], r.writePayload, 0o600)
	}
	return nil
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}
