# CLAUDE.md - AI Assistant Guide for Cryptolink/OxygenPay

> **Repository Status:** This repository has been archived as of 2024-07-01. The project did not find product-market fit but remains available for forking and continued development.

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

**Project:** OxygenPay - A cloud or self-hosted crypto payment gateway
**Architecture:** Monorepo with Go backend + React frontends
**Deployment Model:** Single binary with embedded UIs
**License:** Apache License 2.0

### What This System Does
- Non-custodial cryptocurrency payment processing
- Multi-tenant merchant platform
- Automatic hot wallet management with built-in KMS
- Support for Ethereum, Polygon, TRON, BNB, USDT, USDC
- Payment links and webhook integrations
- No full-node requirements (uses Tatum blockchain provider)

---

## Architecture & Structure

### Directory Layout

```
/home/user/Cryptolink/
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
│   ├── api-*/             # Generated API clients
│   ├── graceful/          # Graceful shutdown
│   └── ...
├── scripts/                # Database & utilities
│   ├── migrations/       # SQL migrations (30 files)
│   └── queries/          # SQLC query definitions
├── ui-dashboard/          # Merchant dashboard (React + Ant Design)
│   ├── src/
│   ├── vite.config.ts
│   └── package.json
├── ui-payment/            # Payment UI (React + Tailwind)
│   ├── src/
│   ├── vite.config.ts
│   └── package.json
├── web/                    # Static assets & API docs
│   └── redoc/            # Swagger documentation
├── .github/                # GitHub Actions workflows
│   └── workflows/
│       ├── ci.yml        # Linting
│       └── release.yml   # Docker build & push
├── main.go                # Application entry point
├── Dockerfile             # Multi-stage Docker build
├── Makefile               # Build & development tasks
├── go.mod                 # Go dependencies
└── docker-compose.local.yml
```

### Architectural Layers

```
┌─────────────────────────────────────────┐
│         HTTP Layer (Echo)               │
│  (handlers, middleware, routing)        │
├─────────────────────────────────────────┤
│       Service Layer (Business Logic)    │
│  (payment, wallet, merchant, etc.)      │
├─────────────────────────────────────────┤
│     Repository Layer (SQLC-generated)   │
│        (type-safe database queries)     │
├─────────────────────────────────────────┤
│          Database (PostgreSQL)          │
└─────────────────────────────────────────┘

         External Dependencies:
    ┌──────────┐  ┌──────────┐  ┌──────────┐
    │  Tatum   │  │ TronGrid │  │   KMS    │
    │ Provider │  │   API    │  │ (BoltDB) │
    └──────────┘  └──────────┘  └──────────┘
```

### Key Design Patterns

1. **Service Locator Pattern** (`internal/locator/`)
   - Centralized dependency injection
   - Lazy initialization with `sync.Once`
   - Single source of truth for all services

2. **Repository Pattern** (`internal/db/repository/`)
   - SQLC-generated type-safe queries
   - Store pattern for transaction handling

3. **Event-Driven Architecture** (`internal/bus/`)
   - In-memory pub/sub using EventBus
   - Async event handlers for payment lifecycle
   - Topics: `"topic:payment.created"`, `"topic:user.created"`, etc.

4. **Clean Architecture**
   - Strict separation: HTTP → Service → Repository → Database
   - Domain models in service packages
   - Provider abstraction for external APIs

5. **Microservices-Ready**
   - Can run as single binary OR separate components:
     - `oxygen web` - HTTP server
     - `oxygen kms` - Key Management Service
     - `oxygen scheduler` - Background jobs

---

## Technology Stack

### Backend (Go 1.20)

**Core Framework:**
- **Web:** Echo v4 (`labstack/echo`)
- **CLI:** Cobra (`spf13/cobra`)
- **Config:** cleanenv (`ilyakaznacheev/cleanenv`) - YAML + ENV

**Database:**
- **Driver:** PostgreSQL via pgx/v4 (`jackc/pgx`)
- **Migrations:** sql-migrate (`rubenv/sql-migrate`)
- **Query Builder:** SQLC (compile-time SQL → Go code generation)
- **KMS Storage:** BoltDB (embedded key-value store)

**Blockchain:**
- **Ethereum:** go-ethereum (`ethereum/go-ethereum`)
- **Bitcoin Utils:** btcsuite/btcutil
- **HD Wallets:** go-hdwallet
- **Provider SDK:** Custom Tatum SDK (`oxygenpay/tatum-sdk`)

