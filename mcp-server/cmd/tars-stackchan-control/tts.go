package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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
	case "install", "status":
		// Implemented in Phase 3 (Homebrew service + CLI helpers).
		fmt.Fprintf(stderr, "tts %s is not implemented yet\n", args[0])
		return 2
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
	cacheDir := fs.String("cache-dir", "", "WAV cache directory (env TARS_STACKCHAN_TTS_CACHE, default .work/tts-cache)")
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
