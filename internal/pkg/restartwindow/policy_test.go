package restartwindow

import (
	"testing"
	"time"
)

const nightly = `{"timezone":"Europe/Moscow","windows":[{"cron":"0 2 * * *","duration":"1h"}]}`

func at(s string) time.Time {
	t, e := time.Parse(time.RFC3339, s)
	if e != nil {
		panic(e)
	}
	return t
}
func TestWindowBoundaries(t *testing.T) {
	for _, tt := range []struct {
		at   string
		open bool
	}{
		{"2026-09-09T22:59:59Z", false}, {"2026-09-09T23:00:00Z", true},
		{"2026-09-09T23:59:59Z", true}, {"2026-09-10T00:00:00Z", false},
	} {
		t.Run(tt.at, func(t *testing.T) {
			open, _, e := Evaluate(nightly, at(tt.at))
			if e != nil || open != tt.open {
				t.Fatalf("open=%v err=%v", open, e)
			}
		})
	}
	_, next, e := Evaluate(nightly, at("2026-09-09T22:59:59Z"))
	if e != nil || !next.Equal(at("2026-09-09T23:00:00Z")) {
		t.Fatal(next, e)
	}
}
func TestWindowPolicyValidation(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"timezone":"UTC","windows":[]}`, `{"timezone":"Local","windows":[{"cron":"0 2 * * *","duration":"1h"}]}`, `{"timezone":"Invalid/Zone","windows":[{"cron":"0 2 * * *","duration":"1h"}]}`, `{"timezone":"UTC","windows":[{"cron":"@every 1h","duration":"1h"}]}`, `{"timezone":"UTC","windows":[{"cron":"0 2 * * *","duration":"0s"}]}`, `{"timezone":"UTC","windows":[{"cron":"0 2 * * *","duration":"25h"}]}`, `{"timezone":"UTC","windows":[{"cron":"0 2 * * *","duration":"1h","typo":true}]}`, nightly + ` {}`} {
		if _, _, err := Evaluate(raw, time.Now()); err == nil {
			t.Errorf("accepted invalid policy %s", raw)
		}
	}
	open, _, err := Evaluate("", time.Now())
	if !open || err != nil {
		t.Fatal("absent policy changed default")
	}
}
func TestOvernightAndDST(t *testing.T) {
	for _, tt := range []struct {
		policy, instant string
		want            bool
	}{
		{`{"timezone":"UTC","windows":[{"cron":"0 23 * * *","duration":"2h"}]}`, "2026-09-10T00:30:00Z", true},
		{`{"timezone":"America/New_York","windows":[{"cron":"0 1 * * *","duration":"2h"}]}`, "2026-03-08T07:30:00Z", true},
		{`{"timezone":"America/New_York","windows":[{"cron":"0 1 * * *","duration":"2h"}]}`, "2026-03-08T08:00:00Z", false},
		{`{"timezone":"America/New_York","windows":[{"cron":"0 1 * * *","duration":"1h"}]}`, "2026-11-01T06:30:00Z", true},
	} {
		o, _, e := Evaluate(tt.policy, at(tt.instant))
		if e != nil || o != tt.want {
			t.Errorf("%s: open=%v err=%v", tt.instant, o, e)
		}
	}
}
