package bodyprovider

import "testing"

func TestStackChanCapabilitiesRespectCameraConfig(t *testing.T) {
	withCamera, err := NewStackChanProvider(&fakeStackChanBridge{}, StackChanProviderConfig{
		Name:          "stackchan",
		CameraEnabled: true,
	})
	if err != nil {
		t.Fatalf("NewStackChanProvider: %v", err)
	}
	if got := capabilityList(withCamera.Capabilities()); got != "vision,hearing,speech,expression,motion,led" {
		t.Fatalf("capabilities with camera = %q", got)
	}

	audioOnly, err := NewStackChanProvider(&fakeStackChanBridge{}, StackChanProviderConfig{
		Name:          "stackchan",
		CameraEnabled: false,
	})
	if err != nil {
		t.Fatalf("NewStackChanProvider audio-only: %v", err)
	}
	if got := capabilityList(audioOnly.Capabilities()); got != "hearing,speech,expression,motion,led" {
		t.Fatalf("capabilities audio-only = %q", got)
	}
}

func capabilityList(values []Capability) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += ","
		}
		out += string(value)
	}
	return out
}
