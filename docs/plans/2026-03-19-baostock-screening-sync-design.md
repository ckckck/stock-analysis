# Baostock Screening Sync Fallback Design

**Date:** 2026-03-19

**Goal:** Add a Baostock-backed daily-bar source for AI screening sync, with Baostock as the primary source and the existing Sina daily K-line API as fallback.

## Context

- The current screening sync path calls `MarketService.GetScreeningDailyBars()`, which currently delegates directly to `GetKLineData(symbol, "1d", lookbackDays)`.
- The existing Sina daily K-line endpoint is unstable in this environment and can return HTTP `456` or block pages.
- `Baostock` was verified locally to return historical daily data for Shanghai and Shenzhen A-shares.
- The user wants this added as an independent implementation in this project, not by copying files from `daily_stock_analysis`.

## Approved Scope

- Only change the AI screening sync daily-bar path.
- Do not change other K-line consumers such as intraday, weekly, monthly, meeting tools, or main UI chart calls.
- Use `Baostock` as the first choice for screening sync daily bars.
- Fall back to the existing Sina daily-bar path only when `Baostock` fails or returns no usable bars.

## Recommended Architecture

### 1. Dedicated screening daily-bar source chain

Add a screening-specific source chain instead of changing the general `GetKLineData()` flow:

- `jcp/internal/services/baostock_daily_bar_source.go`
  - Encapsulates Baostock login/logout lifecycle
  - Converts `sh600000` / `sz000001` into `sh.600000` / `sz.000001`
  - Queries daily bars for a computed date range
  - Maps Baostock rows into `[]models.KLineData`

- `jcp/internal/services/screening_daily_bar_source.go`
  - Defines the screening sync source chain
  - Calls sources in order: `Baostock -> Sina`
  - Aggregates context-rich errors when both sources fail

- `jcp/internal/services/market_service.go`
  - Updates `GetScreeningDailyBars()` to call the new screening-specific chain
  - Leaves `GetKLineData()` behavior unchanged

### 2. Why not change `GetKLineData()` globally

That would widen the regression surface across:

- main stock detail K-line fetches
- tool integrations
- non-daily periods
- any UI that already depends on Sina semantics

The user request is scoped to sync. The dedicated screening path keeps the change local.

## Data Flow

1. `ScreeningSyncService.Sync()` asks `source.GetScreeningDailyBars(symbol, lookbackDays)`.
2. `MarketService.GetScreeningDailyBars()` invokes the screening-specific source chain.
3. The chain tries `Baostock` first.
4. If `Baostock` returns usable bars, the chain returns immediately.
5. If `Baostock` fails or returns empty data, the chain falls back to Sina daily K-line fetch.
6. If both fail, the returned error contains both failure contexts.

## Error Handling

- `Baostock` failure should include symbol and whether the failure happened during code conversion, login, query, row parsing, or empty response handling.
- Sina fallback should preserve the newer HTTP status and response-preview error behavior already added.
- Combined failure should be explicit, for example:
  - `screening daily bars for sh600000 failed: baostock: ...; sina fallback: ...`

## Testing Strategy

### Unit tests

Add screening-source-focused tests without real network calls:

- `Baostock` success short-circuits fallback
- `Baostock` failure triggers Sina fallback
- empty `Baostock` result triggers Sina fallback
- both sources failing returns a combined error
- code conversion accepts only Shanghai/Shenzhen stock symbols needed by screening sync

### Existing regression tests

Keep and rerun the existing `market_service_test.go` status-check regression to ensure the Sina fallback still surfaces HTTP status clearly.

## Risks

- `Baostock` is better for historical daily sync but may lag intraday or same-day settlement, so fallback to Sina remains useful.
- If screening stock lists later include unsupported symbols, the `Baostock` source should fail fast and let Sina attempt fallback.
- Introducing direct `Baostock` dependency into Go requires verifying the library installs cleanly in this project environment.

## Acceptance Criteria

- AI screening sync daily bars use `Baostock` first and Sina second.
- Non-screening K-line flows remain unchanged.
- Failover is covered by tests.
- Errors are clearer than the current raw JSON-parse failure mode.
