-- New enum value only. Postgres forbids using a value added by ALTER TYPE
-- within the same transaction that added it, and golang-migrate runs each
-- migration file as one transaction, so this file must do nothing else.
ALTER TYPE leave_type ADD VALUE 'UNPAID';
