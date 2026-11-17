package alert

import (
	"testing"
	"time"
)

func TestPunchMessageConvertsUTCToIST(t *testing.T) {
	utc := time.Date(2026, 11, 17, 4, 12, 0, 0, time.UTC) // 09:42 IST
	title, msg := PunchMessage("IN", "Asha Rao", utc)
	if title != "OFFICE IN" || msg != "Asha Rao Office In at 09:42 AM" {
		t.Fatalf("got %q / %q", title, msg)
	}
	title, msg = PunchMessage("OUT", "Asha Rao", time.Date(2026, 11, 17, 12, 30, 0, 0, time.UTC))
	if title != "OFFICE OUT" || msg != "Asha Rao Office Out at 06:00 PM" {
		t.Fatalf("got %q / %q", title, msg)
	}
}
