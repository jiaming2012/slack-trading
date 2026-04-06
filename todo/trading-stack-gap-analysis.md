# Trading Stack Architecture — Gap Analysis & Recommendations

**Prepared for:** Panel Review  
**Date:** April 4, 2026  
**Scope:** Full architecture review of the five-component feedback loop trading system  
**Classification:** Internal — Confidential

---

## Executive Summary

The architecture is well-designed in its core feedback loop structure. The EV-weighted training pipeline, fidelity checker as a trust gate, and regime-conditional modeling all reflect mature quantitative thinking. However, the review identified **12 material gaps** across four domains: resilience, risk management, model governance, and operational safety. Seven are high-severity — meaning they could cause silent capital loss or cascade failures before any alert fires.

---

## Finding 1: No Portfolio-Level Risk Management Layer

**Severity: CRITICAL**

The architecture optimizes individual strategies in isolation. There is no component that manages aggregate exposure, cross-strategy correlation, or total portfolio drawdown. The Strategy Optimizer tunes stop/target/sizing per strategy, but nothing prevents five strategies from simultaneously going max-long in correlated tech names during a sector rotation.

**What can go wrong:** A regime shift hits and three "independent" strategies all hold overlapping positions. The individual stop losses are respected, but aggregate drawdown exceeds what the account can absorb. Each strategy looks fine in isolation; the portfolio doesn't.

**Recommendation — Portfolio Risk Overlay**

Add a thin risk layer between the Simulator output and Live Trades that enforces:

- Max aggregate exposure (gross and net)
- Max sector/correlation concentration
- Portfolio-level drawdown circuit breaker (e.g., halt all entries if rolling 5-day portfolio drawdown > X%)
- Per-strategy capital allocation caps weighted by EV rank

| | |
|---|---|
| **Cost** | ~2 weeks of Go development; lightweight — runs as a pre-trade gate, not a new service |
| **Benefit** | Prevents correlated blowups that no single-strategy optimizer can detect; this is the single highest-ROI addition to the system |
| **Trade-off** | Introduces a constraint that may reject individually profitable trades; requires tuning the concentration limits without being overly conservative |

---

## Finding 2: No Live Trading Circuit Breaker

**Severity: CRITICAL**

The architecture has no emergency stop mechanism for live trading. If the market flash-crashes, a data feed goes stale, or a deployment bug sends malformed orders, there is nothing in the system that halts execution autonomously.

**What can go wrong:** A bad scanner config deploys (even with versioning and rollback), the Simulator happily generates trades from garbage candidates, and live orders fire before the weekly fidelity check catches the drift. Losses accumulate for up to a week.

**Recommendation — Kill Switch + Anomaly Guards**

Implement a multi-layered halt system:

- **Hard kill switch:** Manual API endpoint and physical dashboard button that halts all order submission instantly
- **Anomaly guard:** Auto-halt if any of the following occur within a rolling window:
  - Order rejection rate exceeds threshold
  - Fill prices deviate > N% from expected
  - Number of trades per hour exceeds 2σ of historical norm
  - Data feed staleness detected (last tick age > configurable threshold)
- **Cooldown protocol:** After any auto-halt, require manual acknowledgment before resuming

| | |
|---|---|
| **Cost** | 1 week of development; integrates into existing Temporal workflows as a pre-execution check |
| **Benefit** | Caps worst-case loss from operational failures; converts catastrophic tail events into bounded incidents |
| **Trade-off** | False halts during legitimate high-activity periods (earnings season, FOMC days); requires tuning thresholds per regime |

---

## Finding 3: Single-Host, Single-Region Infrastructure

**Severity: HIGH**

The entire stack runs on Hetzner with no documented failover, redundancy, or disaster recovery plan. Hetzner is cost-effective but provides no SLA comparable to cloud providers. A host failure, network partition, or datacenter incident takes the entire system offline — including live position management.

