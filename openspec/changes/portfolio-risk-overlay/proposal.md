# Portfolio Risk Overlay

## Why

Strategies are optimized in isolation, so nothing stops several of them from simultaneously going max-long in correlated names; each strategy respects its own stops while aggregate portfolio drawdown quietly exceeds what the account can absorb. This change adds a thin pre-trade risk gate — wired into the Simulation path only — that enforces portfolio-level exposure, concentration, drawdown, and per-strategy allocation limits before an order reaches the Broker seam.

## What Changes

- Add a new `riskoverlay` subpackage under `tradingstack` containing a **pure, deterministic limit-evaluation engine** (`Evaluate`) that takes a snapshot of portfolio state plus a proposed order and returns an allow/reject decision with the specific limit breaches — no database access, so every breach is unit-testable with fixtures alone.
- Enforce **five limit families**, each independently configurable and each with its own breach path:
  - Max aggregate **gross** exposure and max aggregate **net** exposure.
  - Max **sector/correlation concentration** (single-sector share of gross exposure).
  - Portfolio-level **drawdown circuit breaker**: halt all *entries* when rolling 5-session portfolio drawdown exceeds a configurable X%.
  - **Per-strategy capital allocation caps weighted by EV rank**, derived from `strategy_ev_weights.ev_weight` (owned by `trading-stack-schema`).
  - **Crowding consumption**: reject (or tighten) entries into tickers flagged crowded by `crowding-detection` for the current scan cycle.
- Guarantee a **risk-reducing-orders-always-pass** invariant: orders that reduce or close an existing logical position are never blocked, even while the circuit breaker has halted entries.
- Read all limit values from an **options-config-YAML-pattern** config block with documented defaults, loaded via the existing `utils.GetEnv` / YAML conventions.
- Wire the gate into the **Simulation-mode order-placement path only**, behind an operator flag that is **default-off for Paper and Margin modes**. Enabling the gate on live/paper paths is operator-gated and explicitly out of scope for this change. **Not BREAKING** — Simulation behavior is unchanged when the gate is disabled, and the gate defaults to permissive-observe unless configured otherwise.
- Add a `task test:portfolio-risk-overlay` Taskfile target running this package's unit tests (new operator-visible command per project convention).
- No changes to any existing playground table, model, or migration, and no changes to the `trading-stack-schema` or `crowding-detection` tables — this change reads those models, it does not alter them.

## Capabilities

### New Capabilities

- `portfolio-risk-limits`
- `simulation-risk-gate`

### Modified Capabilities

(none)

## Impact

- New package: `src/go/tradingstack/riskoverlay/` (state types, limit engine, config loader, crowding lookup interface + fake, simulation-path gate adapter, tests). Reads `StrategyEvWeight` from `github.com/jiaming2012/slack-trading/src/go/tradingstack` and `CrowdingMetric` / `CrowdingFlaggedCandidate` from the crowding package — both read-only.
- One integration seam in the existing Simulation order path (guarded so Paper/Margin are untouched); `taskfile.yml` gains `test:portfolio-risk-overlay`. No other existing files change behavior.
- **Depends on `trading-stack-schema`** (for `strategy_ev_weights` / `StrategyEvWeight`) and **`crowding-detection`** (for the flagged-candidate read surface). This change must be authored and merged after both land.
- Safety-critical: this is a trade-gating control. Per the overnight plan it carries a **mandatory G6 Fable adversarial review** in addition to the deterministic gates.
- Verification is local-only: pure-function unit tests over fixtures plus `go build` / `task test`. No prod DB, no live/paper broker orders, no new external infrastructure. Live/paper enablement and real portfolio-state wiring are deferred (see design.md → Deferred validation).
