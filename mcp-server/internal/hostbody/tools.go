package hostbody

import (
	"os/exec"
	"strings"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bodyprovider"
)

type LookPathFunc func(string) (string, error)

type Tools struct {
	Sox       string
	FFmpeg    string
	ImageSnap string
	AFPlay    string
	Say       string
}

type CapabilityOptions struct {
	CameraEnabled bool
	TTSConfigured bool
}

func ProbeTools(look LookPathFunc) Tools {
	if look == nil {
		look = exec.LookPath
	}
	return Tools{
		Sox:       probe(look, "sox"),
		FFmpeg:    probe(look, "ffmpeg"),
		ImageSnap: probe(look, "imagesnap"),
		AFPlay:    probe(look, "afplay"),
		Say:       probe(look, "say"),
	}
}

func (t Tools) Capabilities(opts CapabilityOptions) []bodyprovider.Capability {
	out := make([]bodyprovider.Capability, 0, 3)
	if opts.CameraEnabled && t.HasCamera() {
		out = append(out, bodyprovider.CapabilityVision)
	}
	if t.HasAudio() {
		out = append(out, bodyprovider.CapabilityHearing)
	}
	if t.HasSpeech(opts.TTSConfigured) {
		out = append(out, bodyprovider.CapabilitySpeech)
	}
	return out
}

func (t Tools) HasAudio() bool {
	return strings.TrimSpace(t.Sox) != "" || strings.TrimSpace(t.FFmpeg) != ""
}

func (t Tools) HasCamera() bool {
	return strings.TrimSpace(t.ImageSnap) != "" || strings.TrimSpace(t.FFmpeg) != ""
}

func (t Tools) HasSpeech(ttsConfigured bool) bool {
	if strings.TrimSpace(t.Say) != "" {
		return true
	}
	return strings.TrimSpace(t.AFPlay) != "" && ttsConfigured
}

func probe(look LookPathFunc, name string) string {
	path, err := look(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(path)
}
