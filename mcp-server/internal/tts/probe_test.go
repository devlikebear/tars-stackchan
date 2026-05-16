package tts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestProbeHealth(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	}))
	defer ok.Close()
	if err := ProbeHealth(context.Background(), ok.URL); err != nil {
		t.Fatalf("healthy relay: unexpected error %v", err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer bad.Close()
	if err := ProbeHealth(context.Background(), bad.URL); err == nil {
		t.Fatal("503 relay: expected error")
	}

	if err := ProbeHealth(context.Background(), "http://127.0.0.1:1"); err == nil {
		t.Fatal("unreachable relay: expected error")
	}
}

func TestProbeMDNS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		_, err := ProbeMDNS(context.Background(), "tars-stackchan-tts.local")
		if err == nil || !strings.Contains(err.Error(), "requires macOS") {
			t.Fatalf("non-darwin err = %v, want requires macOS", err)
		}
		return
	}

	orig := dscacheutilLookup
	t.Cleanup(func() { dscacheutilLookup = orig })

	dscacheutilLookup = func(context.Context, string) (string, error) {
		return "name: tars-stackchan-tts.local\nip_address: 192.168.219.115\nip_address: 192.168.219.115\n", nil
	}
	ip, err := ProbeMDNS(context.Background(), "tars-stackchan-tts.local")
	if err != nil || ip != "192.168.219.115" {
		t.Fatalf("ProbeMDNS = %q,%v want 192.168.219.115,nil", ip, err)
	}

	dscacheutilLookup = func(context.Context, string) (string, error) {
		return "name: missing\n", nil
	}
	if _, err := ProbeMDNS(context.Background(), "missing.local"); err == nil {
		t.Fatal("expected unresolved error when no ip_address line")
	}
}
