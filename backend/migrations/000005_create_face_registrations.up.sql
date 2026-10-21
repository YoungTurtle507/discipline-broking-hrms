CREATE TYPE face_sample_type AS ENUM ('FRONT', 'LEFT', 'RIGHT', 'LIGHTING', 'ADDITIONAL');

CREATE TABLE face_registrations (
    id BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    sample_type face_sample_type NOT NULL,
    embedding DOUBLE PRECISION[] NOT NULL,
    model_name TEXT NOT NULL,
    model_version TEXT NOT NULL,
    quality_score DOUBLE PRECISION NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    registered_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_face_registrations_employee ON face_registrations(employee_id);
CREATE INDEX idx_face_registrations_active ON face_registrations(employee_id, is_active);
