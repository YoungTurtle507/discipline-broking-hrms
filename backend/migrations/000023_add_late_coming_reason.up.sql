-- A fourth reason, valid only on an IN: a manual correction can flag an
-- arrival as "Late Coming" even when the recorded time itself would read as
-- on-time, forcing late-arrival classification. Not offered by the live
-- kiosk — arrival time alone already decides lateness there.
ALTER TYPE attendance_out_reason ADD VALUE 'LATE_COMING';
