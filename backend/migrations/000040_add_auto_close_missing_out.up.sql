-- AUTO_CLOSED marks a synthetic OUT event the system inserted itself because
-- an employee scanned IN but never scanned OUT that day — distinct from a
-- real face scan, an admin's manual correction, or an employee's own
-- self-report, so reports/exports can always tell the three apart.
ALTER TYPE attendance_source ADD VALUE IF NOT EXISTS 'AUTO_CLOSED';

ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'ATTENDANCE_AUTO_CLOSED';
