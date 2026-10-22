package leave

import "time"

// ValidLeaveTypes are the leave types an applicant can pick. HALF_DAY also
// exists on the leave_type enum but is deliberately excluded here — this
// duplicate's attendance module already has its own independent Half Day
// mechanism (the OUT_REASON prompt during a mid-day OUT scan), so a
// user-applied "Half Day leave" would double up with that rather than
// complement it.
var ValidLeaveTypes = map[string]bool{"PAID": true, "COMPOFF": true, "UNPAID": true}
var ValidDecisions = map[string]bool{"APPROVED": true, "REJECTED": true}

type Request struct {
	ID            int64      `json:"id"`
	EmployeeID    int64      `json:"employee_id"`
	EmployeeName  string     `json:"employee_name"`
	LeaveType     string     `json:"leave_type"`
	StartDate     string     `json:"start_date"`
	EndDate       string     `json:"end_date"`
	Reason        string     `json:"reason"`
	Status        string     `json:"status"`
	RequestedAt   time.Time  `json:"requested_at"`
	DecidedBy     *int64     `json:"decided_by"`
	DecidedByName *string    `json:"decided_by_name"`
	DecidedAt     *time.Time `json:"decided_at"`
	// DocumentPath is where the supporting document was saved, if one was
	// attached — never sent verbatim to the client; HasDocument is the field
	// the frontend reads.
	DocumentPath *string `json:"-"`
	HasDocument  bool    `json:"has_document"`
}
