package policyschedule

import (
	"testing"
	"time"
)

func TestWindow(t *testing.T) {
	raw := `{"days":[1,2,3,4,5],"start":"09:00","end":"17:00","tz":"UTC"}`
	w, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	inside := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC) // Tuesday
	if !w.Allows(inside) {
		t.Fatal("Tuesday 10:00 must be allowed")
	}
	if w.Allows(time.Date(2026, 10, 27, 18, 0, 0, 0, time.UTC)) {
		t.Fatal("Tuesday 18:00 must be blocked")
	}
	if w.Allows(time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)) {
		t.Fatal("Sunday must be blocked")
	}
	if _, err := Parse(`{"days":[1],"start":"17:00","end":"09:00"}`); err == nil {
		t.Fatal("expected start/end validation")
	}
}
