package tts

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, gemini *httptest.Server) (*httptest.Server, Config) {
	t.Helper()
	cfg := Config{
		APIKey:           "k",
		Token:            "secret",
		Model:            "m",
		Voice:            "Kore",
		EndpointTemplate: gemini.URL + "/{model}",
		SampleRate:       24000,
		CacheDir:         t.TempDir(),
	}
	ts := httptest.NewServer(NewHandler(cfg))
	t.Cleanup(ts.Close)
	return ts, cfg
}

func fakeGemini(t *testing.T, pcm []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{"candidates": []any{map[string]any{
			"content": map[string]any{"parts": []any{map[string]any{
				"inlineData": map[string]any{"data": base64.StdEncoding.EncodeToString(pcm)},
			}}},
		}}}
		out, _ := json.Marshal(body)
		_, _ = w.Write(out)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHealth(t *testing.T) {
	gem := fakeGemini(t, []byte{1})
	ts, _ := newTestServer(t, gem)
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != "ok\n" {
		t.Fatalf("health = %d %q, want 200 \"ok\\n\"", resp.StatusCode, body)
	}
}

func TestUnknownPath404(t *testing.T) {
	gem := fakeGemini(t, []byte{1})
	ts, _ := newTestServer(t, gem)
	resp, err := http.Get(ts.URL + "/nope")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestTTSRequiresToken(t *testing.T) {
	gem := fakeGemini(t, []byte{1})
	ts, _ := newTestServer(t, gem)
	resp, err := http.Get(ts.URL + "/api/tts?text=hi")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestTTSReturnsWAVAndCaches(t *testing.T) {
	wantPCM := []byte{9, 8, 7, 6}
	calls := 0
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body := map[string]any{"candidates": []any{map[string]any{
			"content": map[string]any{"parts": []any{map[string]any{
				"inlineData": map[string]any{"data": base64.StdEncoding.EncodeToString(wantPCM)},
			}}},
		}}}
		out, _ := json.Marshal(body)
		_, _ = w.Write(out)
	}))
	defer gem.Close()
	ts, _ := newTestServer(t, gem)

	for i := 0; i < 2; i++ {
		resp, err := http.Get(ts.URL + "/api/tts?token=secret&text=hello")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if resp.Header.Get("Content-Type") != "audio/wav" {
			t.Fatalf("content-type = %q", resp.Header.Get("Content-Type"))
		}
		if string(body[:4]) != "RIFF" || string(body[8:12]) != "WAVE" {
			t.Fatalf("not a WAV body")
		}
		if string(body[44:]) != string(wantPCM) {
			t.Fatalf("pcm payload mismatch")
		}
	}
	if calls != 1 {
		t.Fatalf("gemini called %d times, want 1 (cache miss then hit)", calls)
	}
}

func TestTTSValidatesText(t *testing.T) {
	gem := fakeGemini(t, []byte{1})
	ts, _ := newTestServer(t, gem)

	resp, _ := http.Get(ts.URL + "/api/tts?token=secret&text=")
	if resp.StatusCode != 400 {
		t.Fatalf("empty text status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()

	long := strings.Repeat("가", 241)
	resp, _ = http.Get(ts.URL + "/api/tts?token=secret&text=" + long)
	if resp.StatusCode != 400 {
		t.Fatalf("241-rune text status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestRedactPath(t *testing.T) {
	got := redactPath("/api/tts?token=supersecret&text=hi")
	if strings.Contains(got, "supersecret") {
		t.Fatalf("token leaked in %q", got)
	}
	if !strings.Contains(got, "token=%3Credacted%3E") && !strings.Contains(got, "token=<redacted>") {
		t.Fatalf("redaction missing in %q", got)
	}
	if !strings.Contains(got, "text=hi") {
		t.Fatalf("non-token query dropped in %q", got)
	}
}
