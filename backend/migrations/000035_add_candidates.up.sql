-- Recruitment/interview pipeline: one row per candidate, from application
-- through to joining. Admin-only for now (see the permission grant below) —
-- not even HR or Super Admin get it automatically beyond Super Admin's usual
-- blanket bypass.
CREATE TYPE candidate_experience_type AS ENUM ('FRESHER', 'EXPERIENCED');

CREATE TYPE candidate_notice_period AS ENUM (
    'IMMEDIATE', 'FIFTEEN_DAYS', 'THIRTY_DAYS', 'FORTYFIVE_DAYS',
    'SIXTY_DAYS', 'NINETY_DAYS', 'CURRENTLY_SERVING'
);

CREATE TYPE candidate_status AS ENUM (
    'APPLIED', 'SCHEDULED', 'INTERVIEWED', 'SELECTED', 'REJECTED', 'ON_HOLD', 'OFFERED', 'JOINED'
);

CREATE TABLE candidates (
    id BIGSERIAL PRIMARY KEY,
    full_name TEXT NOT NULL,
    mobile_no TEXT NOT NULL,
    email TEXT NOT NULL,
    education TEXT NOT NULL DEFAULT '',
    position TEXT NOT NULL,
    experience_type candidate_experience_type NOT NULL,
    -- Total months of experience — zero and unused for a FRESHER. Stored as a
    -- single number rather than separate years/months columns; the frontend
    -- splits it into a Years + Months pair for entry/display.
    experience_months INT NOT NULL DEFAULT 0,
    current_ctc NUMERIC(10,2),
    expected_ctc NUMERIC(10,2),
    notice_period candidate_notice_period NOT NULL DEFAULT 'IMMEDIATE',
    location TEXT NOT NULL DEFAULT '',
    status candidate_status NOT NULL DEFAULT 'APPLIED',
    -- When the interview is actually scheduled for — separate from
    -- created_at, which is just when this record was added to the pipeline.
    interview_at TIMESTAMPTZ,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_candidates_status ON candidates(status);
CREATE INDEX idx_candidates_created_at ON candidates(created_at);

INSERT INTO permissions (code, description) VALUES
    ('candidates.manage', 'Manage the candidate/interview recruitment pipeline');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'ADMIN' AND p.code = 'candidates.manage';
