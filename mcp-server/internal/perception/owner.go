package perception

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bodyprovider"
)

// OwnerProfile is the locally-stored owner fingerprint. Raw reference media
// is written next to it (0600) and NEVER leaves the device (privacy policy,
// codebase-analysis §5); only sha256 digests live in the JSON so the
// identifier can match without re-reading large blobs.
type OwnerProfile struct {
	Name        string   `json:"name"`
	FaceHashes  []string `json:"face_hashes"`
	VoiceHashes []string `json:"voice_hashes"`
	CreatedAt   int64    `json:"created_at"`
}

// Enrolled reports whether a usable owner fingerprint exists.
func (p *OwnerProfile) Enrolled() bool {
	return p != nil && (len(p.FaceHashes) > 0 || len(p.VoiceHashes) > 0)
}

// OwnerStore persists the profile + reference media under an absolute user
// data dir (mirrors tts.DefaultCacheDir's "works as a background service"
// rationale, but uses a config/data dir since this is durable identity).
type OwnerStore struct{ Dir string }

// DefaultOwnerDir resolves an absolute, writable owner-data directory.
func DefaultOwnerDir() string {
	if base, err := os.UserConfigDir(); err == nil && base != "" {
		return filepath.Join(base, "tars-stackchan", "owner")
	}
	return ".work/owner"
}

func (s OwnerStore) profilePath() string { return filepath.Join(s.Dir, "profile.json") }

// Load returns the stored profile, or an empty (not-enrolled) profile when
// none exists. A corrupt file is treated as not-enrolled (safe default).
func (s OwnerStore) Load() (*OwnerProfile, error) {
	data, err := os.ReadFile(s.profilePath())
	if errors.Is(err, os.ErrNotExist) {
		return &OwnerProfile{}, nil
	}
	if err != nil {
		return nil, err
	}
	var p OwnerProfile
	if json.Unmarshal(data, &p) != nil {
		return &OwnerProfile{}, nil
	}
	return &p, nil
}

// Reset removes all owner data (re-enroll from scratch).
func (s OwnerStore) Reset() error {
	if err := os.RemoveAll(s.Dir); err != nil {
		return err
	}
	return nil
}

// Enroll captures faceN snapshots + voiceM clips from the bridge and writes
// the profile + reference media (0600 files, 0700 dir). Existing data is
// replaced.
func (s OwnerStore) Enroll(name string, faces []bodyprovider.CameraSnapshot, voices []bodyprovider.AudioClip) (*OwnerProfile, error) {
	if len(faces) == 0 && len(voices) == 0 {
		return nil, errors.New("enroll: at least one face or voice sample is required")
	}
	if err := os.RemoveAll(s.Dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return nil, err
	}

	profile := &OwnerProfile{Name: name, CreatedAt: time.Now().Unix()}
	for i, f := range faces {
		if len(f.Data) == 0 {
			continue
		}
		h := digest(f.Data)
		if err := os.WriteFile(filepath.Join(s.Dir, fmt.Sprintf("face-%02d.jpg", i)), f.Data, 0o600); err != nil {
			return nil, err
		}
		profile.FaceHashes = append(profile.FaceHashes, h)
	}
	for i, v := range voices {
		if len(v.Data) == 0 {
			continue
		}
		h := digest(v.Data)
		if err := os.WriteFile(filepath.Join(s.Dir, fmt.Sprintf("voice-%02d.wav", i)), v.Data, 0o600); err != nil {
			return nil, err
		}
		profile.VoiceHashes = append(profile.VoiceHashes, h)
	}
	if !profile.Enrolled() {
		return nil, errors.New("enroll: all captured samples were empty")
	}
	sort.Strings(profile.FaceHashes)
	sort.Strings(profile.VoiceHashes)

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(s.profilePath(), data, 0o600); err != nil {
		return nil, err
	}
	return profile, nil
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