**Infrastructure:**
- **Logging:** zerolog (structured JSON logging)
- **Scheduler:** cron v3 (`robfig/cron`)
- **Event Bus:** EventBus (`asaskevich/EventBus`)
- **Sessions:** gorilla/sessions
- **OAuth:** golang.org/x/oauth2 (Google)

**Utilities:**
- **Functional:** lo (`samber/lo`)
- **UUID:** google/uuid
- **Validation:** go-playground/validator/v10

### Frontend

#### Dashboard UI (`ui-dashboard/`)
- **Framework:** React 18.2 + TypeScript
- **UI Library:** Ant Design 5.1.6 + Ant Design Pro Components
- **Data Fetching:** TanStack Query v4 (react-query)
- **Routing:** React Router DOM 6.4
- **HTTP:** Axios
- **Build Tool:** Vite
- **Styling:** Sass
- **Analytics:** PostHog

#### Payment UI (`ui-payment/`)
- **Framework:** React 18.2 + TypeScript
- **Styling:** Tailwind CSS 3.2
- **Forms:** Formik + Yup
- **QR Codes:** QRCode.react
- **Routing:** React Router DOM 6.4
- **Build Tool:** Vite
- **Analytics:** PostHog

**Code Quality (Both UIs):**
- ESLint + Prettier
- Husky + lint-staged (pre-commit hooks)
- TypeScript strict mode

---

## Development Workflows

### Initial Setup

```bash
# Clone repository
git clone https://github.com/oxygenpay/oxygen.git
cd oxygen

# Copy configuration
cp config/oxygen.example.yml config/oxygen.yml

# Start PostgreSQL (via Docker)
docker-compose -f docker-compose.local.yml up -d postgres

# Run migrations
make migrate-up

# Install frontend dependencies
cd ui-dashboard && npm install && cd ..
cd ui-payment && npm install && cd ..

# Generate code (SQLC + Swagger models)
make codegen

# Run development server
make run
```

### Environment Configuration

**Primary Config File:** `config/oxygen.yml`
- YAML-first with ENV variable overrides
- Use `oxygen env` command to list all ENV variables
- Can skip config file entirely with `--skip-config` flag

**Key Configuration Sections:**
```yaml
logger:
  level: debug  # debug, info, warn, error

oxygen:
  postgres:
    dsn: "postgresql://user:pass@localhost:5432/oxygen"

  server:
    port: 3000
    web_path: "/path/to/ui-dashboard/dist"
    payment_path: "/path/to/ui-payment/dist"

  processing:
    payment_expiration: 15m

  auth:
    email_allowed: ["admin@example.com"]
    google_oauth_enabled: true

kms:
  server:
    port: 3001
  store:
    path: "/tmp/kms.db"

providers:
  tatum:
    api_key: "YOUR_API_KEY"
    base_url: "https://api.tatum.io"

  kms:
    address: "http://localhost:3001"
```

### Makefile Commands

```bash
# Code Generation
make codegen          # Generate SQLC queries + Swagger models
make swagger          # Generate Swagger models only

# Building
make build            # Build binary → bin/oxygen
make build-frontend   # Build both UIs
make local            # Build everything + run locally

# Running Services
make run              # Run web server (port 3000)
make run-kms          # Run KMS server (port 3001)
make run-scheduler    # Run background job scheduler

# Database
make migrate-up       # Run pending migrations
make migrate-down     # Rollback last migration

# Code Quality
make lint             # Run golangci-lint
make test             # Run tests with race detector

# Docker
make docker-build     # Build Docker image
make docker-local     # Run full stack via docker-compose
```

### Code Generation Workflow

**SQLC (SQL → Go):**
```bash
# 1. Write SQL queries in scripts/queries/*.sql
# 2. Run code generation
make codegen

# Generated files appear in:
# internal/db/repository/db.go
# internal/db/repository/models.go
# internal/db/repository/*.sql.go
```

**Swagger (OpenAPI → Go models):**
```bash
# API specs in api/proto/
# Generated clients in pkg/api-*/
make swagger
```

### Adding a New API Endpoint

1. **Define OpenAPI spec** in `api/proto/merchant/` or relevant file
2. **Regenerate models:** `make swagger`
3. **Add handler** in `internal/server/http/{api}/`
4. **Register route** in `internal/server/http/router.go`
5. **Implement service logic** in `internal/service/{domain}/`
6. **Add database queries** in `scripts/queries/{domain}.sql` (if needed)
7. **Regenerate SQLC:** `make codegen`
8. **Write tests** in `{package}_test.go`

