CREATE TABLE holidays (
    id BIGSERIAL PRIMARY KEY,
    holiday_date DATE NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE leave_type AS ENUM ('PAID', 'COMPOFF');
CREATE TYPE leave_request_status AS ENUM ('PENDING', 'APPROVED', 'REJECTED');

CREATE TABLE leave_requests (
    id BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    leave_type leave_type NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    status leave_request_status NOT NULL DEFAULT 'PENDING',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_by BIGINT REFERENCES users(id),
    decided_at TIMESTAMPTZ
);

CREATE INDEX idx_leave_requests_employee ON leave_requests(employee_id);
CREATE INDEX idx_leave_requests_dates ON leave_requests(employee_id, start_date, end_date) WHERE status = 'APPROVED';

-- A full ledger, not a running balance column — every leave/compoff
-- balance-changing event (grant, deduction, penalty, compoff earned/used)
-- is its own row, so history stays auditable and balances are always a SUM.
CREATE TABLE leave_ledger (
    id BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    leave_type leave_type NOT NULL,
    entry_date DATE NOT NULL,
    amount NUMERIC(5,2) NOT NULL,
    reason TEXT NOT NULL,
    related_leave_request_id BIGINT REFERENCES leave_requests(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_leave_ledger_employee ON leave_ledger(employee_id, leave_type, entry_date);

-- A unique row per employee-date prevents the calculation engine from ever
-- double-applying an unapproved-leave penalty or a compoff-earned credit if
-- a day gets recalculated more than once.
CREATE UNIQUE INDEX idx_leave_ledger_dedupe ON leave_ledger(employee_id, leave_type, entry_date, reason);
