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

// Serve starts the relay HTTP server and blocks until the context is cancelled
// or an OS interrupt is received, then drains gracefully. It mirrors the
// startup banner and token requirement of tts-remote-server.py main().
func Serve(ctx context.Context, cfg Config, host string, port int) error {
	if ResolveToken(cfg.Token) == "" {
		return errors.New("TARS_STACKCHAN_TTS_TOKEN or TARS_STACKCHAN_TOKEN is required")
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	srv := &http.Server{Addr: addr, Handler: NewHandler(cfg)}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

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