**What can go wrong:** Host goes down while positions are open. No system is running to manage stops or exits. Positions drift unmanaged until infrastructure is manually restored.

**Recommendation — Tiered Resilience**

Full HA is overkill for this stack's scale. Instead, implement a tiered approach:

- **Tier 1 (immediate):** Broker-side stop losses for every open position — ensures exits happen even if your infrastructure is completely down. This is non-negotiable.
- **Tier 2 (short-term):** Automated PostgreSQL backups to object storage (Hetzner Storage Box or Backblaze B2) with tested restore runbook. Snapshot-based VM recovery with a documented RTO.
- **Tier 3 (medium-term):** Warm standby on a second Hetzner region or DigitalOcean. The Go services are stateless; PostgreSQL streaming replication handles state. Reserved IP failover via the `godo` client you've already explored.

| | |
|---|---|
| **Cost** | Tier 1: zero (broker feature). Tier 2: ~$5–10/mo storage + 2 days of runbook work. Tier 3: ~$40–50/mo for standby host + 1 week setup. |
| **Benefit** | Tier 1 alone eliminates the catastrophic scenario of unmanaged open positions. Tier 3 gives <15 minute RTO. |
| **Trade-off** | Tier 3 adds operational complexity (replication lag monitoring, failover testing). For a solo operator, Tier 1 + Tier 2 may be the right stopping point until AUM justifies full HA. |

---

## Finding 4: Regime Classification Is a Single Point of Fragility

**Severity: HIGH**

The entire system pivots on `regime_tag` — it determines which model weights the scanner uses, which strategy parameters the optimizer tunes, and how the EV tracker segments performance. But the regime classifier itself is barely specified. There's a `regime_confidence` field and a 0.7 filter, but no detail on:

- How regime transitions are detected and how quickly
- What happens during ambiguous / transitional periods
- Whether regime tags are mutually exclusive or can blend
- How a misclassified regime propagates through the system

**What can go wrong:** The regime classifier lags a transition by 3–5 days. During that window, the scanner uses "trending" weights while the market has shifted to "high_vol." Candidates are scored on the wrong features, simulated under the wrong parameters, and pushed to live. The fidelity checker catches it a week later, but by then, several losing trades have executed.

**Recommendation — Regime Robustness Layer**

- Expose regime as a probability distribution, not a hard label (e.g., `{trending: 0.4, high_vol: 0.5, mean_reverting: 0.1}`)
- During ambiguous periods (max probability < 0.6), blend scanner configs proportionally or default to a conservative "uncertain" config with tighter filters and smaller position sizes
- Add a regime transition detector that triggers an immediate fidelity check and temporarily pauses optimizer updates
- Log regime transitions as first-class events in EventStoreDB for post-hoc analysis

| | |
|---|---|
| **Cost** | Moderate refactor: scanner must accept blended configs; ~1–2 weeks |
| **Benefit** | Eliminates the hard-label cliff-edge problem; graceful degradation during the most dangerous market periods (transitions) |
| **Trade-off** | Blended configs are harder to backtest cleanly; adds complexity to the scanner optimizer's training labels |

---

## Finding 5: Feedback Loop Overfitting Risk

**Severity: HIGH**

Three nested optimization loops (Strategy Optimizer weekly, Scanner Optimizer monthly, EV Tracker quarterly) create a classic multi-loop overfitting risk. The Strategy Optimizer tunes parameters on sim data, the Scanner Optimizer retrains on those sim outcomes, and the EV Tracker adjusts weights that influence both. Over time, the system can converge on an increasingly narrow set of trades that happened to work historically — a tightly coupled system that looks great on in-sample metrics but is brittle to any distributional shift.

**What can go wrong:** After several months, the scanner surfaces a very narrow candidate profile (e.g., mid-cap tech with specific RSI/volume signatures). The strategy optimizer has tuned entries and exits to perfection for that profile. Then the market regime shifts in a way the regime classifier doesn't fully capture, and the narrow profile stops working. The system's "intelligence" has become overspecialization.