### Adding a Database Migration

```bash
# Create migration
sql-migrate new -env="local" create_new_table

# Edit files in scripts/migrations/
# - XXXXXX_create_new_table.sql (up)
# - XXXXXX_create_new_table.sql (down)

# Apply migration
make migrate-up

# Rollback if needed
make migrate-down
```

---

## Key Conventions & Patterns

### Code Organization

**File Naming:**
- Service implementations: `service.go`
- Domain models: `model.go`
- Tests: `*_test.go`
- Mocks: `mock/mock_*.go` (generated by Mockery)

**Package Naming:**
- Use lowercase, single-word package names
- Service packages: `internal/service/{domain}/`
- Avoid stutter: `payment.Payment` not `payment.PaymentService`

**Error Handling:**
```go
// Use wrapped errors for context
if err != nil {
    return fmt.Errorf("unable to create payment: %w", err)
}

// Service layer errors include context
return nil, fmt.Errorf("merchant %d: %w", merchantID, ErrNotFound)
```

### Service Layer Patterns

**Service Interface Pattern:**
```go
// internal/service/payment/service.go

type Service interface {
    CreatePayment(ctx context.Context, params CreateParams) (*Payment, error)
    GetPayment(ctx context.Context, id int64) (*Payment, error)
}

type service struct {
    store      repository.Store
    logger     *zerolog.Logger
    blockchain blockchain.Service
    // ... dependencies
}

func New(deps ServiceDependencies) Service {
    return &service{...}
}
```

**Repository Access:**
```go
// Always use repository.Store for transactions
func (s *service) CreatePayment(ctx context.Context, params CreateParams) (*Payment, error) {
    var payment *Payment

    err := s.store.RunTransaction(ctx, func(q repository.Querier) error {
        // Use Querier interface for all DB operations
        dbPayment, err := q.CreatePayment(ctx, repository.CreatePaymentParams{...})
        if err != nil {
            return err
        }
        payment = mapToModel(dbPayment)
        return nil
    })

    return payment, err
}
```

### Event Publishing Pattern

```go
// internal/bus/topics.go
const (
    TopicPaymentCreated   = "topic:payment.created"
    TopicTransactionFound = "topic:transaction.found"
)

// Publishing events
s.bus.Publish(bus.TopicPaymentCreated, payment)

// Subscribing to events (in service constructor)
func New(bus *EventBus, ...) Service {
    s := &service{...}
    bus.SubscribeAsync(bus.TopicPaymentCreated, s.handlePaymentCreated, false)
    return s
}

func (s *service) handlePaymentCreated(payment *payment.Payment) {
    // Handle event asynchronously
}
```

### Logging Conventions

```go
// Use structured logging with zerolog
s.logger.Info().
    Int64("payment_id", payment.ID).
    Str("status", payment.Status).
    Msg("payment created")

s.logger.Error().
    Err(err).
    Int64("merchant_id", merchantID).
    Msg("unable to fetch merchant")
```

### Money Handling

```go
// Always use internal/money package for precision
amount := money.NewFromFloat64("ETH", 0.5)
usd := money.USD.FromStringMust("100.50")

// Comparisons
if amount.Gt(threshold) {
    // ...
}

// Arithmetic
total := amount.Add(fee)
```

### Blockchain Network IDs

```go
// Defined in internal/money/blockchain.go
money.Blockchain("ETH")          // Ethereum mainnet
money.Blockchain("ETH_GOERLI")   // Ethereum testnet
money.Blockchain("TRON")         // TRON mainnet
money.Blockchain("MATIC")        // Polygon
money.Blockchain("BSC")          // Binance Smart Chain
```

### Authentication Middleware

**API Token Auth:**
```go
// internal/server/http/middleware/auth.go
middleware.TokenAuth(merchantService)
```

**Session Auth:**
```go
middleware.SessionAuth(manager)
```

**CSRF Protection:**
```go
middleware.CSRF(config)
```

### Configuration Access

```go
// Use viper-like access pattern
cfg := config.Get()

// Access nested values
dbDSN := cfg.Oxygen.Postgres.DSN
apiKey := cfg.Providers.Tatum.APIKey
```

---

## Common Development Tasks

### Adding a New Blockchain Network

1. **Update money package** (`internal/money/blockchain.go`)
   - Add network constant
   - Update `SupportedBlockchains()`

2. **Add provider support** (`internal/provider/tatum/`)
   - Implement network-specific methods

