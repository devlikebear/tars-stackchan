package tts

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

// ServeOptions carries network and discovery settings for Serve.
type ServeOptions struct {
	Host string
	Port int

	// MDNS enables `dns-sd` advertising of MDNSHostname. When false the relay
	// still serves; clients must reach it by IP (TARS_STACKCHAN_TTS_HOST).
	MDNS         bool
	MDNSHostname string
}

// Serve starts the relay HTTP server and blocks until the context is cancelled
// or an OS interrupt is received, then drains gracefully. It mirrors the
// startup banner and token requirement of tts-remote-server.py main() and adds
// lifecycle-bound mDNS advertising (Phase 2).
func Serve(ctx context.Context, cfg Config, opts ServeOptions) error {
	if ResolveToken(cfg.Token) == "" {
		return errors.New("TARS_STACKCHAN_TTS_TOKEN or TARS_STACKCHAN_TOKEN is required")
	}

	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	srv := &http.Server{Addr: addr, Handler: NewHandler(cfg)}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if opts.MDNS {
		hostname := opts.MDNSHostname
		if hostname == "" {
			hostname = DefaultTTSHostname
		}
		stopMDNS, err := Advertise(ctx, DefaultServiceName, hostname, opts.Port)
		if err != nil {
			fmt.Printf("mDNS advertise error: %v (continuing; use TARS_STACKCHAN_TTS_HOST=<ip>)\n", err)
		}
		defer stopMDNS()
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("TARS Stack-chan Gemini TTS: http://%s/api/tts?text=hello\n", addr)
		fmt.Printf("Gemini model: %s, voice: %s\n", NormalizeModel(cfg.Model), NormalizeVoice(cfg.Voice))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), readinessTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
