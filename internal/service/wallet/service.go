package wallet

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/cryptolink/cryptolink/internal/db/repository"
	kmswallet "github.com/cryptolink/cryptolink/internal/kms/wallet"
	"github.com/cryptolink/cryptolink/internal/service/blockchain"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	ErrNotFound                     = errors.New("wallet not found")
	ErrBalanceNotFound              = errors.New("balance not found")
	ErrInvalidBlockchain            = errors.New("invalid blockchain provided")
	ErrInvalidType                  = errors.New("invalid type provided")
	ErrInsufficientBalance          = errors.New("insufficient balance")
	ErrInsufficienceMerchantBalance = errors.Wrap(ErrInsufficientBalance, "merchant")
)

const (
	TypeInbound  Type = "inbound"
	TypeOutbound Type = "outbound"
)

type BlockchainService interface {
	blockchain.Convertor
}

type Service struct {
	blockchain BlockchainService
	store      repository.Storage
	logger     *zerolog.Logger
}

// Wallet represents a legacy hot wallet record. The struct is retained for
// compatibility with transaction and watcher code that references it, but
// no new wallets are created (CryptoLink is non-custodial).
type Wallet struct {
	ID                           int64
	CreatedAt                    time.Time
	UUID                         uuid.UUID
	Address                      string
	Blockchain                   kmswallet.Blockchain
	Type                         Type
	ConfirmedMainnetTransactions int64
	PendingMainnetTransactions   int64
	ConfirmedTestnetTransactions int64
	PendingTestnetTransactions   int64
}

type Type string

func New(
	blockchainService BlockchainService,
	store *repository.Store,
	logger *zerolog.Logger,
) *Service {
	log := logger.With().Str("channel", "wallet_service").Logger()

	return &Service{
		blockchain: blockchainService,
		store:      store,
		logger:     &log,
	}
}

// GetByID is a stub that always returns ErrNotFound.
// Hot wallets have been removed (CryptoLink is non-custodial).
func (s *Service) GetByID(_ context.Context, _ int64) (*Wallet, error) {
	return nil, ErrNotFound
}

// GetByUUID is a stub that always returns ErrNotFound.
// Hot wallets have been removed (CryptoLink is non-custodial).
func (s *Service) GetByUUID(_ context.Context, _ uuid.UUID) (*Wallet, error) {
	return nil, ErrNotFound
}
