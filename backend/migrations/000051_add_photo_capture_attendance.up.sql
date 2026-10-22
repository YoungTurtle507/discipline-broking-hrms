-- PHOTO_CAPTURE replaces FACE going forward: attendance marking now picks
-- the employee explicitly (no 1:N face match), and just captures a live,
-- anti-spoof-checked photo stamped with date/time. FACE/SELF/AUTO_CLOSED
-- stay in the enum so existing historical events keep their recorded source.
ALTER TYPE attendance_source ADD VALUE 'PHOTO_CAPTURE';

-- Path (relative to STORAGE_PATH) to the stamped capture photo for this
-- event, e.g. "attendance-photos/37/1724.jpg". NULL for every pre-existing
-- row and for a MANUAL correction, which never has a captured photo.
ALTER TABLE attendance_events ADD COLUMN photo_path TEXT;
