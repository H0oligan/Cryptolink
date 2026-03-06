# CryptoLink — Tatum Independence Roadmap

## Mission
Remove all dependency on Tatum (a paid third-party blockchain infrastructure provider) and make CryptoLink a fully self-contained, non-custodial crypto payment gateway.

## Current State
- Tatum provides: exchange rates, EVM RPC proxy, transaction broadcasting, webhook address monitoring, HMAC validation
- Free plan limit: 5 webhook subscriptions total (blocks new merchant onboarding)
- 3 merchants waiting to onboard
- SOL/XMR providers already independent from Tatum

## Security Principles
- Non-custodial: CryptoLink never holds merchant funds
- All RPC connections use HTTPS
- Rate limiting on all external API calls
- Multi-source price feeds to prevent price manipulation
- No private keys transmitted over network (KMS is local)
- Input validation on all blockchain data (addresses, amounts, tx hashes)

---

## Phase 1 — Core Independence (Target: Tatum-free operation)

### Step 1.1: Create RPC Provider
- **Status:** DONE
- **Files:** `internal/provider/rpc/provider.go`
- **What:** Replace `provider/tatum/provider_rpc.go` with configurable direct RPC endpoints
- **RPC Sources:** Public free endpoints + configurable overrides
  - ETH: `https://eth.llamarpc.com` / Ankr
  - MATIC: `https://polygon-rpc.com`
  - BSC: `https://bsc-dataseed.binance.org`
  - ARBITRUM: `https://arb1.arbitrum.io/rpc`
  - AVAX: `https://api.avax.network/ext/bc/C/rpc`
- **Security:** All URLs validated as HTTPS, connection timeouts enforced

### Step 1.2: Create Price Feed Provider
- **Status:** DONE
- **Files:** `internal/provider/pricefeed/provider.go`
- **What:** Replace Tatum exchange rate API with multi-source price feed
- **Sources:**
  - Primary: Binance public API (no key needed, 1200 req/min)
  - Fallback: CoinGecko API (free, 10-30 req/min)
- **Features:** 30s TTL cache, outlier detection, graceful fallback
- **Security:** Validate price data ranges, reject anomalous rates

### Step 1.3: Replace Transaction Broadcasting
- **Status:** DONE
- **Files:** `internal/service/blockchain/service_broadcaster.go`
- **What:** Replace Tatum SDK broadcast (ETH/MATIC/BSC) with direct RPC `eth_sendRawTransaction`
- **Note:** ARBITRUM/AVAX already use direct RPC; TRON/SOL/XMR already independent

### Step 1.4: Build Address Watcher Service
- **Status:** DONE
- **Files:** `internal/service/watcher/service.go`
- **What:** Replace Tatum webhook subscriptions with internal blockchain polling
- **How:**
  1. Watched addresses table tracks payment addresses + expected currency
  2. Poller checks balances every 10-15s in batches
  3. EVM: `eth_getBalance` + ERC20 `balanceOf()` calls
  4. BTC: Blockstream/mempool.space API (free, no key)
  5. Balance change detected -> triggers same `ProcessInboundTransaction` flow
- **Security:** Validate all RPC responses, handle chain reorgs

### Step 1.5: Remove Tatum Provider & SDK
- **Status:** DONE
- **Files:** Remove `internal/provider/tatum/`, update locator, config, go.mod
- **What:** Delete Tatum provider, remove `oxygenpay/tatum-sdk` dependency
- **Cleanup:** Remove `tatum_subscription_id` from webhook registration flows

### Step 1.6: Update Config & Wiring
- **Status:** DONE
- **Files:** `config/config.go`, `locator/locator.go`, `app/app.go`
- **What:** Add RPC + PriceFeed config, remove Tatum config, rewire services

---

## Phase 2 — Hardening & Reliability

### Step 2.1: Multi-RPC Failover
- **Status:** DONE
- **What:** If primary RPC endpoint fails, automatically try backup endpoints
- **Config:** 4+ endpoints per chain (primary, fallback, 2 extras), health tracking with 2min recovery

### Step 2.2: Multi-Source Price Validation
- **Status:** DONE
- **What:** Cross-check prices between Binance and CoinGecko, reject if >5% divergence
- **Security:** Prevents price manipulation attacks

### Step 2.3: Database Cleanup
- **Status:** DONE
- **What:** Remove `tatum_subscription_id` columns from wallets, xpub_wallets, derived_addresses, evm_collector_wallets tables
- **Migration:** `scripts/migrations/20260306120000-remove_tatum_columns.sql`

### Step 2.4: Test Updates
- **Status:** DONE
- **What:** Removed `mocks_tatum.go.bak`, cleaned up all tatum references in tests and domain models

---

## Phase 3 — BTC Full Support

### Step 3.1: BTC Broadcasting
- **Status:** PENDING
- **Files:** `internal/provider/bitcoin/provider.go`
- **What:** Broadcast BTC transactions via Blockstream API or mempool.space (free, no key, no node)
- **Note:** No 500GB Bitcoin Core download needed

### Step 3.2: BTC Address Watching
- **Status:** PENDING
- **What:** Add BTC balance checking to Address Watcher via Blockstream/mempool.space API

### Step 3.3: BTC End-to-End Test
- **Status:** PENDING
- **What:** Full payment flow test on testnet

---

## Phase 4 — SOL & XMR Verification

### Step 4.1: Verify SOL Provider
- **Status:** PENDING
- **What:** SOL provider exists, verify it works end-to-end for merchants

### Step 4.2: Verify XMR Provider
- **Status:** PENDING
- **What:** XMR provider exists, verify it works end-to-end for merchants

---

## Dependency Map (What Replaces What)

| Tatum Function | Replacement | Free? |
|---------------|-------------|-------|
| Exchange rates API | Binance + CoinGecko | Yes |
| EVM RPC proxy | Direct public RPCs | Yes |
| ETH/MATIC/BSC broadcast | Direct RPC eth_sendRawTransaction | Yes |
| Webhook subscriptions | Internal Address Watcher poller | Yes (self-hosted) |
| HMAC validation | Removed (no external webhooks) | N/A |
| BTC broadcast | Blockstream/mempool.space API | Yes |
| BTC address monitoring | Blockstream/mempool.space API | Yes |
