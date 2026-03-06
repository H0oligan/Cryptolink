package blockchain

import (
	"github.com/cryptolink/cryptolink/internal/provider/bitcoin"
	"github.com/cryptolink/cryptolink/internal/provider/monero"
	"github.com/cryptolink/cryptolink/internal/provider/pricefeed"
	"github.com/cryptolink/cryptolink/internal/provider/rpc"
	"github.com/cryptolink/cryptolink/internal/provider/solana"
	"github.com/cryptolink/cryptolink/internal/provider/trongrid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	ErrValidation         = errors.New("invalid data provided")
	ErrCurrencyNotFound   = errors.New("currency not found")
	ErrNoTokenAddress     = errors.New("token should have contract address filled")
	ErrParseMoney         = errors.New("unable to parse money value")
	ErrInsufficientFunds  = errors.New("wallet has insufficient funds")
	ErrInvalidTransaction = errors.New("transaction is invalid")
)

type Providers struct {
	RPC       *rpc.Provider
	PriceFeed *pricefeed.Provider
	Trongrid  *trongrid.Provider
	Solana    *solana.Provider
	Monero    *monero.Provider
	Bitcoin   *bitcoin.Provider
}

type Service struct {
	*CurrencyResolver
	providers Providers
	logger    *zerolog.Logger
}

func New(currencies *CurrencyResolver, providers Providers, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "blockchain_service").Logger()

	return &Service{
		CurrencyResolver: currencies,
		providers:        providers,
		logger:           &log,
	}
}
