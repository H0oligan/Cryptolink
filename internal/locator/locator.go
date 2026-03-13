// package locator represents simple Service Locator pattern.
package locator

import (
	"context"
	"sync"

	"github.com/cryptolink/cryptolink/internal/auth"
	"github.com/cryptolink/cryptolink/internal/bus"
	"github.com/cryptolink/cryptolink/internal/config"
	"github.com/cryptolink/cryptolink/internal/db/connection/pg"
	"github.com/cryptolink/cryptolink/internal/db/repository"
	"github.com/cryptolink/cryptolink/internal/lock"
	"github.com/cryptolink/cryptolink/internal/log"
	"github.com/cryptolink/cryptolink/internal/provider/bitcoin"
	"github.com/cryptolink/cryptolink/internal/provider/pricefeed"
	"github.com/cryptolink/cryptolink/internal/provider/rpc"
	"github.com/cryptolink/cryptolink/internal/provider/trongrid"
	"github.com/cryptolink/cryptolink/internal/service/blockchain"
	"github.com/cryptolink/cryptolink/internal/service/merchant"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/processing"
	"github.com/cryptolink/cryptolink/internal/service/email"
	"github.com/cryptolink/cryptolink/internal/service/evmcollector"
	"github.com/cryptolink/cryptolink/internal/service/registry"
	"github.com/cryptolink/cryptolink/internal/service/subscription"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/cryptolink/cryptolink/internal/service/user"
	"github.com/cryptolink/cryptolink/internal/service/wallet"
	"github.com/cryptolink/cryptolink/internal/service/watcher"
	"github.com/cryptolink/cryptolink/internal/service/xpub"
	"github.com/cryptolink/cryptolink/pkg/graceful"
	"github.com/rs/zerolog"
)

type Locator struct {
	ctx    context.Context
	config *config.Config
	once   map[string]*sync.Once

	logger *zerolog.Logger

	// Database
	db    *pg.Connection
	repo  *repository.Queries
	store *repository.Store

	// Event
	eventBus *bus.PubSub

	// Providers
	rpcProvider       *rpc.Provider
	priceFeedProvider *pricefeed.Provider
	trongridProvider  *trongrid.Provider
	bitcoinProvider   *bitcoin.Provider

	// Services
	registryService      *registry.Service
	blockchainService    *blockchain.Service
	userService          *user.Service
	locker               *lock.Locker
	merchantService      *merchant.Service
	tokenManager         *auth.TokenAuthManager
	googleAuth           *auth.GoogleOAuthManager
	transactionService   *transaction.Service
	paymentService       *payment.Service
	walletService        *wallet.Service
	xpubService          *xpub.Service
	evmCollectorService  *evmcollector.Service
	processingService    *processing.Service
	watcherService       *watcher.Service
	subscriptionService  *subscription.Service
	emailService         *email.Service
	jobLogger            *log.JobLogger
}

func New(ctx context.Context, cfg *config.Config, logger *zerolog.Logger) *Locator {
	return &Locator{
		config: cfg,
		ctx:    ctx,
		logger: logger,
		once:   make(map[string]*sync.Once, 128),
	}
}

func (loc *Locator) DB() *pg.Connection {
	loc.init("db", func() {
		db, err := pg.Open(loc.ctx, loc.config.Oxygen.Postgres, loc.logger)
		if err != nil {
			loc.logger.Fatal().Err(err).Msg("unable to open pg database")
			return
		}

		if err := db.Ping(loc.ctx); err != nil {
			loc.logger.Fatal().Err(err).Msg("unable to ping postgres")
			return
		}

		loc.db = db

		graceful.AddCallback(db.Shutdown)
	})

	return loc.db
}

func (loc *Locator) Repository() *repository.Queries {
	loc.init("repo", func() {
		loc.repo = repository.New(loc.DB())
	})

	return loc.repo
}

func (loc *Locator) Store() *repository.Store {
	loc.init("store", func() {
		loc.store = repository.NewStore(loc.DB())
	})

	return loc.store
}

func (loc *Locator) EventBus() *bus.PubSub {
	loc.init("event.bus", func() {
		loc.eventBus = bus.NewPubSub(loc.ctx, true, loc.logger)
	})

	return loc.eventBus
}

func (loc *Locator) Locker() *lock.Locker {
	loc.init("locker", func() {
		loc.locker = lock.New(loc.Store())
	})

	return loc.locker
}

func (loc *Locator) RPCProvider() *rpc.Provider {
	loc.init("provider.rpc", func() {
		loc.rpcProvider = rpc.New(loc.config.Providers.RPC, loc.logger)
	})

	return loc.rpcProvider
}

func (loc *Locator) PriceFeedProvider() *pricefeed.Provider {
	loc.init("provider.pricefeed", func() {
		loc.priceFeedProvider = pricefeed.New(loc.config.Providers.PriceFeed, loc.logger)
	})

	return loc.priceFeedProvider
}

func (loc *Locator) TrongridProvider() *trongrid.Provider {
	loc.init("provider.trongrid", func() {
		loc.trongridProvider = trongrid.New(loc.config.Providers.Trongrid, loc.logger)
	})

	return loc.trongridProvider
}

