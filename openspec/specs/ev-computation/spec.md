# ev-computation Specification

## Purpose
TBD - created by archiving change ev-tracker. Update Purpose after archive.
## Requirements
### Requirement: EV per strategy per regime

The system SHALL compute expected value using the formula `EV = (win_rate × avg_win) − (loss_rate × avg_loss)`, grouped independently by `(strategy_id, regime)`. A closed trade with `pnl > 0` is a win, `pnl < 0` is a loss, and `pnl == 0` is a breakeven that is excluded from win and loss counts. `win_rate` SHALL be `wins / decided` and `loss_rate` SHALL be `losses / decided` where `decided = wins + losses`, so that `win_rate + loss_rate = 1` whenever at least one decided trade exists. `avg_win` SHALL be the mean pnl of winning trades and `avg_loss` SHALL be the mean **absolute** pnl (a positive magnitude) of losing trades.

#### Scenario: Known fixture EV is computed exactly

- **WHEN** a strategy/regime group has 6 winners averaging +2.0 pnl and 4 losers averaging −1.0 pnl (10 decided trades)
- **THEN** win_rate is 0.6, avg_win is 2.0, loss_rate is 0.4, avg_loss is 1.0, and EV equals `(0.6 × 2.0) − (0.4 × 1.0) = 0.8`

#### Scenario: Breakeven trades are excluded from win/loss counts

- **WHEN** a group has 3 winners, 2 losers, and 1 breakeven trade (`pnl == 0`)
- **THEN** `decided` is 5, the breakeven trade contributes to neither `avg_win` nor `avg_loss`, and `win_rate + loss_rate` equals 1.0

#### Scenario: Groups are independent per strategy and regime

- **WHEN** trades for the same `strategy_id` are split across two distinct regimes
- **THEN** EV is computed separately for each `(strategy_id, regime)` pair and no trade from one regime affects the other's EV

### Requirement: Rolling 30-day, 90-day, and all-time windows

The system SHALL compute EV over three windows relative to a caller-supplied as-of timestamp: 30-day (trades closed within `[asOf − 30d, asOf]`), 90-day (`[asOf − 90d, asOf]`), and all-time (every trade at or before `asOf`). The as-of timestamp SHALL be a parameter, never read from the wall clock, so results are reproducible.

#### Scenario: Trade outside the 30-day window is excluded from ev_30d only

- **WHEN** a trade closed 45 days before the as-of timestamp
- **THEN** that trade is excluded from `ev_30d` but included in `ev_90d` and `ev_all_time`

#### Scenario: Trade after the as-of timestamp is excluded from every window

- **WHEN** a trade closed after the as-of timestamp
- **THEN** it is excluded from `ev_30d`, `ev_90d`, and `ev_all_time`

### Requirement: EV trend slope via least-squares regression

The system SHALL compute an EV trend slope for each `(strategy_id, regime)` group by partitioning that group's trades into consecutive equal-length time buckets (default 30 days) ordered oldest-to-newest, computing per-bucket EV for each bucket that contains at least one decided trade, and fitting an ordinary-least-squares slope of per-bucket EV against integer bucket index. When fewer than two non-empty buckets exist the slope SHALL be reported as null (undefined).

#### Scenario: Linearly increasing per-bucket EV yields the known slope

- **WHEN** a group has four consecutive non-empty buckets whose per-bucket EV values are 0.0, 0.2, 0.4, 0.6 at bucket indices 0,1,2,3
- **THEN** the computed slope equals 0.2 (the exact OLS slope of that linear series)

#### Scenario: Fewer than two non-empty buckets yields a null slope

- **WHEN** a group's trades all fall within a single bucket
- **THEN** the slope is null (undefined) rather than zero

### Requirement: Decay classification and EV weight bands

The system SHALL classify each `(strategy_id, regime)` group and assign an EV weight strictly from its slope: slope > 0.1 → weight 1.0 with status improving (scale up); 0 ≤ slope ≤ 0.1 → weight 0.7 with status stable (maintain); slope < 0 → weight 0.3 with status decaying (flag for review or retirement). When the slope is null (insufficient data) the weight SHALL be 0.7 with status insufficient-data. The band boundaries SHALL be inclusive as stated: exactly 0.1 and exactly 0.0 both fall in the stable band.

#### Scenario: Improving strategy scales up

- **WHEN** a group's slope is 0.2
- **THEN** its ev_weight is 1.0 and its status is improving

#### Scenario: Stable band boundaries map to weight 0.7

- **WHEN** a group's slope is exactly 0.1, or exactly 0.0
- **THEN** its ev_weight is 0.7 and its status is stable in both cases

#### Scenario: Decaying strategy is downweighted and flagged

- **WHEN** a group's slope is −0.05
- **THEN** its ev_weight is 0.3 and its status is decaying (flagged for review)

#### Scenario: Insufficient data defaults to the stable weight

- **WHEN** a group's slope is null because it has fewer than two non-empty buckets
- **THEN** its ev_weight is 0.7 and its status is insufficient-data

### Requirement: Strategy ranking by EV

The system SHALL expose, as a computed output, a ranking of `(strategy_id, regime)` groups ordered by descending all-time EV, with a deterministic tie-break by `strategy_id` then `regime`.

#### Scenario: Groups are ranked by descending EV

- **WHEN** three groups have all-time EV of 0.8, 0.1, and −0.3
- **THEN** the ranking orders them 0.8, 0.1, −0.3 and assigns ranks 1, 2, 3 respectively

### Requirement: Deterministic computation

The computation SHALL be a pure function of its trade inputs and the as-of timestamp: given identical inputs it SHALL produce byte-identical EV, slope, weight, and ranking outputs across repeated runs, and SHALL NOT read the wall clock, random sources, or external state.

#### Scenario: Repeated runs on the same fixture match

- **WHEN** the same fixture trade set and as-of timestamp are computed twice
- **THEN** every group's ev_30d, ev_90d, ev_all_time, ev_slope, ev_weight, and rank are identical between the two runs

