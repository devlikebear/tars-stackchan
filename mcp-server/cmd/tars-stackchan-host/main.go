package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/buildinfo"
	"github.com/devlikebear/tars-stackchan/mcp-server/internal/hostbody"
)

const binaryName = "tars-stackchan-host"

type stderrLogger struct{ w io.Writer }

func (l stderrLogger) Printf(format string, args ...any) {
	fmt.Fprintf(l.w, format+"\n", args...)
}

func main() {
	os.Exit(runCommand(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runCommand(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "--version", "-version", "version":
			fmt.Fprintln(stdout, buildinfo.String(binaryName))
			return 0
		case "-h", "--help", "help":
			printUsage(stdout)
			return 0
		case "serve":
			return runServe(stderr)
		case "probe":
			return runProbe(stdout)
		default:
			fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
			printUsage(stderr)
			return 2
		}
	}

	host := newHost()
	if err := hostbody.NewServer(host).Serve(context.Background(), stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "tars-stackchan host MCP server failed: %v\n", err)
		return 1
	}
	return 0
}

func newHost() hostbody.Host {
	cfg := hostbody.LoadConfig()
	tools := hostbody.ProbeTools(nil)
	return hostbody.Host{
		Tools:      tools,
		Runner:     hostbody.ExecRunner{},
		WorkDir:    cfg.CacheDir,
		TTSBaseURL: cfg.TTSBaseURL,
		TTSToken:   cfg.TTSToken,
		SayVoice:   cfg.SayVoice,
	}
}

func runServe(stderr io.Writer) int {
	cfg := hostbody.LoadConfig()
	tools := hostbody.ProbeTools(nil)
	log := stderrLogger{w: stderr}

	var sink hostbody.Sink = hostbody.LogSink{Log: log}
	if strings.TrimSpace(cfg.TARSBaseURL) != "" {
		sink = hostbody.TARSSink{
			BaseURL:  cfg.TARSBaseURL,
			Provider: cfg.ProviderName,
			Token:    cfg.TARSToken,
		}
	} else {
		log.Printf("[hostbody] TARS not configured; using log sink")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	err := hostbody.Run(ctx, hostbody.CaptureDeps{
		Capturer: hostbody.Capturer{
			Tools:   tools,
			Runner:  hostbody.ExecRunner{},
			WorkDir: cfg.CacheDir,
		},
		Sink:       sink,
		Summarizer: hostbody.StubSummarizer{},
		Config:     cfg,
		Log:        log,
	})
	if err != nil {
		fmt.Fprintf(stderr, "tars-stackchan host companion failed: %v\n", err)
		return 1
	}
	return 0
}

func runProbe(stdout io.Writer) int {
	cfg := hostbody.LoadConfig()
	tools := hostbody.ProbeTools(nil)
	fmt.Fprintf(stdout, "provider: %s\n", cfg.ProviderName)
	fmt.Fprintf(stdout, "capabilities: %v\n", tools.Capabilities(hostbody.CapabilityOptions{
		CameraEnabled: cfg.CameraEnabled,
		TTSConfigured: strings.TrimSpace(cfg.TTSBaseURL) != "" && strings.TrimSpace(cfg.TTSToken) != "",
	}))
	fmt.Fprintf(stdout, "tools: sox=%t ffmpeg=%t imagesnap=%t say=%t afplay=%t\n",
		tools.Sox != "", tools.FFmpeg != "", tools.ImageSnap != "", tools.Say != "", tools.AFPlay != "")
	return 0
}

func printUsage(stdout io.Writer) {
	fmt.Fprintf(stdout, `Usage:
  %[1]s             run the host MCP server (stdio; exposes host_speak)
  %[1]s serve       run the Mac host perception companion loop
  %[1]s probe       print host tool and capability status
  %[1]s --version   print version

Environment:
  TARS_STACKCHAN_HOST_TARS_BASE_URL / TARS_STACKCHAN_TARS_BASE_URL
  TARS_STACKCHAN_HOST_PROVIDER
  TARS_STACKCHAN_HOST_SESSION_ID
  TARS_STACKCHAN_HOST_OWNER
  TARS_STACKCHAN_HOST_CAMERA
  TARS_STACKCHAN_HOST_AUDIO_MS
  TARS_STACKCHAN_HOST_IDLE_INTERVAL
  TARS_STACKCHAN_HOST_TTS_BASE_URL, TARS_STACKCHAN_HOST_TTS_TOKEN
`, binaryName)
}
