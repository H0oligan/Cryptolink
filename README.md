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
  <a href="https://cryptolink.cc/doc">Documentation</a> ·
  <a href="https://cryptolink.cc/dashboard/login?mode=register">Try It Free</a>
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
| Data collection | Extensive | **None** |
| Open source | No | **100% MIT** |
| Monthly cost | Variable + per-tx | **From $0/month flat** |

> *"Every $10,000 in sales costs you $320 with Stripe. With CryptoLink: $0."*

---

## How It Works

```
You (xpub key) → CryptoLink Server → unique address → Customer sends crypto
                                                              ↓
                                                    Directly to your wallet
                                              (no intermediary, ever)
```

1. **You provide your xpub** — an extended public key that derives receiving addresses without exposing private keys (BIP32/BIP44)
2. **CryptoLink generates unique addresses** — one per payment, mathematically derived from your xpub
3. **Customer pays on-chain** — funds go directly to your address, not through any CryptoLink infrastructure
4. **You get a webhook notification** — your server is notified of payment status changes
5. **Private keys stay offline** — CryptoLink never needs them, never sees them

---

## Features

- **Non-custodial** — private keys never touch the server; funds go directly to your wallet via HD derivation
- **Self-hosted** — runs on your own infrastructure; no one can freeze, restrict, or access your system
- **Multi-merchant** — one installation supports unlimited merchants
- **REST API** — full programmatic control over payments, webhooks, and payment links
- **Payment links** — shareable URLs for no-code checkout (donations, invoices, etc.)
- **Subscription plans** — Free / Starter / Growth / Business / Enterprise — flat monthly fee, no per-tx cut
- **EVM smart contracts** — optional `CollectorFactory` + `MerchantCollector` contracts for advanced EVM collection flows
- **Admin panel** — manage merchants, users, subscription plans, and email settings
- **Email notifications** — configurable SMTP for payment events and volume alerts
- **Security audited** — constant-time HMAC, SSRF protection, HSTS, rate limiting, CSRF, parameterized SQL

---

## Supported Currencies

| Currency | Network | Token Standard |
|---|---|---|
| **BTC** | Bitcoin | Native |
| **ETH** | Ethereum | Native |
| **USDT** | Ethereum | ERC-20 |
| **USDC** | Ethereum | ERC-20 |
| **TRX** | TRON | Native |
| **USDT** | TRON | TRC-20 |
| **XMR** | Monero | Native (requires `monero-wallet-rpc`) |

---

## Quick Start

### Requirements

