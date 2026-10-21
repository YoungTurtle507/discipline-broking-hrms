-- One row per employee per document type — Offer Letter, Aadhar Card, and a
-- single Marksheet slot (10th or 12th, whichever the employee has). A
-- re-upload replaces the file in place (see the unique constraint), the same
-- "one current file" model leave documents already use.
CREATE TYPE employee_document_type AS ENUM ('OFFER_LETTER', 'AADHAR_CARD', 'MARKSHEET');

CREATE TABLE employee_documents (
    id BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    document_type employee_document_type NOT NULL,
    file_name TEXT NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    uploaded_by BIGINT REFERENCES users(id),
    UNIQUE (employee_id, document_type)
);
