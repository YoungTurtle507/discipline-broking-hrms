-- Attendance events can now come from an admin as well as a face scan. The
-- spec is explicit that corrections are additive records, never overwrites, so
-- a forgotten check-out is fixed by adding the missing event rather than
-- editing anything — and the source records how each event got there, so a
-- manually entered scan is never mistaken for a verified face match.
CREATE TYPE attendance_source AS ENUM ('FACE', 'MANUAL');

ALTER TABLE attendance_events
    ADD COLUMN source attendance_source NOT NULL DEFAULT 'FACE',
    ADD COLUMN note TEXT NOT NULL DEFAULT '';

-- Every existing row came from the face pipeline, which the default already
-- records correctly.

CREATE INDEX idx_attendance_events_source ON attendance_events(source)
    WHERE source = 'MANUAL';
