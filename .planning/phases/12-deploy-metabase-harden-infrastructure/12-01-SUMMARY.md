---
phase: 12-deploy-metabase-harden-infrastructure
plan: 01
subsystem: infra
tags: [metabase, postgres, docker-compose, read-only-user, firewall]

# Dependency graph
requires:
  - phase: 07-production-deploy
    provides: Production Docker Compose and .env.prod.template
provides:
  - docker-compose.metabase.yaml for local Metabase on Windows Desktop
  - infra/init-metabase.sql for metabaseappdb + metabase_ro setup
  - .env.metabase.template for local Metabase configuration
  - .env.prod.template updated with METABASE_RO_PASSWORD
affects: [14-core-trading-dashboards, 15-backtest-persistence]

# Tech tracking
tech-stack:
  added: [metabase v0.59.4]
  patterns: [separate compose file for desktop services, idempotent SQL init scripts]

key-files:
  created:
    - docker-compose.metabase.yaml
    - infra/init-metabase.sql
    - .env.metabase.template
  modified:
    - .env.prod.template
    - .gitignore

key-decisions:
  - "Metabase runs on Windows Desktop, not DO droplet -- saves 4GB server memory"
  - "Used \\gexec for idempotent CREATE DATABASE (cannot use IF NOT EXISTS)"
  - "metabase_ro gets SELECT-only with DEFAULT PRIVILEGES for future tables"

patterns-established:
  - "Separate docker-compose files for local-only services (not in prod compose)"
  - "Idempotent SQL scripts in infra/ for database provisioning"

requirements-completed: [INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-05]

# Metrics
duration: 2min
completed: 2026-03-30
---

# Phase 12 Plan 01: Deploy Metabase & Harden Infrastructure Summary

**Local Metabase docker-compose (v0.59.4 on port 3001) with idempotent SQL creating metabaseappdb + read-only metabase_ro user on DO Postgres**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-30T01:23:39Z
- **Completed:** 2026-03-30T01:25:51Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Idempotent SQL script that creates metabaseappdb (Metabase internal state) and metabase_ro (SELECT-only on playground database)
- Local docker-compose for Windows Desktop with memory cap (1.5GB), JVM tuning (-Xmx768m), and healthcheck
- Environment templates for both production DB credentials and local Metabase configuration
- Firewall prerequisite documented in compose file comments (DO Cloud Firewall TCP 5432)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create init SQL and update env templates** - `e5312fb` (feat)
2. **Task 2: Create local Metabase docker-compose and firewall documentation** - `9e1c31f` (feat)

## Files Created/Modified
- `infra/init-metabase.sql` - Idempotent script: metabaseappdb database + metabase_ro role with SELECT-only grants
- `docker-compose.metabase.yaml` - Local Metabase container for Windows Desktop (port 3001, mem_limit 1.5GB)
- `.env.metabase.template` - Template for MB_DB_* vars, JAVA_OPTS memory cap, Jetty port
- `.env.prod.template` - Added METABASE_RO_PASSWORD placeholder section
- `.gitignore` - Added exception for .env.metabase.template

## Decisions Made
- Used `\gexec` pattern for idempotent CREATE DATABASE (standard IF NOT EXISTS not available for CREATE DATABASE in Postgres)
- Granted SELECT on sequences as well as tables (Metabase introspection needs this)
- Set DEFAULT PRIVILEGES for grodt role so future tables auto-grant SELECT to metabase_ro
- docker-compose.prod.yaml intentionally not modified per D-01

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added .gitignore exception for .env.metabase.template**
- **Found during:** Task 1
- **Issue:** `.env.*` gitignore pattern blocked tracking of `.env.metabase.template`
- **Fix:** Added `!.env.metabase.template` exception (matching existing `!.env.prod.template` pattern)
- **Files modified:** .gitignore
- **Verification:** git add succeeded after the exception
- **Committed in:** e5312fb (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Minor gitignore fix, no scope creep.

## Issues Encountered
- `docker compose config` validation requires `.env.metabase` file to exist. Validated by temporarily copying template to `.env.metabase`, confirmed valid YAML, then removed.

## User Setup Required

To use Metabase:
1. Run `infra/init-metabase.sql` against production Postgres: `psql -h 159.89.226.131 -U grodt -d postgres -f infra/init-metabase.sql`
2. Change the `CHANGEME_METABASE_RO` password in the SQL or ALTER ROLE after running
3. Open DO Cloud Firewall: add inbound TCP 5432 from home IP
4. Copy `.env.metabase.template` to `.env.metabase` and fill in real credentials
5. Run `docker compose -f docker-compose.metabase.yaml up -d` on Windows Desktop
6. Visit `http://localhost:3001` to complete Metabase first-run setup

## Next Phase Readiness
- Metabase infrastructure ready for dashboard creation in Phase 14
- SQL views for trading analytics can be built once Metabase is connected
- No blockers for subsequent phases

---
*Phase: 12-deploy-metabase-harden-infrastructure*
*Completed: 2026-03-30*
