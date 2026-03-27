# Phase 7: Production Deployment - Research

**Researched:** 2026-03-26
**Domain:** Infrastructure deployment (Docker Compose on DigitalOcean)
**Confidence:** HIGH

## Summary

This phase deploys the grodt trading app and its observability stack (grafana/otel-lgtm) to a single DigitalOcean droplet running Docker Compose. The architecture is straightforward: one droplet, one Docker Compose file combining the trading app container, databases (PostgreSQL + EventStoreDB), and otel-lgtm. All inter-service communication is localhost.

The main complexities are: (1) the Docker image is 5.5GB so it must be built on the droplet (no registry push needed), (2) Grafana anonymous auth must be explicitly disabled via `GF_AUTH_ANONYMOUS_ENABLED=false` environment variable (the otel-lgtm startup script respects user-provided env vars), and (3) DO Cloud Firewall should be used instead of UFW because Docker bypasses UFW iptables rules.

**Primary recommendation:** Use a 4GB RAM / 2 vCPU DO droplet with Docker marketplace image, deploy via `git clone` + `docker compose up`, use DO Cloud Firewall to expose only ports 22 (SSH) and 3000 (Grafana).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Single Digital Ocean droplet running Docker Compose. Both the trading app and observability stack on the same machine.
- **D-02:** Provision via DO MCP server (Claude creates droplet interactively during execution).
- **D-03:** Everything on DO -- no cross-cloud networking needed. Vultr K8s deployment is NOT part of this phase (existing Vultr setup remains but is not the target).
- **D-04:** Keep `grafana/otel-lgtm` all-in-one image for production (same as dev). Single operator, single container.
- **D-05:** Trading app (Go server) runs as a separate Docker container on the same droplet, communicating with otel-lgtm via localhost.
- **D-06:** Python strategy client runs on the same droplet (or connects remotely -- Claude's discretion).
- **D-07:** Grafana uses basic auth (username + password) via environment variables. Sufficient for single operator.
- **D-08:** No TLS for v1 -- HTTP only. TLS deferred to future work.
- **D-09:** OTLP endpoint is localhost-only (Go server -> otel-lgtm on same machine). No external OTLP ingestion needed since everything is on the same droplet.
- **D-10:** Production Docker Compose extends the existing `observability/docker-compose.yaml` with the trading app container.
- **D-11:** Environment variables for production (OTEL_EXPORTER_OTLP_ENDPOINT, database credentials, API keys) managed via `.env` file on the droplet.

### Claude's Discretion
- Droplet size (CPU, RAM) -- based on trading app + observability requirements
- Docker Compose file organization (single file vs override files)
- How to deploy code to the droplet (git clone, docker push, scp)
- Python strategy client deployment method
- Grafana admin password management
- Firewall rules (which ports to expose)
- Whether to use DO managed Postgres or run Postgres in Docker on the droplet

### Deferred Ideas (OUT OF SCOPE)
- TLS via Let's Encrypt + Caddy reverse proxy (future enhancement)
- Terraform for infrastructure-as-code (future -- if infra grows beyond single droplet)
- DO managed Postgres (future -- if database needs grow)
- Migration from Vultr K8s to DO (broader scope than this phase)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DEPLOY-01 | Observability stack (Loki, Grafana, Tempo, Prometheus, OTel Collector) deployed to Digital Ocean | otel-lgtm all-in-one image provides all components; DO droplet with Docker marketplace image is the target |
| DEPLOY-02 | Go server and Python client send telemetry to Digital Ocean endpoint | OTLP endpoint is localhost:4318 on same droplet; existing OTEL_EXPORTER_OTLP_ENDPOINT env var works unchanged |
| DEPLOY-03 | Grafana accessible via web with authentication | GF_AUTH_ANONYMOUS_ENABLED=false + GF_SECURITY_ADMIN_PASSWORD; DO Cloud Firewall exposes port 3000 |
</phase_requirements>

## Standard Stack

### Core
| Component | Version/Image | Purpose | Why Standard |
|-----------|--------------|---------|--------------|
| DigitalOcean Droplet | s-2vcpu-4gb | Single VM hosting all services | $24/mo, sufficient for single-operator trading + observability |
| DO Docker Marketplace Image | Ubuntu 20.04 + Docker CE 28.1.1 + Compose 2.36.0 | Pre-configured Docker host | Eliminates manual Docker installation |
| grafana/otel-lgtm | latest | All-in-one observability backend | Already used in dev (Phase 6), locked decision D-04 |
| PostgreSQL | 13 | Trading data persistence | Same version as existing eventstoredb/docker-compose.yaml |
| EventStoreDB | 24.2.0-jammy | Event sourcing | Same version as existing setup |

### Supporting
| Component | Purpose | When to Use |
|-----------|---------|-------------|
| DO Cloud Firewall | Network-level port filtering | Always -- replaces UFW to avoid Docker iptables bypass |
| DO MCP Server | Droplet provisioning via Claude | During initial setup (D-02) |
| git | Code deployment to droplet | Clone repo, pull updates |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| git clone deployment | Docker registry push | Registry adds complexity; 5.5GB images are slow to push; building on-droplet is simpler for single machine |
| DO Cloud Firewall | UFW on droplet | Docker bypasses UFW iptables rules -- containers get exposed even when UFW blocks the port |
| Single compose file | Compose override files | Override files add indirection; single file is clearer for a single deployment target |
| Docker Postgres | DO Managed Postgres | Managed DB adds cost ($15+/mo) and network latency; Docker Postgres is simpler for single-droplet |

## Architecture Patterns

### Production Docker Compose Structure

Single `docker-compose.prod.yaml` at repo root combining all services:

```yaml
# docker-compose.prod.yaml
version: "3.4"

services:
  # --- Databases ---
  eventstore.db:
    # (same as eventstoredb/docker-compose.yaml)

  postgres:
    # (same as eventstoredb/docker-compose.yaml)

  # --- Observability ---
  otel-lgtm:
    image: grafana/otel-lgtm
    ports:
      - "3000:3000"       # Grafana UI (externally accessible)
      - "4317:4317"       # OTLP gRPC (localhost only via firewall)
      - "4318:4318"       # OTLP HTTP (localhost only via firewall)
    environment:
      - GF_AUTH_ANONYMOUS_ENABLED=false
      - GF_SECURITY_ADMIN_PASSWORD=${GF_ADMIN_PASSWORD}
      - GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH=/otel-lgtm/grafana/conf/provisioning/dashboards/custom/grodt-live-simulation.json
      - SLACK_OPTION_ALERTS_WEBHOOK_URL=${SLACK_OPTION_ALERTS_WEBHOOK_URL}
    volumes:
      - otel-lgtm-data:/data
      - ./observability/dashboards/dashboards-provider.yaml:/otel-lgtm/grafana/conf/provisioning/dashboards/custom.yaml:ro
      - ./observability/dashboards/grodt-live-simulation.json:/otel-lgtm/grafana/conf/provisioning/dashboards/custom/grodt-live-simulation.json:ro
      - ./observability/alerting/alerting.yaml:/otel-lgtm/grafana/conf/provisioning/alerting/grodt-alerting.yaml:ro

  # --- Trading App ---
  grodt:
    build:
      context: .
      dockerfile: Dockerfile
    depends_on:
      - postgres
      - eventstore.db
      - otel-lgtm
    environment:
      # Loaded from .env.prod
    env_file:
      - .env.prod
    ports:
      - "8080:8080"       # REST API (localhost only via firewall)
      - "5051:5051"       # Twirp RPC (localhost only via firewall)

volumes:
  otel-lgtm-data:
  eventstore-volume-data:
  eventstore-volume-logs:
  postgres-volume-data:
```

### Recommended File Layout (new files)
```
docker-compose.prod.yaml          # Production compose (all services)
.env.prod.template                # Template for production env vars (no secrets)
```

### Deployment Flow Pattern
```
1. DO MCP server creates droplet (Docker marketplace image)
2. DO MCP server creates Cloud Firewall (SSH + Grafana only)
3. SSH into droplet
4. git clone the repo
5. Create .env.prod from template (add secrets)
6. docker compose -f docker-compose.prod.yaml up -d --build
7. Verify: curl http://<droplet-ip>:3000 shows Grafana login
```

### Network Architecture
```
Internet
  |
  v
DO Cloud Firewall
  |-- Allow TCP 22  (SSH)
  |-- Allow TCP 3000 (Grafana UI)
  |-- Block everything else
  |
  v
Droplet (single machine)
  |
  |-- grodt container (ports 8080, 5051)
  |-- otel-lgtm container (ports 3000, 4317, 4318)
  |-- postgres container (port 5432)
  |-- eventstore.db container (ports 1113, 2113)
  |
  All inter-container traffic via Docker network (localhost)
```

### Anti-Patterns to Avoid
- **Using UFW for Docker port control:** Docker manipulates iptables directly, bypassing UFW rules. Published container ports will be accessible from the internet regardless of UFW configuration. Use DO Cloud Firewall instead.
- **Setting GF_SECURITY_ADMIN_PASSWORD after first boot:** Grafana only reads admin password env var on first startup. If the volume already has Grafana data, the password won't change. Delete the volume or use Grafana CLI to reset.
- **Pushing 5.5GB images to a registry:** The grodt Docker image is 5.5GB due to Conda, TA-Lib, and Go dependencies. Build on the droplet instead.
- **Hardcoding secrets in docker-compose.yaml:** Use `.env.prod` file with `env_file:` directive. Keep `.env.prod.template` in git (without secrets) as documentation.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Firewall rules | iptables/UFW scripts | DO Cloud Firewall | Docker bypasses UFW; DO firewall operates at network edge before traffic reaches the VM |
| Droplet provisioning | Manual DO console steps | DO MCP Server | Locked decision D-02; MCP can create droplet + firewall programmatically |
| Grafana auth | Custom reverse proxy auth | GF_AUTH_ANONYMOUS_ENABLED=false + GF_SECURITY_ADMIN_PASSWORD | Built-in Grafana basic auth is sufficient for single operator |
| Service health checks | Custom monitoring scripts | Docker Compose healthcheck + Grafana alerts | Existing ALERT-01/02/03 from Phase 6 cover service health |
| Log rotation | Manual logrotate config | Docker logging driver defaults | Docker handles container log rotation; otel-lgtm handles telemetry retention |

## Common Pitfalls

### Pitfall 1: Docker Image Build Fails on 4GB Droplet
**What goes wrong:** Go compilation + Conda env creation in Docker can consume 3-4GB RAM, causing OOM kills on small droplets.
**Why it happens:** The multi-stage Dockerfile (base -> base2 -> app) downloads and compiles Go, Python deps, and TA-Lib.
**How to avoid:** The Dockerfile inherits from `grodt-base-image-2:3.9.0` which is on Vultr registry. Either: (a) build base images locally and transfer, (b) push base images to Docker Hub or DO Container Registry, or (c) use a swap file on the droplet during build.
**Warning signs:** `docker build` exits with code 137 (OOM killed).

### Pitfall 2: Grafana Password Ignored on Restart
**What goes wrong:** Changing `GF_SECURITY_ADMIN_PASSWORD` in `.env.prod` has no effect after first boot.
**Why it happens:** Grafana writes the admin password to its SQLite database on first start. Subsequent starts read from the database, ignoring the env var.
**How to avoid:** Set the correct password BEFORE first `docker compose up`. If password needs changing later, use `grafana-cli admin reset-admin-password <new-password>` inside the container.
**Warning signs:** Login with new password fails; old password still works.

### Pitfall 3: OTLP Ports Exposed to Internet
**What goes wrong:** Ports 4317/4318 (OTLP) are accessible from the internet, allowing anyone to push telemetry data.
**Why it happens:** Docker publishes ports to 0.0.0.0 by default; UFW cannot block them.
**How to avoid:** DO Cloud Firewall blocks everything except SSH (22) and Grafana (3000). Alternatively, bind OTLP ports to 127.0.0.1 in docker-compose: `"127.0.0.1:4318:4318"`.
**Warning signs:** `nmap <droplet-ip>` shows ports 4317/4318 open.

### Pitfall 4: Base Image Not Available on Droplet
**What goes wrong:** `docker build` fails because `FROM ewr.vultrcr.com/grodt/grodt-base-image-2:3.9.0` cannot be pulled from the Vultr Container Registry.
**Why it happens:** The Vultr registry may require authentication or may not be publicly accessible from the DO droplet.
**How to avoid:** Either (a) make the Vultr registry image public, (b) push base images to Docker Hub, (c) push to DO Container Registry, or (d) build the base images on the droplet first using Dockerfile.base and Dockerfile.base2. Option (d) is slowest but has no registry dependency.
**Warning signs:** `docker build` fails with "unauthorized" or "not found" on the FROM line.

### Pitfall 5: EventStoreDB URL Needs Updating
**What goes wrong:** App tries to connect to `esdb://admin:changeit@eventstoredb.eventstoredb.svc.cluster.local:2113` (K8s service DNS).
**Why it happens:** The production configmap uses Kubernetes DNS names, not localhost.
**How to avoid:** In `.env.prod`, set `EVENTSTOREDB_URL=esdb://admin:changeit@eventstore.db:2113?tls=false&keepAliveTimeout=10000&keepAliveInterval=10000` using Docker Compose service name.
**Warning signs:** App logs show ESDB connection refused or DNS resolution failure.

### Pitfall 6: OPTIONS_CONFIG_FILE Path Mismatch
**What goes wrong:** Server fails to load options config because `PROJECT_DIR` or config path doesn't match container layout.
**Why it happens:** Production K8s uses `PROJECT_DIR=/app` but Docker container uses `WORKDIR /app/slack-trading`.
**How to avoid:** Set `PROJECT_DIR=/app/slack-trading` in `.env.prod`. Set `OPTIONS_CONFIG_FILE=options-config-prod.yaml` (the production config file exists at `src/go/options-config-prod.yaml`).
**Warning signs:** Server startup error about missing config file.

## Code Examples

### Production .env.prod Template

```bash
# .env.prod.template -- copy to .env.prod and fill in secrets

# Application
GO_ENV=production
ENV=production
PROJECT_DIR=/app/slack-trading
ANACONDA_HOME=/opt/conda
PORT=8080
LOG_LEVEL=info
DRY_RUN=false
OPTIONS_CONFIG_FILE=options-config-prod.yaml

# OpenTelemetry (localhost -- same machine)
OTEL_SERVICE_NAME=grodt
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-lgtm:4318

# Grafana
GF_ADMIN_PASSWORD=CHANGEME

# PostgreSQL (Docker Compose service name)
POSTGRES_DB=playground
POSTGRES_USER=grodt
POSTGRES_PASSWORD=CHANGEME
POSTGRES_HOST=postgres
POSTGRES_PORT=5432

# EventStoreDB (Docker Compose service name)
EVENTSTOREDB_URL=esdb://admin:changeit@eventstore.db:2113?tls=false&keepAliveTimeout=10000&keepAliveInterval=10000

# Tradier API
TRADIER_LIVE_NON_TRADES_BEARER_TOKEN=
TRADIER_LIVE_TRADES_BEARER_TOKEN=
TRADIER_SANDBOX_NON_TRADES_BEARER_TOKEN=
TRADIER_SANDBOX_TRADES_BEARER_TOKEN=
TRADIER_STOCK_QUOTES_URL=https://api.tradier.com/v1/markets/quotes
TRADIER_MARKET_CALENDAR_URL=https://api.tradier.com/v1/markets/calendar
TRADIER_OPTION_CHAIN_URL=https://api.tradier.com/v1/markets/options/chains
TRADIER_OPTION_EXPIRATIONS_URL=https://api.tradier.com/v1/markets/options/expirations
TRAIER_QUOTES_HISTORY_URL=https://api.tradier.com/v1/markets/history
TRADIER_MARKET_TIMESALES_URL=https://api.tradier.com/v1/markets/timesales
TRADIER_LIVE_ACCOUNT_ID=6YA49543
TRADIER_LIVE_TRADES_ACCOUNT_ID=6YA49543
TRADIER_LIVE_TRADES_URL_TEMPLATE=https://api.tradier.com/v1/accounts/%s/orders
TRADIER_LIVE_POSITIONS_URL_TEMPLATE=https://api.tradier.com/v1/accounts/%s/positions
TRADIER_LIVE_BALANCES_URL_TEMPLATE=https://api.tradier.com/v1/accounts/%s/balances
TRADIER_SANDBOX_ACCOUNT_ID=6YA49543
TRADIER_SANDBOX_TRADES_ACCOUNT_ID=VA12962195
TRADIER_SANDBOX_TRADES_URL_TEMPLATE=https://sandbox.tradier.com/v1/accounts/%s/orders
TRADIER_SANDBOX_POSITIONS_URL_TEMPLATE=https://sandbox.tradier.com/v1/accounts/%s/positions
TRADIER_SANDBOX_BALANCES_URL_TEMPLATE=https://sandbox.tradier.com/v1/accounts/%s/balances

# Polygon
POLYGON_API_KEY=

# Google Sheets
GOOGLE_SECURITY_KEY_JSON_BASE64=

# Slack
SLACK_OPTION_ALERTS_WEBHOOK_URL=

# Oanda
OANDA_BEARER_TOKEN=
OANDA_FX_QUOTES_URL_BASE=https://api-fxtrade.oanda.com/v3/instruments/%s/candles
```

### Grafana Auth Configuration (in Docker Compose)
```yaml
# Source: grafana/docker-otel-lgtm GitHub Discussion #354
# run-grafana.sh respects user-provided env vars over defaults
environment:
  - GF_AUTH_ANONYMOUS_ENABLED=false          # Disable anonymous access
  - GF_SECURITY_ADMIN_PASSWORD=${GF_ADMIN_PASSWORD}  # Set from .env.prod
  # GF_SECURITY_ADMIN_USER defaults to "admin" -- no need to set
```

### DO Cloud Firewall Rules (via MCP or API)
```
Inbound Rules:
  - Protocol: TCP, Port: 22,   Sources: 0.0.0.0/0 (SSH)
  - Protocol: TCP, Port: 3000, Sources: 0.0.0.0/0 (Grafana)
  - Protocol: ICMP                                  (ping)

Outbound Rules:
  - Protocol: TCP,  Ports: All, Destinations: 0.0.0.0/0
  - Protocol: UDP,  Ports: All, Destinations: 0.0.0.0/0
  - Protocol: ICMP,             Destinations: 0.0.0.0/0
```

### OTEL Endpoint Configuration
```bash
# In Docker Compose, services reference each other by service name.
# The Go server container connects to otel-lgtm by Docker DNS:
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-lgtm:4318

# This replaces the dev localhost URL since containers are on different
# Docker network interfaces (not sharing host network).
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Vultr K8s + Flux GitOps | DO Droplet + Docker Compose | This phase | Simpler ops for single-operator, no K8s overhead |
| Vultr Container Registry | Build on droplet | This phase | No registry dependency; slower deploys but simpler |
| UFW for firewall | DO Cloud Firewall | Docker iptables bypass well-documented since 2020 | Network-edge filtering avoids Docker bypass entirely |
| GF_AUTH_ANONYMOUS_ENABLED only via run-grafana.sh override | Environment variable respected by startup script | Recent otel-lgtm updates | Simpler auth configuration |

## Open Questions

1. **Base Image Availability**
   - What we know: Dockerfile `FROM ewr.vultrcr.com/grodt/grodt-base-image-2:3.9.0` requires Vultr registry access
   - What's unclear: Whether the Vultr registry is publicly accessible from DO
   - Recommendation: During execution, test pulling the image. If it fails, build base images on the droplet (Dockerfile.base then Dockerfile.base2) or push to Docker Hub. Building base images takes 15-30 minutes but only needs to happen once.

2. **Python Client Deployment**
   - What we know: Python strategy client needs conda env with grodt dependencies. The Dockerfile already includes the conda env.
   - What's unclear: Whether the Python client should run inside the grodt container or as a separate process
   - Recommendation: Run Python client inside the grodt container (it already has conda + all deps). Start it via a wrapper script or supervisor process. This avoids a second container build and shares the conda env.

3. **Swap File for Build**
   - What we know: 4GB RAM droplet may OOM during Docker build
   - What's unclear: Exact memory usage during `go mod download` + `go build` in container
   - Recommendation: Create a 2GB swap file on the droplet before building: `fallocate -l 2G /swapfile && mkswap /swapfile && swapon /swapfile`

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| DO MCP Server | Droplet provisioning (D-02) | Yes (remote MCP) | Current | Manual DO console |
| Docker + Compose | All containers | Yes (marketplace image) | CE 28.1.1 + Compose 2.36.0 | -- |
| git | Code deployment | Yes (Ubuntu default) | -- | scp / rsync |
| Vultr Container Registry | Base Docker image | Unknown | -- | Build base images on droplet |

**Missing dependencies with no fallback:**
- None -- all critical dependencies have fallbacks.

**Missing dependencies with fallback:**
- Vultr Container Registry access from DO -- fallback is building base images on the droplet.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Manual smoke tests (infrastructure phase) |
| Config file | N/A |
| Quick run command | `curl -s http://<droplet-ip>:3000/api/health` |
| Full suite command | See checklist below |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DEPLOY-01 | Observability stack running on DO | smoke | `curl -s http://<ip>:3000/api/health \| jq .database` | N/A (manual) |
| DEPLOY-02 | Telemetry flowing to DO endpoint | smoke | `curl -s http://<ip>:3000/api/datasources \| jq '.[].name'` | N/A (manual) |
| DEPLOY-03 | Grafana accessible with auth | smoke | `curl -s -o /dev/null -w '%{http_code}' http://<ip>:3000` returns 302 (redirect to login) | N/A (manual) |

### Sampling Rate
- **Per task commit:** Verify compose services are running: `docker compose ps`
- **Per wave merge:** Full smoke test checklist
- **Phase gate:** All smoke tests pass, Grafana login confirmed

### Wave 0 Gaps
None -- this is an infrastructure phase with manual verification. No test framework needed.

## Project Constraints (from CLAUDE.md)

- Go module: `github.com/jiaming2012/slack-trading` (Go 1.22.4)
- Python env: `grodt` conda env, numpy pinned to 1.26.4
- Current registry: `ewr.vultrcr.com/grodt/` (Vultr)
- `gh` alias: User's shell aliases `gh` to `git checkout`. Use `/usr/local/bin/gh` or `command gh`.
- Production options config: `options-config-prod.yaml` (exists at `src/go/options-config-prod.yaml`)
- Server entrypoint: `cmd/main.go`
- K8s resource requests from current deployment: 500m CPU, 1Gi RAM (limits: 2Gi)

## Sources

### Primary (HIGH confidence)
- Existing codebase files: `Dockerfile`, `Dockerfile.base`, `Dockerfile.base2`, `observability/docker-compose.yaml`, `eventstoredb/docker-compose.yaml`, `.clusters/production/deployment.yaml`, `.clusters/production/configmap.yaml`, `.env`
- [grafana/docker-otel-lgtm GitHub](https://github.com/grafana/docker-otel-lgtm) - auth configuration, run-grafana.sh env var behavior
- [DO Marketplace Docker Image](https://docs.digitalocean.com/products/marketplace/catalog/docker/) - Ubuntu 20.04, Docker CE 28.1.1, Compose 2.36.0
- [DO Cloud Firewalls](https://docs.digitalocean.com/products/networking/firewalls/) - network-edge filtering

### Secondary (MEDIUM confidence)
- [grafana/docker-otel-lgtm Discussion #354](https://github.com/grafana/docker-otel-lgtm/discussions/354) - auth enable/disable verified approach
- [DO MCP Server](https://github.com/digitalocean-labs/mcp-digitalocean) - droplet + firewall provisioning capabilities
- [Docker UFW bypass](https://docs.docker.com/engine/network/packet-filtering-firewalls/) - official Docker docs on iptables behavior
- [DO Droplet Pricing](https://www.digitalocean.com/pricing/droplets) - s-2vcpu-4gb at $24/mo

### Tertiary (LOW confidence)
- Memory requirements for otel-lgtm in production -- no official figures found; 4GB total for all services is an estimate based on K8s resource requests (2Gi for app) + community reports (4Gi for otel-lgtm max)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all components are existing (otel-lgtm, Postgres, ESDB) or well-documented (DO droplets)
- Architecture: HIGH - single Docker Compose on single VM is a well-understood pattern
- Pitfalls: HIGH - Docker/UFW bypass is extensively documented; Grafana auth behavior confirmed via GitHub discussion

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (30 days -- stable infrastructure components)
