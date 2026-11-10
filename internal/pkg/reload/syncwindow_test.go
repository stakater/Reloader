package reload

import (
	"strings"
	"testing"
	"time"
)

func mustParseSyncWindows(t *testing.T, value string) SyncWindows {
	t.Helper()
	windows, err := ParseSyncWindows(value)
	if err != nil {
		t.Fatalf("ParseSyncWindows() error = %v", err)
	}
	return windows
}

func utc(hour, minute int) time.Time {
	return time.Date(2026, time.November, 10, hour, minute, 0, 0, time.UTC)
}

func TestParseSyncWindows(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantCount int
		wantErr   string
	}{
		{
			name: "yaml list from the issue",
			value: `- kind: allow
  schedule: "0 2 * * *"
  duration: 1h
  timeZone: Europe/Berlin
- kind: deny
  schedule: "0 22 * * *"
  duration: 2h
`,
			wantCount: 2,
		},
		{
			name:      "json list",
			value:     `[{"kind":"deny","schedule":"@daily","duration":"30m"}]`,
			wantCount: 1,
		},
		{
			name:      "empty list",
			value:     `[]`,
			wantCount: 0,
		},
		{
			name:    "not a list",
			value:   `kind: allow`,
			wantErr: "invalid sync windows",
		},
		{
			name:    "unknown field",
			value:   `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h","manualSync":true}]`,
			wantErr: "invalid sync windows",
		},
		{
			name:    "unknown kind",
			value:   `[{"kind":"Allow","schedule":"0 2 * * *","duration":"1h"}]`,
			wantErr: `kind must be "allow" or "deny"`,
		},
		{
			name:    "invalid schedule",
			value:   `[{"kind":"allow","schedule":"0 2 * *","duration":"1h"}]`,
			wantErr: "invalid schedule",
		},
		{
			name:    "time zone inside schedule",
			value:   `[{"kind":"allow","schedule":"CRON_TZ=UTC 0 2 * * *","duration":"1h"}]`,
			wantErr: "use timeZone instead",
		},
		{
			name:    "missing duration",
			value:   `[{"kind":"allow","schedule":"0 2 * * *"}]`,
			wantErr: "invalid duration",
		},
		{
			name:    "zero duration",
			value:   `[{"kind":"allow","schedule":"0 2 * * *","duration":"0s"}]`,
			wantErr: "duration must be positive",
		},
		{
			name:    "negative duration",
			value:   `[{"kind":"allow","schedule":"0 2 * * *","duration":"-1h"}]`,
			wantErr: "duration must be positive",
		},
		{
			name:    "unknown time zone",
			value:   `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h","timeZone":"Mars/Olympus"}]`,
			wantErr: "invalid time zone",
		},
		{
			name:    "schedule that never fires",
			value:   `[{"kind":"allow","schedule":"0 0 30 2 *","duration":"1h"}]`,
			wantErr: "never fires",
		},
		{
			name:    "@every is not a calendar schedule",
			value:   `[{"kind":"deny","schedule":"@every 1h","duration":"30m"}]`,
			wantErr: "must be a cron expression",
		},
		{
			name:      "calendar descriptor",
			value:     `[{"kind":"allow","schedule":"@weekly","duration":"1h"}]`,
			wantCount: 1,
		},
		{
			name:    "too many windows",
			value:   "[" + strings.Repeat(`{"kind":"deny","schedule":"0 2 * * *","duration":"1h"},`, 16) + `{"kind":"deny","schedule":"0 2 * * *","duration":"1h"}]`,
			wantErr: "at most 16 windows",
		},
		{
			name:    "invalid spread",
			value:   `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h","spread":"soon"}]`,
			wantErr: "invalid spread",
		},
		{
			name:    "negative spread",
			value:   `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h","spread":"-1m"}]`,
			wantErr: "spread must not be negative",
		},
		{
			name: "error names the failing window",
			value: `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h"},
				{"kind":"deny","schedule":"0 2 * * *","duration":"soon"}]`,
			wantErr: "invalid sync window 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			windows, err := ParseSyncWindows(tt.value)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseSyncWindows() error = nil, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseSyncWindows() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSyncWindows() error = %v", err)
			}
			if len(windows) != tt.wantCount {
				t.Errorf("ParseSyncWindows() returned %d windows, want %d", len(windows), tt.wantCount)
			}
		})
	}
}

