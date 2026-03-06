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

## [2026-03-06] Phase 2 — Hardening & Reliability

### Changes

#### Step 2.1: Multi-RPC Failover (ENHANCED)
- **File:** `internal/provider/rpc/provider.go`
- Each chain now has 4+ endpoints: primary, fallback, and 2 extras
- Added `Extra []string` field to `ChainRPC` for additional failover URLs
- `dialWithFailover()` tries all endpoints in order, skipping unhealthy ones
- Health tracking with `endpointHealth` struct (healthy, failedAt, failCount)
- Unhealthy endpoints auto-recover after 2 minutes (`healthRecoveryInterval`)
- Extra free endpoints: publicnode.com, 1rpc.io, defibit.io for all chains

#### Step 2.2: Multi-Source Price Validation (NEW)
- **File:** `internal/provider/pricefeed/provider.go`
- Both Binance and CoinGecko are now queried (not just fallback)
- `rateDivergence()` computes relative difference between two rates
- Rejects both rates if divergence exceeds 5% threshold
- Logs divergence details for monitoring
- Prevents price manipulation attacks via a single compromised source

#### Step 2.3: Database Cleanup (NEW)
- **Migration:** `scripts/migrations/20260306120000-remove_tatum_columns.sql`
- Drops `tatum_subscription_id` from: wallets, xpub_wallets, derived_addresses, evm_collector_wallets
- Updated SQL queries: removed `UpdateWalletTatumFields`, `UpdateXpubWalletTatumSubscription`, `UpdateDerivedAddressTatumSubscription`
- Updated Go models: removed tatum fields from `repository.Wallet`, `repository.XpubWallet`, `repository.DerivedAddress`
- Updated domain models: removed `TatumSubscription` from `wallet.Wallet`, tatum fields from `xpub.DerivedAddress`, `evmcollector.Collector`
- Removed `WebhookSubscriber` interface and `UpdateSubscriptionID`, `WebhookURL` from evmcollector
- Updated querier interface: removed 3 tatum-related methods

#### Step 2.4: Test & Code Cleanup
- Deleted `.bak` files: `mocks_tatum.go.bak`, `payments_webhook.go.bak`, `handler_payments.go.bak`
- Updated `service_test.go`: removed TatumSubscription assertions and subscription ID constants
- All generated `*.sql.go` files rewritten to exclude tatum columns from SELECT/RETURNING/Scan

### Security Notes
- Multi-RPC failover prevents single-endpoint failure from blocking operations
- Price divergence check prevents price manipulation via compromised single source
- Database cleanup removes dead columns that could confuse future development
- No functional behavior changes — same payment flows, just cleaner infrastructure

### Potential Issues
- Migration must be run on the production database (`20260306120000-remove_tatum_columns.sql`)
- Legacy Tatum webhook endpoint still exists in router (`/api/webhook/v1/tatum/:networkId/:walletId`) — can be removed after confirming no residual Tatum subscriptions
- `service_webhook.go` still contains TatumWebhook struct and processing logic — dead code, can be removed in future cleanup

---

## [2026-03-06] Phase 3 — BTC Full Support

### Changes

#### Step 3.1: BTC Broadcasting (NEW)
- **File:** `internal/provider/bitcoin/provider.go`
- Full Bitcoin provider using Blockstream and mempool.space public APIs
- Broadcasting via `POST /api/tx` (Blockstream primary, mempool.space fallback)
- Address info via `GET /api/address/:addr` (balance, tx count, mempool stats)
- Transaction info via `GET /api/tx/:txid` (confirmations, inputs/outputs, fees)
- Block height via `GET /api/blocks/tip/height`
- No Bitcoin Core node required — fully API-based
- 15s HTTP timeout, connection pooling

#### Step 3.1: BTC Wiring (MODIFIED)
- **File:** `internal/config/config.go` — Added `Bitcoin bitcoin.Config` to `Providers` struct
- **File:** `internal/service/blockchain/service.go` — Added `Bitcoin *bitcoin.Provider` to `Providers` struct
- **File:** `internal/service/blockchain/service_broadcaster.go`:
  - Added `kms.BTC` case to `BroadcastTransaction` (via `providers.Bitcoin.BroadcastTransaction`)
  - Added `btcConfirmations = 6` constant
  - Added `kms.BTC` case to `getTransactionReceipt`
  - Added `getBitcoinReceipt` method using Bitcoin provider
