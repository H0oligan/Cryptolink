# Available Coins and Tokens

This document lists all cryptocurrencies and tokens supported by the payment gateway.

## Summary

- **Total Blockchains**: 10
- **Total Currencies**: 23 (including 1 deprecated)
- **Active Currencies**: 22

---

## 1. Bitcoin (BTC) ⭐ NEWLY ENABLED

**Network ID**: mainnet | testnet
**Block Explorer**: https://blockchair.com/bitcoin

| Ticker | Name | Type | Decimals | Contract Address | Min Withdrawal |
|--------|------|------|----------|------------------|----------------|
| BTC | BTC | Coin | 8 | - | $50 |

**Features**:
- Original cryptocurrency (est. 2009)
- Most secure and decentralized network
- Store of value and digital gold
- BIP21 payment URI support

**Payment URI**: `bitcoin:address?amount=value`

---

## 2. Ethereum (ETH)

**Network ID**: 1 (Mainnet) | 5 (Goerli Testnet)
**Block Explorer**: https://etherscan.io

| Ticker | Name | Type | Decimals | Contract Address | Min Withdrawal |
|--------|------|------|----------|------------------|----------------|
| ETH | ETH | Coin | 18 | - | $40 |
| ETH_USDT | USDT | Token | 6 | 0xdac17f958d2ee523a2206206994597c13d831ec7 | $40 |
| ETH_USDC | USDC | Token | 6 | 0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48 | $40 |

---

## 3. Polygon (MATIC)

**Network ID**: 137 (Mainnet) | 80001 (Mumbai Testnet)
**Block Explorer**: https://polygonscan.com

| Ticker | Name | Type | Decimals | Contract Address | Min Withdrawal |
|--------|------|------|----------|------------------|----------------|
| MATIC | MATIC | Coin | 18 | - | $10 |
| MATIC_USDT | USDT | Token | 6 | 0xc2132d05d31c914a87c6611c10748aeb04b58e8f | $10 |
| MATIC_USDC | USDC | Token | 6 | 0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174 | $10 |

---

## 4. Tron (TRON)

**Network ID**: mainnet | testnet
**Block Explorer**: https://tronscan.org

| Ticker | Name | Type | Decimals | Contract Address | Min Withdrawal |
|--------|------|------|----------|------------------|----------------|
| TRON | TRON | Coin | 6 | - | $10 |
| TRON_USDT | USDT | Token | 6 | TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t | $10 |

---

## 5. BNB Chain (BSC)

**Network ID**: 56 (Mainnet) | 97 (Testnet)
**Block Explorer**: https://bscscan.com

| Ticker | Name | Type | Decimals | Contract Address | Min Withdrawal | Status |
|--------|------|------|----------|------------------|----------------|--------|
| BNB | BNB | Coin | 18 | - | $10 | Active |
| BSC_USDT | USDT | Token | 18 | 0x55d398326f99059fF775485246999027B3197955 | $10 | Active |
| BSC_BUSD | BUSD | Token | 18 | 0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56 | $10 | **DEPRECATED** |

---

## 6. Arbitrum One (ARBITRUM) ⭐ NEW

**Network ID**: 42161 (Mainnet) | 421614 (Sepolia Testnet)
**Block Explorer**: https://arbiscan.io

| Ticker | Name | Type | Decimals | Contract Address | Min Withdrawal |
|--------|------|------|----------|------------------|----------------|
| ARB | ETH | Coin | 18 | - | $20 |
| ARBITRUM_USDT | USDT | Token | 6 | 0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9 | $20 |
| ARBITRUM_USDC | USDC | Token | 6 | 0xaf88d065e77c8cC2239327C5EDb3A432268e5831 | $20 |

**Features**:
- Layer 2 Ethereum scaling solution
- Low transaction fees (~$0.10-0.50)
- Fast confirmations (2-3 seconds)
- Full Ethereum compatibility

---

## 7. Avalanche C-Chain (AVAX) ⭐ NEW

**Network ID**: 43114 (Mainnet) | 43113 (Fuji Testnet)
**Block Explorer**: https://snowtrace.io

| Ticker | Name | Type | Decimals | Contract Address | Min Withdrawal |
|--------|------|------|----------|------------------|----------------|
| AVAX | AVAX | Coin | 18 | - | $15 |
| AVAX_USDT | USDT | Token | 6 | 0x9702230A8Ea53601f5cD2dc00fDBc13d4dF4A8c7 | $15 |
| AVAX_USDC | USDC | Token | 6 | 0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E | $15 |

**Features**:
- High-throughput blockchain
- Sub-second finality
- Low fees (~$0.10-0.30)
- EVM-compatible

---

## 8. Solana (SOL) ⭐ NEW

**Network ID**: mainnet-beta | devnet
**Block Explorer**: https://explorer.solana.com

| Ticker | Name | Type | Decimals | Contract Address | Min Withdrawal |
|--------|------|------|----------|------------------|----------------|
| SOL | SOL | Coin | 9 | - | $5 |
| SOL_USDT | USDT | Token | 6 | Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB | $5 |
| SOL_USDC | USDC | Token | 6 | EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v | $5 |