3. **Update wallet service** (`internal/service/wallet/`)
   - Add network creation logic

4. **Update migrations** (if new table columns needed)

5. **Update frontend** (both UIs)
   - Add network icons to `assets/icons/crypto/`
   - Update network selection components

### Adding a Background Job

```go
// 1. Define job in internal/scheduler/scheduler.go
func (s *scheduler) setupJobs() {
    s.cron.AddFunc("@every 2m", s.newJobFunc(
        "my_job_name",
        s.myJobHandler,
    ))
}

// 2. Implement handler
func (s *scheduler) myJobHandler(ctx context.Context) error {
    // Job logic here
    return nil
}
```

### Creating a New Service

```bash
# 1. Create package directory
mkdir -p internal/service/myservice

# 2. Create files
touch internal/service/myservice/service.go
touch internal/service/myservice/model.go
touch internal/service/myservice/service_test.go

# 3. Add to service locator (internal/locator/locator.go)
```

### Adding Frontend Features

**Dashboard UI:**
```bash
cd ui-dashboard

# Add new page component
src/pages/MyNewPage/index.tsx

# Add route in src/App.tsx

# Add API hook in src/queries/

# Build for production
npm run build
```

**Payment UI:**
```bash
cd ui-payment

# Similar structure
src/components/
src/pages/
src/api/

npm run build
```

---

## Testing Strategy

### Test Organization

```
internal/
  service/
    payment/
      service.go
      service_test.go        # Unit tests
  test/
    database.go              # Test DB utilities
    must.go                  # Assertion helpers
    integration.go           # Integration test base
    fakes/                   # Fake implementations
    mock/                    # Mockery-generated mocks
```

### Running Tests

```bash
# All tests with race detector
make test

# Specific package
go test -v ./internal/service/payment/...

# With coverage
go test -cover ./...

# Integration tests (require PostgreSQL)
go test -tags=integration ./internal/test/
```

### Writing Unit Tests

```go
// internal/service/payment/service_test.go
func TestService_CreatePayment(t *testing.T) {
    // Arrange
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockStore := mock.NewMockStore(ctrl)
    logger := test.Must.Logger()

    svc := New(ServiceDeps{
        Store:  mockStore,
        Logger: logger,
    })

    // Setup expectations
    mockStore.EXPECT().
        CreatePayment(gomock.Any(), gomock.Any()).
        Return(&repository.Payment{ID: 1}, nil)

    // Act
    payment, err := svc.CreatePayment(ctx, params)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, int64(1), payment.ID)
}
```

### Integration Test Pattern

```go
// Use internal/test/integration.go
func TestIntegration(t *testing.T) {
    test.Integration(t, func(t *testing.T, db *sql.DB) {
        // Test with real database
        store := repository.New(db)
        // ... test logic
    })
}
```

### Mock Generation

```bash
# Using Mockery (if installed)
mockery --name=Service --dir=internal/service/payment --output=internal/service/payment/mock
```

---

## Deployment & Build

### Local Development

```bash
# Full stack with Docker
make docker-local

# Services available at:
# - Web UI: http://localhost:3000
# - API: http://localhost:3000/api
# - KMS: http://localhost:3001
# - PostgreSQL: localhost:5432
```

### Production Build

```bash
# Build binary with embedded frontends
make build-frontend  # Build UIs first
make build          # Build Go binary with embedded UIs

# Binary includes:
# - Go application
# - Dashboard UI (embedded)
# - Payment UI (embedded)
# - API documentation

# Run in production
EMBED_FRONTEND=true ./bin/oxygen web
```

### Docker Build

```bash
# Multi-stage Dockerfile
docker build -t oxygen:latest .

# Run container
docker run -p 3000:3000 \
  -e OXYGEN_POSTGRES_DSN="postgres://..." \
  -e PROVIDERS_TATUM_APIKEY="..." \
  oxygen:latest web
```

### Docker Compose (Production)

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: oxygen
      POSTGRES_USER: oxygen
      POSTGRES_PASSWORD: secret
    volumes:
      - postgres_data:/var/lib/postgresql/data

  kms:
    image: oxygen:latest
    command: kms
    environment:
      KMS_STORE_PATH: /data/kms.db
    volumes:
      - kms_data:/data

  web:
    image: oxygen:latest
    command: web
    ports:
      - "3000:3000"
    environment:
      OXYGEN_POSTGRES_DSN: "postgres://oxygen:secret@postgres:5432/oxygen"
      PROVIDERS_KMS_ADDRESS: "http://kms:3001"
    depends_on:
      - postgres
      - kms

  scheduler:
    image: oxygen:latest
    command: scheduler
    environment:
      OXYGEN_POSTGRES_DSN: "postgres://oxygen:secret@postgres:5432/oxygen"
    depends_on:
      - postgres