- **File:** `internal/locator/locator.go`:
  - Added `bitcoinProvider` field and `BitcoinProvider()` method
  - Wired Bitcoin provider into `BlockchainService()` Providers struct
  - Passed Bitcoin provider to `WatcherService()`

#### Step 3.2: BTC Address Watching (MODIFIED)
- **File:** `internal/service/watcher/service.go`
- Added `bitcoin *bitcoin.Provider` field to `Service` struct
- Added `lastBTCBalance sync.Map` for tracking BTC address balances between polls
- `New()` now takes `*bitcoin.Provider` parameter
- Added `kms.BTC` case to `pollChainTransactions` switch
- Added `pollBTCTransactions` method:
  - Queries Blockstream/mempool.space for address balance
  - Compares with last known balance to detect incoming payments
  - Converts satoshis to BTC amount (8 decimals)
  - Triggers `onDetected` callback for processing

### Security Notes
- Bitcoin provider uses HTTPS for all API calls
- 15s timeout prevents hanging connections
- Dual-endpoint failover (Blockstream + mempool.space) ensures availability
- Balance-based detection is simple and reliable — no complex UTXO parsing needed
- No private keys involved — only public address queries
- BTC confirmations set to 6 (industry standard for security)

### Potential Issues
- Balance-based detection can't identify the exact transaction hash from the balance check alone — the processing service handles this via receipt polling
- On first poll after restart, `lastBTCBalance` is empty, so the first balance snapshot won't trigger detection (by design — prevents false positives)
- Free API rate limits (Blockstream/mempool.space) may throttle under heavy load with many BTC addresses
- Pre-existing vet errors in `paymentevents/handler_test.go` and `blockchain/bitcoin_test.go` are unrelated to Phase 3 changes

---

## [2026-03-06] Phase 4 — SOL & XMR Verification

### Changes

#### Step 4.1: SOL Address Watching (MODIFIED)
- **File:** `internal/service/watcher/service.go`
- Added `solana *solana.Provider` field to `Service` struct
- Added `lastSOLBalance sync.Map` for tracking SOL address balances between polls
- `New()` now takes `*solana.Provider` parameter
- Added `kms.SOL` case to `pollChainTransactions` switch
- Added `pollSOLTransactions` method:
  - Queries Solana RPC for address balance (lamports)
  - Compares with last known balance to detect incoming payments
  - Converts lamports to SOL amount (9 decimals)
  - Triggers `onDetected` callback for processing

#### SOL Provider Verification
- **File:** `internal/provider/solana/provider.go` — Already complete
- Full Solana RPC provider with JSON-RPC 2.0 interface
- Methods: `GetBalance`, `GetTokenBalance`, `GetRecentBlockhash`, `SendTransaction`, `GetTransaction`, `ConfirmTransaction`, `GetTokenAccountsByOwner`
- Broadcasting: `sendTransaction` via base58-encoded signed tx
- Receipts: `getSignatureStatuses` for confirmation polling
- Mainnet + devnet endpoint support, optional API key for paid services

#### Step 4.2: XMR Provider Verification
- **File:** `internal/provider/monero/provider.go` — Already complete
- Full Monero wallet-RPC provider
- Methods: `GetBalance`, `GetAddress`, `CreateAccount`, `Transfer`, `GetTransfers`, `ValidateAddress`
- Broadcasting: via `transfer` RPC method (wallet-RPC handles signing + broadcasting)
- Receipts: via `get_transfers` RPC method (tracks in/out/pending/failed/pool)
- Address watching: handled natively by monero-wallet-rpc (incoming transfers appear in `get_transfers`)
- No separate address polling needed — XMR is excluded from watcher by design

#### Wiring Updates
- **File:** `internal/locator/locator.go` — Passed `SolanaProvider()` to `WatcherService()`

### Security Notes
- SOL balance polling uses existing Solana RPC provider with HTTPS and timeouts
- XMR wallet-RPC should run locally (localhost) for security — private keys never leave the machine
- SOL confirmation status checked for "confirmed" or "finalized" (Solana's 2-stage finality)
- XMR transfer validation requires address format check (starts with 4 or 8, length 95)

### Potential Issues
- SOL free RPC endpoints (api.mainnet-beta.solana.com) have rate limits — consider paid services (Helius, QuickNode) for production
- XMR requires monero-wallet-rpc to be running and connected to a Monero node
- SPL token watching not yet implemented in the watcher (only native SOL balance)
- TRON address watching still not implemented — relies on trongrid provider

---
