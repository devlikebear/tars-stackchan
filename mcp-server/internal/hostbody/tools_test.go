package hostbody

import (
	"errors"
	"reflect"
	"testing"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bodyprovider"
)

func TestProbeToolsAndCapabilities(t *testing.T) {
	look := func(name string) (string, error) {
		paths := map[string]string{
			"sox":       "/opt/homebrew/bin/sox",
			"imagesnap": "/opt/homebrew/bin/imagesnap",
			"say":       "/usr/bin/say",
		}
		if path, ok := paths[name]; ok {
			return path, nil
		}
		return "", errors.New("missing")
	}

	tools := ProbeTools(look)
	got := tools.Capabilities(CapabilityOptions{CameraEnabled: true})
	want := []bodyprovider.Capability{
		bodyprovider.CapabilityVision,
		bodyprovider.CapabilityHearing,
		bodyprovider.CapabilitySpeech,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("capabilities = %v, want %v", got, want)
	}
}

func TestCapabilitiesGracefullySkipMissingTools(t *testing.T) {
	tools := Tools{FFmpeg: "/usr/local/bin/ffmpeg", AFPlay: "/usr/bin/afplay"}

	got := tools.Capabilities(CapabilityOptions{CameraEnabled: false, TTSConfigured: true})
	want := []bodyprovider.Capability{
		bodyprovider.CapabilityHearing,
		bodyprovider.CapabilitySpeech,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("audio-only capabilities = %v, want %v", got, want)
	}

	got = tools.Capabilities(CapabilityOptions{CameraEnabled: true})
	want = []bodyprovider.Capability{
		bodyprovider.CapabilityVision,
		bodyprovider.CapabilityHearing,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("no-tts capabilities = %v, want %v", got, want)
	}
}
