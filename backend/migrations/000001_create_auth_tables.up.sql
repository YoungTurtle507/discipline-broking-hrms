CREATE TABLE roles (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    full_name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_roles (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

INSERT INTO roles (name) VALUES
    ('SUPER_ADMIN'), ('ADMIN'), ('HR'), ('MANAGER'), ('EMPLOYEE');

INSERT INTO permissions (code, description) VALUES
    ('employees.manage', 'Create, edit, and manage employee records'),
    ('attendance.manage', 'View and manage attendance for all employees'),
    ('attendance.view_team', 'View attendance for direct reports'),
    ('attendance.view_own', 'View own attendance'),
    ('leave.manage', 'Manage leave types, requests, and balances for all employees'),
    ('leave.approve', 'Approve or reject any employee''s leave request'),
    ('leave.approve_team', 'Approve or reject direct reports'' leave requests'),
    ('leave.request_own', 'Submit own leave requests'),
    ('reports.view', 'View HR reports'),
    ('settings.manage', 'Manage system-wide settings'),
    ('profile.view_own', 'View own profile'),
    ('profile.manage_own', 'Edit own profile');

-- SUPER_ADMIN gets every permission that exists.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'SUPER_ADMIN';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'ADMIN' AND p.code IN (
    'employees.manage', 'attendance.manage', 'leave.manage', 'leave.approve',
    'reports.view', 'settings.manage', 'profile.view_own', 'profile.manage_own'
);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'HR' AND p.code IN (
    'employees.manage', 'attendance.manage', 'leave.manage', 'leave.approve',
    'reports.view', 'profile.view_own', 'profile.manage_own'
);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'MANAGER' AND p.code IN (
    'attendance.view_team', 'leave.approve_team', 'profile.view_own', 'profile.manage_own'
);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'EMPLOYEE' AND p.code IN (
    'attendance.view_own', 'leave.request_own', 'profile.view_own', 'profile.manage_own'
);
