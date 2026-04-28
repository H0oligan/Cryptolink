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
  <a href="https://cryptolink.cc">Website</a> ·
  <a href="https://cryptolink.cc/docs">Documentation</a> ·
  <a href="https://cryptolink.cc/merchants/login?mode=register">Try It Free</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License">
  <img src="https://img.shields.io/badge/go-1.21+-00ADD8.svg" alt="Go 1.21+">
  <img src="https://img.shields.io/badge/non--custodial-✓-10b981.svg" alt="Non-custodial">
  <img src="https://img.shields.io/badge/open%20source-✓-6366f1.svg" alt="Open Source">
</p>

---

## Why CryptoLink?

Every centralized payment processor takes a cut of every transaction, collects your data, and can freeze your account without warning. CryptoLink is different by design:

| | Stripe / Coinbase / BitPay | **CryptoLink** |
|---|---|---|
| Per-transaction fees | 2.9% + $0.30 forever | **Zero** |
| KYC required | Yes | **No** |
| Can freeze your account | Yes | **No — self-hosted** |
| Custodial | Yes — they hold your funds | **No — direct to your wallet** |
| Data collection | Extensive | **None beyond the email you sign up with** |
| Open source | No | **100% MIT** |
| Monthly cost | Variable + per-tx | **From $0/month flat** |

> *"Every $10,000 in sales costs you $320 with Stripe. With CryptoLink: $0."*

---

## How It Works

CryptoLink supports two non-custodial collection mechanisms. Both deliver funds to a wallet **you alone control** — CryptoLink never holds keys for either path.

### 1. Smart-contract collectors (EVM chains + TRON) — *primary path*

```
Merchant deploys a personal "MerchantCollector" clone (one-time, ~$0.50)
        │
        ▼
Customer pays the clone address  →  funds sit in YOUR clone
        │
        ▼
Merchant clicks "Withdraw"  →  contract sweeps to your MetaMask/TronLink
                              (only `owner` can withdraw — set at deploy)
```