- Linux server (Ubuntu 22.04+ recommended)
- Go 1.21+
- PostgreSQL 14+
- Node.js 18+
- [Tatum API key](https://tatum.io) (free tier available)

### 1. Clone & Configure

```bash
git clone https://github.com/H0oligan/Cryptolink.git
cd Cryptolink
cp config/cryptolink.example.yml config/cryptolink.yml
# Edit config/cryptolink.yml with your DB, Tatum key, and SMTP settings
```

### 2. Build Frontend

```bash
# Dashboard UI
cd ui-dashboard
npm install
npx vite build
cp -r dist/ /path/to/public_html/dashboard/

# Payment UI
cd ../ui-payment
npm install
npx vite build
cp -r dist/ /path/to/public_html/p/
```

### 3. Build Backend

> **Important:** Build the frontend **before** the Go binary. Go uses `//go:embed dist/*` to embed the SPA at compile time.

```bash
go build \
  -ldflags "-w -s -X 'main.gitVersion=v1.0.0' -X 'main.embedFrontend=1'" \
  -o ./bin/cryptolink .
```

### 4. Database Setup

```bash
# Run migrations (first time only)
DB_MIGRATE_ON_START=true ./bin/cryptolink serve-web --config=./config/cryptolink.yml
```

### 5. Run

```bash
# Web server
DB_MIGRATE_ON_START=false nohup ./bin/cryptolink serve-web \
  --config=./config/cryptolink.yml > ./logs/web.log 2>&1 &

# KMS (Key Management Service) — can run on a separate, isolated machine
nohup ./bin/cryptolink serve-kms \
  --config=./config/cryptolink.yml > /dev/null 2>&1 &
```

### 6. Nginx Reverse Proxy

```nginx
server {
    listen 443 ssl;
    server_name yourdomain.com;

    # Serve static files directly
    location ~* \.(js|css|png|svg|ico|woff2)$ {
        root /path/to/public_html;
        expires max;
    }

    # API & SPA routes proxied to Go
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## API Integration

### Authentication

All API requests require a token header:

```
X-CRYPTOLINK-TOKEN: your-api-token
```

Create tokens in: Dashboard → Merchant → API Tokens.

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
    $signature = $request->header('X-Tatum-Signature');
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

→ **[Full documentation with Node.js, Python, WooCommerce examples →](https://cryptolink.cc/doc)**

---

## Non-Custodial Architecture

CryptoLink never holds or sees your funds. Here's the cryptographic guarantee:

1. **You generate** a mnemonic → derive xpub key (offline, in your own wallet)
2. **You provide** only the xpub to CryptoLink — it cannot spend funds, only derive addresses
3. **CryptoLink derives** `address = xpub + derivation_path(index)` per payment
4. **Customer sends** to that address **directly on the blockchain**
5. **Your private key** (the only key that can spend) **never touches the server**

This is mathematically enforced — not a policy promise. Verify it yourself:

- Address derivation: [`internal/kms/wallet/`](./internal/kms/wallet/)
- Payment flow: [`internal/service/payment/`](./internal/service/payment/)

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

→ **[Full security audit report →](https://cryptolink.cc/doc#security-audit)**

---

## Project Structure

```
├── cmd/                    # CLI entry points (serve-web, serve-kms, migrate…)
├── config/                 # Config files — gitignored, use cryptolink.example.yml
├── contracts/              # EVM smart contracts (CollectorFactory, MerchantCollector)
├── internal/
│   ├── app/                # Application bootstrap
│   ├── auth/               # Authentication (session, token, Google OAuth)
│   ├── config/             # Config struct definitions
│   ├── db/                 # PostgreSQL connection & sqlc-generated queries
│   ├── event/              # Event bus (payment events, user events)
│   ├── kms/                # Key Management Service
│   ├── locator/            # Service locator / dependency injection
│   ├── provider/           # Blockchain providers (Tatum, TronGrid, Monero)
│   ├── scheduler/          # Background jobs (payment expiry, balance checks)
│   ├── server/http/        # Echo HTTP server, middleware, API handlers
│   ├── service/            # Business logic (payment, merchant, subscription, email)
│   └── util/               # Shared utilities
├── scripts/migrations/     # SQL migration files
├── ui-dashboard/           # Merchant dashboard (React + Vite)
└── ui-payment/             # Customer payment page (React + Vite)
```

---

## Subscription Plans

| Plan | Price | Monthly Volume |
|---|---|---|
| Free | $0/month | up to $1,000 |
| Starter | $9.99/month | up to $10,000 |
| Growth | $29.99/month | up to $50,000 |
| Business | $79.99/month | up to $250,000 |
| Enterprise | $199.99/month | Unlimited |

**Zero per-transaction fees at every tier.**

---

## Contributing

1. Fork the repository
2. Create a branch: `git checkout -b feature/your-feature`
3. Commit your changes — **never commit credentials, config files, or `.env` files**
4. Open a Pull Request

The `config/*.yml` and `.env` files are gitignored — keep them that way.

---

## License

[MIT License](./LICENSE) — free to use, modify, and self-host.

---

<p align="center">
  <a href="https://cryptolink.cc">cryptolink.cc</a> ·
  <a href="https://cryptolink.cc/doc">Docs</a> ·
  <a href="https://cryptolink.cc/privacy">Privacy Policy</a> ·
  <a href="https://github.com/H0oligan/Cryptolink">GitHub</a>
</p>
