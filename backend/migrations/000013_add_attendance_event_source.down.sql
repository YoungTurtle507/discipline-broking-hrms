DROP INDEX idx_attendance_events_source;
ALTER TABLE attendance_events DROP COLUMN source, DROP COLUMN note;
DROP TYPE attendance_source;
