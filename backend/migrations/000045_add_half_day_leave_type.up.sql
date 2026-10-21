-- A Half Day taken via a mid-day OUT scan (attendance.ValidOutReasons
-- ["HALF_DAY"]) now also gets its own leave_requests row — auto-approved,
-- since it documents something that already happened rather than something
-- awaiting a decision, with the same optional supporting-document upload
-- every other leave type already has. See leave.Repository.RecordHalfDay.
--
-- Deliberately excluded from calculation.Repository.hasApprovedLeave (and its
-- summary/range equivalents) so it's never mistaken for a full day's approved
-- leave and doesn't override the day's actual worked-hours-derived status.
ALTER TYPE leave_type ADD VALUE 'HALF_DAY';
