-- partitioning.sql
-- OpenSpec-Change: db-partitioning-retention
--
-- Converts the two v4 trading-stack tables that grow indefinitely under daily
-- multi-strategy scanning -- scan_results and sim_outcomes (created as plain
-- tables by tradingstack.MigrateTradingStack, see the trading-stack-schema
-- change) -- into native PostgreSQL RANGE-partitioned tables, keyed on
-- scanned_at / simulated_at respectively.
--
-- BREAKING: both tables' primary key becomes composite -- (id, scanned_at) and
-- (id, simulated_at) -- instead of a bare `id`. This is a hard Postgres
-- requirement: any unique/PK constraint on a partitioned table must include
-- all partition-key columns.
--
-- BREAKING: the sim_outcomes.scan_result_id -> scan_results.id foreign key is
-- dropped (the column is kept). Native partitioning requires the referenced
-- constraint to include the partition key column, which a bare `id` can no
-- longer satisfy once scan_results is partitioned. Referential integrity for
-- this relationship becomes an application-layer concern -- see
-- openspec/changes/db-partitioning-retention/design.md, Decision 3.
--
-- No DEFAULT partition is created for either table: an insert whose
-- partition-key value falls in an unprovisioned month fails loudly with
-- Postgres's native "no partition of relation found for row" error rather
-- than being silently absorbed. See src/go/tradingstack/partitioning for the
-- Go routine that provisions partitions ahead of need.
--
-- Idempotent: safe to run multiple times. On the first run against a
-- database where trading-stack-schema already created scan_results /
-- sim_outcomes as plain tables, this file renames the plain table aside,
-- recreates it partitioned, migrates any existing rows into per-month
-- partitions it provisions on the fly, and drops the renamed original. On
-- subsequent runs (or on a fresh database where the tables don't exist yet),
-- it is a no-op beyond ensuring the current month's partition exists.
--
-- Registered in infra/migrate.py's SQL_FILES list. Also embedded and applied
-- verbatim from Go tests (src/go/tradingstack/db/apply.go) so the migration
-- has a single source of truth shared by both the production (Python-driven)
-- and test (Go-driven) apply paths.
--
-- feature_distributions, strategy_ev_weights, simulator_fidelity, and
-- scanner_configs are NEVER touched by this file.

-- =============================================================
-- 0. Drop the sim_outcomes -> scan_results FK first
-- =============================================================
--
-- Must run before scan_results is renamed/recreated below: while the FK
-- exists, renaming scan_results (which the FK depends on) blocks the later
-- DROP TABLE of the renamed original. Dropping it here is also the direct
-- implementation of Decision 3 in design.md (column kept, DB-level FK
-- dropped, integrity enforced at the application layer).
DO $$ BEGIN
    ALTER TABLE sim_outcomes DROP CONSTRAINT IF EXISTS fk_sim_outcomes_scan_result;
EXCEPTION WHEN undefined_table THEN NULL; END $$;

-- =============================================================
-- 1. scan_results -> PARTITION BY RANGE (scanned_at)
-- =============================================================

-- Step 1a: if a plain (non-partitioned) scan_results table already exists,
-- rename it aside so the partitioned replacement can take its name.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'scan_results'
    ) AND NOT EXISTS (
        SELECT 1 FROM pg_partitioned_table pt
        JOIN pg_class c ON c.oid = pt.partrelid
        WHERE c.relname = 'scan_results'
    ) THEN
        ALTER TABLE scan_results RENAME TO scan_results_unpartitioned;
    END IF;
END $$;

-- Step 1b: create the partitioned parent (no-op if it already exists).
CREATE TABLE IF NOT EXISTS scan_results (
    id                uuid NOT NULL,
    scanned_at        timestamptz NOT NULL,
    ticker            text NOT NULL,
    regime_tag        text,
    regime_confidence numeric,
    price             numeric,
    volume_ratio      numeric,
    rsi_14            numeric,
    atr_pct           numeric,
    short_interest    numeric,
    sector            text,
    scanner_score     numeric,
    scanner_version   text,
    data_as_of        timestamptz,
    PRIMARY KEY (id, scanned_at)
) PARTITION BY RANGE (scanned_at);

-- Step 1c: restore the data_as_of <= scanned_at invariant (idempotent: a
-- duplicate_object error on re-add is swallowed).
DO $$ BEGIN
    ALTER TABLE scan_results
        ADD CONSTRAINT chk_scan_results_data_as_of
        CHECK (data_as_of <= scanned_at);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Step 1d: if there was a plain table with pre-existing rows, provision a
