package perception

import (
	"context"
	"errors"
	"time"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/stackchan"
)

// Deps are the perception loop's collaborators. Bridge must implement the
// optional PerceptionBridge capability (camera/audio/sensors).
type Deps struct {
	Bridge     stackchan.PerceptionBridge
	Summarizer Summarizer
	Identifier Identifier
	Sink       Sink
	Config     Config
	Log        Logger
	// Now is injectable for tests; defaults to time.Now.
	Now func() time.Time
}

func (d *Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func (d *Deps) logf(format string, args ...any) {
	if d.Log != nil {
		d.Log.Printf(format, args...)
	}
}

// triggerState is the loop's mutable decision state. recent holds capture
// times within the trailing hour for the rate limit.
type triggerState struct {
	lastTrigger time.Time
	lastIdle    time.Time
	recent      []time.Time
}

func (s *triggerState) rateLimited(now time.Time, maxPerHour int) bool {
	cutoff := now.Add(-time.Hour)
	kept := s.recent[:0]
	for _, t := range s.recent {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	s.recent = kept
	return len(s.recent) >= maxPerHour
}

func (s *triggerState) record(now time.Time) {
	s.recent = append(s.recent, now)
	s.lastTrigger = now
	s.lastIdle = now // any capture also satisfies the idle cadence
}

// decideTrigger is pure: given config, state, the latest sensor reading and
// the current time, decide whether to capture and why. Event triggers beat
// idle; both honor debounce and the hourly rate limit.
func decideTrigger(cfg Config, st *triggerState, sensor stackchan.SensorState, now time.Time) (bool, string) {
	if st.rateLimited(now, cfg.MaxCapturesPerHour) {
		return false, ""
	}
	eventCandidate := sensor.Motion || sensor.SoundLevel >= cfg.SoundLevelThreshold
	if eventCandidate {
		if st.lastTrigger.IsZero() || now.Sub(st.lastTrigger) >= cfg.MinTriggerInterval {
			return true, "event"
		}
	}
	if st.lastIdle.IsZero() || now.Sub(st.lastIdle) >= cfg.IdleSnapshotInterval {
		return true, "idle"
	}
	return false, ""
}

// step performs one poll → decide → (capture, summarize, sink) cycle. It is
// the unit tests drive directly with a fake clock and mock bridge. Capture
// and sink errors are logged, never fatal — the loop must keep running.
func (d *Deps) step(ctx context.Context, st *triggerState) bool {
	now := d.now()
	sensor, err := d.Bridge.Sensors(ctx)
	if err != nil {
		d.logf("[perceive] sensors read failed: %v", err)
		return false
	}

	fire, reason := decideTrigger(d.Config, st, sensor, now)
	if !fire {
		return false
	}
	st.record(now)

	var snap stackchan.CameraSnapshot
	if d.Config.CameraEnabled {
		s, err := d.Bridge.CameraSnapshot(ctx, stackchan.SnapshotOptions{MaxWidth: d.Config.SnapshotMaxWidth})
		if err != nil {
			d.logf("[perceive] camera snapshot failed: %v", err)
		}
		snap = s
	}
	clip, err := d.Bridge.AudioClip(ctx, stackchan.AudioOptions{DurationMs: d.Config.AudioClipMs})
	if err != nil {
		d.logf("[perceive] audio clip failed: %v", err)
	}

	ts := now.UnixMilli()
	imageRef, audioRef := cacheMedia(d.Config.CacheDir, ts, snap, clip)

	identity, err := d.Identifier.Identify(ctx, snap, clip)
	if err != nil {
		d.logf("[perceive] identify failed: %v", err)
		identity = Identity{Label: LabelUnknown, Modality: "none"}
	}

	summary, salience, err := d.Summarizer.Summarize(ctx, snap, clip, reason)
	if err != nil {
		d.logf("[perceive] summarize failed: %v", err)
		return true // captured but unusable; counted to respect rate limits
	}

	obs := Observation{
		TS:       ts,
		Trigger:  reason,
		Summary:  identityClause(identity) + summary,
		Salience: salience,
		Identity: identity,
		ImageRef: imageRef,
		AudioRef: audioRef,
	}
	if err := d.Sink.Post(ctx, obs); err != nil {
		d.logf("[perceive] sink post failed: %v", err)
	}
	return true
}

// Run drives the perception loop until ctx is cancelled.
func Run(ctx context.Context, d Deps) error {
	if d.Bridge == nil {
		return errors.New("perception: bridge is required")
	}
	if d.Summarizer == nil {
		return errors.New("perception: summarizer is required")
	}
	if d.Identifier == nil {
		return errors.New("perception: identifier is required")
	}
	if d.Sink == nil {
		return errors.New("perception: sink is required")
	}
	if d.Config.SensorPollInterval <= 0 {
		d.Config.SensorPollInterval = defaultSensorPoll
	}

	ticker := time.NewTicker(d.Config.SensorPollInterval)
	defer ticker.Stop()

	st := &triggerState{}
	d.logf("[perceive] loop started (poll=%s idle=%s min-trigger=%s max/h=%d camera=%t tars=%t)",
		d.Config.SensorPollInterval, d.Config.IdleSnapshotInterval,
		d.Config.MinTriggerInterval, d.Config.MaxCapturesPerHour,
		d.Config.CameraEnabled, d.Config.TARSConfigured())

	for {
		select {
		case <-ctx.Done():
			d.logf("[perceive] loop stopped: %v", ctx.Err())
			return nil
		case <-ticker.C:
			d.step(ctx, st)
		}
	}
}
