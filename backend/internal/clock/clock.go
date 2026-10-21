// Package clock is the single source of "what time is it" for everything
// attendance-related — recording a scan, deciding cutoffs, deciding what
// "today" is. Every one of those reads through Now() here rather than
// time.Now() directly, so the whole system's notion of the current time can
// be shifted in exactly one place without touching any calculation rule.
package clock

import "time"

// delay is how far behind the real wall clock this software's own "now"
// runs. Change this one constant to adjust it — every caller of Now()
// picks up the new value automatically.
const delay = 4 * time.Minute

// Now returns the time this software treats as "right now" — real time
// minus delay.
func Now() time.Time {
	return time.Now().Add(-delay)
}
