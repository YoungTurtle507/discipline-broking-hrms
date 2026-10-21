package calculation

import (
	"context"
	"sync/atomic"
)

// PolicySettings holds every calculation-affecting number a Super Admin can
// edit from Organization → Calculation Settings. Each field mirrors what used
// to be a hardcoded constant in engine.go/range_summary.go — see those files'
// own comments for what each one actually controls. UninformedLeavePenaltyDays
// isn't read by CalculateDay itself (it's applied in repository.go's ledger
// write, not the pure calculation), the rest are threaded through
// DayContext.Policy.
type PolicySettings struct {
	LateStreakLimit              int `json:"late_streak_limit"`
	HalfDayCutoffHour            int `json:"half_day_cutoff_hour"`
	HalfDayCutoffMinute          int `json:"half_day_cutoff_minute"`
	AbsentCutoffHour             int `json:"absent_cutoff_hour"`
	AbsentCutoffMinute           int `json:"absent_cutoff_minute"`
	StayLatePassThresholdMinutes int `json:"stay_late_pass_threshold_minutes"`
	UninformedLeavePenaltyDays   int `json:"uninformed_leave_penalty_days"`
	// EndOfShiftGraceMinutes is how early an OUT scan can land before the
	// employee's own shift end and still count as a normal, no-reason-needed
	// Office Out — not read by CalculateDay itself, only by
	// Repository.IsBeforeShiftEnd (see attendance.needsOutReason). The
	// mirror-image of a shift's own start-of-day grace period, at the other
	// end of the day.
	EndOfShiftGraceMinutes int `json:"end_of_shift_grace_minutes"`
	// LunchTimeLimitMinutes is how long a LUNCH-tagged break can run before
	// it counts as Lunch OT for that day — read by CalculateDay against the
	// gap between the LUNCH OUT and the next IN (see PunchTimes).
	LunchTimeLimitMinutes int `json:"lunch_time_limit_minutes"`
	// LateBand1EndMinutes/LateBand2EndMinutes and their *StreakLimit
	// counterparts are the two-band late-arrival streak system: Band 1 runs
	// from the employee's own shift start to LateBand1EndMinutes later, Band 2
	// from there to LateBand2EndMinutes later — each with its own independent
	// monthly strike count (every Nth late arrival in that band is a strike).
	// Not yet read by CalculateDay itself — LateStreakLimit and the existing
	// single late window remain the engine's actual behavior until it's
	// updated to use these instead. Configurable now so a Super Admin can set
	// the numbers ahead of that change taking effect.
	LateBand1EndMinutes  int `json:"late_band1_end_minutes"`
	LateBand1StreakLimit int `json:"late_band1_streak_limit"`
	LateBand2EndMinutes  int `json:"late_band2_end_minutes"`
	LateBand2StreakLimit int `json:"late_band2_streak_limit"`
	// LunchOTStreakLimit/LunchOTPenaltyDays are Lunch OT's own monthly strike
	// system, same shape as the Late Streak above: every LunchOTStreakLimit-th
	// Lunch OT day (see LunchTimeLimitMinutes/ExLunchOT) this month deducts
	// LunchOTPenaltyDays from the employee's leave balance via a ledger entry
	// — unlike Late Streak, this never changes that day's own Status, since a
	// long lunch doesn't make the rest of the day a Half Day on its own.
	LunchOTStreakLimit int     `json:"lunch_ot_streak_limit"`
	LunchOTPenaltyDays float64 `json:"lunch_ot_penalty_days"`
}

// DefaultPolicySettings are this engine's own long-standing values — what
// every one of these was hardcoded to before this feature existed. A fresh
// install with no settings row, or a DayContext that simply doesn't set
// Policy (every existing test, for instance), behaves exactly as before.
func DefaultPolicySettings() PolicySettings {
	return PolicySettings{
		LateStreakLimit:              3,
		HalfDayCutoffHour:            10,
		HalfDayCutoffMinute:          31,
		AbsentCutoffHour:             14,
		AbsentCutoffMinute:           36,
		StayLatePassThresholdMinutes: 30,
		UninformedLeavePenaltyDays:   2,
		EndOfShiftGraceMinutes:       10,
		LunchTimeLimitMinutes:        60,
		LateBand1EndMinutes:          30,
		LateBand1StreakLimit:         3,
		LateBand2EndMinutes:          90,
		LateBand2StreakLimit:         2,
		LunchOTStreakLimit:           3,
		LunchOTPenaltyDays:           0.5,
	}
}

