package tts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

// ProbeHealth checks the relay /health endpoint. It is shared by `tts status`
// and the Phase 4 doctor so the two never drift.
func ProbeHealth(ctx context.Context, baseURL string) error {
	url := strings.TrimRight(baseURL, "/") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("relay unreachable at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("relay /health returned %d", resp.StatusCode)
	}
	if strings.TrimSpace(string(body)) != "ok" {
		return fmt.Errorf("relay /health body = %q, want \"ok\"", strings.TrimSpace(string(body)))
	}
	return nil
}

// dscacheutil is indirected for tests.
var dscacheutilLookup = func(ctx context.Context, hostname string) (string, error) {
	out, err := exec.CommandContext(ctx, "dscacheutil", "-q", "host", "-a", "name", hostname).Output()
	return string(out), err
}

// ProbeMDNS resolves an mDNS hostname to its first IPv4 via the macOS system
// resolver (dscacheutil), matching how the device's resolver behaves. Returns
// the resolved address so callers can surface it. macOS only — elsewhere it
// returns a clear unsupported error (the relay target is macOS desktop).
func ProbeMDNS(ctx context.Context, hostname string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("mDNS probe requires macOS; set TARS_STACKCHAN_TTS_HOST=<ip> instead")
	}
	out, err := dscacheutilLookup(ctx, hostname)
	if err != nil {
		return "", fmt.Errorf("resolve %s failed: %w", hostname, err)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "ip_address:"); ok {
			if ip := strings.TrimSpace(rest); ip != "" {
				return ip, nil
			}
		}
	}
	return "", fmt.Errorf("%s did not resolve via mDNS", hostname)
}
