package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/tts"
)

// defaultTTSPort mirrors the argparse default in tts-remote-server.py:
// TARS_STACKCHAN_TTS_PORT, otherwise 18080.
func defaultTTSPort() int {
	if v := strings.TrimSpace(os.Getenv("TARS_STACKCHAN_TTS_PORT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 18080
}

func runTTS(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: tars-stackchan-control tts serve [flags]")
		return 2
	}
	switch args[0] {
	case "serve":
		return runTTSServe(args[1:], stderr)
	case "status":
		return runTTSStatus(args[1:], stderr)
	case "install":
		return runTTSInstall(stderr)
	default:
		fmt.Fprintf(stderr, "unknown tts subcommand %q\n", args[0])
		return 2
	}
}

func runTTSServe(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("tts serve", flag.ContinueOnError)
	fs.SetOutput(stderr)

	host := fs.String("host", "0.0.0.0", "bind host")
	port := fs.Int("port", defaultTTSPort(), "bind port (env TARS_STACKCHAN_TTS_PORT)")
	apiKey := fs.String("api-key", "", "Gemini API key (env TARS_STACKCHAN_GEMINI_API_KEY / GEMINI_API_KEY)")
	token := fs.String("token", "", "relay token (env TARS_STACKCHAN_TTS_TOKEN / TARS_STACKCHAN_TOKEN)")
	model := fs.String("model", "", "Gemini model (env TARS_STACKCHAN_TTS_MODEL)")
	voice := fs.String("voice", "", "Gemini voice (env TARS_STACKCHAN_TTS_VOICE)")
	endpoint := fs.String("gemini-endpoint-template", "", "Gemini endpoint template (env TARS_STACKCHAN_GEMINI_ENDPOINT)")
	sampleRate := fs.Int("sample-rate", 0, "PCM sample rate (default 24000)")
	promptPrefix := fs.String("prompt-prefix", "", "prepended to every utterance (env TARS_STACKCHAN_TTS_PROMPT_PREFIX)")
	cacheDir := fs.String("cache-dir", "", "WAV cache directory (env TARS_STACKCHAN_TTS_CACHE, default: user cache dir)")
	mdns := fs.Bool("mdns", true, "advertise the relay over mDNS (macOS dns-sd)")
	mdnsHostname := fs.String("mdns-hostname", tts.DefaultTTSHostname, "mDNS hostname to advertise")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg := tts.Resolve(tts.Flags{
		APIKey:           *apiKey,
		Token:            *token,
		Model:            *model,
		Voice:            *voice,
		EndpointTemplate: *endpoint,
		SampleRate:       *sampleRate,
		PromptPrefix:     *promptPrefix,
		CacheDir:         *cacheDir,
	})

	if err := tts.Serve(context.Background(), cfg, tts.ServeOptions{
		Host:         *host,
		Port:         *port,
		MDNS:         *mdns,
		MDNSHostname: *mdnsHostname,
	}); err != nil {
		fmt.Fprintf(stderr, "tts serve failed: %v\n", err)
		return 1
	}
	return 0
}

func runTTSStatus(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("tts status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	port := fs.Int("port", defaultTTSPort(), "relay port to probe")
	hostname := fs.String("mdns-hostname", tts.DefaultTTSHostname, "mDNS hostname to resolve")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", *port)
	healthErr := tts.ProbeHealth(ctx, baseURL)
	if healthErr != nil {
		fmt.Fprintf(stderr, "relay: DOWN (%v)\n", healthErr)
		fmt.Fprintln(stderr, "hint: start it with 'brew services start tars-stackchan' or 'tars-stackchan-control tts serve'")
		return 1
	}
	fmt.Fprintf(stderr, "relay: OK (%s/health)\n", baseURL)

	ip, mdnsErr := tts.ProbeMDNS(ctx, *hostname)
	if mdnsErr != nil {
		fmt.Fprintf(stderr, "mDNS:  UNRESOLVED (%v)\n", mdnsErr)
		fmt.Fprintf(stderr, "hint: bake a fixed IP fallback with TARS_STACKCHAN_TTS_HOST=<mac-ip> and re-flash\n")
		return 0
	}
	fmt.Fprintf(stderr, "mDNS:  OK (%s -> %s)\n", *hostname, ip)
	return 0
}

func runTTSInstall(stderr io.Writer) int {
	fmt.Fprint(stderr, `Run the Gemini TTS relay as a background service:

  export TARS_STACKCHAN_TOKEN=...        # same token baked into the firmware MOD
  export GEMINI_API_KEY=...              # or TARS_STACKCHAN_GEMINI_API_KEY
  brew services start tars-stackchan

The relay advertises `+tts.DefaultTTSHostname+` over mDNS, so the device
finds the current relay IP at boot without a re-flash.

If mDNS does not resolve on your network, re-flash the firmware with a
fixed IP fallback: TARS_STACKCHAN_TTS_HOST=<mac-ip>

Check it any time with: tars-stackchan-control tts status
`)
	return 0
}