func TestSyncWindows_NextAllowed(t *testing.T) {
	tests := []struct {
		name    string
		windows string
		at      time.Time
		want    time.Time
	}{
		{
			name:    "no windows always allows",
			windows: `[]`,
			at:      utc(10, 0),
			want:    utc(10, 0),
		},
		{
			name:    "inside allow window",
			windows: `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h"}]`,
			at:      utc(2, 30),
			want:    utc(2, 30),
		},
		{
			name:    "allow window opens at its start",
			windows: `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h"}]`,
			at:      utc(2, 0),
			want:    utc(2, 0),
		},
		{
			name:    "allow window is closed at its end",
			windows: `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h"}]`,
			at:      utc(3, 0),
			want:    utc(2, 0).AddDate(0, 0, 1),
		},
		{
			name:    "outside allow window waits for the next one",
			windows: `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h"}]`,
			at:      utc(10, 0),
			want:    utc(2, 0).AddDate(0, 0, 1),
		},
		{
			name:    "outside deny window allows",
			windows: `[{"kind":"deny","schedule":"0 22 * * *","duration":"2h"}]`,
			at:      utc(21, 0),
			want:    utc(21, 0),
		},
		{
			name:    "inside deny window waits for it to close",
			windows: `[{"kind":"deny","schedule":"0 22 * * *","duration":"2h"}]`,
			at:      utc(23, 0),
			want:    utc(0, 0).AddDate(0, 0, 1),
		},
		{
			name: "deny inside allow wins",
			windows: `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h"},
				{"kind":"deny","schedule":"30 2 * * *","duration":"10m"}]`,
			at:   utc(2, 35),
			want: utc(2, 40),
		},
		{
			name: "deny covering the rest of the allow window waits for the next allow window",
			windows: `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h"},
				{"kind":"deny","schedule":"30 2 * * *","duration":"1h"}]`,
			at:   utc(2, 35),
			want: utc(2, 0).AddDate(0, 0, 1),
		},
		{
			name: "any active allow window allows",
			windows: `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h"},
				{"kind":"allow","schedule":"0 12 * * *","duration":"1h"}]`,
			at:   utc(12, 15),
			want: utc(12, 15),
		},
		{
			name:    "overlapping occurrences extend the window",
			windows: `[{"kind":"deny","schedule":"0 0,1 * * *","duration":"90m"}]`,
			at:      utc(0, 10),
			want:    utc(2, 30),
		},
		{
			name:    "schedule is evaluated in its time zone",
			windows: `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h","timeZone":"Europe/Berlin"}]`,
			// 02:00 in Berlin is 01:00 UTC in November (CET, UTC+1).
			at:   utc(2, 30),
			want: utc(1, 0).AddDate(0, 0, 1),
		},
		{
			name:    "inside a window in another time zone",
			windows: `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h","timeZone":"Europe/Berlin"}]`,
			at:      utc(1, 30),
			want:    utc(1, 30),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			windows := mustParseSyncWindows(t, tt.windows)
			got, found := windows.NextAllowed(tt.at)
			if !found {
				t.Fatalf("NextAllowed(%s) found no allowed time", tt.at)
			}
			if !got.Equal(tt.want) {
				t.Errorf("NextAllowed(%s) = %s, want %s", tt.at, got.UTC(), tt.want)
			}
			if allowed := windows.Allowed(tt.at); allowed != tt.at.Equal(tt.want) {
				t.Errorf("Allowed(%s) = %v, want %v", tt.at, allowed, tt.at.Equal(tt.want))
			}
		})
	}
}

func TestSyncWindows_NextAllowed_AlwaysDenied(t *testing.T) {
	// Hourly occurrences lasting 90 minutes never leave a gap.
	windows := mustParseSyncWindows(t, `[{"kind":"deny","schedule":"0 * * * *","duration":"90m"}]`)
	at := utc(10, 0)

	got, found := windows.NextAllowed(at)
	if found {
		t.Fatalf("NextAllowed() found %s, want no allowed time", got)
	}
	if !got.After(at) {
		t.Errorf("NextAllowed() = %s, want a time after %s to evaluate again", got, at)
	}
	if windows.Allowed(got) {
		t.Errorf("Allowed(%s) = true, want the returned time to still be blocked", got)
	}
}

func TestSyncWindows_NextBlocked(t *testing.T) {
	tests := []struct {
		name      string
		windows   string
		at        time.Time
		want      time.Time
		wantFound bool
	}{
		{
			name:      "allow window closes at its end",
			windows:   `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h"}]`,
			at:        utc(2, 0),
			want:      utc(3, 0),
			wantFound: true,
		},
		{
			name:      "deny window blocks at its start",
			windows:   `[{"kind":"deny","schedule":"0 22 * * *","duration":"2h"}]`,
			at:        utc(10, 0),
			want:      utc(22, 0),
			wantFound: true,
		},
		{
			name:      "already blocked",
			windows:   `[{"kind":"deny","schedule":"0 9 * * *","duration":"2h"}]`,
			at:        utc(10, 0),
			want:      utc(10, 0),
			wantFound: true,
		},
		{
			name:    "no windows never block",
			windows: `[]`,
			at:      utc(10, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := mustParseSyncWindows(t, tt.windows).NextBlocked(tt.at)
			if found != tt.wantFound {
				t.Fatalf("NextBlocked(%s) found = %v, want %v", tt.at, found, tt.wantFound)
			}
			if found && !got.Equal(tt.want) {
				t.Errorf("NextBlocked(%s) = %s, want %s", tt.at, got.UTC(), tt.want)
			}
		})
	}
}

func TestSyncWindows_Spread(t *testing.T) {
	const def = 10 * time.Minute
	tests := []struct {
		name    string
		windows string
		want    time.Duration
	}{
		{"no window sets a spread", `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h"}]`, def},
		{"zero spread", `[{"kind":"allow","schedule":"0 2 * * *","duration":"1h","spread":"0s"}]`, 0},
		{
			"largest spread wins",
			`[{"kind":"allow","schedule":"0 2 * * *","duration":"1h","spread":"5m"},
			  {"kind":"deny","schedule":"0 22 * * *","duration":"1h","spread":"20m"}]`,
			20 * time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustParseSyncWindows(t, tt.windows).Spread(def); got != tt.want {
				t.Errorf("Spread() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSyncWindows_LongOverlappingWindowsEvaluateQuickly(t *testing.T) {
	// Every minute a window opens that lasts for years: each instant is covered by millions
	// of occurrences, which must not be walked one by one.
	windows := mustParseSyncWindows(t, `[
		{"kind":"deny","schedule":"* * * * *","duration":"87600h"},
		{"kind":"allow","schedule":"* * * * *","duration":"87600h"}]`)
	start := time.Now()
	windows.NextAllowed(utc(10, 0))
	windows.NextBlocked(utc(10, 0))
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("evaluating took %s, want well under a second", elapsed)
	}
}
