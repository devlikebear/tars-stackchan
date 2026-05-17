package hostbody

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bodyprovider"
)

type Speaker struct {
	Tools      Tools
	Runner     Runner
	WorkDir    string
	TTSBaseURL string
	TTSToken   string
	SayVoice   string
	Client     *http.Client
}

func (s Speaker) Speak(ctx context.Context, text string, volume *float64) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("host speech text is required")
	}
	if strings.TrimSpace(s.Tools.Say) != "" {
		args := []string{}
		if voice := strings.TrimSpace(s.SayVoice); voice != "" {
			args = append(args, "-v", voice)
		}
		args = append(args, text)
		return s.runner().Run(ctx, s.Tools.Say, args...)
	}
	if strings.TrimSpace(s.Tools.AFPlay) != "" && strings.TrimSpace(s.TTSBaseURL) != "" {
		path, err := s.fetchTTS(ctx, text)
		if err != nil {
			return err
		}
		return s.runner().Run(ctx, s.Tools.AFPlay, path)
	}
	return fmt.Errorf("no host speech tool found (install say, or configure afplay with TARS_STACKCHAN_HOST_TTS_BASE_URL)")
}

func (s Speaker) SpeakRequest(ctx context.Context, req bodyprovider.SpeechRequest) (bodyprovider.ActionResult, error) {
	if err := s.Speak(ctx, req.Text, req.Volume); err != nil {
		return bodyprovider.ActionResult{}, err
	}
	return bodyprovider.ActionResult{OK: true, Action: "speak", State: map[string]any{"text": strings.TrimSpace(req.Text)}}, nil
}

func (s Speaker) runner() Runner {
	if s.Runner != nil {
		return s.Runner
	}
	return ExecRunner{}
}

func (s Speaker) fetchTTS(ctx context.Context, text string) (string, error) {
	endpoint, err := url.Parse(strings.TrimRight(s.TTSBaseURL, "/") + "/api/tts")
	if err != nil {
		return "", err
	}
	q := endpoint.Query()
	q.Set("text", text)
	if token := strings.TrimSpace(s.TTSToken); token != "" {
		q.Set("token", token)
	}
	endpoint.RawQuery = q.Encode()

	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("host tts relay returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	path, err := tempPath(s.WorkDir, "host-tts-*.wav")
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", err
	}
	return path, nil
}