```

### GitHub Actions Release

**Workflow:** `.github/workflows/release.yml`

**Trigger:** Create GitHub release

**Process:**
1. Build ui-payment → artifact
2. Build ui-dashboard → artifact
3. Download artifacts
4. Build Docker image with embedded UIs
5. Push to `ghcr.io/oxygenpay/oxygen:{version}`
6. Push to `ghcr.io/oxygenpay/oxygen:latest`

**LDFLAGS:**
```bash
-X main.gitVersion=${VERSION}
-X main.gitCommit=${COMMIT}
-X main.embedFrontend=true
```

### Environment Variables

```bash
# List all available ENV variables
./oxygen env

# Key variables:
OXYGEN_POSTGRES_DSN           # Database connection
OXYGEN_SERVER_PORT            # HTTP port (default: 3000)
PROVIDERS_TATUM_APIKEY        # Tatum API key
PROVIDERS_TRONGRID_APIKEY     # TronGrid API key
PROVIDERS_KMS_ADDRESS         # KMS server address
LOGGER_LEVEL                  # Log level (debug, info, warn, error)
```

---

## AI Assistant Guidelines

### When Working with This Codebase

**DO:**
- ✅ Use the Makefile for all build tasks (`make codegen`, `make test`, etc.)
- ✅ Run `make codegen` after modifying SQL queries or OpenAPI specs
- ✅ Follow the service-repository-handler layer pattern
- ✅ Use structured logging with zerolog (never `fmt.Println`)
- ✅ Use the `internal/money` package for all currency operations
- ✅ Write tests alongside new features
- ✅ Use the service locator (`internal/locator/`) for dependency injection
- ✅ Publish events via the event bus for cross-service communication
- ✅ Wrap database operations in `store.RunTransaction()` when needed
- ✅ Add database queries to SQLC files, never write raw SQL in Go code
- ✅ Respect the archived status - this is reference code

**DON'T:**
- ❌ Write raw SQL in Go code (use SQLC queries)
- ❌ Bypass the service layer from HTTP handlers
- ❌ Use `float64` for money (use `internal/money.Money`)
- ❌ Hardcode blockchain addresses or API keys
- ❌ Skip error wrapping (`fmt.Errorf(..., %w, err)`)
- ❌ Create services without adding them to the locator
- ❌ Ignore transaction boundaries for multi-table operations
- ❌ Use global variables or `init()` functions for configuration
- ❌ Mix frontend and backend logic
- ❌ Commit without running `make lint` and `make test`

### File Modification Patterns

**Adding a Database Query:**
```bash
# 1. Add query to scripts/queries/payment.sql
# 2. Run: make codegen
# 3. Use generated function in internal/db/repository/
```

**Modifying an API Endpoint:**
```bash
# 1. Update api/proto/merchant/merchant-v1.yml
# 2. Run: make swagger
# 3. Update handler in internal/server/http/merchantapi/
# 4. Update service in internal/service/{domain}/
```

**Adding a Service Dependency:**
```go
// 1. Add to service constructor
func New(
    store repository.Store,
    logger *zerolog.Logger,
    newDep NewDependency, // Add here
) Service {
    return &service{
        store:  store,
        logger: logger,
        newDep: newDep, // And here
    }
}

// 2. Update locator in internal/locator/locator.go
func (l *Locator) PaymentService() payment.Service {
    l.oncePaymentService.Do(func() {
        l.paymentService = payment.New(
            l.Store(),
            l.logger,
            l.NewDepService(), // Add dependency here
        )
    })
    return l.paymentService
}
```

### Understanding Payment Flow

```
1. Customer visits payment link or creates payment via API
   ↓
2. Payment service creates payment record (status: pending)
   ↓
3. Customer selects payment method (crypto + blockchain)
   ↓
4. Wallet service assigns hot wallet address
   ↓
5. Payment UI shows QR code + address
   ↓
6. Customer sends crypto to address
   ↓
7. Tatum webhook notifies of incoming transaction
   ↓
8. Blockchain service verifies transaction
   ↓
9. Processing service updates payment (status: confirmed)
   ↓
10. Event bus publishes payment.confirmed event
   ↓
11. Merchant receives webhook notification
   ↓