**Features**:
- Ultra-fast blockchain (50,000+ TPS)
- Extremely low fees (~$0.0001-0.001)
- Near-instant finality
- SPL token standard

---

## 9. Monero (XMR) ⭐ NEW

**Network ID**: mainnet | testnet
**Block Explorer**: https://xmrchain.net

| Ticker | Name | Type | Decimals | Contract Address | Min Withdrawal |
|--------|------|------|----------|------------------|----------------|
| XMR | XMR | Coin | 12 | - | $20 |

**Features**:
- Privacy-focused cryptocurrency
- Untraceable transactions
- Ring signatures and stealth addresses
- Dynamic block size

**⚠️ Important Notes**:
- Monero implementation is simplified
- For production use, integrate proper Monero libraries
- Privacy features require special handling

---

## Currency Statistics

### By Blockchain Type

| Category | Count |
|----------|-------|
| EVM-Compatible Chains | 5 (ETH, MATIC, BSC, ARBITRUM, AVAX) |
| Non-EVM Chains | 5 (BTC, TRON, SOL, XMR) |
| **Total Blockchains** | **10** |

### By Currency Type

| Type | Count |
|------|-------|
| Native Coins | 10 |
| Tokens | 13 (12 active + 1 deprecated) |
| **Total** | **23** |

### By Stablecoin Availability

| Stablecoin | Available On | Count |
|------------|--------------|-------|
| USDT | ETH, MATIC, TRON, BSC, ARBITRUM, AVAX, SOL | 7 |
| USDC | ETH, MATIC, BSC, ARBITRUM, AVAX, SOL | 6 |
| BUSD | BSC (deprecated) | 1 |

---

## Transaction Fee Comparison

| Blockchain | Avg Fee (Native Transfer) | Avg Fee (Token Transfer) | Speed |
|------------|---------------------------|--------------------------|-------|
| Bitcoin | $1-5 | N/A | 10-30min |
| Ethereum | $5-50 | $10-100 | 12-15s |
| Polygon | $0.01-0.05 | $0.02-0.10 | 2-3s |
| BNB Chain | $0.10-0.50 | $0.20-1.00 | 3s |
| Arbitrum | $0.10-0.50 | $0.20-1.00 | 2-3s |
| Avalanche | $0.10-0.30 | $0.20-0.60 | <1s |
| Solana | $0.0001-0.001 | $0.0001-0.001 | <1s |
| Tron | $0.01-0.05 | $1-2 (TRC20) | 3s |
| Monero | $0.01-0.10 | N/A | 2min |

---

## Minimum Withdrawal Thresholds

| Blockchain | Threshold (USD) | Reason |
|------------|-----------------|--------|
| Bitcoin | $50 | Transaction fees + UTXO dust |
| Ethereum | $40 | High gas fees |
| Polygon | $10 | Low fees |
| BNB Chain | $10 | Moderate fees |
| Tron | $10 | Low fees |
| Arbitrum | $20 | L2 fees |
| Avalanche | $15 | Moderate fees |
| Solana | $5 | Ultra-low fees |
| Monero | $20 | Privacy overhead |

---

## Hot Wallet Batching Support

All blockchains support the **hot wallet batching optimization** implemented in this payment gateway:

✅ **50% fee savings** by sweeping multiple hot wallets directly to merchant
✅ **No internal consolidation** needed
✅ **On-demand withdrawals** only when merchant requests

**How it works**:
1. Customer payments go to unique hot wallets
2. When merchant withdraws, system sweeps ALL hot wallets with balance
3. Parallel transactions from hot wallets → merchant address
4. Single consolidated payment to merchant
5. Saves 50% in blockchain fees!

---

## Network Identifiers Reference

### EVM Chains

| Blockchain | Mainnet ID | Testnet ID |
|------------|------------|------------|
| Ethereum | 1 | 5 (Goerli) |
| Polygon | 137 | 80001 (Mumbai) |
| BNB Chain | 56 | 97 |
| Arbitrum | 42161 | 421614 (Sepolia) |
| Avalanche | 43114 | 43113 (Fuji) |

### Non-EVM Chains

| Blockchain | Mainnet | Testnet |
|------------|---------|---------|
| Bitcoin | mainnet | testnet |
| Tron | mainnet | testnet |
| Solana | mainnet-beta | devnet |
| Monero | mainnet | testnet |

---

## Adding More Coins

To add a new coin/token:

1. **For EVM-compatible chains**: Add to `currencies.json` with appropriate network ID and contract address
2. **For new blockchain types**:
   - Add blockchain constant to `internal/kms/wallet/wallet.go`
   - Create wallet provider (e.g., `solana.go`, `monero.go`)
   - Add validation function
   - Register provider in `internal/kms/app.go`
   - Add currency definitions to `currencies.json`
   - Add block explorer to `internal/service/blockchain/currencies.go`

---

## Deprecated Currencies

| Ticker | Blockchain | Reason | Date |
|--------|------------|--------|------|
| BSC_BUSD | BNB Chain | Binance discontinued BUSD | 2024 |

---

**Last Updated**: 2025-11-24
**Version**: 2.1.0 (Enabled Bitcoin support)
**Total Active Currencies**: 22
