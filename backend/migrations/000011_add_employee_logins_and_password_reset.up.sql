-- Employees can now have their own login account. The username is the
-- employee code (EMP0001), created by an admin/super-admin who also sets the
-- initial password. Nullable because existing employees have no account yet,
-- and because an employee record must be creatable without one.
ALTER TABLE employees ADD COLUMN user_id BIGINT UNIQUE REFERENCES users(id);

-- Records when a password was last changed, so every session issued before
-- that moment can be rejected. Sessions live in Redis with no user->sessions
-- index, and adding one would couple its TTL to the session TTL and lose
-- everything on a Redis restart. Instead each session stores its issue time
-- and the auth middleware — which already re-reads this row on every request
-- — compares the two. An admin resetting a password therefore logs that user
-- out everywhere on their very next request.
ALTER TABLE users ADD COLUMN password_changed_at TIMESTAMPTZ NOT NULL DEFAULT now();
