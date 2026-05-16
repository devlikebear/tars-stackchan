package tts

import "testing"

func TestNormalizeVoice(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"known lowercase", "kore", "Kore"},
		{"known mixed case", "KoRe", "Kore"},
		{"empty falls back to default", "", "Kore"},
		{"whitespace trimmed then default", "   ", "Kore"},
		{"alias with surrounding spaces", "  puck  ", "Puck"},
		{"unknown passthrough verbatim", "CustomVoice", "CustomVoice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeVoice(tc.in); got != tc.want {
				t.Fatalf("NormalizeVoice(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeModel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", DefaultGeminiModel},
		{"   ", DefaultGeminiModel},
		{" custom-model ", "custom-model"},
	}
	for _, tc := range cases {
		if got := NormalizeModel(tc.in); got != tc.want {
			t.Fatalf("NormalizeModel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
