// Package policyschedule parses project change windows.
package policyschedule

import (
	"encoding/json"
	"fmt"
	"time"
)

// Window is a weekly allowed merge window.
type Window struct {
	Days  []int  `json:"days"`  // ISO weekdays 1..7
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
	TZ    string `json:"tz"`    // IANA zone
}

// Parse validates JSON and returns a Window.
func Parse(raw string) (Window, error) {
	var w Window
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		return Window{}, fmt.Errorf("change window: invalid JSON: %w", err)
	}
	if len(w.Days) == 0 {
		return Window{}, fmt.Errorf("change window: days are required")
	}
	for _, day := range w.Days {
		if day < 1 || day > 7 {
			return Window{}, fmt.Errorf("change window: days must be 1..7")
		}
	}
	if _, err := time.Parse("15:04", w.Start); err != nil {
		return Window{}, fmt.Errorf("change window: start must be HH:MM")
	}
	if _, err := time.Parse("15:04", w.End); err != nil {
		return Window{}, fmt.Errorf("change window: end must be HH:MM")
	}
	if w.Start >= w.End {
		return Window{}, fmt.Errorf("change window: start must be before end")
	}
	if w.TZ == "" {
		w.TZ = "UTC"
	}
	if _, err := time.LoadLocation(w.TZ); err != nil {
		return Window{}, fmt.Errorf("change window: unknown time zone")
	}
	return w, nil
}

// Allows reports whether t is inside the window.
func (w Window) Allows(t time.Time) bool {
	loc, err := time.LoadLocation(w.TZ)
	if err != nil {
		return false
	}
	local := t.In(loc)
	for _, day := range w.Days {
		if int(local.Weekday()) == day%7 { // Go Sunday=0; ISO Sunday=7
			clock := local.Format("15:04")
			return clock >= w.Start && clock < w.End
		}
	}
	return false
}