func (loc *Locator) BitcoinProvider() *bitcoin.Provider {
	loc.init("provider.bitcoin", func() {
		loc.bitcoinProvider = bitcoin.New(loc.config.Providers.Bitcoin, loc.logger)
	})

	return loc.bitcoinProvider
}

func (loc *Locator) RegistryService() *registry.Service {
	loc.init("service.registry", func() {
		loc.registryService = registry.New(loc.Repository(), loc.logger)
	})

	return loc.registryService
}

func (loc *Locator) BlockchainService() *blockchain.Service {
	loc.init("service.blockchain", func() {
		currencies := blockchain.NewCurrencies()
		if err := blockchain.DefaultSetup(currencies); err != nil {
			loc.logger.Fatal().Err(err).Msg("unable to setup currencies")
		}

		loc.blockchainService = blockchain.New(
			currencies,
			blockchain.Providers{
				RPC:       loc.RPCProvider(),
				PriceFeed: loc.PriceFeedProvider(),
				Trongrid:  loc.TrongridProvider(),
				Bitcoin:   loc.BitcoinProvider(),
			},
			loc.logger,
		)
	})

	return loc.blockchainService
}

func (loc *Locator) UserService() *user.Service {
	loc.init("service.user", func() {
		loc.userService = user.New(loc.Store(), loc.EventBus(), loc.RegistryService(), loc.logger)
	})

	return loc.userService
}

func (loc *Locator) MerchantService() *merchant.Service {
	loc.init("service.merchant", func() {
		loc.merchantService = merchant.New(loc.Repository(), loc.BlockchainService(), loc.logger)
	})

	return loc.merchantService
}

func (loc *Locator) TokenManagerService() *auth.TokenAuthManager {
	loc.init("service.tokenManager", func() {
		loc.tokenManager = auth.NewTokenAuth(loc.Repository(), loc.logger)
	})

	return loc.tokenManager
}

func (loc *Locator) GoogleAuth() *auth.GoogleOAuthManager {
	loc.init("service.auth.google", func() {
		loc.googleAuth = auth.NewGoogleOAuth(loc.config.Oxygen.Auth.Google, loc.logger)
	})

	return loc.googleAuth
}

func (loc *Locator) TransactionService() *transaction.Service {
	loc.init("service.transaction", func() {
		loc.transactionService = transaction.New(
			loc.Store(),
			loc.BlockchainService(),
			loc.WalletService(),
			loc.logger,
		)
	})

	return loc.transactionService
}

func (loc *Locator) PaymentService() *payment.Service {
	loc.init("service.payment", func() {
		loc.paymentService = payment.New(
			loc.Repository(),
			loc.config.Oxygen.Processing.PaymentFrontendPath(),
			loc.TransactionService(),
			loc.MerchantService(),
			loc.WalletService(),
			loc.BlockchainService(),
			loc.EventBus(),
			loc.logger,
		)
	})

	return loc.paymentService
}

func (loc *Locator) WalletService() *wallet.Service {
	loc.init("service.wallet", func() {
		loc.walletService = wallet.New(loc.BlockchainService(), loc.Store(), loc.logger)
	})

	return loc.walletService
}

func (loc *Locator) XpubService() *xpub.Service {
	loc.init("service.xpub", func() {
		loc.xpubService = xpub.New(loc.Store(), loc.logger)
	})

	return loc.xpubService
}

func (loc *Locator) EvmCollectorService() *evmcollector.Service {
	loc.init("service.evmcollector", func() {
		loc.evmCollectorService = evmcollector.New(loc.DB().Pool, loc.config.Evm.Config, loc.logger)
	})

	return loc.evmCollectorService
}

func (loc *Locator) SubscriptionService() *subscription.Service {
	loc.init("service.subscription", func() {
		loc.subscriptionService = subscription.New(loc.DB().Pool, loc.logger)
	})

	return loc.subscriptionService
}

func (loc *Locator) EmailService() *email.Service {
	loc.init("service.email", func() {
		loc.emailService = email.New(loc.DB().Pool, loc.logger)
	})

	return loc.emailService
}

func (loc *Locator) WatcherService() *watcher.Service {
	loc.init("service.watcher", func() {
		loc.watcherService = watcher.New(
			loc.config.Oxygen.Watcher,
			loc.RPCProvider(),
			loc.BitcoinProvider(),
			loc.TrongridProvider(),
			loc.TransactionService(),
			loc.WalletService(),
			loc.logger,
		)
	})

	return loc.watcherService
}

func (loc *Locator) ProcessingService() *processing.Service {
	loc.init("service.processing", func() {
		loc.processingService = processing.New(
			loc.config.Oxygen.Processing,
			loc.WalletService(),
			loc.MerchantService(),
			loc.PaymentService(),
			loc.TransactionService(),
			loc.XpubService(),
			loc.EvmCollectorService(),
			loc.EmailService(),
			loc.SubscriptionService(),
			loc.BlockchainService(),
			loc.EventBus(),
			loc.Locker(),
			loc.logger,
		)
	})

	return loc.processingService
}

func (loc *Locator) JobLogger() *log.JobLogger {
	loc.init("service.jogLogger", func() {
		loc.jobLogger = log.NewJobLogger(loc.Store())
	})

	return loc.jobLogger
}

func (loc *Locator) init(key string, f func()) {
	if loc.once[key] == nil {
		loc.once[key] = &sync.Once{}
	}

	loc.once[key].Do(f)
}
