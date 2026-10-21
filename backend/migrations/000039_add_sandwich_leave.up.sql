-- Sandwich Leave: when an employee's Unpaid leave ends right before a
-- Weekly Off/Holiday run and another Unpaid leave request picks up right
-- after it, the gap no longer stays a free day — it gets auto-filled with
-- its own leave_requests row (is_sandwich = true) and ledger deduction, the
-- same as if the employee had taken it directly. is_sandwich distinguishes
-- these system-generated rows from a normal request that merely happens to
-- span a holiday within its own dates (which still lets the holiday win —
-- see calculation.DayContext.IsSandwichedLeave).
ALTER TABLE leave_requests ADD COLUMN is_sandwich BOOLEAN NOT NULL DEFAULT false;

ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'SANDWICH_LEAVE_APPLIED';
