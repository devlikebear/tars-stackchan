package tts

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestDNSSDProxyArgs(t *testing.T) {
	got := dnssdProxyArgs("tars-stackchan-tts", "tars-stackchan-tts.local", 18080, "192.168.1.5")
	want := []string{"-P", "tars-stackchan-tts", "_http._tcp", "local", "18080", "tars-stackchan-tts.local", "192.168.1.5"}
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestAdvertiseNoopWhenDNSSDMissing(t *testing.T) {
	// Force the "dns-sd unavailable" branch so the test is deterministic on
	// both the darwin dev host and the Linux CI runner.
	orig := lookPath
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() { lookPath = orig })

	stop, err := Advertise(context.Background(), DefaultServiceName, DefaultTTSHostname, 18080)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stop == nil {
		t.Fatal("stop func must be non-nil")
	}
	stop() // must be safe to call
}

func TestPrimaryIPv4(t *testing.T) {
	ip, err := primaryIPv4()
	if err != nil {
		t.Skipf("no primary IPv4 route in this environment: %v", err)
	}
	if net.ParseIP(ip) == nil || net.ParseIP(ip).To4() == nil {
		t.Fatalf("primaryIPv4 = %q, not a valid IPv4", ip)
	}
}

func TestDefaultsAreStableConstants(t *testing.T) {
	if DefaultTTSHostname != "tars-stackchan-tts.local" {
		t.Fatalf("DefaultTTSHostname = %q (firmware bake script depends on this)", DefaultTTSHostname)
	}
	if DefaultServiceName != "tars-stackchan-tts" {
		t.Fatalf("DefaultServiceName = %q", DefaultServiceName)
	}
}
