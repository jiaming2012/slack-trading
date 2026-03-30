-- init-metabase.sql
-- Idempotent setup for Metabase: app database + read-only trading user
-- Run against production Postgres as superuser (e.g., grodt)
-- Usage: psql -h 159.89.226.131 -U grodt -d postgres -f infra/init-metabase.sql
--
-- Safe to run multiple times -- all operations check before creating.

-- =============================================================
-- 1. Create metabaseappdb (Metabase internal state: questions, dashboards, settings)
-- =============================================================
-- CREATE DATABASE cannot run inside a transaction block, so we use a DO block
-- with a shell-out pattern. However, since DO blocks can't run CREATE DATABASE
-- either, we check and advise.
SELECT 'CREATE DATABASE metabaseappdb OWNER grodt'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'metabaseappdb')\gexec

-- =============================================================
-- 2. Create metabase_ro role (read-only access to trading data)
-- =============================================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'metabase_ro') THEN
        CREATE ROLE metabase_ro WITH LOGIN PASSWORD 'CHANGEME_METABASE_RO';
    END IF;
END
$$;

-- =============================================================
-- 3. Grant read-only access on the playground database
-- =============================================================
-- Connect to playground database for schema/table grants
\c playground

-- Allow metabase_ro to connect to this database
GRANT CONNECT ON DATABASE playground TO metabase_ro;

-- Allow access to the public schema
GRANT USAGE ON SCHEMA public TO metabase_ro;

-- Grant SELECT on all existing tables
GRANT SELECT ON ALL TABLES IN SCHEMA public TO metabase_ro;

-- Grant SELECT on all existing sequences (needed for some Metabase introspection)
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO metabase_ro;

-- Auto-grant SELECT on future tables created by grodt
ALTER DEFAULT PRIVILEGES FOR ROLE grodt IN SCHEMA public
    GRANT SELECT ON TABLES TO metabase_ro;

ALTER DEFAULT PRIVILEGES FOR ROLE grodt IN SCHEMA public
    GRANT SELECT ON SEQUENCES TO metabase_ro;
