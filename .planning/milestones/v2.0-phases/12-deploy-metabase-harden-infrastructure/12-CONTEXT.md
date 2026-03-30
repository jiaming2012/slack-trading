# Phase 12: Deploy Metabase & Harden Infrastructure - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning

<domain>
## Phase Boundary

Deploy Metabase as an analytics layer connecting to the existing production Postgres on the DO droplet. Metabase runs on the user's Windows desktop (Docker Desktop), not on the DO droplet. Infrastructure hardening ensures Postgres is safely accessible from the desktop.

</domain>

<decisions>
## Implementation Decisions

### D-01: Metabase hosting
- Metabase runs on the user's Windows desktop via Docker Desktop, NOT on the DO droplet
- Connects remotely to DO Postgres at 159.89.226.131:5432
- This saves DO droplet memory (no JVM on the 4GB server)
- INFRA-01 scope changes: no Metabase container in docker-compose.prod.yaml
- INFRA-03 (JVM memory cap) becomes a local Docker Desktop concern, not DO

### D-02: Postgres remote access
- Open port 5432 in DO Cloud Firewall, restricted to user's home IP address
- User will provide their home IP (or it will be detected at deploy time)
- metabase_ro read-only user still required for safety

### D-03: Metabase admin credentials
- Set via environment variable (like Grafana's GF_ADMIN_PASSWORD)
- Use MB_ADMIN_EMAIL and MB_ADMIN_FIRST_NAME env vars for initial setup
- Password set on first login (Metabase doesn't support pre-set admin password via env var the same way Grafana does — admin setup happens on first browser visit)

### D-04: Init SQL automation
- Automated init script that creates:
  - `metabaseappdb` database for Metabase internal state
  - `metabase_ro` read-only user with SELECT on playground database
- Script runs as part of Postgres container startup (docker-entrypoint-initdb.d)
- Also needs to run against existing production Postgres (one-time migration)

### D-05: Metabase access control
- Metabase accessible via localhost on desktop (no internet exposure needed since it runs locally)
- No Cloud Firewall rule for Metabase port (3001) — it's not on DO

### Claude's Discretion
- Local docker-compose file for Metabase (separate from production compose)
- Exact Metabase version (v0.59.4 per research)
- JAVA_OPTS memory settings for local Docker Desktop
- Metabase Postgres app DB connection string format

</decisions>

<specifics>
## Specific Ideas

- Create `docker-compose.metabase.yaml` (or similar) for local desktop use
- Create `infra/init-metabase.sql` for the DB setup script
- The init SQL should be idempotent (CREATE IF NOT EXISTS)
- Document the setup: "run this SQL against production, then docker compose up locally"

</specifics>

<deferred>
## Deferred Ideas

- Metabase dashboard provisioning as code (JSON export) — decide in Phase 14 when building dashboards
- Droplet upgrade to 8GB — not needed since Metabase runs locally

</deferred>

---

*Phase: 12-deploy-metabase-harden-infrastructure*
*Context gathered: 2026-03-30 via discuss-phase*
