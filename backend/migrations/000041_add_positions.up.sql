-- Position/seniority level (Senior, Junior, ...) — a flat classification
-- alongside Department and Designation, not scoped to either: a "Senior"
-- can exist in any department/designation, unlike a Designation which
-- belongs to exactly one department.
CREATE TABLE positions (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO positions (title) VALUES ('Senior'), ('Junior');

ALTER TABLE employees ADD COLUMN position_id BIGINT REFERENCES positions(id);
