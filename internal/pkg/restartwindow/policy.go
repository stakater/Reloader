// Copyright 2026 Doska. Licensed under the Apache License, Version 2.0.
package restartwindow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/robfig/cron/v3"
)

const (
	PolicyAnnotation  = "reloader.stakater.com/restart-window"
	PendingAnnotation = "reloader.stakater.com/pending"
	AppliedAnnotation = "reloader.stakater.com/applied"
	RestartAnnotation = "reloader.stakater.com/restart-hash"
)

type Window struct {
	Cron     string `json:"cron"`
	Duration string `json:"duration"`
}
type Policy struct {
	Timezone string   `json:"timezone"`
	Windows  []Window `json:"windows"`
}

// Evaluate returns whether a rollout may START now and the next opening.
// Windows are half-open [start, end); durations are elapsed time across DST.
func Evaluate(raw string, now time.Time) (bool, time.Time, error) {
	if raw == "" {
		return true, time.Time{}, nil
	}
	var p Policy
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return false, time.Time{}, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return false, time.Time{}, fmt.Errorf("expected one window policy")
	}
	if p.Timezone == "" || p.Timezone == "Local" {
		return false, time.Time{}, fmt.Errorf("explicit IANA timezone required")
	}
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return false, time.Time{}, err
	}
	if len(p.Windows) == 0 || len(p.Windows) > 16 {
		return false, time.Time{}, fmt.Errorf("expected 1 to 16 windows")
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	open := false
	var next time.Time
	for _, w := range p.Windows {
		if strings.Contains(w.Cron, "TZ=") {
			return false, time.Time{}, fmt.Errorf("set timezone in policy, not cron")
		}
		schedule, err := parser.Parse(w.Cron)
		if err != nil {
			return false, time.Time{}, err
		}
		duration, err := time.ParseDuration(w.Duration)
		if err != nil || duration < time.Minute || duration > 24*time.Hour {
			return false, time.Time{}, fmt.Errorf("window duration must be between 1m and 24h")
		}
		// Any start strictly after now-duration and <= now opens the window.
		start := schedule.Next(now.In(loc).Add(-duration))
		if !start.IsZero() && !start.After(now) {
			open = true
		}
		candidate := schedule.Next(now.In(loc))
		if candidate.IsZero() {
			return false, time.Time{}, fmt.Errorf("cron has no next occurrence")
		}
		if next.IsZero() || candidate.Before(next) {
			next = candidate
		}
	}
	return open, next, nil
}
