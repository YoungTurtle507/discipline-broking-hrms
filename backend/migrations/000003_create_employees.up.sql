CREATE TYPE employee_status AS ENUM (
    'ACTIVE', 'INACTIVE', 'ON_NOTICE', 'RESIGNED', 'TERMINATED', 'SUSPENDED'
);

CREATE TABLE employees (
    id BIGSERIAL PRIMARY KEY,
    employee_code TEXT NOT NULL UNIQUE,
    full_name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    phone TEXT NOT NULL DEFAULT '',
    photo_url TEXT NOT NULL DEFAULT '',
    joining_date DATE NOT NULL,
    department_id BIGINT REFERENCES departments(id),
    designation_id BIGINT REFERENCES designations(id),
    shift_id BIGINT REFERENCES shifts(id),
    reporting_manager_id BIGINT REFERENCES employees(id),
    status employee_status NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_employees_department ON employees(department_id);
CREATE INDEX idx_employees_designation ON employees(designation_id);
CREATE INDEX idx_employees_manager ON employees(reporting_manager_id);
CREATE INDEX idx_employees_status ON employees(status);
