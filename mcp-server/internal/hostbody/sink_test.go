package hostbody

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTARSSinkPostsHostPercept(t *testing.T) {
	var gotPath, gotAuth string
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sink := TARSSink{BaseURL: server.URL, Provider: "host", Token: "secret"}
	err := sink.Post(context.Background(), Percept{
		Provider:   "host",
		Modality:   "audio",
		Owner:      "owner",
		Summary:    "owner spoke near the Mac",
		MediaRef:   "host-audio.wav",
		Trigger:    "event",
		Salience:   0.8,
		SessionID:  "sess_main",
		CapturedAt: time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if gotPath != "/v1/embodiment/percept/host" || gotAuth != "Bearer secret" {
		t.Fatalf("request = path %q auth %q", gotPath, gotAuth)
	}
	if got["x-embodiment"] != true || got["source"] != "host" || got["owner"] != "owner" || got["modality"] != "audio" {
		t.Fatalf("payload = %#v", got)
	}
	if got["media_ref"] != "host-audio.wav" || got["session_id"] != "sess_main" {
		t.Fatalf("payload refs = %#v", got)
	}
}

func newTTSServer(t *testing.T, payload []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tts" || r.URL.Query().Get("token") != "secret" {
			t.Fatalf("unexpected tts request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
}
