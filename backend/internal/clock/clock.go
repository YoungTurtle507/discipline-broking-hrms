// Package clock is the single source of "what time is it" for everything
// attendance-related — recording a scan, deciding cutoffs, deciding what
// "today" is. Every one of those reads through Now() here rather than
// time.Now() directly, so the whole system's notion of the current time can
// be adjusted in exactly one place without touching any calculation rule.
package clock

import "time"

// Now returns the time this software treats as "right now" — real wall-clock
// time, exactly, with no offset.
func Now() time.Time {
	return time.Now()
}
