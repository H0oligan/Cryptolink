-- +migrate Up
ALTER TABLE users ADD COLUMN is_super_admin boolean DEFAULT false;
CREATE INDEX users_is_super_admin ON users(is_super_admin) WHERE is_super_admin = true;

-- +migrate Down
DROP INDEX IF EXISTS users_is_super_admin;
ALTER TABLE users DROP COLUMN is_super_admin;