-- partition for every distinct month present, migrate the rows, then drop
-- the renamed original. No-op if scan_results_unpartitioned doesn't exist.
DO $$
DECLARE
    r RECORD;
    part_name text;
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'scan_results_unpartitioned'
    ) THEN
        FOR r IN SELECT DISTINCT date_trunc('month', scanned_at) AS month FROM scan_results_unpartitioned LOOP
            part_name := 'scan_results_y' || to_char(r.month, 'YYYY') || '_m' || to_char(r.month, 'MM');
            EXECUTE format(
                'CREATE TABLE IF NOT EXISTS %I PARTITION OF scan_results FOR VALUES FROM (%L) TO (%L)',
                part_name, r.month, r.month + interval '1 month'
            );
        END LOOP;

        INSERT INTO scan_results (
            id, scanned_at, ticker, regime_tag, regime_confidence, price, volume_ratio,
            rsi_14, atr_pct, short_interest, sector, scanner_score, scanner_version, data_as_of
        )
        SELECT
            id, scanned_at, ticker, regime_tag, regime_confidence, price, volume_ratio,
            rsi_14, atr_pct, short_interest, sector, scanner_score, scanner_version, data_as_of
        FROM scan_results_unpartitioned;

        DROP TABLE scan_results_unpartitioned;
    END IF;
END $$;

-- Step 1e: always ensure the current month's partition exists, even on a
-- brand-new database with no migrated rows to derive months from.
DO $$
DECLARE
    month_start date := date_trunc('month', now());
    part_name text := 'scan_results_y' || to_char(date_trunc('month', now()), 'YYYY') || '_m' || to_char(date_trunc('month', now()), 'MM');
BEGIN
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF scan_results FOR VALUES FROM (%L) TO (%L)',
        part_name, month_start, month_start + interval '1 month'
    );
END $$;

-- =============================================================
-- 2. sim_outcomes -> PARTITION BY RANGE (simulated_at)
-- =============================================================

-- Step 2a: if a plain (non-partitioned) sim_outcomes table already exists,
-- rename it aside so the partitioned replacement can take its name.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'sim_outcomes'
    ) AND NOT EXISTS (
        SELECT 1 FROM pg_partitioned_table pt
        JOIN pg_class c ON c.oid = pt.partrelid
        WHERE c.relname = 'sim_outcomes'
    ) THEN
        ALTER TABLE sim_outcomes RENAME TO sim_outcomes_unpartitioned;
    END IF;
END $$;

-- Step 2b: create the partitioned parent (no-op if it already exists).
-- NOTE: scan_result_id is retained as a plain uuid column with NO foreign
-- key to scan_results(id) -- see the BREAKING note at the top of this file.
CREATE TABLE IF NOT EXISTS sim_outcomes (
    id             uuid NOT NULL,
    scan_result_id uuid,
    simulated_at   timestamptz NOT NULL,
    strategy_id    text,
    entry_price    numeric,
    exit_price     numeric,
    stop_price     numeric,
    target_price   numeric,
    pnl_pct        numeric,
    hold_days      integer,
    exit_reason    text,
    max_drawdown   numeric,
    outcome_label  text,
    PRIMARY KEY (id, simulated_at)
) PARTITION BY RANGE (simulated_at);

COMMENT ON COLUMN sim_outcomes.scan_result_id IS
    'References scan_results.id. No database-level foreign key: partitioning '
    'scan_results on scanned_at means its only unique/PK constraint no longer '
    'satisfies a bare-id reference target. Referential integrity is enforced '
    'at the application/service layer. See db-partitioning-retention design.md.';

-- Step 2c: restore the exit_reason allowed-values invariant (idempotent).
DO $$ BEGIN
    ALTER TABLE sim_outcomes
        ADD CONSTRAINT chk_sim_outcomes_exit_reason
        CHECK (exit_reason IN ('stop', 'target', 'timeout', 'signal_exit'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Step 2d: if there was a plain table with pre-existing rows, provision a
-- partition for every distinct month present, migrate the rows, then drop
-- the renamed original. No-op if sim_outcomes_unpartitioned doesn't exist.
DO $$
DECLARE
    r RECORD;
    part_name text;
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'sim_outcomes_unpartitioned'
    ) THEN
        FOR r IN SELECT DISTINCT date_trunc('month', simulated_at) AS month FROM sim_outcomes_unpartitioned LOOP
            part_name := 'sim_outcomes_y' || to_char(r.month, 'YYYY') || '_m' || to_char(r.month, 'MM');
            EXECUTE format(
                'CREATE TABLE IF NOT EXISTS %I PARTITION OF sim_outcomes FOR VALUES FROM (%L) TO (%L)',
                part_name, r.month, r.month + interval '1 month'
            );
        END LOOP;

        INSERT INTO sim_outcomes (
            id, scan_result_id, simulated_at, strategy_id, entry_price, exit_price,
            stop_price, target_price, pnl_pct, hold_days, exit_reason, max_drawdown, outcome_label
        )
        SELECT
            id, scan_result_id, simulated_at, strategy_id, entry_price, exit_price,
            stop_price, target_price, pnl_pct, hold_days, exit_reason, max_drawdown, outcome_label
        FROM sim_outcomes_unpartitioned;

        DROP TABLE sim_outcomes_unpartitioned;
    END IF;
END $$;

-- Step 2e: always ensure the current month's partition exists.
DO $$
DECLARE
    month_start date := date_trunc('month', now());
    part_name text := 'sim_outcomes_y' || to_char(date_trunc('month', now()), 'YYYY') || '_m' || to_char(date_trunc('month', now()), 'MM');
BEGIN
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF sim_outcomes FOR VALUES FROM (%L) TO (%L)',
        part_name, month_start, month_start + interval '1 month'
    );
END $$;
