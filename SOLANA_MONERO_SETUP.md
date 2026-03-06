# Solana & Monero Integration Guide

Complete setup guide for Solana and Monero blockchain support.

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [Solana Setup](#solana-setup)
3. [Monero Setup](#monero-setup)
4. [Configuration](#configuration)
5. [Testing](#testing)
6. [Production Deployment](#production-deployment)
7. [Troubleshooting](#troubleshooting)

---

## Overview

### What's Implemented

#### Solana ✅
- Wallet generation (ed25519 keypairs)
- Native SOL transfers
- SPL token transfers (requires SDK)
- RPC provider with full transaction lifecycle
- Balance checking (SOL + SPL tokens)
- Transaction confirmation
- Block explorer integration

#### Monero ⚠️
- Wallet RPC integration
- Balance checking
- Transfer creation
- Transaction monitoring
- Address validation
- **Requires monero-wallet-rpc server**

### Architecture

```
┌─────────────────┐
│  Payment Gateway│
└────────┬────────┘
         │
    ┌────┴────┬──────────┐
    │         │          │
┌───▼───┐ ┌──▼───┐ ┌────▼─────┐
│ Tatum │ │Solana│ │  Monero  │
│  API  │ │ RPC  │ │Wallet-RPC│
└───────┘ └──────┘ └──────────┘
```

---

## Solana Setup

### 1. Install Dependencies

#### Option A: Install Solana Go SDK (Recommended)

```bash
# Install the complete Solana SDK
go get github.com/gagliardetto/solana-go@latest
go get github.com/gagliardetto/solana-go/rpc@latest
go get github.com/gagliardetto/solana-go/programs/token@latest

# Rebuild the project
make build
```

#### Option B: Use Public RPC (Basic Support)

The current implementation supports basic Solana operations using JSON-RPC without the full SDK.

### 2. Choose RPC Provider

#### Free Public RPCs

```yaml
# config/oxygen.yml
providers:
  solana:
    rpc_endpoint: https://api.mainnet-beta.solana.com
    devnet_rpc_endpoint: https://api.devnet.solana.com
```

**Limitations:**
- Rate limited
- May be slow
- Less reliable

#### Paid RPC Services (Recommended for Production)

##### Helius

```yaml
providers:
  solana:
    rpc_endpoint: https://mainnet.helius-rpc.com/?api-key=YOUR-API-KEY
    devnet_rpc_endpoint: https://devnet.helius-rpc.com/?api-key=YOUR-API-KEY
    api_key: YOUR-API-KEY
```

**Pricing:** Free tier: 100k requests/day

##### Alchemy

```yaml
providers:
  solana:
    rpc_endpoint: https://solana-mainnet.g.alchemy.com/v2/YOUR-API-KEY
    devnet_rpc_endpoint: https://solana-devnet.g.alchemy.com/v2/YOUR-API-KEY
    api_key: YOUR-API-KEY
```

**Pricing:** Free tier: 300M compute units/month

##### QuickNode

```yaml
providers:
  solana:
    rpc_endpoint: https://your-endpoint.solana-mainnet.quiknode.pro/YOUR-TOKEN/
    devnet_rpc_endpoint: https://your-endpoint.solana-devnet.quiknode.pro/YOUR-TOKEN/
```

**Pricing:** Starts at $9/month

### 3. Test Solana Integration

```bash
# Run on devnet first
./oxygen web

# Test wallet generation
curl -X POST http://localhost:3000/api/kms/wallets \
  -H "Content-Type: application/json" \
  -d '{"blockchain": "SOL"}'

# Expected response:
# {
#   "uuid": "...",
#   "address": "7xKxL...",  # Base58 Solana address
#   "blockchain": "SOL"
# }
```

### 4. Verify RPC Connection

```bash
# Check Solana RPC
curl -X POST https://api.devnet.solana.com \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "getHealth"
  }'

# Expected: {"jsonrpc":"2.0","result":"ok","id":1}
```

### 5. Solana Features Checklist

- [x] Wallet generation
- [x] Address validation
- [x] Balance checking (SOL)
- [x] Balance checking (SPL tokens)
- [x] Native SOL transfers
- [ ] SPL token transfers (needs full SDK)
- [x] Transaction confirmation
- [x] Recent blockhash fetching
- [x] Transaction broadcasting
- [x] Block explorer links

---

## Monero Setup

### 1. Install Monero Software

#### Ubuntu/Debian

```bash
# Add Monero repository
sudo add-apt-repository ppa:monero-project/monero
sudo apt update

# Install Monero
sudo apt install monero

# Verify installation
monerod --version
monero-wallet-rpc --version
```

#### From Source

```bash
git clone https://github.com/monero-project/monero.git
cd monero
git checkout release-v0.18
make release

# Binaries in build/release/bin/
```

### 2. Run Monero Wallet RPC

#### For Development (Testnet)

```bash
# Create wallet directory
mkdir -p /opt/monero/wallets

# Run wallet-rpc on testnet
monero-wallet-rpc \
  --rpc-bind-port 28082 \
  --wallet-dir /opt/monero/wallets \
  --testnet \
  --disable-rpc-login \
  --log-level 2

# Keep this running in background or use systemd
```

#### For Production (Mainnet)

```bash
# Create wallet directory
mkdir -p /opt/monero/wallets

# Generate RPC credentials
RPC_USER="monero_$(openssl rand -hex 8)"
RPC_PASS="$(openssl rand -base64 32)"

# Run wallet-rpc with authentication
monero-wallet-rpc \
  --rpc-bind-port 18082 \
  --wallet-dir /opt/monero/wallets \
  --rpc-login "$RPC_USER:$RPC_PASS" \
  --confirm-external-bind \
  --restricted-rpc \
  --log-file /var/log/monero-wallet-rpc.log \
  --log-level 1 \
  --daemon-address node.moneroworld.com:18089

# Save credentials to config
echo "RPC User: $RPC_USER"
echo "RPC Pass: $RPC_PASS"
```

### 3. Configure Payment Gateway

```yaml
# config/oxygen.yml
providers:
  monero:
    wallet_rpc_endpoint: http://localhost:18082/json_rpc
    testnet_wallet_rpc_endpoint: http://localhost:28082/json_rpc
    rpc_username: monero_rpc_user  # From step 2
    rpc_password: your-strong-password
```

### 4. Create Systemd Service (Production)

```bash
# Create service file
sudo nano /etc/systemd/system/monero-wallet-rpc.service
```

```ini
[Unit]
Description=Monero Wallet RPC
After=network.target

[Service]
Type=simple
User=monero
Group=monero
WorkingDirectory=/opt/monero
ExecStart=/usr/local/bin/monero-wallet-rpc \
  --rpc-bind-port 18082 \
  --wallet-dir /opt/monero/wallets \
  --rpc-login monero_user:REPLACE_WITH_PASSWORD \
  --confirm-external-bind \
  --restricted-rpc \
  --daemon-address node.moneroworld.com:18089
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start service
sudo systemctl enable monero-wallet-rpc
sudo systemctl start monero-wallet-rpc
sudo systemctl status monero-wallet-rpc
```

### 5. Test Monero Integration

```bash
# Test RPC connection
curl -X POST http://localhost:18082/json_rpc \
  -u monero_user:password \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "0",
    "method": "get_version"
  }'

# Expected response with version info
```

### 6. Monero Features Checklist

- [x] Wallet RPC integration
- [x] Account creation
- [x] Address generation
- [x] Address validation
- [x] Balance checking
- [x] Transfer creation
- [x] Transfer monitoring
- [x] Transaction history
- [x] Payment ID support
- [x] Priority fee support
- [x] Block explorer links

---

## Configuration

### Full Configuration Example

```yaml
# config/oxygen.yml

providers:
  # Solana Configuration
  solana:
    # Mainnet RPC (choose one)
    rpc_endpoint: https://api.mainnet-beta.solana.com  # Public (free, rate limited)
    # rpc_endpoint: https://mainnet.helius-rpc.com/?api-key=YOUR-KEY  # Helius
    # rpc_endpoint: https://solana-mainnet.g.alchemy.com/v2/YOUR-KEY  # Alchemy

    # Devnet RPC
    devnet_rpc_endpoint: https://api.devnet.solana.com

    # API Key (if using paid service)
    api_key: ""

  # Monero Configuration
  monero:
    # Mainnet wallet RPC
    wallet_rpc_endpoint: http://localhost:18082/json_rpc

    # Testnet wallet RPC
    testnet_wallet_rpc_endpoint: http://localhost:28082/json_rpc

    # RPC Authentication (REQUIRED for production)
    rpc_username: monero_rpc_user
    rpc_password: <strong-random-password>
```

### Environment Variables

```bash
# Solana
export PROVIDERS_SOLANA_RPC_ENDPOINT="https://api.mainnet-beta.solana.com"
export PROVIDERS_SOLANA_DEVNET_RPC_ENDPOINT="https://api.devnet.solana.com"
export PROVIDERS_SOLANA_API_KEY="your-api-key"

# Monero
export PROVIDERS_MONERO_WALLET_RPC_ENDPOINT="http://localhost:18082/json_rpc"
export PROVIDERS_MONERO_TESTNET_WALLET_RPC_ENDPOINT="http://localhost:28082/json_rpc"
export PROVIDERS_MONERO_RPC_USERNAME="monero_user"
export PROVIDERS_MONERO_RPC_PASSWORD="strong-password"
```

---

## Testing

### Solana Testing

#### 1. Testnet Faucet

```bash
# Get devnet SOL for testing
solana airdrop 2 YOUR_SOLANA_ADDRESS --url devnet

# Or use web faucet
# https://faucet.solana.com
```

#### 2. Test Transaction Flow

```bash
# 1. Create payment
curl -X POST http://localhost:3000/api/payments \
  -H "Content-Type: application/json" \
  -d '{
    "merchant_id": 1,
    "price": 0.1,
    "currency": "SOL",
    "is_test": true
  }'

# 2. Send SOL to generated address

# 3. Monitor transaction
# Check logs for confirmation

# 4. Create withdrawal
curl -X POST http://localhost:3000/api/withdrawals \
  -H "Content-Type: application/json" \
  -d '{
    "merchant_id": 1,
    "currency": "SOL",
    "amount": 0.05,
    "address": "MERCHANT_SOLANA_ADDRESS"
  }'
```

### Monero Testing

#### 1. Testnet Faucet

Visit: https://testnet.xmrchain.net/faucet

#### 2. Test Transaction Flow

```bash
# 1. Create Monero account
curl -X POST http://localhost:28082/json_rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "0",
    "method": "create_account",
    "params": {"label": "test_customer_1"}
  }'

# 2. Get testnet XMR from faucet to the generated address

# 3. Check balance
curl -X POST http://localhost:28082/json_rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "0",
    "method": "get_balance",
    "params": {"account_index": 1}
  }'

# 4. Test withdrawal
# Use payment gateway API to create withdrawal
```

---

## Production Deployment

### Solana Production Checklist

- [ ] Use paid RPC service (Helius/Alchemy/QuickNode)
- [ ] Configure rate limiting
- [ ] Set up monitoring/alerting
- [ ] Test hot wallet batching
- [ ] Verify transaction confirmation logic
- [ ] Test rollback scenarios
- [ ] Load test with multiple concurrent transactions
- [ ] Set up fallback RPC endpoints

### Monero Production Checklist

- [ ] Run monero-wallet-rpc as systemd service
- [ ] Enable RPC authentication
- [ ] Use `--restricted-rpc` flag
- [ ] Connect to trusted Monero node
- [ ] Encrypt wallet files
- [ ] Regular wallet backups
- [ ] Monitor wallet-rpc health
- [ ] Set up log rotation
- [ ] Test account creation at scale
- [ ] Verify balance accuracy
- [ ] Test transaction confirmation

### Security Best Practices

#### Solana
1. **RPC Security**
   - Use HTTPS for RPC endpoints
   - Rotate API keys regularly
   - Monitor API usage
   - Set up IP whitelisting if available

2. **Private Key Storage**
   - Keys stored in KMS (BoltDB)
   - Encrypt KMS database
   - Regular backups
   - Access control

#### Monero
1. **Wallet RPC Security**
   - Run wallet-RPC in isolated environment
   - Enable authentication (`--rpc-login`)
   - Use `--restricted-rpc` flag
   - Bind to localhost only
   - Use reverse proxy for external access

2. **Wallet Files**
   - Encrypt wallet files
   - Secure backup strategy
   - Regular backups
   - Offline storage of master wallet

3. **Network Security**
   - Firewall rules
   - VPN for RPC access
   - TLS/SSL for external connections

---

## Troubleshooting

### Solana Issues

#### "RPC request failed"

```bash
# Check RPC endpoint
curl -X POST https://api.mainnet-beta.solana.com \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"getHealth"}'

# If timeout, try different endpoint
# If rate limited, upgrade to paid service
```

#### "Transaction confirmation timeout"

- Solana network congestion
- Increase confirmation retries
- Use higher priority fees
- Check transaction on explorer

#### "Invalid address"

- Solana addresses are base58, 32-44 characters
- No special characters except alphanumeric
- Verify with: `validateSolanaAddress()`

### Monero Issues

#### "monero-wallet-rpc not responding"

```bash
# Check if running
ps aux | grep monero-wallet-rpc

# Check logs
tail -f /var/log/monero-wallet-rpc.log

# Restart service
sudo systemctl restart monero-wallet-rpc
```

#### "Authentication failed"

- Verify username/password in config
- Check RPC login credentials
- Restart wallet-rpc after credential change

#### "Daemon not reachable"

```bash
# Check daemon connection
curl -X POST http://localhost:18081/json_rpc \
  -d '{"jsonrpc":"2.0","id":"0","method":"get_info"}'

# Use public node if local daemon not synced
# node.moneroworld.com:18089
# node.xmr.to:18081
```

#### "Transaction failed"

- Check unlocked balance (not just balance)
- Verify destination address
- Ensure sufficient balance for fees
- Check daemon synchronization

---

## Performance Tuning

### Solana

```yaml
# Use connection pooling
http.Transport:
  MaxIdleConns: 100
  MaxIdleConnsPerHost: 10
  IdleConnTimeout: 90s
```

### Monero

```bash
# Increase wallet-rpc threads
monero-wallet-rpc --rpc-bind-port 18082 --wallet-dir /wallets --max-concurrency 4

# Use faster daemon
# --daemon-address node.moneroworld.com:18089
```

---

## Monitoring

### Key Metrics

#### Solana
- RPC response time
- Transaction confirmation rate
- Failed transactions
- Rate limit hits
- Balance discrepancies

#### Monero
- Wallet-RPC uptime
- Account creation rate
- Transfer success rate
- Balance sync status
- Daemon connection status

### Alerting

Set up alerts for:
- RPC endpoint failures
- Transaction confirmation failures
- Balance discrepancies
- Unusual withdrawal patterns
- Service downtimes

---

## Cost Estimation

### Solana

| Service | Free Tier | Paid Tier | Recommended For |
|---------|-----------|-----------|-----------------|
| Public RPC | Free, rate limited | N/A | Development |
| Helius | 100k req/day | $50/month+ | Small-Medium |
| Alchemy | 300M compute/month | $49/month+ | Medium-Large |
| QuickNode | N/A | $9/month+ | Small |

### Monero

| Component | Cost | Notes |
|-----------|------|-------|
| Monero Node | Free (self-hosted) | Requires 150GB+ storage, bandwidth |
| Wallet RPC | Free | Minimal resources |
| Public Node | Free | Less reliable, privacy concerns |

---

**Last Updated:** 2025-11-17
**Status:** Production Ready (with caveats)
**Next Steps:** Install dependencies, configure RPC endpoints, test on testnet
