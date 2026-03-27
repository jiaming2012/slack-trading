# Phase 7: Production Deployment - Context

**Gathered:** 2026-03-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Deploy BOTH the trading app (Go server + Python client) AND the observability stack to Digital Ocean. Everything runs on a single DO droplet using Docker Compose. Grafana accessible via web with basic auth. No TLS for v1.

</domain>

<decisions>
## Implementation Decisions

### Infrastructure
- **D-01:** Single Digital Ocean droplet running Docker Compose. Both the trading app and observability stack on the same machine.
- **D-02:** Provision via DO MCP server (Claude creates droplet interactively during execution).
- **D-03:** Everything on DO — no cross-cloud networking needed. Vultr K8s deployment is NOT part of this phase (existing Vultr setup remains but is not the target).

### Stack Architecture
- **D-04:** Keep `grafana/otel-lgtm` all-in-one image for production (same as dev). Single operator, single container.
- **D-05:** Trading app (Go server) runs as a separate Docker container on the same droplet, communicating with otel-lgtm via localhost.
- **D-06:** Python strategy client runs on the same droplet (or connects remotely — Claude's discretion).

### Security & Auth
- **D-07:** Grafana uses basic auth (username + password) via environment variables. Sufficient for single operator.
- **D-08:** No TLS for v1 — HTTP only. TLS deferred to future work.
- **D-09:** OTLP endpoint is localhost-only (Go server → otel-lgtm on same machine). No external OTLP ingestion needed since everything is on the same droplet.

### Deployment Approach
- **D-10:** Production Docker Compose extends the existing `observability/docker-compose.yaml` with the trading app container.
- **D-11:** Environment variables for production (OTEL_EXPORTER_OTLP_ENDPOINT, database credentials, API keys) managed via `.env` file on the droplet.

### Claude's Discretion
- Droplet size (CPU, RAM) — based on trading app + observability requirements
- Docker Compose file organization (single file vs override files)
- How to deploy code to the droplet (git clone, docker push, scp)
- Python strategy client deployment method
- Grafana admin password management
- Firewall rules (which ports to expose)
- Whether to use DO managed Postgres or run Postgres in Docker on the droplet

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Deployment
- `.clusters/production/deployment.yaml` — Current Vultr K8s deployment (reference for env vars and container config)
- `Dockerfile` — Production Docker image (FROM grodt-base-image-2:3.9.0)
- `Dockerfile.base` — Base image with Python, Conda, TA-Lib
- `Dockerfile.base2` — Second layer with Go, module deps, conda env
- `.env` — Environment variables (API keys, database credentials)

### Observability Stack (from Phase 6)
- `observability/docker-compose.yaml` — otel-lgtm + dashboard/alerting volume mounts
- `observability/dashboards/` — Dashboard JSON + provider YAML
- `observability/alerting/alerting.yaml` — Alert rules + Slack contact point

### Server
- `cmd/main.go` — Server entrypoint with OTel init
- `cmd/run-dev.sh` — Dev runner (reference for env var setup)
- `taskfile.yml` — Build and deploy tasks

</canonical_refs>

<code_context>
## Existing Code Insights

### Docker Build
- Multi-layer Docker build: base (Ubuntu + Python + TA-Lib) → base2 (Go + deps) → app
- Current registry: `ewr.vultrcr.com/grodt/` (Vultr) — may need DO registry or direct build
- `task app:build` builds the production image

### Database Requirements
- PostgreSQL (GORM auto-migrate on startup)
- EventStoreDB (event sourcing)
- Both currently run via `eventstoredb/docker-compose.yaml` locally or port-forwarded from Vultr K8s

### Integration Points
- OTEL_EXPORTER_OTLP_ENDPOINT needs to point to localhost:4318 (otel-lgtm on same machine)
- Grafana on port 3000 (needs to be exposed externally)
- Twirp on port 5051 (internal only unless Python client connects remotely)

</code_context>

<deferred>
## Deferred Ideas

- TLS via Let's Encrypt + Caddy reverse proxy (future enhancement)
- Terraform for infrastructure-as-code (future — if infra grows beyond single droplet)
- DO managed Postgres (future — if database needs grow)
- Migration from Vultr K8s to DO (broader scope than this phase)

</deferred>

---

*Phase: 07-production-deployment*
*Context gathered: 2026-03-27*