12. Scheduler job transfers funds to internal wallet
```

### Key Security Considerations

1. **Private Keys:** Never logged, stored encrypted in KMS (BoltDB)
2. **API Tokens:** Hashed in database, never exposed in logs
3. **CSRF Protection:** Required for all dashboard state-changing operations
4. **Rate Limiting:** Applied per merchant
5. **Distributed Locking:** Prevents concurrent wallet operations
6. **Webhook Signatures:** Validate incoming Tatum webhooks
7. **SQL Injection:** Prevented by SQLC parameterized queries

### Common Pitfalls

1. **Forgetting to embed frontends:** Set `EMBED_FRONTEND=true` for production
2. **Using wrong money precision:** Always use `money.Money`, never `float64`
3. **Skipping transactions:** Multi-table operations need `store.RunTransaction()`
4. **Hardcoding network IDs:** Use `money.Blockchain()` constants
5. **Missing event handlers:** Subscribe in service constructor
6. **Not handling graceful shutdown:** Use `pkg/graceful` for cleanup

### Useful Commands for AI Assistants

```bash
# Find service interface definition
find internal/service -name "service.go"

# Find all SQLC queries for a domain
cat scripts/queries/payment.sql

# Find API endpoint definition
grep -r "POST /payment" api/proto/

# Find event topics
grep -r "const Topic" internal/bus/

# Check test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Find migration by name
ls scripts/migrations/ | grep -i "wallet"
```

### API Documentation Locations

- **Merchant API:** `/home/user/Cryptolink/api/proto/merchant/merchant-v1.yml`
- **Payment API:** `/home/user/Cryptolink/api/proto/payment/payment-v1.yml`
- **Dashboard API:** `/home/user/Cryptolink/api/proto/merchant/dashboard-v1.yml`
- **KMS API:** `/home/user/Cryptolink/api/proto/kms/kms-v1.yml`
- **Webhooks:** `/home/user/Cryptolink/api/proto/merchant/webhooks.md`
- **Rendered Docs:** `/home/user/Cryptolink/web/redoc/` (Swagger UI)

### Database Schema Reference

**Key Tables (30 migrations):**
- `users` - User accounts with email/OAuth
- `merchants` - Tenant entities (multi-tenancy)
- `api_tokens` - Merchant API authentication
- `wallets` - Cryptocurrency hot wallets
- `addresses` - Blockchain addresses per wallet
- `customers` - Payment customers (email/optional)
- `payments` - Payment records with status tracking
- `payment_links` - Predefined invoices with slugs
- `transactions` - Blockchain transaction tracking
- `balances` - Merchant balance per currency
- `withdrawals` - Withdrawal requests
- `wallet_locks` - Distributed locking mechanism
- `merchant_addresses` - Whitelisted withdrawal addresses
- `registries` - System key-value store
- `job_logs` - Scheduler execution logs

---

## Quick Reference

### Port Mapping
- **3000** - Web server (API + Dashboard + Payment UI)
- **3001** - KMS server
- **5432** - PostgreSQL

### Log Levels
- `debug` - Verbose logging (development)
- `info` - Standard logging (production)
- `warn` - Warnings only
- `error` - Errors only

### Payment Statuses
- `pending` - Awaiting customer action
- `in_progress` - Transaction detected
- `success` - Payment confirmed
- `failed` - Payment failed/expired
- `cancelled` - Manually cancelled

### Wallet Types
- `inbound` - Receives customer payments (many)
- `outbound` - Internal accumulation wallet (one per currency)

### Supported Test Networks
- `ETH_GOERLI` - Ethereum Goerli testnet
- `TRON_TESTNET` - TRON Shasta testnet
- `MATIC_MUMBAI` - Polygon Mumbai testnet
- `BSC_TESTNET` - BSC testnet

---

## Additional Resources

- **API Specs:** `/home/user/Cryptolink/api/proto/`
- **Migrations:** `/home/user/Cryptolink/scripts/migrations/`
- **SQLC Config:** `/home/user/Cryptolink/scripts/sqlc.yaml`
- **Linter Config:** `/home/user/Cryptolink/.golangci.yml`
- **Docker Compose:** `/home/user/Cryptolink/docker-compose.local.yml`

---

**Last Updated:** 2025-11-15
**Repository Status:** Archived (2024-07-01)
**Maintained By:** AI-generated documentation based on codebase analysis

For questions about architecture decisions or patterns, refer to the source code and inline comments. This is reference-quality code suitable for learning and forking.