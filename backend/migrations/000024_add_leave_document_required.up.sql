-- Whether this request's document upload was ever promised at all — the
-- upload itself always stayed optional/attach-later, but the post-return
-- "you still haven't uploaded it" reminder should only ever fire for a
-- request where the employee actually checked the "I'll provide a document"
-- box when applying. Existing rows default to true so nobody already in the
-- reminder pipeline silently drops out of it.
ALTER TABLE leave_requests ADD COLUMN document_required boolean NOT NULL DEFAULT true;
