package reload

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"sigs.k8s.io/yaml"
)

// SyncWindowKind says whether a sync window allows or denies reloads while it is active.
type SyncWindowKind string

const (
	// SyncWindowKindAllow permits reloads only while the window is active.
	SyncWindowKindAllow SyncWindowKind = "allow"
	// SyncWindowKindDeny blocks reloads while the window is active.
	SyncWindowKindDeny SyncWindowKind = "deny"
)

// maxSyncWindowSteps bounds the search for the next allowed or blocked time. Every step
// jumps to the next instant at which the answer can change, so the bound only matters for
// windows that keep reloads blocked for a very long time; the search then stops at a time
// that is still blocked, which is a safe time to look again.
const maxSyncWindowSteps = 64

// maxSyncWindows bounds how many windows one annotation may define, which bounds the work
// of evaluating it on every reconcile.
const maxSyncWindows = 16

// cronParser accepts standard five-field cron expressions and descriptors such as @daily,
// the same format Argo CD sync windows use.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// SyncWindowSpec is one entry of the sync-windows annotation.
type SyncWindowSpec struct {
	// Kind is either "allow" or "deny".
	Kind SyncWindowKind `json:"kind"`
	// Schedule is a cron expression for when the window opens.
	Schedule string `json:"schedule"`
	// Duration is how long the window stays open, as a Go duration such as "1h".
	Duration string `json:"duration"`
	// TimeZone is the IANA time zone the schedule is evaluated in. Empty means UTC.
	TimeZone string `json:"timeZone,omitempty"`
	// Spread overrides how long after a window opens the reloads it deferred are spread
	// over, as a Go duration such as "30m". Empty uses Reloader's --sync-window-spread.
	Spread string `json:"spread,omitempty"`
}

type syncWindow struct {
	kind     SyncWindowKind
	schedule cron.Schedule
	duration time.Duration
	location *time.Location
	// spread is negative when the window does not set one.
	spread time.Duration
}

// SyncWindows is a parsed set of sync windows. Reloads are blocked while any deny window
// is active and, when allow windows exist, also while none of them is active.
type SyncWindows []syncWindow

// ParseSyncWindows parses a sync-windows annotation value, a YAML or JSON list of
// SyncWindowSpec entries.
func ParseSyncWindows(value string) (SyncWindows, error) {
	var specs []SyncWindowSpec
	if err := yaml.UnmarshalStrict([]byte(value), &specs); err != nil {
		return nil, fmt.Errorf("invalid sync windows: %w", err)
	}
	if len(specs) > maxSyncWindows {
		return nil, fmt.Errorf("invalid sync windows: at most %d windows are allowed, got %d", maxSyncWindows, len(specs))
	}

	now := time.Now()
	windows := make(SyncWindows, 0, len(specs))
	for i, spec := range specs {
		w, err := parseSyncWindow(spec, now)
		if err != nil {
			return nil, fmt.Errorf("invalid sync window %d: %w", i, err)
		}
		windows = append(windows, w)
	}
	return windows, nil
}

func parseSyncWindow(spec SyncWindowSpec, now time.Time) (syncWindow, error) {
	switch spec.Kind {
	case SyncWindowKindAllow, SyncWindowKindDeny:
	default:
		return syncWindow{}, fmt.Errorf("kind must be %q or %q, got %q", SyncWindowKindAllow, SyncWindowKindDeny, spec.Kind)
	}

	// The cron parser also accepts a TZ= prefix, which would compete with timeZone.
	if strings.HasPrefix(spec.Schedule, "TZ=") || strings.HasPrefix(spec.Schedule, "CRON_TZ=") {
		return syncWindow{}, errors.New("schedule must not set a time zone, use timeZone instead")
	}
	schedule, err := cronParser.Parse(spec.Schedule)
	if err != nil {
		return syncWindow{}, fmt.Errorf("invalid schedule %q: %w", spec.Schedule, err)
	}
	// "@every" repeats relative to whenever it is asked, so it has no fixed start times a
	// window could open at.
	if _, ok := schedule.(*cron.SpecSchedule); !ok {
		return syncWindow{}, fmt.Errorf("schedule %q must be a cron expression or a calendar descriptor such as @daily", spec.Schedule)
	}

	duration, err := time.ParseDuration(spec.Duration)
	if err != nil {
		return syncWindow{}, fmt.Errorf("invalid duration %q: %w", spec.Duration, err)
	}
	if duration <= 0 {
		return syncWindow{}, fmt.Errorf("duration must be positive, got %q", spec.Duration)
	}

	location := time.UTC
	if spec.TimeZone != "" {
		location, err = time.LoadLocation(spec.TimeZone)
		if err != nil {
			return syncWindow{}, fmt.Errorf("invalid time zone %q: %w", spec.TimeZone, err)
		}
	}

	spread := time.Duration(-1)
	if spec.Spread != "" {
		spread, err = time.ParseDuration(spec.Spread)
		if err != nil {
			return syncWindow{}, fmt.Errorf("invalid spread %q: %w", spec.Spread, err)
		}
		if spread < 0 {
			return syncWindow{}, fmt.Errorf("spread must not be negative, got %q", spec.Spread)
		}
	}

	// A schedule such as "0 0 30 2 *" parses but never fires, which would make an allow
	// window block reloads forever.
	if schedule.Next(now.In(location)).IsZero() {
		return syncWindow{}, fmt.Errorf("schedule %q never fires", spec.Schedule)
	}

	return syncWindow{kind: spec.Kind, schedule: schedule, duration: duration, location: location, spread: spread}, nil
}

