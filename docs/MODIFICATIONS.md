# CryptoLink — Modifications Log

## Purpose
This file tracks all changes made during the Tatum Independence project.
Each entry includes: what changed, why, security considerations, and potential issues.

---

## [2026-03-06] Project Setup

### Changes
- Created `feature/tatum-independence` branch from `main`
- Stashed pre-existing uncommitted changes (`git stash: WIP: pre-tatum-removal uncommitted changes`)
- Created project documentation: `docs/ROADMAP.md`, `docs/MODIFICATIONS.md`
- Created Claude Code hooks for automated commit workflow
- Created `.claude/hooks.json` for build verification

### Security Notes
- All stashed changes preserved — no work lost
- Branch isolation ensures main is untouched until merge

### Potential Issues
- Stashed changes need to be re-applied after this feature branch merges
- The stash includes UI and backend changes (19 files) that may conflict

---

## [2026-03-06] Phase 1 Core — Steps 1.1, 1.2, 1.3, 1.5 (partial), 1.6

### Changes

#### Step 1.1: RPC Provider (NEW)
- **File:** `internal/provider/rpc/provider.go`
- Created direct EVM RPC provider replacing Tatum RPC proxy
- Configurable endpoints per chain with fallback support
- Default free public RPCs: LlamaRPC (ETH), polygon-rpc.com (MATIC), bsc-dataseed (BSC), arbitrum.io (ARBITRUM), avax.network (AVAX)

#### Step 1.2: Price Feed Provider (NEW)
- **File:** `internal/provider/pricefeed/provider.go`
- Multi-source exchange rates: Binance (primary), CoinGecko (fallback)
- 30s TTL cache, rate validation, stablecoin self-pricing
- Replaces Tatum ExchangeRate API entirely

#### Step 1.3: Transaction Broadcasting (MODIFIED)
- **File:** `internal/service/blockchain/service_broadcaster.go`
- ETH/MATIC/BSC now broadcast via direct RPC `eth_sendRawTransaction`
- Removed Tatum SDK broadcast calls and `parseBroadcastError()`
- ARBITRUM/AVAX already used direct RPC; TRON/SOL/XMR already independent

#### Step 1.5 (partial): Tatum Provider Isolation
- All production code references to Tatum provider replaced
- `service_convertor.go`: Uses pricefeed provider instead of Tatum exchange rates
- `service_fees.go`: Uses RPC provider instead of Tatum RPC
- `service_broadcaster.go`: Uses RPC provider for all EVM chains
- `processing/service.go`: Webhook subscription methods stubbed as no-ops
- `processing/service_webhook.go`: HMAC validation stubbed (no external webhooks)
- `merchantapi/handler.go`: Removed Tatum provider dependency
- `merchantapi/evm_collector.go`: Removed Tatum webhook subscription block

#### Step 1.6: Config & Wiring (MODIFIED)
- `config/config.go`: `Tatum tatum.Config` -> `RPC rpc.Config` + `PriceFeed pricefeed.Config`
- `locator/locator.go`: `TatumProvider()` -> `RPCProvider()` + `PriceFeedProvider()`
- `app/app.go`: Updated NewHandler call (removed tatum/webhook args)

#### Test Infrastructure Updates
- **NEW:** `internal/test/mocks_pricefeed.go` — PriceFeedMock replacing TatumMock
- **DELETED:** `internal/test/mocks_tatum.go`
- **MODIFIED:** `internal/test/mocks_kms.go` — removed SetupSubscription calls
- **MODIFIED:** `service_convertor_test.go` — uses PriceFeedMock
- **MODIFIED:** 10+ test files — `TatumMock.SetupRates` -> `PriceFeedMock.SetupRates`

#### Pre-existing Files Isolated
- `internal/db/repository/payments_webhook.go` -> `.bak` (stashed feature, referenced undefined field)
- `internal/server/http/subscriptionapi/handler_payments.go` -> `.bak` (stashed feature, undefined method)
- `internal/test/mocks_tatum.go` -> `.bak` (replaced by mocks_pricefeed.go)

### Security Notes
- All RPC URLs validated as HTTPS in provider config
- Connection timeouts enforced (15s default)
- Price feed validates rate ranges (positive, < 1e12)
- Stablecoin rates hardcoded to 1.0 to prevent manipulation
- No private keys or sensitive data exposed in new providers

### Potential Issues
- `mocks_tatum.go` still exists as `.bak` — can be deleted after full test pass
- Webhook subscription methods are no-ops — Step 1.4 (Address Watcher) will provide real replacement
- `internal/provider/tatum/` directory still exists — full deletion in Step 1.5 completion
- Pre-existing `.bak` files need to be re-integrated after stash pop

---

## [2026-03-06] Step 1.4 — Address Watcher Service

### Changes

#### Address Watcher Service (NEW)
- **File:** `internal/service/watcher/service.go`
- Replaces Tatum webhook subscriptions with direct blockchain polling
- Scans recent blocks for native coin transfers to watched addresses
- Uses `eth_getLogs` with ERC-20 Transfer events for token detection
- Supports all 5 EVM chains: ETH, MATIC, BSC, ARBITRUM, AVAX
- TRON/SOL/XMR skipped (handled by their own providers, Phase 4)
- Configurable: block scan depth (default 50), max concurrency (default 4)
- Tracks last scanned block per chain to avoid redundant rescanning
- Uses callback pattern (`OnTransferDetected`) to avoid circular imports

#### Scheduler Integration
- **File:** `internal/scheduler/handler.go`
- Added `WatchPendingAddresses()` job — bridges watcher to processing
- `ProcessingService` interface extended with `ProcessInboundTransaction`
- `New()` now takes `*watcher.Service` parameter
- Job runs every 15 seconds (faster than old 30s Tatum webhook latency)

#### Config & Wiring
- `config/config.go`: Added `Watcher watcher.Config` to Oxygen struct
- `locator/locator.go`: Added `WatcherService()` method
- `app/app.go`: Watcher service passed to scheduler, cron job registered

#### Test Updates
- `scheduler/handler_test.go`: Updated `scheduler.New()` call (pass nil watcher)
- Removed stale test functions referencing non-existent scheduler methods
- `test/mock/processing.go`: Added `ProcessInboundTransaction` to mock

### Security Notes
- Watcher only reads from blockchain (no writes, no private key access)
- Block scanning uses existing RPC provider with HTTPS and timeouts
- No new attack surface — reuses existing `ProcessInboundTransaction` flow
- Rate limited by MaxConcurrency config (default 4 parallel RPC calls)

### Potential Issues
- High block scan depth on chains with fast block times may increase RPC load
- Free RPC endpoints may rate-limit during large initial scans
- TRON/SOL/XMR address watching not yet implemented (Phase 4)
- `lastScannedBlock` is in-memory — resets on server restart (first scan after restart may be slow)

---