The clone factory uses [EIP-1167 minimal proxies](https://eips.ethereum.org/EIPS/eip-1167) — each merchant gets their own isolated 45-byte clone of a single shared `MerchantCollectorV2` implementation. The clone's `owner` is set to the merchant's wallet at deploy time and **cannot be changed**. CryptoLink (the platform) has no admin function, no upgrade key, no escape hatch.

### 2. xpub HD-derivation (Bitcoin) — *for chains without smart contracts*

```
You provide an xpub/ypub/zpub (extended public key)
        │
        ▼
CryptoLink derives a unique BIP44/49/84 address per payment
        │
        ▼
Customer sends BTC directly to that address — the private key never touches the server
```

Your private key stays in your hardware wallet / cold storage. CryptoLink has *only* the extended public key, which can derive addresses but **cannot spend**. This is mathematically enforced by the BIP32 standard.

---

## Supported Currencies

**17 cryptocurrencies across 7 blockchains:**

| Blockchain | Coins / Tokens | Collection Method |
|---|---|---|
| **Bitcoin** | BTC | xpub HD-derivation (BIP44/49/84) |
| **Ethereum** | ETH, USDT (ERC-20), USDC (ERC-20) | Smart-contract collector |
| **Polygon** | MATIC, USDT, USDC | Smart-contract collector |
| **TRON** | TRX, USDT (TRC-20) | Smart-contract collector (clone factory) |
| **BNB Chain** | BNB, USDT | Smart-contract collector |
| **Arbitrum** | ETH, USDT, USDC | Smart-contract collector |
| **Avalanche** | AVAX, USDT, USDC | Smart-contract collector |

**26 fiat currencies** for invoice pricing: USD, EUR, GBP, CAD, AUD, CHF, JPY, CNY, INR, BRL, MXN, KRW, SGD, HKD, SEK, NOK, DKK, PLN, CZK, TRY, ZAR, NZD, THB, AED, SAR, RUB.

> Solana and Monero are **not** supported. Their key cryptography (ed25519 / CryptoNote) is incompatible with non-custodial xpub derivation, and no equivalent of the EVM smart-contract collector exists for them. Custodial integrations exist elsewhere — CryptoLink will not implement one.

---

## Features

- **Non-custodial by construction** — funds flow only to wallets you control; the platform cannot move, freeze, or seize them
- **Self-hosted** — runs on your own infrastructure; no external SaaS dependency
- **Multi-merchant** — one installation supports unlimited merchants; admin panel for super-admin operators
- **Smart-contract clone factory** — merchants deploy a personal collector clone in one transaction (~$0.50 of gas)
- **Robust EVM payment detection** — event-based watcher (`eth_getLogs` on the collector's `Received(address,uint256)` log) catches direct sends *and* payments routed through exchange batch withdrawals, dispersers, multisigs, and payment splitters that are invisible to top-level transaction scans
- **xpub / ypub / zpub support** — auto-detects BIP44 (legacy), BIP49 (P2SH-SegWit), BIP84 (native SegWit `bc1q…`)
- **Multi-fiat invoicing** — price in any of 26 fiat currencies; merchant-configurable volatility-fee markup applied at conversion
- **REST API** — full programmatic control over payments, webhooks, payment links, customers
- **Payment links** — shareable URLs for no-code checkout (donations, invoices, etc.)
- **Subscription plans** — Free / Starter / Growth / Business / Enterprise — flat monthly fee, **zero per-transaction cut**
- **Underpayment handling** — automatic detection, grace period for top-up, configurable behavior
- **Email notifications** — Brevo/SMTP; payment events, volume alerts (80/90/100%), underpayments, marketing
- **Admin panel** — separate SPA at `/admin` for super-admin tasks (merchants, users, plans, contracts, marketing)
- **Security audited** — constant-time HMAC, SSRF blocklist, HSTS / CSRF / CSP, rate-limited auth, parameterized SQL, bcrypt

---

## Anonymity & What Goes On-Chain

CryptoLink is built for privacy-conscious operators. Here is **exactly** what is and isn't visible to whom:

### What is NEVER published or known to CryptoLink

- Your **private keys** — they never leave your hardware wallet / cold storage / browser extension.
- Your **legal identity** — there is no KYC. Sign-up requires only an email address, which you can rotate.
- Your **customer list / order data / business volume** — stored only in *your* self-hosted database.
- The **mapping between a merchant account and a real-world identity** — CryptoLink-the-software has no such record. (If you self-host on a VPS you paid for with KYC'd fiat, that's an operational concern under your control.)

### What IS publicly visible (because all blockchains are public ledgers)

- **The deployed smart-contract addresses themselves.** EVM and TRON contracts live on a public chain; their bytecode is published and verified on block explorers (Etherscan, Tronscan, etc.). This is unavoidable for any on-chain payment system.
- **Your collector clone's address and its owner.** When a merchant deploys a clone, the factory emits a `CloneCreated(owner, clone)` event. Anyone watching the chain can see "this owner address deployed a clone." There is no name, email, or business name attached — only a 20-byte address.
- **Payment transactions to your address.** Every blockchain transaction is public. Anyone with your collector address can see incoming payments and the on-chain withdrawal to your personal wallet. This is true of *all* on-chain payments, custodial or not.
- **The CryptoLink open-source code** — including the Solidity contracts in [`contracts/`](./contracts/). Publishing them is a *feature*, not a leak: it lets anyone verify the contract has no backdoor.

### Are the contracts safe to publish?

**Yes, and they are intentionally public.** Read them yourself in [`contracts/`](./contracts/):

- [`MerchantCollectorV2.sol`](./contracts/MerchantCollectorV2.sol) — proxy-compatible collector, withdraws only to the immutable `owner` set at clone-init time.
- [`CryptoLinkCloneFactory.sol`](./contracts/CryptoLinkCloneFactory.sol) — EIP-1167 minimal-proxy factory.
- [`MerchantCollector.sol`](./contracts/MerchantCollector.sol) — original non-proxy variant (kept for reference).

There are **no hardcoded admin addresses, merchant addresses, or backdoor functions** in any of these. The factory itself has no admin powers over deployed clones. We publish the source so users can audit, verify on-chain bytecode matches source, and re-deploy on chains we don't yet support.

> **Deployed addresses are not published here.** Each operator is expected to deploy their own factory per chain (compile with `tron_v0.8.25+commit.77bd169f`, optimizer 200 runs, evmVersion `paris`). EIP-1167 clones are 45 bytes and don't need separate verification — block explorers auto-detect them.

---

## Quick Start

### Requirements

- Linux server (Ubuntu 22.04+ recommended)
- Go 1.21+
- PostgreSQL 14+
- Node.js 18+
- An RPC endpoint per chain you want to support (Infura/Alchemy/Ankr/your own node)

### 1. Clone & Configure

```bash
git clone https://github.com/H0oligan/Cryptolink.git
cd Cryptolink

# Create your config (no example file is shipped — see internal/config/config.go for the full struct)
cp config/cryptolink.yml.template config/cryptolink.yml   # if a template exists, otherwise create from scratch
# Edit config/cryptolink.yml with your DB URL, RPC endpoints, and SMTP settings
```

Run `./bin/cryptolink env` after building to see all supported environment variables.

### 2. Build Frontend

The Go binary embeds the SPAs at compile time via `//go:embed`, so frontend **must** build before backend.

```bash
# Merchant dashboard
cd ui-dashboard
npm install
VITE_ROOTPATH=/merchants/ npx vite build              # outputs ./dist
npx vite build --config vite.admin.config.ts          # outputs ./dist-admin (admin SPA)

# Payment page (customer-facing checkout)
cd ../ui-payment
npm install
npx vite build                                        # outputs ./dist
```

### 3. Build Backend

```bash
go build \
  -ldflags "-w -s -X 'main.gitVersion=v1.0.0' -X 'main.embedFrontend=1'" \
  -o ./bin/cryptolink .
```

### 4. Database Setup

```bash
# First-run migrations
DB_MIGRATE_ON_START=true ./bin/cryptolink serve-web --config=./config/cryptolink.yml
# After first boot, set DB_MIGRATE_ON_START=false (default Go struct value is true — explicit override is required)
```

Or run migrations standalone:

```bash
./bin/cryptolink migrate --config=./config/cryptolink.yml
```

### 5. Run

```bash
# All-in-one (web + scheduler in a single process — recommended for small deployments)
DB_MIGRATE_ON_START=false nohup ./bin/cryptolink all-in-one \
  --config=./config/cryptolink.yml > ./logs/web.log 2>&1 &

# OR split into web and scheduler processes:
nohup ./bin/cryptolink serve-web    --config=./config/cryptolink.yml > ./logs/web.log 2>&1 &
nohup ./bin/cryptolink run-scheduler --config=./config/cryptolink.yml > ./logs/sched.log 2>&1 &
```

Available subcommands: `all-in-one`, `serve-web`, `run-scheduler`, `migrate`, `create-user`, `list-balances`, `topup-balance`, `env`.

### 6. Nginx Reverse Proxy

```nginx
server {
    listen 443 ssl;
    server_name yourdomain.com;

    # Static SPA assets served by nginx
    location ~* \.(js|css|png|svg|ico|woff2)$ {
        root /path/to/public_html;
        expires max;
    }

    # Customer payment SPA — served from disk, NOT from Go embed
    location /p/ {
        alias /path/to/public_html/p/;
        try_files $uri $uri/ /p/index.html;
    }

    # API + dashboard SPAs proxy to Go
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## Two Separate SPAs

CryptoLink ships **two independent React/Vite frontends**:

- **Merchant panel** at `/merchants/*` — built from `ui-dashboard/` with `VITE_ROOTPATH=/merchants/`
- **Admin panel** at `/admin/*` — built from `ui-dashboard/` using `vite.admin.config.ts` (separate entry, super-admin only)

A super-admin user can use both: log in at `/admin/login` for admin work, `/merchants/login` for their own merchant account. Old `/dashboard/*` URLs 301-redirect to `/merchants/*`.

---

## API Integration

### Authentication

All API requests require a token header:

```
X-CRYPTOLINK-TOKEN: your-api-token
```

Create tokens in: **Merchant Panel → Settings → API Tokens**.

### Create a Payment

```bash
curl -X POST https://yourdomain.com/api/merchant/v1/merchant/{merchantId}/payment \
  -H "X-CRYPTOLINK-TOKEN: your-api-token" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "order-uuid-here",
    "currency": "USDT_TRON",
    "price": 99.00,
    "description": "Order #1234",
    "redirectUrl": "https://yourstore.com/thank-you",
    "metadata": {"order_id": "1234", "customer": "user@example.com"}
  }'
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "url": "https://yourdomain.com/p/550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "amount": "99.00",
  "currency": "USDT_TRON"
}
```

Redirect your customer to `url`. CryptoLink handles the rest.

### Subscription enforcement

When a merchant exceeds their plan's monthly volume / payment count, the payment-create endpoint returns **HTTP 402** with `status: "limit_exceeded"` and an `upgrade_url`. Handle this in your integration to prompt the merchant to upgrade.

### Laravel Example

```php
// Create payment
$response = Http::withHeaders([
    'X-CRYPTOLINK-TOKEN' => config('cryptolink.token'),
])->post(config('cryptolink.url') . "/api/merchant/v1/merchant/{$merchantId}/payment", [
    'id'          => $order->uuid,
    'currency'    => 'USDT_TRON',
    'price'       => $order->total,
    'redirectUrl' => route('payment.success'),
]);

return redirect($response->json('url'));
```

```php
// Verify webhook
public function handle(Request $request): Response
{
    $signature = $request->header('X-Signature');
    $body      = $request->getContent();
    $expected  = base64_encode(hash_hmac('sha512', $body, config('cryptolink.hmac_secret'), true));

    if (!hash_equals($expected, $signature ?? '')) {
        return response('Unauthorized', 401);
    }

    if ($request->json('status') === 'success') {
        Order::where('uuid', $request->json('id'))->update(['status' => 'paid']);
    }

    return response('OK');
}
```

→ **[Full documentation with Node.js, Python, WooCommerce examples →](https://cryptolink.cc/docs)**

---

## Multi-Fiat & Volatility-Fee Markup

Merchants can:

1. **Pick a base fiat currency** (one of 26: USD, EUR, GBP, JPY, …) — invoices and dashboard totals render in that currency.
2. **Set a global volatility-fee percent** — applied during fiat→crypto conversion to absorb price swings between invoice creation and on-chain confirmation.

Configuration lives in **Merchant Panel → Currencies & Fees** (`/merchants/currencies`). The fee is shown to customers on the payment page as *"Includes X% volatility fee set by merchant."*

Subscription plan prices and volume limits are always denominated in **USD** (system-level, not merchant-level).

---

## Subscription Plans

| Plan | Price | Monthly Volume |
|---|---|---|
| Free | $0/month | up to $1,000 |
| Starter | $9.99/month | up to $10,000 |
| Growth | $29.99/month | up to $50,000 |
| Business | $79.99/month | up to $250,000 |
| Enterprise | $199.99/month | Unlimited |

**Zero per-transaction fees at every tier.** Plan limits are enforced server-side: payment count, monthly volume, and merchant count are all checked before invoice creation.

Subscription payments are made in crypto **through CryptoLink itself** — there is no credit-card processor in the loop.

---

## Security

A full security audit was performed in February 2025. Key findings and resolutions:

| Finding | Severity | Status |
|---|---|---|
| HMAC signature used `==` (timing attack) | High | ✅ Fixed — `crypto/subtle.ConstantTimeCompare` |
| Sensitive data in error logs | Medium | ✅ Fixed — removed |
| SSRF via webhook callback URLs | High | ✅ Fixed — RFC-1918 blocklist |
| Missing HTTP security headers | Medium | ✅ Fixed — HSTS, X-Frame-Options, CSP, etc. |
| Rate limiting on auth endpoints | — | ✅ Already present |
| CSRF protection | — | ✅ Already present |
| Parameterized SQL (no injection) | — | ✅ Already present |
| bcrypt password hashing | — | ✅ Already present |

Subsequent dependency hardening: Echo framework upgraded, `golang-jwt/jwt v3` removed, npm vulnerabilities patched in both SPAs.

→ **[Full security audit report →](https://cryptolink.cc/docs#security-audit)**

---

## Project Structure

```
├── cmd/                    # CLI entry points (serve-web, all-in-one, run-scheduler, migrate, create-user, …)
├── config/                 # Config files — gitignored, must be created locally
├── contracts/              # Solidity sources for MerchantCollector(V2) + CloneFactory (PUBLIC by design)
├── internal/
│   ├── app/                # Application bootstrap
│   ├── auth/               # Authentication (session, token, Google OAuth)
│   ├── bus/                # Event bus
│   ├── config/             # Config struct definitions
│   ├── db/                 # PostgreSQL connection & sqlc-generated queries
│   ├── event/              # Payment + user event types
│   ├── kms/                # Key-derivation utilities (xpub/ypub/zpub → addresses)
│   ├── locator/            # Service locator / dependency injection
│   ├── money/              # Fiat & crypto money types, 26 fiat currency definitions
│   ├── provider/           # Blockchain / pricefeed providers (RPC, Bitcoin, TronGrid, Binance, etc.)
│   ├── scheduler/          # Background jobs (payment expiry, balance checks, watchers)
│   ├── server/http/        # Echo HTTP server, middleware, all API handlers
│   │   ├── merchantapi/    # Merchant-facing API
│   │   ├── paymentapi/     # Customer-facing payment-page API
│   │   ├── emailapi/       # Email settings + logs
│   │   ├── subscriptionapi/# Subscription plans + billing
│   │   ├── marketingapi/   # Marketing / unsubscribe endpoints
│   │   └── internalapi/    # Internal-only endpoints
│   ├── service/
│   │   ├── payment/        # Payment lifecycle
│   │   ├── processing/     # Incoming-tx processing, fee markup, webhooks
│   │   ├── watcher/        # Block-by-block address watching (15s poll)
│   │   ├── evmcollector/   # EVM smart-contract collector logic (balances, withdraw)
│   │   ├── xpub/           # BIP44/49/84 derivation
│   │   ├── subscription/   # Plan enforcement + usage tracking
│   │   ├── marketing/      # Email campaigns + unsubscribe
│   │   └── …               # merchant, user, email, contact, blockchain, transaction, registry, wallet
│   └── webhook/            # Outbound webhook delivery + HMAC signing
├── pkg/                    # Public Go packages (api models)
├── scripts/migrations/     # SQL migration files
├── ui-dashboard/           # Merchant + Admin SPAs (React + Vite, Ant Design v5 dark + Matrix Neon theme)
└── ui-payment/             # Customer-facing payment-page SPA (React + Vite)
```

---

## Contributing

1. Fork the repository
2. Create a branch: `git checkout -b feature/your-feature`
3. Commit your changes — **never commit credentials, config files, or `.env` files**
4. Open a Pull Request

The `config/*.yml`, `.env`, and any `*.pem` / `*.key` / `secrets/` files are gitignored — keep them that way.

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
