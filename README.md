<p align="center">
  <a href="https://cryptolink.cc">
    <img src="https://cryptolink.cc/logo.svg" height="80" alt="CryptoLink logo">
  </a>
</p>

<h1 align="center">CryptoLink</h1>

<p align="center">
  <strong>Self-hosted · Non-custodial · Open Source</strong><br>
  Accept crypto payments without trusting anyone. Your keys, your funds, your infrastructure.
</p>

<p align="center">
  <strong>Current release: <code>v1.0</code></strong> — see the <a href="https://cryptolink.cc/docs/#changelog">changelog</a> for what changed since.<br>
  Single-decimal version scheme: <code>1.0</code> → <code>2.0</code> → <code>3.0</code>, bundled releases only.
</p>

<p align="center">
  <a href="https://cryptolink.cc">Website</a> ·
  <a href="https://cryptolink.cc/docs">Documentation</a> ·
  <a href="https://cryptolink.cc/docs/#changelog">Changelog</a> ·
  <a href="https://cryptolink.cc/merchants/login?mode=register">Try It Free</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-1.0-10b981.svg" alt="Version 1.0">
  <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License">
  <img src="https://img.shields.io/badge/go-1.25+-00ADD8.svg" alt="Go 1.25+">
  <img src="https://img.shields.io/badge/non--custodial-✓-10b981.svg" alt="Non-custodial">
  <img src="https://img.shields.io/badge/open%20source-✓-6366f1.svg" alt="Open Source">
</p>

---

## Why CryptoLink?

Centralized payment processors take a cut of every transaction, collect your data, and can freeze your account without warning. CryptoLink is different by design:

| | Stripe / Coinbase Commerce / BitPay | **CryptoLink** |
|---|---|---|
| Per-transaction fees | Yes, on every sale | **Zero** |
| KYC required | Yes | **No** |
| Can freeze your account | Yes | **No — self-hosted** |
| Custodial | Yes — they hold your funds | **No — straight to a contract only you control** |
| Data collection | Extensive | **None beyond the email you sign up with** |
| Open source | No | **100% MIT** |
| Monthly cost | Variable + per-tx | **From $0/month flat** |

---

## How It Works

CryptoLink supports two non-custodial collection mechanisms. Both deliver funds to a wallet **you alone control** — CryptoLink never holds keys for either path.

### 1. Smart-contract collectors (EVM chains + TRON) — *primary path*

```
Merchant connects ANY wallet and deploys a personal "MerchantCollector" (one-time)
        │      (the signing address becomes the contract's permanent owner)
        ▼
Customer pays the collector address — by QR / copy-paste, or one click with their own wallet
        │
        ▼
Merchant clicks "Withdraw All"  →  the contract sends everything to the owner wallet
                                  (only `owner` can withdraw — enforced on-chain)
```

