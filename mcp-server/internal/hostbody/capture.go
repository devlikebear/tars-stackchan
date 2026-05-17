package hostbody

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/bodyprovider"
)

type Capturer struct {
	Tools   Tools
	Runner  Runner
	WorkDir string
}

func (c Capturer) CaptureAudio(ctx context.Context, duration time.Duration) (bodyprovider.AudioClip, error) {
	if duration <= 0 {
		duration = defaultAudioClip
	}
	path, err := tempPath(c.WorkDir, "host-audio-*.wav")
	if err != nil {
		return bodyprovider.AudioClip{}, err
	}
	defer os.Remove(path)

	runner := c.runner()
	switch {
	case strings.TrimSpace(c.Tools.Sox) != "":
		seconds := strconv.FormatFloat(duration.Seconds(), 'f', 3, 64)
		err = runner.Run(ctx, c.Tools.Sox, "-q", "-d", "-r", "16000", "-c", "1", "-b", "16", path, "trim", "0", seconds)
	case strings.TrimSpace(c.Tools.FFmpeg) != "":
		seconds := strconv.FormatFloat(duration.Seconds(), 'f', 3, 64)
		err = runner.Run(ctx, c.Tools.FFmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "avfoundation", "-i", ":0", "-t", seconds, "-ar", "16000", "-ac", "1", "-y", path)
	default:
		return bodyprovider.AudioClip{}, fmt.Errorf("no host audio capture tool found (install sox or ffmpeg)")
	}
	if err != nil {
		return bodyprovider.AudioClip{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return bodyprovider.AudioClip{}, err
	}
	return bodyprovider.AudioClip{ContentType: "audio/wav", Data: data, DurationMs: int(duration / time.Millisecond)}, nil
}

func (c Capturer) CaptureCamera(ctx context.Context) (bodyprovider.CameraSnapshot, error) {
	path, err := tempPath(c.WorkDir, "host-camera-*.jpg")
	if err != nil {
		return bodyprovider.CameraSnapshot{}, err
	}
	defer os.Remove(path)

	runner := c.runner()
	switch {
	case strings.TrimSpace(c.Tools.ImageSnap) != "":
		err = runner.Run(ctx, c.Tools.ImageSnap, "-w", "1", path)
	case strings.TrimSpace(c.Tools.FFmpeg) != "":
		err = runner.Run(ctx, c.Tools.FFmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "avfoundation", "-framerate", "1", "-i", "0:none", "-frames:v", "1", "-y", path)
	default:
		return bodyprovider.CameraSnapshot{}, fmt.Errorf("no host camera capture tool found (install imagesnap or ffmpeg)")
	}
	if err != nil {
		return bodyprovider.CameraSnapshot{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return bodyprovider.CameraSnapshot{}, err
	}
	return bodyprovider.CameraSnapshot{ContentType: "image/jpeg", Data: data}, nil
}

func (c Capturer) runner() Runner {
	if c.Runner != nil {
		return c.Runner
	}
	return ExecRunner{}
}

func tempPath(dir, pattern string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return filepath.Clean(path), nil
}