// activeUntil reports whether the window is active at t and, if so, when the occurrence
// covering t ends. Occurrences cover [start, start+duration).
func (w syncWindow) activeUntil(t time.Time) (time.Time, bool) {
	t = t.In(w.location)
	start := w.schedule.Next(t.Add(-w.duration))
	if start.IsZero() || start.After(t) {
		return time.Time{}, false
	}
	// Occurrences overlap when the duration exceeds the schedule interval, and the window
	// stays active until the latest one that has started ends. An occurrence lies in
	// (t-lookback, t] exactly when lookback exceeds t minus the latest start, so a binary
	// search finds that start in a few dozen steps however many occurrences overlap.
	// Schedules fire on whole minutes, so one second of precision is exact.
	lo, hi := time.Duration(0), w.duration
	for hi-lo > time.Second {
		mid := lo + (hi-lo)/2
		if next := w.schedule.Next(t.Add(-mid)); !next.IsZero() && !next.After(t) {
			hi = mid
		} else {
			lo = mid
		}
	}
	if latest := w.schedule.Next(t.Add(-hi)); !latest.IsZero() && !latest.After(t) {
		start = latest
	}
	return start.Add(w.duration), true
}

// Allowed reports whether reloads are allowed at t.
func (ws SyncWindows) Allowed(t time.Time) bool {
	allowed, _ := ws.evaluate(t)
	return allowed
}

// NextAllowed returns the earliest time at or after t at which reloads are allowed, and
// true. If the search budget runs out first it returns the latest time examined and false;
// reloads are blocked until at least then.
func (ws SyncWindows) NextAllowed(t time.Time) (time.Time, bool) {
	for range maxSyncWindowSteps {
		allowed, boundary := ws.evaluate(t)
		if allowed {
			return t, true
		}
		if boundary.IsZero() {
			return t, false
		}
		t = boundary
	}
	return t, false
}

// NextBlocked returns the earliest time at or after t at which reloads are blocked, and
// true. It returns false when no blocked time is found within the search budget, for
// example when only deny windows far in the future exist.
func (ws SyncWindows) NextBlocked(t time.Time) (time.Time, bool) {
	for range maxSyncWindowSteps {
		allowed, boundary := ws.evaluate(t)
		if !allowed {
			return t, true
		}
		if boundary.IsZero() {
			return time.Time{}, false
		}
		t = boundary
	}
	return time.Time{}, false
}

// Spread returns how long deferred reloads are spread over after a window opens: the
// largest spread a window sets, or defaultSpread when none sets one.
func (ws SyncWindows) Spread(defaultSpread time.Duration) time.Duration {
	spread := time.Duration(-1)
	for _, w := range ws {
		spread = max(spread, w.spread)
	}
	if spread < 0 {
		return defaultSpread
	}
	return spread
}

// evaluate reports whether reloads are allowed at t, and the earliest instant after t at
// which that answer can change. While blocked by closed allow windows only an allow window
// opening can unblock; while blocked by a deny window only it closing can; while allowed,
// a deny window opening or an allow window closing can block.
func (ws SyncWindows) evaluate(t time.Time) (bool, time.Time) {
	type state struct {
		active   bool
		end      time.Time
		nextOpen time.Time
	}
	states := make([]state, len(ws))
	var hasAllow, inAllow, inDeny bool
	for i, w := range ws {
		end, active := w.activeUntil(t)
		states[i] = state{active: active, end: end}
		if !active {
			states[i].nextOpen = w.schedule.Next(t.In(w.location))
		}
		if w.kind == SyncWindowKindAllow {
			hasAllow = true
			inAllow = inAllow || active
		} else {
			inDeny = inDeny || active
		}
	}
	allowed := !inDeny && (!hasAllow || inAllow)

	var boundary time.Time
	consider := func(b time.Time) {
		if !b.IsZero() && (boundary.IsZero() || b.Before(boundary)) {
			boundary = b
		}
	}
	for i, w := range ws {
		st := states[i]
		isAllow := w.kind == SyncWindowKindAllow
		switch {
		case !allowed && hasAllow && !inAllow:
			if isAllow {
				consider(st.nextOpen)
			}
		case !allowed:
			if !isAllow && st.active {
				consider(st.end)
			}
		case isAllow && st.active:
			consider(st.end)
		case !isAllow && !st.active:
			consider(st.nextOpen)
		}
	}
	return allowed, boundary
}
