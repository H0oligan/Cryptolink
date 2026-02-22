# CLAUDE.md - AI Assistant Guide for CryptoLink

## Table of Contents
1. [Repository Overview](#repository-overview)
2. [Architecture & Structure](#architecture--structure)
3. [Technology Stack](#technology-stack)
4. [Development Workflows](#development-workflows)
5. [Key Conventions & Patterns](#key-conventions--patterns)
6. [Common Development Tasks](#common-development-tasks)
7. [Testing Strategy](#testing-strategy)
8. [Deployment & Build](#deployment--build)
9. [AI Assistant Guidelines](#ai-assistant-guidelines)

---

## Repository Overview

**Project:** CryptoLink - A self-hosted, non-custodial, decentralized cryptocurrency payment gateway
**Architecture:** Monorepo with Go backend + React frontends
**Deployment Model:** Single binary with embedded UIs, served behind Nginx
**License:** Apache License 2.0
**Website:** https://cryptolink.cc
**GitHub:** https://github.com/H0oligan/Cryptolink

### What This System Does
- Non-custodial cryptocurrency payment processing (your keys, your funds)
- Multi-tenant merchant platform with privacy-first design
- HD wallet key derivation (xpub-based, no third-party custody)
- Automatic hot wallet management with built-in KMS
- Support for 8 blockchains: Ethereum, Polygon, TRON, BNB Chain, Arbitrum, Avalanche, Solana, Monero
- 20+ supported cryptocurrencies including USDT, USDC stablecoins
- Payment links and webhook integrations
- REST API for programmatic payment creation
- No KYC requirements, no data collection
- Uses Tatum blockchain provider (no full-node requirements)

### Branding
- **Name:** CryptoLink (previously forked from OxygenPay/o2pay)
- **API Header:** `X-CRYPTOLINK-TOKEN` (not X-O2PAY-TOKEN)
- **Domain:** cryptolink.cc
- **Theme Colors:** Primary #6366f1 (indigo), Secondary #10b981 (green), Dark #0f172a
- **Logo:** SVG chain-link mark (two interlocking rings, one open = freedom)
- **Go module path:** `github.com/oxygenpay/oxygen` (cannot be changed without breaking all imports - this is internal only)

---

## Architecture & Structure

### Directory Layout

```
/home/cryptolink/oxygen/src/Cryptolink/     # Source code
├── api/                    # OpenAPI/Swagger specifications
│   └── proto/             # API definitions (merchant, payment, KMS, webhooks)
├── cmd/                    # CLI commands (Cobra-based)
│   ├── kms.go             # Key Management Service server
│   ├── migrate.go         # Database migrations
│   ├── scheduler.go       # Background job scheduler
│   └── web.go             # HTTP server
├── config/                 # Configuration examples
│   └── oxygen.example.yml # Main config template
├── internal/               # Private application code
│   ├── bus/               # Event bus (pub/sub pattern)
│   ├── db/                # Database layer
│   │   ├── connection/   # DB drivers (PostgreSQL, BoltDB)
│   │   └── repository/   # SQLC-generated type-safe queries
│   ├── locator/           # Service locator (dependency injection)
│   ├── lock/              # Distributed locking
│   ├── money/             # Precision money/currency handling
│   ├── provider/          # External service providers
│   │   ├── tatum/        # Tatum blockchain API
│   │   └── trongrid/     # TronGrid API
│   ├── scheduler/         # Cron job definitions
│   ├── server/            # HTTP server
│   │   └── http/         # Echo routes, handlers, middleware
│   ├── service/           # Business logic layer
│   │   ├── blockchain/   # Blockchain operations
│   │   ├── merchant/     # Merchant management
│   │   ├── payment/      # Payment processing
│   │   ├── processing/   # Transaction processing
│   │   ├── wallet/       # Wallet management
│   │   └── ...
│   └── test/              # Test utilities, mocks, fakes
├── pkg/                    # Public/reusable packages
├── scripts/                # Database & utilities
│   ├── migrations/       # SQL migrations
│   └── queries/          # SQLC query definitions
├── ui-dashboard/          # Merchant dashboard (React + Ant Design)
├── ui-payment/            # Payment UI (React + Tailwind)
├── web/                    # Static assets & API docs
├── main.go                # Application entry point
├── Dockerfile             # Multi-stage Docker build
├── Makefile               # Build & development tasks
└── go.mod                 # Go dependencies

/home/cryptolink/web/cryptolink.cc/public_html/  # Deployed website
├── index.html             # Homepage (static, SEO-optimized)
├── doc/index.html         # API documentation page
├── logo.svg               # Main SVG logo mark
├── favicon.svg            # Favicon (dark background version)
├── favicon-*.png          # PNG favicons (16, 32)
├── apple-touch-icon.png   # iOS icon (180x180)
├── android-chrome-*.png   # Android icons (192, 512)
├── dashboard/             # Dashboard frontend (deployed Vite build)
│   ├── index.html
│   ├── assets/
│   ├── fav/              # Dashboard favicons
│   └── site.webmanifest
└── p/                     # Payment UI frontend (deployed Vite build)
    ├── index.html
    ├── assets/           # Includes crypto currency SVG icons
    ├── fav/              # Payment favicons
    └── site.webmanifest
```

### Key Design Patterns

1. **Service Locator Pattern** (`internal/locator/`) - Centralized dependency injection with lazy initialization
2. **Repository Pattern** (`internal/db/repository/`) - SQLC-generated type-safe queries
3. **Event-Driven Architecture** (`internal/bus/`) - In-memory pub/sub for payment lifecycle
4. **Clean Architecture** - Strict separation: HTTP → Service → Repository → Database
5. **Non-Custodial by Design** - HD wallets, xpub key derivation, funds go directly to merchant wallets

---

## Technology Stack

### Backend (Go)

- **Web:** Echo v4
- **CLI:** Cobra
- **Config:** cleanenv (YAML + ENV)
- **Database:** PostgreSQL via pgx/v4, SQLC for queries, sql-migrate for migrations
- **KMS Storage:** BoltDB (embedded key-value store)
- **Blockchain:** go-ethereum, btcsuite, go-hdwallet, Custom Tatum SDK
- **Logging:** zerolog (structured JSON)
- **Auth:** gorilla/sessions, golang.org/x/oauth2 (Google), email+password
- **Auth Header:** `X-CRYPTOLINK-TOKEN` (defined in `internal/server/http/middleware/auth.go`)

### Frontend

#### Dashboard UI (`ui-dashboard/`)
- React 18 + TypeScript, Ant Design 5, TanStack Query v4, React Router 6, Axios, Vite, Sass

#### Payment UI (`ui-payment/`)
- React 18 + TypeScript, Tailwind CSS 3, Formik + Yup, QRCode.react, Vite

---

## Development Workflows

### Build & Deploy Commands (Production - CryptoLink server)

```bash
# Build Go backend
cd /home/cryptolink/oxygen/src/Cryptolink
go build -ldflags "-w -s -X 'main.gitVersion=local' -X 'main.gitCommit=local' -X 'main.embedFrontend=1'" -o /home/cryptolink/oxygen/bin/oxygen .

# Build Dashboard UI
cd /home/cryptolink/oxygen/src/Cryptolink/ui-dashboard
npx vite build

# Build Payment UI
cd /home/cryptolink/oxygen/src/Cryptolink/ui-payment
npx vite build

# Deploy Dashboard
cp ui-dashboard/dist/assets/* /home/cryptolink/web/cryptolink.cc/public_html/dashboard/assets/
cp ui-dashboard/dist/index.html /home/cryptolink/web/cryptolink.cc/public_html/dashboard/index.html
cp ui-dashboard/dist/fav/* /home/cryptolink/web/cryptolink.cc/public_html/dashboard/fav/
cp ui-dashboard/dist/site.webmanifest /home/cryptolink/web/cryptolink.cc/public_html/dashboard/site.webmanifest

# Deploy Payment UI
cp ui-payment/dist/assets/* /home/cryptolink/web/cryptolink.cc/public_html/p/assets/
cp ui-payment/dist/index.html /home/cryptolink/web/cryptolink.cc/public_html/p/index.html
cp ui-payment/dist/fav/* /home/cryptolink/web/cryptolink.cc/public_html/p/fav/
cp ui-payment/dist/site.webmanifest /home/cryptolink/web/cryptolink.cc/public_html/p/site.webmanifest

# Restart server (find PID first)
pgrep -af "oxygen serve-web"
kill <PID>
nohup /home/cryptolink/oxygen/bin/oxygen serve-web --config=/home/cryptolink/oxygen/config/oxygen.yml >> /home/cryptolink/oxygen/logs/web.log 2>> /home/cryptolink/oxygen/logs/web.error.log &
```

### Server Configuration
- **Config file:** `/home/cryptolink/oxygen/config/oxygen.yml`
- **Server port:** 3000 (behind Nginx)
- **Nginx serves:** static files from `/home/cryptolink/web/cryptolink.cc/public_html/`
- **Go binary serves:** API endpoints, dashboard SPA, payment SPA
- **Logs:** `/home/cryptolink/oxygen/logs/web.log` and `web.error.log`

### Login/Registration Flow
- Login page: `/dashboard/login`
- Register: `/dashboard/login?mode=register` (query param triggers register mode)
- Supports email+password and Google OAuth
- Login page source: `ui-dashboard/src/pages/login-page/login-page.tsx`

---

## Key Conventions & Patterns

### Code Organization
- Service implementations: `service.go`, Domain models: `model.go`
- Use wrapped errors: `fmt.Errorf("unable to create payment: %w", err)`
- Always use `internal/money` package for currency operations
- Use structured logging with zerolog
- Publish events via `internal/bus/` for cross-service communication

### Authentication
- API token auth: `X-CRYPTOLINK-TOKEN` header
- Session auth for dashboard users
- CSRF protection for dashboard state-changing operations
- Token header constant: `internal/server/http/middleware/auth.go` line 18

---

## Deployment & Build

### Production Architecture
```
[Client] → [Nginx] → static files from public_html/
                   → proxy /api/* to Go server :3000
                   → proxy /dashboard/* to Go server :3000
                   → proxy /p/* to Go server :3000
```

### Important: After Frontend Builds
- Old hashed JS/CSS assets may remain in deployed directories
- Clean old `index-*.js` files before deploying new build to avoid stale references
- Always copy ALL of `dist/assets/*`, `dist/fav/*`, and `dist/index.html`

---

## AI Assistant Guidelines

### Branding Rules
- Always use "CryptoLink" (not OxygenPay, O2Pay, or o2pay)
- API header is `X-CRYPTOLINK-TOKEN`
- Domain is `cryptolink.cc`
- Do NOT change Go module path `github.com/oxygenpay/oxygen` (would break all imports)
- Logo: SVG chain-link design at `/home/cryptolink/web/cryptolink.cc/public_html/logo.svg`
- Favicons generated from `favicon.svg` using sharp/ImageMagick

### When Modifying the System
- Read files before editing them
- After changing frontend source, rebuild with Vite and deploy to public_html
- After changing Go code, rebuild binary and restart server
- Test changes are live by curling the endpoints
- Keep the non-custodial, privacy-first messaging consistent across all pages

### Key File Locations
- **Homepage:** `/home/cryptolink/web/cryptolink.cc/public_html/index.html`
- **Doc page:** `/home/cryptolink/web/cryptolink.cc/public_html/doc/index.html`
- **Dashboard source:** `/home/cryptolink/oxygen/src/Cryptolink/ui-dashboard/`
- **Payment UI source:** `/home/cryptolink/oxygen/src/Cryptolink/ui-payment/`
- **Go backend:** `/home/cryptolink/oxygen/src/Cryptolink/`
- **API specs:** `/home/cryptolink/oxygen/src/Cryptolink/api/proto/`
- **Go binary:** `/home/cryptolink/oxygen/bin/oxygen`
- **Server config:** `/home/cryptolink/oxygen/config/oxygen.yml`
- **Auth middleware:** `/home/cryptolink/oxygen/src/Cryptolink/internal/server/http/middleware/auth.go`

### Payment Flow
```
1. Customer visits payment link or creates payment via API
2. Payment service creates payment record (status: pending)
3. Customer selects crypto + blockchain
4. Wallet service assigns HD-derived address
5. Payment UI shows QR code + address
6. Customer sends crypto
7. Tatum webhook notifies of incoming transaction
8. Blockchain service verifies transaction
9. Processing service updates payment (status: confirmed)
10. Merchant receives webhook notification
11. Funds are in merchant's non-custodial wallet
```

---

**Last Updated:** 2026-02-15
**Maintained By:** CryptoLink team
