package perception

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bodyprovider"
)

func idCfg() Config {
	return Config{OwnerConfidenceMin: 0.6, StrangerConfidenceMax: 0.4}
}

func TestStubIdentifier(t *testing.T) {
	ownerFace := []byte("OWNER-FACE-BYTES")
	ownerVoice := []byte("OWNER-VOICE-BYTES")
	otherFace := []byte("SOMEONE-ELSE-FACE")
	otherVoice := []byte("SOMEONE-ELSE-VOICE")

	enrolled := &OwnerProfile{
		FaceHashes:  []string{digest(ownerFace)},
		VoiceHashes: []string{digest(ownerVoice)},
		CreatedAt:   1,
	}

	tests := []struct {
		name     string
		profile  *OwnerProfile
		snap     bodyprovider.CameraSnapshot
		clip     bodyprovider.AudioClip
		want     string
		modality string
	}{
		{
			name:     "not enrolled -> unknown",
			profile:  &OwnerProfile{},
			snap:     bodyprovider.CameraSnapshot{Data: ownerFace},
			want:     LabelUnknown,
			modality: "none",
		},
		{
			name:     "owner face+voice match -> owner (both)",
			profile:  enrolled,
			snap:     bodyprovider.CameraSnapshot{Data: ownerFace},
			clip:     bodyprovider.AudioClip{Data: ownerVoice},
			want:     LabelOwner,
			modality: "both",
		},
		{
			name:     "owner voice only -> owner (voice)",
			profile:  enrolled,
			clip:     bodyprovider.AudioClip{Data: ownerVoice},
			want:     LabelOwner,
			modality: "voice",
		},
		{
			name:     "different person both -> stranger (both)",
			profile:  enrolled,
			snap:     bodyprovider.CameraSnapshot{Data: otherFace},
			clip:     bodyprovider.AudioClip{Data: otherVoice},
			want:     LabelStranger,
			modality: "both",
		},
		{
			name:     "conflict: owner face, stranger voice -> unknown",
			profile:  enrolled,
			snap:     bodyprovider.CameraSnapshot{Data: ownerFace},
			clip:     bodyprovider.AudioClip{Data: otherVoice},
			want:     LabelUnknown,
			modality: "both",
		},
		{
			name:     "no media at all -> unknown",
			profile:  enrolled,
			want:     LabelUnknown,
			modality: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := StubIdentifier{Profile: tt.profile, Config: idCfg()}
			got, err := id.Identify(context.Background(), tt.snap, tt.clip)
			if err != nil {
				t.Fatalf("identify: %v", err)
			}
			if got.Label != tt.want || got.Modality != tt.modality {
				t.Fatalf("got (%s,%s), want (%s,%s)", got.Label, got.Modality, tt.want, tt.modality)
			}
		})
	}
}

func TestOwnerStoreEnrollRoundTripAndPerms(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "owner")
	store := OwnerStore{Dir: dir}

	if p, _ := store.Load(); p.Enrolled() {
		t.Fatal("fresh store should not be enrolled")
	}

	faces := []bodyprovider.CameraSnapshot{{Data: []byte("f1")}, {Data: []byte("f2")}}
	voices := []bodyprovider.AudioClip{{Data: []byte("v1")}}
	prof, err := store.Enroll("me", faces, voices)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if prof.Name != "me" || len(prof.FaceHashes) != 2 || len(prof.VoiceHashes) != 1 {
		t.Fatalf("profile = %#v", prof)
	}

	loaded, err := store.Load()
	if err != nil || !loaded.Enrolled() || loaded.Name != "me" {
		t.Fatalf("reload mismatch: %#v err=%v", loaded, err)
	}

	// The fixture used to enroll must now identify as owner.
	id := StubIdentifier{Profile: loaded, Config: idCfg()}
	got, _ := id.Identify(context.Background(), bodyprovider.CameraSnapshot{Data: []byte("f1")}, bodyprovider.AudioClip{Data: []byte("v1")})
	if got.Label != LabelOwner {
		t.Fatalf("enrolled fixture should be owner, got %s", got.Label)
	}

	// File permissions: profile.json 0600, dir 0700.
	fi, err := os.Stat(filepath.Join(dir, "profile.json"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("profile.json perm = %v err=%v, want 0600", fi.Mode().Perm(), err)
	}
	di, _ := os.Stat(dir)
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("owner dir perm = %v, want 0700", di.Mode().Perm())
	}

	if err := store.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if p, _ := store.Load(); p.Enrolled() {
		t.Fatal("store should be empty after reset")
	}
}

func TestEnrollRejectsEmptySamples(t *testing.T) {
	store := OwnerStore{Dir: filepath.Join(t.TempDir(), "o")}
	if _, err := store.Enroll("x", nil, nil); err == nil {
		t.Fatal("expected error with no samples")
	}
	if _, err := store.Enroll("x", []bodyprovider.CameraSnapshot{{Data: nil}}, nil); err == nil {
		t.Fatal("expected error when all samples empty")
	}
}

func TestStepAttachesOwnerIdentityAndClause(t *testing.T) {
	cfg := baseCfg()
	cfg.CacheDir = t.TempDir()
	cfg.OwnerConfidenceMin = 0.6
	cfg.StrangerConfidenceMax = 0.4
	face := []byte{0xFF, 0xD8, 0xFF, 0x01}
	prof := &OwnerProfile{FaceHashes: []string{digest(face)}, CreatedAt: 1}

	fb := &fakeBridge{
		sensor: bodyprovider.SensorState{Motion: true},
		snap:   bodyprovider.CameraSnapshot{ContentType: "image/jpeg", Data: face},
	}
	sink := &captureSink{}
	d := Deps{
		Bridge:     fb,
		Summarizer: StubSummarizer{},
		Identifier: StubIdentifier{Profile: prof, Config: cfg},
		Sink:       sink,
		Config:     cfg,
		Now:        func() time.Time { return time.Unix(1700000000, 0) },
	}
	if !d.step(context.Background(), &triggerState{}) {
		t.Fatal("expected step to fire")
	}
	if len(sink.obs) != 1 {
		t.Fatalf("want 1 observation, got %d", len(sink.obs))
	}
	o := sink.obs[0]
	if o.Identity.Label != LabelOwner {
		t.Fatalf("identity = %s, want owner", o.Identity.Label)
	}
	if !strings.HasPrefix(o.Summary, "The owner appears to be present.") {
		t.Fatalf("summary missing owner clause: %q", o.Summary)
	}
}
