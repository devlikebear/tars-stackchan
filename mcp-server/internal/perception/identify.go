package perception

import (
	"context"
	"slices"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

// Identity labels.
const (
	LabelOwner    = "owner"
	LabelStranger = "stranger"
	LabelUnknown  = "unknown"
)

// Identity is the 3-state owner decision attached to an observation.
type Identity struct {
	Label      string  `json:"label"`      // owner | stranger | unknown
	Confidence float64 `json:"confidence"` // 0..1
	Modality   string  `json:"modality"`   // face | voice | both | none
}

// Identifier scores a captured moment against the enrolled owner. Abstracted
// so the mock/offline path is deterministic and Gemini is opt-in.
type Identifier interface {
	Identify(ctx context.Context, snap stackchan.CameraSnapshot, clip stackchan.AudioClip) (Identity, error)
}

// modalityScore is a per-modality "same person as owner" probability.
// present=false means that modality was unavailable.
type modalityScore struct {
	present bool
	score   float64
}

// fuse applies the 3-state rule. Owner needs the available modalities to
// agree above OwnerConfidenceMin; stranger needs them at/below
// StrangerConfidenceMax; anything in-between or conflicting is unknown
// (safe default).
func fuse(face, voice modalityScore, cfg Config) Identity {
	scores := make([]float64, 0, 2)
	modality := "none"
	switch {
	case face.present && voice.present:
		modality = "both"
		scores = append(scores, face.score, voice.score)
	case face.present:
		modality = "face"
		scores = append(scores, face.score)
	case voice.present:
		modality = "voice"
		scores = append(scores, voice.score)
	default:
		return Identity{Label: LabelUnknown, Confidence: 0, Modality: "none"}
	}

	allOwner, allStranger := true, true
	minScore, maxScore := 1.0, 0.0
	for _, s := range scores {
		if s < cfg.OwnerConfidenceMin {
			allOwner = false
		}
		if s > cfg.StrangerConfidenceMax {
			allStranger = false
		}
		minScore = min(minScore, s)
		maxScore = max(maxScore, s)
	}
	switch {
	case allOwner:
		return Identity{Label: LabelOwner, Confidence: minScore, Modality: modality}
	case allStranger:
		return Identity{Label: LabelStranger, Confidence: 1 - maxScore, Modality: modality}
	default:
		// Conflicting modalities or mid-band score.
		return Identity{Label: LabelUnknown, Confidence: 0, Modality: modality}
	}
}

// StubIdentifier is deterministic and offline: a captured sample scores 1.0
// for a modality iff its sha256 matches an enrolled reference, else 0.0. With
// the mock bridge (same fixture as enrolled) this yields `owner`; a different
// fixture yields `stranger`; no profile yields `unknown`.
type StubIdentifier struct {
	Profile *OwnerProfile
	Config  Config
}

func (s StubIdentifier) Identify(_ context.Context, snap stackchan.CameraSnapshot, clip stackchan.AudioClip) (Identity, error) {
	if !s.Profile.Enrolled() {
		return Identity{Label: LabelUnknown, Confidence: 0, Modality: "none"}, nil
	}
	face := modalityScore{}
	if len(snap.Data) > 0 && len(s.Profile.FaceHashes) > 0 {
		face.present = true
		if slices.Contains(s.Profile.FaceHashes, digest(snap.Data)) {
			face.score = 1.0
		}
	}
	voice := modalityScore{}
	if len(clip.Data) > 0 && len(s.Profile.VoiceHashes) > 0 {
		voice.present = true
		if slices.Contains(s.Profile.VoiceHashes, digest(clip.Data)) {
			voice.score = 1.0
		}
	}
	return fuse(face, voice, s.Config), nil
}

// identityClause renders a short natural-language prefix so the observation
// summary itself carries the owner signal (TARS persona reads plain text).
func identityClause(id Identity) string {
	switch id.Label {
	case LabelOwner:
		return "The owner appears to be present. "
	case LabelStranger:
		return "Someone who is not the owner is present. "
	default:
		return "An unidentified presence. "
	}
}