**Recommendation — Overfitting Countermeasures**

- **Hold-out validation:** Reserve 20% of labeled data from every optimizer training cycle. Track in-sample vs. out-of-sample performance gap. Alert if gap exceeds threshold.
- **Candidate diversity floor:** Require the scanner to maintain a minimum sector/market-cap dispersion in its top-N output. Penalize configs that produce homogeneous candidate lists.
- **Regularization budget:** Add explicit regularization to the Bayesian optimization (e.g., penalize strategy configs that deviate too far from baseline defaults). Prefer robust over optimal.
- **Periodic "reset" benchmark:** Monthly, run the scanner with a baseline (non-optimized) config and compare candidate quality. If the optimized config's edge over baseline is shrinking, the optimization may be fitting noise.

| | |
|---|---|
| **Cost** | Hold-out and reset benchmark: ~3 days. Diversity floor: 1 week. Regularization: integrated into existing optimizer. |
| **Benefit** | Keeps the system generalizable; prevents the silent death of over-optimized strategies that look healthy until they abruptly stop working |
| **Trade-off** | Regularization and diversity constraints will reduce peak in-sample performance. This is the correct trade — you're buying robustness. |

---

## Finding 6: Fidelity Checker Runs Too Infrequently

**Severity: HIGH**

The fidelity checker runs weekly. That means drift between the simulator and live execution can accumulate for up to seven trading days before detection. During that window, the optimizers continue to trust simulator output that may not reflect reality.

**What can go wrong:** A broker changes its routing logic on Monday, increasing fill slippage by 15bps. The fidelity checker doesn't run until Friday. Five days of trades execute with uncalibrated expectations. The strategy optimizer might even "tune" parameters to compensate for phantom performance that's actually slippage — optimizing toward an illusion.

**Recommendation — Continuous Fidelity Monitoring**

- Run a lightweight fidelity comparison on every live trade in near-real-time: compare actual fill price, execution time, and realized slippage against what the simulator would have predicted
- Aggregate these per-trade deltas into a running drift score updated daily
- Keep the full weekly deep comparison, but add a daily "early warning" threshold that pauses optimizer ingestion of new sim data if intra-week drift spikes

| | |
|---|---|
| **Cost** | Moderate: requires hooking into the order execution callback path; ~1 week to build the per-trade comparison + daily aggregation |
| **Benefit** | Reduces worst-case detection lag from 5 trading days to <1 day; prevents optimizers from training on stale sim assumptions |
| **Trade-off** | Per-trade fidelity checks add latency to the execution path if done synchronously. Recommend async: log the comparison, don't gate the trade on it. |

---

## Finding 7: EV Calculation Ignores Transaction Costs and Capacity

**Severity: MEDIUM-HIGH**

The EV formula is `(win_rate × avg_win) − (loss_rate × avg_loss)`. This omits commissions, ECN fees, borrow costs for shorts, and market impact. For a strategy trading frequently or in less-liquid names, these costs can erode 30–50% of gross EV. The system could show a strategy as "EV-positive" while it's actually net-negative after costs.

Additionally, there's no capacity modeling — no estimate of how much capital a strategy can deploy before market impact degrades its own edge.

**Recommendation — Net EV with Cost Model**

- Extend the EV calculation to subtract estimated transaction costs per trade (commissions + estimated spread cost + borrow cost for shorts)
- Store `gross_ev` and `net_ev` separately; all downstream decisions (EV weights, retirement signals) use `net_ev`
- Add a simple capacity estimate: max position size at which estimated market impact exceeds X% of expected edge

| | |
|---|---|
| **Cost** | Low: mostly a schema and calculation change; ~2–3 days |
| **Benefit** | Prevents the system from scaling strategies that only look profitable before costs — a common and expensive mistake in systematic trading |
| **Trade-off** | Cost estimation requires assumptions (average spread, commission tier). Imprecise is still vastly better than zero. |

