package merchantapi

import (
	"github.com/cryptolink/cryptolink/internal/auth"
	"github.com/cryptolink/cryptolink/internal/bus"
	"github.com/cryptolink/cryptolink/internal/service/blockchain"
	"github.com/cryptolink/cryptolink/internal/service/evmcollector"
	"github.com/cryptolink/cryptolink/internal/service/merchant"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/subscription"
	"github.com/cryptolink/cryptolink/internal/service/wallet"
	"github.com/cryptolink/cryptolink/internal/service/xpub"
	"github.com/rs/zerolog"
)

type BlockchainService interface {
	blockchain.Resolver
	blockchain.Convertor
}

type Handler struct {
	merchants       *merchant.Service
	tokens          *auth.TokenAuthManager
	payments        *payment.Service
	wallets         *wallet.Service
	xpubService     *xpub.Service
	evmCollector    *evmcollector.Service
	subscriptions   *subscription.Service
	blockchain      BlockchainService
	publisher       bus.Publisher
	logger          *zerolog.Logger
}

func NewHandler(
	merchants *merchant.Service,
	tokens *auth.TokenAuthManager,
	payments *payment.Service,
	wallets *wallet.Service,
	xpubService *xpub.Service,
	evmCollectorService *evmcollector.Service,
	subscriptionService *subscription.Service,
	blockchainService BlockchainService,
	publisher bus.Publisher,
	logger *zerolog.Logger,
) *Handler {
	log := logger.With().Str("channel", "dashboard_handler").Logger()

	return &Handler{
		merchants:       merchants,
		tokens:          tokens,
		payments:        payments,
		wallets:         wallets,
		xpubService:     xpubService,
		evmCollector:    evmCollectorService,
		subscriptions:   subscriptionService,
		blockchain:      blockchainService,
		publisher:       publisher,
		logger:          &log,
	}
}

func (h *Handler) MerchantService() *merchant.Service {
	return h.merchants
}
