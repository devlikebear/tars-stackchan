package tts

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderEndpoint(t *testing.T) {
	cases := []struct {
		tmpl  string
		model string
		want  string
	}{
		{"https://x/{model}:gen", "a b", "https://x/a+b:gen"},
		{"https://x/%s:gen", "m1", "https://x/m1:gen"},
		{"https://x/static", "m1", "https://x/static"},
	}
	for _, tc := range cases {
		if got := RenderEndpoint(tc.tmpl, tc.model); got != tc.want {
			t.Fatalf("RenderEndpoint(%q,%q) = %q, want %q", tc.tmpl, tc.model, got, tc.want)
		}
	}
}

func TestBuildPayloadShape(t *testing.T) {
	raw, err := BuildPayload("hello", "Kore")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	gen := got["generationConfig"].(map[string]any)
	mods := gen["responseModalities"].([]any)
	if len(mods) != 1 || mods[0] != "AUDIO" {
		t.Fatalf("responseModalities = %v, want [AUDIO]", mods)
	}
	vn := gen["speechConfig"].(map[string]any)["voiceConfig"].(map[string]any)["prebuiltVoiceConfig"].(map[string]any)["voiceName"]
	if vn != "Kore" {
		t.Fatalf("voiceName = %v, want Kore", vn)
	}
}

func geminiAudioResponse(pcm []byte) string {
	body := map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"parts": []any{
						map[string]any{
							"inlineData": map[string]any{
								"data": base64.StdEncoding.EncodeToString(pcm),
							},
						},
					},
				},
			},
		},
	}
	out, _ := json.Marshal(body)
	return string(out)
}

func TestSynthesizePCM(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_GEMINI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	wantPCM := []byte{1, 2, 3, 4}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("missing api key header, got %q", r.Header.Get("x-goog-api-key"))
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "\"AUDIO\"") {
			t.Errorf("payload missing AUDIO modality: %s", raw)
		}
		_, _ = io.WriteString(w, geminiAudioResponse(wantPCM))
	}))
	defer srv.Close()

	cfg := Config{APIKey: "test-key", Model: "m", Voice: "Kore", EndpointTemplate: srv.URL + "/{model}"}
	pcm, err := SynthesizePCM(context.Background(), srv.Client(), cfg, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(pcm) != string(wantPCM) {
		t.Fatalf("pcm = %v, want %v", pcm, wantPCM)
	}
}

func TestSynthesizePCMErrors(t *testing.T) {
	t.Setenv("TARS_STACKCHAN_GEMINI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")

	t.Run("missing api key", func(t *testing.T) {
		_, err := SynthesizePCM(context.Background(), http.DefaultClient, Config{}, "x")
		if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY") {
			t.Fatalf("err = %v, want api key required", err)
		}
	})

	t.Run("http non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(503)
			_, _ = io.WriteString(w, "down")
		}))
		defer srv.Close()
		cfg := Config{APIKey: "k", EndpointTemplate: srv.URL + "/{model}"}
		_, err := SynthesizePCM(context.Background(), srv.Client(), cfg, "x")
		if err == nil || !strings.Contains(err.Error(), "gemini http 503") {
			t.Fatalf("err = %v, want gemini http 503", err)
		}
	})

	t.Run("no audio", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"candidates":[{"content":{"parts":[]}}]}`)
		}))
		defer srv.Close()
		cfg := Config{APIKey: "k", EndpointTemplate: srv.URL + "/{model}"}
		_, err := SynthesizePCM(context.Background(), srv.Client(), cfg, "x")
		if err == nil || !strings.Contains(err.Error(), "did not include inline audio") {
			t.Fatalf("err = %v, want no inline audio", err)
		}
	})

	t.Run("api error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"error":{"status":"PERMISSION_DENIED","message":"nope"}}`)
		}))
		defer srv.Close()
		cfg := Config{APIKey: "k", EndpointTemplate: srv.URL + "/{model}"}
		_, err := SynthesizePCM(context.Background(), srv.Client(), cfg, "x")
		if err == nil || !strings.Contains(err.Error(), "gemini api error PERMISSION_DENIED") {
			t.Fatalf("err = %v, want api error", err)
		}
	})
}

func TestWritePCMAsWAV(t *testing.T) {
	pcm := []byte{0x10, 0x20, 0x30, 0x40}
	var buf strings.Builder
	if err := WritePCMAsWAV(&buf, pcm, 24000); err != nil {
		t.Fatal(err)
	}
	out := []byte(buf.String())
	if len(out) != 44+len(pcm) {
		t.Fatalf("len = %d, want %d", len(out), 44+len(pcm))
	}
	if string(out[0:4]) != "RIFF" || string(out[8:12]) != "WAVE" ||
		string(out[12:16]) != "fmt " || string(out[36:40]) != "data" {
		t.Fatalf("bad WAV magic: %q", out[:40])
	}
	if got := binary.LittleEndian.Uint32(out[4:8]); got != uint32(36+len(pcm)) {
		t.Fatalf("RIFF chunk size = %d, want %d", got, 36+len(pcm))
	}
	if got := binary.LittleEndian.Uint32(out[24:28]); got != 24000 {
		t.Fatalf("sample rate = %d, want 24000", got)
	}
	if got := binary.LittleEndian.Uint16(out[22:24]); got != 1 {
		t.Fatalf("channels = %d, want 1", got)
	}
	if got := binary.LittleEndian.Uint32(out[40:44]); got != uint32(len(pcm)) {
		t.Fatalf("data size = %d, want %d", got, len(pcm))
	}
}