---

## Finding 8: No Shadow/Canary Deployment for Config Changes

**Severity: MEDIUM-HIGH**

Scanner config changes go live via hot-swap polling. Strategy config changes feed directly into the simulator. There is versioning and rollback, but no mechanism to test a new config against live market conditions before promoting it to production.

**What can go wrong:** The scanner optimizer produces a new config that scores well on historical data but interacts poorly with current market microstructure. The config goes live, surfaces bad candidates for a full month (the scanner optimizer's cycle), and the damage compounds through the simulator and into live trades.

**Recommendation — Shadow Mode Pipeline**

- Run new configs in parallel ("shadow mode") for a configurable burn-in period before promotion
- Shadow scanner config runs alongside production, surfaces its own candidates, and those candidates are simulated — but not traded live
- After burn-in, compare shadow vs. production candidate quality on sim outcomes
- Promote only if shadow meets or exceeds production on key metrics

| | |
|---|---|
| **Cost** | Moderate: requires running parallel scanner instances and splitting sim capacity; ~2 weeks to build; doubles scan compute during burn-in |
| **Benefit** | Catches configs that backtest well but fail forward; prevents a full optimization cycle of live losses before detection |
| **Trade-off** | Delays deployment of genuinely improved configs by the burn-in period. In fast-moving regimes, this lag has a cost. Allow manual override for urgent regime-driven changes. |

---

## Finding 9: No Data Feed Redundancy or Staleness Detection

**Severity: MEDIUM**

The Data Layer references "real-time market feed" as a single source. There's no mention of feed health monitoring, fallback providers, or staleness detection. Every downstream component assumes the data is fresh and correct.

**What can go wrong:** The market data provider has an outage or delivers stale/delayed quotes. The scanner computes features on bad data, scores candidates incorrectly, and pushes garbage to the simulator. None of the downstream components can distinguish "bad scan due to bad data" from "bad scan due to bad model."

**Recommendation — Feed Health Layer**

- Add a heartbeat monitor on the data feed: alert if no new ticks received within expected interval per asset class
- Cross-check critical fields (last price, volume) against a secondary source (even a free delayed feed) as a sanity check
- Tag `scan_results` with a `feed_health` flag so downstream components can filter or downweight scans from degraded-feed periods
- If feed staleness exceeds threshold, auto-pause scanning rather than scanning on stale data

| | |
|---|---|
| **Cost** | Low: ~3 days for heartbeat + staleness detection; secondary feed cross-check is optional and depends on data provider costs |
| **Benefit** | Prevents garbage-in-garbage-out cascades; makes data quality a first-class observable in the system |
| **Trade-off** | Secondary feed adds cost and complexity. Heartbeat monitoring alone captures most failure modes. |

---

## Finding 10: EventStoreDB Usage Is Underspecified

**Severity: MEDIUM**

EventStoreDB is listed for trade events, but the architecture doesn't describe event schemas, projection patterns, replay strategy, or how it integrates with the PostgreSQL-based analytical path. It's unclear whether EventStoreDB is being used as a true event-sourced backbone or as a glorified audit log.

**What can go wrong:** If EventStoreDB is the source of truth for trade events but PostgreSQL is where analytics happen, there's a consistency risk between the two. If it's just an audit log, the operational overhead of running EventStoreDB may not be justified.

**Recommendation — Clarify Role and Integration**

Pick one of two clean patterns:

- **Pattern A — Event-Sourced Backbone:** EventStoreDB is the source of truth. PostgreSQL tables are projections rebuilt from events. This gives you full replay capability, time-travel debugging, and audit compliance — but requires building and maintaining projections.
- **Pattern B — PostgreSQL Primary, EventStoreDB as Audit Trail:** PostgreSQL is the source of truth for all analytical and operational data. EventStoreDB captures raw events for compliance, debugging, and replay testing only. Simpler, but no native replay.

| | |
|---|---|
| **Cost** | Pattern A: significant investment in projection infrastructure (~2–4 weeks). Pattern B: minimal, mostly documentation and consistency guarantees. |
| **Benefit** | Pattern A gives full temporal replay (invaluable for debugging optimizer behavior and regulatory questions). Pattern B reduces operational surface. |
| **Trade-off** | For a solo-operator stack at this scale, Pattern B is likely the right call now. Design the schema so Pattern A is achievable later without a rewrite. |

---

## Finding 11: No Strategy Correlation or Crowding Detection

**Severity: MEDIUM**

The system treats each strategy independently. There's no mechanism to detect when multiple strategies converge on the same trades, effectively creating an unintentional concentration that multiplies risk without multiplying edge.

**Recommendation:** Add a post-scan deduplication check that flags when >N% of candidates overlap across strategies in the same scan cycle. Feed overlap metrics into the portfolio risk overlay from Finding 1.

| | |
|---|---|
| **Cost** | Low: ~2 days; a SQL query over the scan queue |
| **Benefit** | Prevents silent concentration buildup |

---

## Finding 12: Database Growth and Retention Strategy Missing

**Severity: LOW-MEDIUM**

`scan_results` and `sim_outcomes` will grow linearly and indefinitely. With multiple strategies per scan batch and daily scanning, you'll accumulate hundreds of thousands of rows per quarter. No partitioning, archival, or retention policy is defined.

**Recommendation:** Partition `scan_results` and `sim_outcomes` by month using PostgreSQL native partitioning. Archive partitions older than 12 months to cold storage. Keep `feature_distributions` and `strategy_ev_weights` indefinitely (small tables, high analytical value).

| | |
|---|---|
| **Cost** | ~1 day for partitioning setup; minimal ongoing |
| **Benefit** | Keeps query performance stable as data grows; prevents the slow degradation that hits unpartitioned tables at ~10M+ rows |

---

## Priority Ranking for Implementation

| Priority | Finding | Rationale |
|---|---|---|
| **P0** | #2 — Circuit Breaker | Prevents unbounded loss from operational failures |
| **P0** | #3 Tier 1 — Broker-Side Stops | Zero-cost, eliminates catastrophic unmanaged-position risk |
| **P1** | #1 — Portfolio Risk Overlay | Prevents correlated blowups invisible to single-strategy optimization |
| **P1** | #7 — Net EV with Costs | Low effort, high impact — prevents scaling money-losing strategies |
| **P1** | #6 — Continuous Fidelity | Reduces detection lag from 5 days to <1 day |
| **P2** | #4 — Regime Robustness | Hardens the system's most fragile dependency |
| **P2** | #5 — Overfitting Countermeasures | Protects long-term viability of the optimization loops |
| **P2** | #8 — Shadow Deployment | Catches bad configs before they go live |
| **P3** | #9 — Feed Health | Prevents garbage-in cascades |
| **P3** | #10 — EventStoreDB Clarification | Architectural hygiene |
| **P3** | #11 — Crowding Detection | Feeds into portfolio risk layer |
| **P3** | #12 — Database Partitioning | Operational hygiene, do before data volume makes it painful |

---

## Closing Assessment

The architecture's core design — a feedback loop with the fidelity checker as a trust gate and EV-weighted training — is sound and reflects genuine quantitative rigor. The gaps identified here are not design flaws so much as missing defensive layers. The highest-priority additions (circuit breaker, broker-side stops, portfolio risk overlay) are relatively low-effort and address the scenarios where the system fails silently and expensively. The medium-priority items (regime robustness, overfitting countermeasures, shadow deployment) are what separate a backtested system from a production-grade one.

The recommended implementation sequence respects the existing build order and can be phased in without disrupting current development.
