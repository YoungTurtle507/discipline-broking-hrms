-- A leave request's supporting document (e.g. a medical certificate) can be
-- uploaded when applying, or later — some proof simply doesn't exist yet at
-- application time (a sick leave taken before a doctor's visit).
ALTER TABLE leave_requests ADD COLUMN document_path text;