On TRON the factory uses [EIP-1167 minimal proxies](https://eips.ethereum.org/EIPS/eip-1167): each merchant gets an isolated 45-byte clone of a shared `MerchantCollectorV2` implementation. On EVM chains each merchant deploys their own `MerchantCollector`. In both cases the `owner` is fixed at deployment and **cannot be changed**. The platform has no admin function, no upgrade key, no escape hatch.

### 2. xpub HD-derivation (Bitcoin) — *for chains without smart contracts*

```
You provide an xpub / ypub / zpub (extended public key)
        │
        ▼
CryptoLink derives a unique BIP44/49/84 address per payment
        │
        ▼
Customer sends BTC directly to that address — the private key never touches the server
```

Your private key stays in your hardware wallet or cold storage. CryptoLink only has the extended public key, which can derive addresses but **cannot spend**.

---

## Wallets: any wallet, desktop or phone

There is no "MetaMask only" or "TronLink only" requirement anywhere — neither for merchants nor for customers. Wallet discovery uses open standards only ([EIP-6963](https://eips.ethereum.org/EIPS/eip-6963) for EVM, the injected TRON providers for TRON). **No WalletConnect / Reown relay or any other third-party service is involved.**

**Merchants** (dashboard header → *Connect Wallet*):

- Desktop: every installed extension is listed (MetaMask, Rabby, OKX, Coinbase, Trust, Brave…), with install links for the rest.
- Same phone: *Open in MetaMask / Trust / OKX / Coinbase / TokenPocket / TronLink* reopens the dashboard inside the wallet app's built-in browser, which asks *"Continue as ad***@example.com"* before signing in.
- PC → phone: *Use my phone* shows a QR code. The phone displays a 6-digit code that must be typed on the PC to approve the sign-in. Links are single-use, expire in 2 minutes, and carry their token in the URL fragment so it never reaches server logs.
- Before a withdrawal, the collector's on-chain `owner()` is compared with the connected account; a different wallet gets *"not the owner of this contract"* instead of a transaction that would revert.
- Before a deployment, the merchant must confirm that the connected address will be the contract's permanent owner.

**Customers** (checkout page → *Pay with wallet*):

- The transaction is built only from the server's [EIP-681](https://eips.ethereum.org/EIPS/eip-681) payment URI — network, token contract, recipient and exact raw amount. The wallet is switched to the right network and the chain is verified before sending; any inconsistency falls back to the QR code.
- Works for invoices created by API, payment links and subscription payments (all of them use the same checkout page), including topping up a partial payment.
- On a phone without an in-browser wallet, the checkout offers *Open in &lt;wallet app&gt;* links and the standard payment URI. Bitcoin uses the QR code / `bitcoin:` URI.

---

## Supported Currencies

**17 cryptocurrencies across 7 blockchains:**

| Blockchain | Coins / Tokens | Collection Method |
|---|---|---|
| **Bitcoin** | BTC | xpub HD-derivation (BIP44/49/84) |
| **Ethereum** | ETH, USDT (ERC-20), USDC (ERC-20) | Smart-contract collector |
| **Polygon** | MATIC, USDT, USDC | Smart-contract collector |
| **TRON** | TRX, USDT (TRC-20) | Smart-contract collector (clone factory) |
| **BNB Chain** | BNB, USDT (BEP-20) | Smart-contract collector |
| **Arbitrum** | ETH, USDT, USDC | Smart-contract collector |
| **Avalanche** | AVAX, USDT, USDC | Smart-contract collector |

Tickers used by the API: `BTC`, `ETH`, `ETH_USDT`, `ETH_USDC`, `MATIC`, `MATIC_USDT`, `MATIC_USDC`, `TRON`, `TRON_USDT`, `BNB`, `BSC_USDT`, `ARB`, `ARBITRUM_USDT`, `ARBITRUM_USDC`, `AVAX`, `AVAX_USDT`, `AVAX_USDC`.

**26 fiat currencies** for invoice pricing: USD, EUR, GBP, CAD, AUD, CHF, JPY, CNY, INR, BRL, MXN, KRW, SGD, HKD, SEK, NOK, DKK, PLN, CZK, TRY, ZAR, NZD, THB, AED, SAR, RUB.

> Solana and Monero are **not** supported. Their key cryptography (ed25519 / CryptoNote) is incompatible with non-custodial xpub derivation, and no equivalent of the smart-contract collector exists for them.

---

## Features

- **Non-custodial by construction** — funds flow only to wallets you control; the platform cannot move, freeze, or seize them
- **Self-hosted** — runs on your own infrastructure; direct blockchain providers, no payment-API vendor in the loop
- **Any wallet, any device** — merchant *Connect Wallet* and customer *Pay with wallet*, desktop extensions and phone wallet apps (see above)
- **Multi-merchant** — one installation supports many merchants; separate admin panel for operators
- **Robust EVM payment detection** — the watcher reads the collector's `Received(address,uint256)` log, so payments routed through exchange batch withdrawals, dispersers, multisigs and payment splitters are detected, not only direct sends
- **Cross-currency payments** — a customer who pays a collector in a different supported coin than the one on the invoice (e.g. native ETH against a USDT invoice) is credited at the live rate instead of being ignored
- **RPC failover** — several RPC endpoints per chain; an endpoint that fails a real request is demoted, not only one that fails a health check
- **xpub / ypub / zpub** — auto-detected (SLIP-0132): BIP44 legacy, BIP49 P2SH-SegWit, BIP84 native SegWit `bc1q…`
- **Partial payments** — an underpayment keeps the invoice open (expiry extended per top-up, capped at +24h), the checkout shows *received / remaining* with a QR for the missing amount, and webhooks report the partial state
- **Multi-fiat invoicing** — price in any of 26 fiat currencies; optional merchant volatility-fee markup applied at conversion
- **REST API + webhooks** — payments, payment links, customers, xpub wallets; HMAC-SHA512 signed webhooks
- **Payment links** — shareable URLs for no-code checkout
- **Subscription plans** — flat monthly fee, **zero per-transaction cut**, limits enforced server-side
- **Email notifications** — any SMTP relay; payment events, volume alerts (80/90/100%), underpayments
- **Admin panel** — separate SPA at `/admin` (merchants, users, plans, contracts, payments, marketing)

---

## Anonymity & What Goes On-Chain

### What is NEVER published or known to CryptoLink

- Your **private keys** — they never leave your wallet (browser extension, phone app or hardware wallet).
- Your **legal identity** — there is no KYC. Sign-up requires only an email address.
- Your **customer list / order data / business volume** — stored only in *your* self-hosted database.

### What IS publicly visible (because blockchains are public ledgers)

- **Deployed contract addresses and their bytecode**, verifiable on block explorers.
- **Your collector's address and its owner address.** No name, email or business name is attached on-chain.
- **Payments to your collector and withdrawals to your wallet** — true of every on-chain payment, custodial or not.
- **The open-source code**, including the Solidity contracts in [`contracts/`](./contracts/). Publishing them lets anyone verify there is no backdoor.

### Are the contracts safe to publish?

**Yes, and they are intentionally public.** Read them in [`contracts/`](./contracts/):

- [`MerchantCollectorV2.sol`](./contracts/MerchantCollectorV2.sol) — proxy-compatible collector; withdraws only to the immutable `owner`.
- [`CryptoLinkCloneFactory.sol`](./contracts/CryptoLinkCloneFactory.sol) — EIP-1167 minimal-proxy factory (TRON).
- [`MerchantCollector.sol`](./contracts/MerchantCollector.sol) — non-proxy collector used on EVM chains.

There are **no hardcoded admin addresses, merchant addresses, or backdoor functions**. Deployed addresses are not published here: each operator deploys their own TRON factory (compile with `tron_v0.8.25+commit.77bd169f`, optimizer 200 runs, `evmVersion paris`). EIP-1167 clones are recognised automatically by block explorers.

---

## Quick Start

### Requirements

- Linux server (Ubuntu 22.04+ / Debian 12+ recommended)
- Go **1.25+**
- PostgreSQL 14+
- Node.js **20.19+** (or 22.12+) — required by Vite 8
- RPC endpoints for the chains you enable (public, paid or your own nodes)

### 1. Clone & configure

```bash
git clone https://github.com/H0oligan/Cryptolink.git
cd Cryptolink
```

Create `config/cryptolink.yml` yourself — no example is shipped because every value is operator-specific (database DSN, RPC URLs, SMTP, secrets). See [`config/README.md`](./config/README.md). After building, `./bin/cryptolink env` lists every supported key and its environment-variable equivalent.

### 2. Build the frontends (before the backend)

The Go binary embeds the dashboard SPAs with `//go:embed`, so the frontends **must** be built first.

```bash
# Merchant + admin dashboards
cd ui-dashboard
npm install --legacy-peer-deps
VITE_ROOTPATH=/merchants/ npx vite build              # → ./dist
npx vite build --config vite.admin.config.ts          # → ./dist-admin

# Customer checkout
cd ../ui-payment
npm install --legacy-peer-deps
npx vite build                                        # → ./dist (serve it at /p/)
```

`--legacy-peer-deps` is needed because some Ant Design Pro packages still declare an antd 5 peer while the dashboard runs antd 6.

### 3. Build the backend

```bash
go build \
  -ldflags "-w -s -X 'main.gitVersion=v1.0' -X 'main.embedFrontend=1'" \
  -o ./bin/cryptolink .
```

### 4. Database

```bash
./bin/cryptolink migrate --config=./config/cryptolink.yml
```

Then run with `DB_MIGRATE_ON_START=false` — the Go default is `true`, so it must be overridden explicitly.

### 5. Run

```bash
# Web + scheduler in one process (recommended for small deployments)
DB_MIGRATE_ON_START=false ./bin/cryptolink all-in-one --config=./config/cryptolink.yml

# or split:
./bin/cryptolink serve-web     --config=./config/cryptolink.yml
./bin/cryptolink run-scheduler --config=./config/cryptolink.yml
```

Subcommands: `all-in-one`, `serve-web`, `run-scheduler`, `migrate`, `create-user`, `list-balances`, `topup-balance`, `recover-payment`, `env`. Use a systemd unit with `Restart=always` in production.

### 6. Reverse proxy (nginx)

```nginx
server {
    listen 443 ssl;
    server_name yourdomain.com;

    # Never serve dotfiles (.env, .git, …)
    location ~ /\. { deny all; }

    # Customer checkout SPA — served from disk, not from the Go binary
    location /p/ {
        alias /var/www/cryptolink/p/;
        try_files $uri $uri/ /p/index.html;
    }

    # API + dashboards
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Step-by-step production instructions (PostgreSQL, systemd, TLS, firewall): **[Self-hosting guide →](https://cryptolink.cc/docs/#self-hosting)**

---

## Two Dashboard SPAs + one checkout

| App | Path | Source |
|---|---|---|
| Merchant panel | `/merchants/*` | `ui-dashboard/` with `VITE_ROOTPATH=/merchants/` |
| Admin panel (super-admin only) | `/admin/*` | `ui-dashboard/` with `vite.admin.config.ts` |
| Customer checkout | `/p/*` | `ui-payment/` |

A super-admin can use both panels: `/admin/login` for operator work, `/merchants/login` for their own merchant account.

---

## API Integration

### Authentication

```
X-CRYPTOLINK-TOKEN: your-api-token
```

Create tokens in **Merchant Panel → Settings → API Tokens**.

### Create a payment

The invoice is priced in **fiat**; the customer picks the coin/network on the checkout page.

```bash
curl -X POST https://yourdomain.com/api/merchant/v1/merchant/{merchantId}/payment \
  -H "X-CRYPTOLINK-TOKEN: your-api-token" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "8b1e5f0a-3c2d-4e6f-9a7b-1c2d3e4f5a6b",
    "orderId": "order#1234",
    "currency": "USD",
    "price": 99.00,
    "description": "Order #1234",
    "redirectUrl": "https://yourstore.com/thank-you"
  }'
```

`id` is a UUID you generate (idempotency key). Response (abridged):

```json
{
  "id": "8b1e5f0a-3c2d-4e6f-9a7b-1c2d3e4f5a6b",
  "orderId": "order#1234",
  "status": "pending",
  "price": "99",
  "currency": "USD",
  "paymentUrl": "https://yourdomain.com/p/pay/8b1e5f0a-3c2d-4e6f-9a7b-1c2d3e4f5a6b"
}
```

Redirect your customer to `paymentUrl`.

### Webhooks

Webhook bodies are signed with **HMAC-SHA512** of the raw body using your webhook secret, base64-encoded, in the `X-Signature` header. Webhook URLs must be public: private, loopback and link-local destinations are refused.

```php
public function handle(Request $request): Response
{
    $expected = base64_encode(hash_hmac('sha512', $request->getContent(), config('cryptolink.webhook_secret'), true));

    if (!hash_equals($expected, $request->header('X-Signature', ''))) {
        return response('Unauthorized', 401);
    }

    if ($request->json('status') === 'success') {
        Order::where('uuid', $request->json('id'))->update(['status' => 'paid']);
    }

    return response('OK');
}
```

### Plan limits

When a merchant exceeds their plan, payment creation returns **HTTP 402** with `status: "limit_exceeded"` and an `upgrade_url`.

→ **[Full API reference, payment states, Laravel / Node.js / Python / WooCommerce examples →](https://cryptolink.cc/docs)**

---

## Subscription Plans

| Plan | Price | Monthly volume |
|---|---|---|
| Free | $0/month | up to $1,000 |
| Starter | $9.99/month | up to $10,000 |
| Growth | $29.99/month | up to $50,000 |
| Business | $79.99/month | up to $250,000 |
| Enterprise | $199.99/month | Unlimited |

**Zero per-transaction fees at every tier.** Payment count, monthly volume and merchant count are checked server-side before an invoice is created. Plan prices are in USD; subscription payments are made in crypto through CryptoLink itself.

---

## Security

- Constant-time HMAC verification; HMAC-SHA512 signed webhooks
- **SSRF protection** on merchant webhook URLs, enforced on the IP actually dialed (covers DNS rebinding and redirects)
- Login throttling per account (not only per IP), no account enumeration on login / registration, verified email required for merchants
- Wallet sign-in handoff: single-use 2-minute tokens kept in the URL fragment, PC approval with a 6-digit code, never silently replaces another signed-in account
- CSRF protection, HSTS / CSP / X-Frame-Options, rate-limited auth, parameterized SQL (sqlc), bcrypt passwords
- Non-custodial contracts: no admin keys, owner fixed at deployment

Found a vulnerability? See **[Reporting vulnerabilities](https://cryptolink.cc/docs/#security-audit)** — please disclose privately first.

---

## Project Structure

```
├── cmd/                    # CLI: all-in-one, serve-web, run-scheduler, migrate, create-user, recover-payment, …
├── config/                 # Operator config — gitignored (see config/README.md)
├── contracts/              # Solidity: MerchantCollector(V2) + CloneFactory (public by design)
├── internal/
│   ├── app/ bus/ locator/  # Bootstrap, event bus, dependency wiring
│   ├── auth/               # Sessions, API tokens, Google OAuth
│   ├── config/             # Config struct (yml + env)
│   ├── db/                 # PostgreSQL + sqlc-generated queries
│   ├── kms/                # xpub/ypub/zpub → address derivation
│   ├── money/              # Fiat & crypto money types (26 fiat currencies)
│   ├── provider/           # RPC (with failover), Bitcoin, TronGrid, price feeds
│   ├── scheduler/          # Background jobs (expiry, reorg recheck, alerts)
│   ├── server/http/        # Echo server, middleware, API handlers (merchant, payment, subscription, …)
│   ├── service/            # payment, processing, watcher, evmcollector, xpub, subscription, …
│   └── webhook/            # Outbound webhooks: HMAC signing + SSRF guard
├── pkg/                    # Public Go API models
├── scripts/migrations/     # SQL migrations
├── ui-dashboard/           # Merchant + admin SPAs (React, Vite, Ant Design 6, Matrix Neon theme)
│   └── src/wallet/         # Wallet discovery, Connect Wallet modal, owner checks, phone handoff
└── ui-payment/             # Customer checkout SPA (React, Vite, Tailwind)
    └── src/wallet/         # Pay with wallet: EIP-681 parsing, chain guard, TRON transfers
```

### Tests

```bash
go test ./...                       # backend (some suites need PostgreSQL)
cd ui-dashboard && npm test         # wallet modules (vitest)
cd ui-payment   && npm test         # checkout payment logic (vitest)
```

---

## Contributing

1. Fork the repository
2. Create a branch: `git checkout -b feature/your-feature`
3. Commit your changes — **never commit credentials, config files, or `.env` files**
4. Open a Pull Request

`config/*.yml`, `.env`, and any `*.pem` / `*.key` / `secrets/` files are gitignored — keep them that way.

---

## License

[MIT License](./LICENSE) — free to use, modify, and self-host.

---

<p align="center">
  <a href="https://cryptolink.cc">cryptolink.cc</a> ·
  <a href="https://cryptolink.cc/docs">Docs</a> ·
  <a href="https://cryptolink.cc/privacy">Privacy Policy</a> ·
  <a href="https://github.com/H0oligan/Cryptolink">GitHub</a>
</p>
