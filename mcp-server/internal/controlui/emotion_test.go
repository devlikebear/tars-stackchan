package controlui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bridge/mock"
)

func newTestServer() *Server {
	return NewServer(ServerConfig{Bridge: mock.New(), BridgeMode: "mock"})
}

func TestEmotionAppliesPresetThroughBridge(t *testing.T) {
	srv := httptest.NewServer(newTestServer())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/emotion", "application/json", strings.NewReader(`{"emotion":"happy"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out emotionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK || out.Action != "set_emotion" || out.Emotion != "happy" {
		t.Fatalf("response = %#v", out)
	}
	// happy preset = expression + LED + motion.
	if len(out.Applied) != 3 {
		t.Fatalf("applied = %d legs, want 3 (expression+led+motion)", len(out.Applied))
	}
}

func TestEmotionSuppressMotion(t *testing.T) {
	srv := httptest.NewServer(newTestServer())
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/api/emotion", "application/json", strings.NewReader(`{"emotion":"happy","motion":false}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	var out emotionResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Applied) != 2 {
		t.Fatalf("applied = %d, want 2 (motion suppressed)", len(out.Applied))
	}
}

func TestEmotionRejectsUnknownEmotionAndFields(t *testing.T) {
	srv := httptest.NewServer(newTestServer())
	defer srv.Close()

	post := func(payload string) int {
		resp, err := http.Post(srv.URL+"/api/emotion", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	if got := post(`{"emotion":"hangry"}`); got != http.StatusBadRequest {
		t.Fatalf("unknown emotion status = %d, want 400", got)
	}
	if got := post(`{"emotion":"happy","bogus":1}`); got != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want 400", got)
	}

	r3, err := http.Get(srv.URL + "/api/emotion")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer r3.Body.Close()
	if r3.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", r3.StatusCode)
	}
}

func TestPerceiveStatusReadOnly(t *testing.T) {
	// Isolate owner dir so the test never depends on a developer's machine.
	t.Setenv("TARS_STACKCHAN_PERCEIVE_OWNER_DIR", t.TempDir())
	t.Setenv("TARS_STACKCHAN_PERCEIVE_CAMERA", "off")

	srv := httptest.NewServer(newTestServer())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/perceive/status")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var st perceiveStatus
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if st.BridgeMode != "mock" || st.CameraEnabled || st.OwnerEnrolled {
		t.Fatalf("unexpected status: %#v", st)
	}
}
