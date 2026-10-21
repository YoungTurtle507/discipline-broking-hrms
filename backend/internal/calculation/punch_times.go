package calculation

import "time"

// PunchTimes derives Office In/Out and the recognized Lunch break (if any)
// from one day's time-ordered raw punches — shared by the Daily Timesheet
// export, the per-employee daily breakdown (reports.EmployeeDailyBreakdown),
// and an employee's own attendance page (selfservice), so "what time did
// they arrive/leave" is computed exactly the same way everywhere it's shown.
// Only a break tagged LUNCH (or the legacy pre-split BREAK) ever populates
// lunchOut/lunchIn — a Personal (or any other) break never does; its own time
// is reported separately by whichever caller needs it (see
// reports.combinedReason for the admin-facing version).
func PunchTimes(events []RawEvent) (officeIn, lunchOut, lunchIn, officeOut *time.Time, lunchMinutes int) {
	if len(events) == 0 {
		return nil, nil, nil, nil, 0
	}

	first := events[0].EventTime
	officeIn = &first

	if last := events[len(events)-1]; last.Direction == "OUT" {
		t := last.EventTime
		officeOut = &t
	}

	for i := 1; i < len(events)-1; i++ {
		if events[i].Direction != "OUT" || (events[i].OutReason != "LUNCH" && events[i].OutReason != "BREAK") {
			continue
		}
		out := events[i].EventTime
		lunchOut = &out
		if events[i+1].Direction == "IN" {
			in := events[i+1].EventTime
			lunchIn = &in
			lunchMinutes = int(in.Sub(out).Minutes())
		}
		break
	}

	return officeIn, lunchOut, lunchIn, officeOut, lunchMinutes
}