// resolvePolicy fills in any zero-valued field of p with its default.
// CalculateDay calls this on ctx.Policy so a caller that never had a reason
// to care about these — overwhelmingly, every existing test — gets identical
// behavior to before this feature existed without needing to set anything.
func resolvePolicy(p PolicySettings) PolicySettings {
	d := DefaultPolicySettings()
	if p.LateStreakLimit == 0 {
		p.LateStreakLimit = d.LateStreakLimit
	}
	if p.HalfDayCutoffHour == 0 && p.HalfDayCutoffMinute == 0 {
		p.HalfDayCutoffHour, p.HalfDayCutoffMinute = d.HalfDayCutoffHour, d.HalfDayCutoffMinute
	}
	if p.AbsentCutoffHour == 0 && p.AbsentCutoffMinute == 0 {
		p.AbsentCutoffHour, p.AbsentCutoffMinute = d.AbsentCutoffHour, d.AbsentCutoffMinute
	}
	if p.StayLatePassThresholdMinutes == 0 {
		p.StayLatePassThresholdMinutes = d.StayLatePassThresholdMinutes
	}
	if p.UninformedLeavePenaltyDays == 0 {
		p.UninformedLeavePenaltyDays = d.UninformedLeavePenaltyDays
	}
	if p.EndOfShiftGraceMinutes == 0 {
		p.EndOfShiftGraceMinutes = d.EndOfShiftGraceMinutes
	}
	if p.LunchTimeLimitMinutes == 0 {
		p.LunchTimeLimitMinutes = d.LunchTimeLimitMinutes
	}
	if p.LateBand1EndMinutes == 0 {
		p.LateBand1EndMinutes = d.LateBand1EndMinutes
	}
	if p.LateBand1StreakLimit == 0 {
		p.LateBand1StreakLimit = d.LateBand1StreakLimit
	}
	if p.LateBand2EndMinutes == 0 {
		p.LateBand2EndMinutes = d.LateBand2EndMinutes
	}
	if p.LateBand2StreakLimit == 0 {
		p.LateBand2StreakLimit = d.LateBand2StreakLimit
	}
	if p.LunchOTStreakLimit == 0 {
		p.LunchOTStreakLimit = d.LunchOTStreakLimit
	}
	if p.LunchOTPenaltyDays == 0 {
		p.LunchOTPenaltyDays = d.LunchOTPenaltyDays
	}
	return p
}

// currentPolicy is the live, admin-configured settings every production
// call site reads when building a DayContext (see summary.go, range_summary.go,
// repository.go) — loaded from the database once at startup and swapped
// whenever a Super Admin saves an edit (see Repository.SetPolicySettings), so
// a change takes effect immediately without a server restart. Tests never
// touch this — they either leave DayContext.Policy unset (resolvePolicy
// covers it) or set exactly the values a given test cares about.
var currentPolicy atomic.Pointer[PolicySettings]

func init() {
	d := DefaultPolicySettings()
	currentPolicy.Store(&d)
}

// CurrentPolicySettings returns the live settings — safe to call from any
// request.
func CurrentPolicySettings() PolicySettings {
	return *currentPolicy.Load()
}

// SetCurrentPolicySettings replaces the live settings in memory. Called at
// startup after loading from the database, and again by
// Repository.SetPolicySettings after a successful save.
func SetCurrentPolicySettings(p PolicySettings) {
	resolved := resolvePolicy(p)
	currentPolicy.Store(&resolved)
}

// LoadPolicySettings reads the single settings row from the database — the
// durable copy behind the in-memory cache every calculation reads from
// (CurrentPolicySettings). Called once at server startup.
func (r *Repository) LoadPolicySettings(ctx context.Context) (PolicySettings, error) {
	var p PolicySettings
	err := r.db.QueryRow(ctx,
		`SELECT late_streak_limit, half_day_cutoff_hour, half_day_cutoff_minute,
		        absent_cutoff_hour, absent_cutoff_minute, stay_late_pass_threshold_minutes,
		        uninformed_leave_penalty_days, end_of_shift_grace_minutes, lunch_time_limit_minutes,
		        late_band1_end_minutes, late_band1_streak_limit, late_band2_end_minutes, late_band2_streak_limit,
		        lunch_ot_streak_limit, lunch_ot_penalty_days
		 FROM calculation_settings WHERE id = 1`,
	).Scan(
		&p.LateStreakLimit, &p.HalfDayCutoffHour, &p.HalfDayCutoffMinute,
		&p.AbsentCutoffHour, &p.AbsentCutoffMinute, &p.StayLatePassThresholdMinutes,
		&p.UninformedLeavePenaltyDays, &p.EndOfShiftGraceMinutes, &p.LunchTimeLimitMinutes,
		&p.LateBand1EndMinutes, &p.LateBand1StreakLimit, &p.LateBand2EndMinutes, &p.LateBand2StreakLimit,
		&p.LunchOTStreakLimit, &p.LunchOTPenaltyDays,
	)
	return p, err
}

// SavePolicySettings persists an edited settings row and immediately swaps
// the in-memory cache to match — a Super Admin's change takes effect on the
// very next calculation, not after a restart.
func (r *Repository) SavePolicySettings(ctx context.Context, p PolicySettings, updatedBy int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE calculation_settings SET
		    late_streak_limit = $1, half_day_cutoff_hour = $2, half_day_cutoff_minute = $3,
		    absent_cutoff_hour = $4, absent_cutoff_minute = $5, stay_late_pass_threshold_minutes = $6,
		    uninformed_leave_penalty_days = $7, end_of_shift_grace_minutes = $8, lunch_time_limit_minutes = $9,
		    late_band1_end_minutes = $10, late_band1_streak_limit = $11,
		    late_band2_end_minutes = $12, late_band2_streak_limit = $13,
		    lunch_ot_streak_limit = $14, lunch_ot_penalty_days = $15,
		    updated_at = now(), updated_by = $16
		 WHERE id = 1`,
		p.LateStreakLimit, p.HalfDayCutoffHour, p.HalfDayCutoffMinute,
		p.AbsentCutoffHour, p.AbsentCutoffMinute, p.StayLatePassThresholdMinutes,
		p.UninformedLeavePenaltyDays, p.EndOfShiftGraceMinutes, p.LunchTimeLimitMinutes,
		p.LateBand1EndMinutes, p.LateBand1StreakLimit, p.LateBand2EndMinutes, p.LateBand2StreakLimit,
		p.LunchOTStreakLimit, p.LunchOTPenaltyDays,
		updatedBy,
	)
	if err != nil {
		return err
	}
	SetCurrentPolicySettings(p)
	return nil
}
