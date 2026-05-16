package tts

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
)

// DefaultTTSHostname is the project-stable mDNS name the relay advertises and
// that prepare-firmware-upload.sh bakes into config.tts.host by default. Single
// source of truth shared with the firmware bake script and Phase 4 doctor.
const DefaultTTSHostname = "tars-stackchan-tts.local"

// DefaultServiceName is the Bonjour service instance name used by `dns-sd -P`.
const DefaultServiceName = "tars-stackchan-tts"

// primaryIPv4 returns the host's outbound-facing IPv4 without sending any
// packets: a UDP "connect" only fixes the local source address. Stdlib only.
func primaryIPv4() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("determine primary IPv4: %w", err)
	}
	defer conn.Close()
	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || udpAddr.IP.To4() == nil {
		return "", fmt.Errorf("no primary IPv4 address available")
	}
	return udpAddr.IP.To4().String(), nil
}

// lookPath is indirected so tests can stub dns-sd discovery.
var lookPath = exec.LookPath

// Advertise publishes `hostname` as an mDNS A record pointing at this host's
// primary IPv4 using the macOS-native `dns-sd -P` proxy registration confirmed
// in the Phase 2 spike. The advertisement is bound to the returned process and
// is torn down when ctx is cancelled or stop() is called, so it never outlives
// the relay (removing the manual lifecycle that contributed to the outage).
//
// On non-darwin or when dns-sd is unavailable it logs and returns a no-op stop
// func rather than failing — speech still works via the IP fallback bake.
func Advertise(ctx context.Context, serviceName, hostname string, port int) (func(), error) {
	noop := func() {}

	if runtime.GOOS != "darwin" {
		fmt.Printf("mDNS advertise skipped: unsupported platform %q (use TARS_STACKCHAN_TTS_HOST=<ip>)\n", runtime.GOOS)
		return noop, nil
	}
	if _, err := lookPath("dns-sd"); err != nil {
		fmt.Println("mDNS advertise skipped: dns-sd not found (use TARS_STACKCHAN_TTS_HOST=<ip>)")
		return noop, nil
	}

	ip, err := primaryIPv4()
	if err != nil {
		fmt.Printf("mDNS advertise skipped: %v (use TARS_STACKCHAN_TTS_HOST=<ip>)\n", err)
		return noop, nil
	}

	// dns-sd -P <Name> _http._tcp local <port> <hostname> <ipv4>
	args := dnssdProxyArgs(serviceName, hostname, port, ip)
	cmd := exec.CommandContext(ctx, "dns-sd", args...)
	if err := cmd.Start(); err != nil {
		return noop, fmt.Errorf("start dns-sd proxy: %w", err)
	}
	fmt.Printf("advertising mDNS %s -> %s:%d\n", hostname, ip, port)

	stop := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}
	return stop, nil
}

// dnssdProxyArgs builds the argument vector for the confirmed proxy command.
// Extracted so tests assert the exact form without exec.
func dnssdProxyArgs(serviceName, hostname string, port int, ipv4 string) []string {
	return []string{
		"-P",
		serviceName,
		"_http._tcp",
		"local",
		strconv.Itoa(port),
		hostname,
		ipv4,
	}
}
