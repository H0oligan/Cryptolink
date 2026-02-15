package xpub

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/wemeetagain/go-hdwallet"
)

type Service struct {
	store  repository.Storage
	logger *zerolog.Logger
}

type XpubWallet struct {
	ID               int64
	UUID             uuid.UUID
	MerchantID       int64
	Blockchain       string
	Xpub             string
	DerivationPath   string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastDerivedIndex int
	IsActive         bool
}

type DerivedAddress struct {
	ID              int64
	UUID            uuid.UUID
	XpubWalletID    int64
	MerchantID      int64
	Blockchain      string
	Address         string
	DerivationPath  string
	DerivationIndex int
	PublicKey       *string
	IsUsed          bool
	PaymentID       *int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

var (
	ErrNotFound         = errors.New("xpub wallet not found")
	ErrAlreadyExists    = errors.New("xpub wallet already exists for this blockchain")
	ErrInvalidXpub      = errors.New("invalid xpub format")
	ErrDerivationFailed = errors.New("failed to derive address")
	ErrAddressNotFound  = errors.New("derived address not found")
)

func New(store repository.Storage, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "xpub_service").Logger()

	return &Service{
		store:  store,
		logger: &log,
	}
}

// CreateXpubWallet creates a new xpub wallet for a merchant
func (s *Service) CreateXpubWallet(ctx context.Context, merchantID int64, blockchain, xpub, derivationPath string) (*XpubWallet, error) {
	// Validate xpub format
	if !s.validateXpub(xpub) {
		return nil, ErrInvalidXpub
	}

	// Check if wallet already exists for this merchant/blockchain
	_, err := s.store.GetXpubWalletByMerchantAndBlockchain(ctx, repository.GetXpubWalletByMerchantAndBlockchainParams{
		MerchantID: merchantID,
		Blockchain: blockchain,
	})
	if err == nil {
		return nil, ErrAlreadyExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	now := time.Now()
	entry, err := s.store.CreateXpubWallet(ctx, repository.CreateXpubWalletParams{
		Uuid:             uuid.New(),
		MerchantID:       merchantID,
		Blockchain:       blockchain,
		Xpub:             xpub,
		DerivationPath:   derivationPath,
		CreatedAt:        now,
		UpdatedAt:        now,
		LastDerivedIndex: sql.NullInt32{Int32: 0, Valid: true},
		IsActive:         sql.NullBool{Bool: true, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	return entryToXpubWallet(entry), nil
}

// GetByID gets xpub wallet by ID
func (s *Service) GetByID(ctx context.Context, id int64) (*XpubWallet, error) {
	entry, err := s.store.GetXpubWalletByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return entryToXpubWallet(entry), nil
}

// GetByUUID gets xpub wallet by UUID
func (s *Service) GetByUUID(ctx context.Context, walletUUID uuid.UUID) (*XpubWallet, error) {
	entry, err := s.store.GetXpubWalletByUUID(ctx, walletUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return entryToXpubWallet(entry), nil
}

// GetByMerchantAndBlockchain gets xpub wallet for a merchant/blockchain
func (s *Service) GetByMerchantAndBlockchain(ctx context.Context, merchantID int64, blockchain string) (*XpubWallet, error) {
	entry, err := s.store.GetXpubWalletByMerchantAndBlockchain(ctx, repository.GetXpubWalletByMerchantAndBlockchainParams{
		MerchantID: merchantID,
		Blockchain: blockchain,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return entryToXpubWallet(entry), nil
}

// ListByMerchantID lists all xpub wallets for a merchant
func (s *Service) ListByMerchantID(ctx context.Context, merchantID int64) ([]*XpubWallet, error) {
	entries, err := s.store.ListXpubWalletsByMerchantID(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	wallets := make([]*XpubWallet, len(entries))
	for i, entry := range entries {
		wallets[i] = entryToXpubWallet(entry)
	}

	return wallets, nil
}

// DeriveAddress derives a new address at the next available index
func (s *Service) DeriveAddress(ctx context.Context, walletID int64) (*DerivedAddress, error) {
	// Get wallet
	w, err := s.store.GetXpubWalletByID(ctx, walletID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	// Get next index
	lastIndex := int32(0)
	if w.LastDerivedIndex.Valid {
		lastIndex = w.LastDerivedIndex.Int32
	}
	nextIndex := int(lastIndex) + 1

	// Derive address from xpub
	address, pubKey, err := s.deriveAddressFromXpub(w.Xpub, w.Blockchain, nextIndex)
	if err != nil {
		return nil, errors.Wrap(err, "failed to derive address")
	}

	// Create derived address record
	now := time.Now()
	entry, err := s.store.CreateDerivedAddress(ctx, repository.CreateDerivedAddressParams{
		Uuid:            uuid.New(),
		XpubWalletID:    walletID,
		MerchantID:      w.MerchantID,
		Blockchain:      w.Blockchain,
		Address:         address,
		DerivationPath:  fmt.Sprintf("%s/%d", w.DerivationPath, nextIndex),
		DerivationIndex: int32(nextIndex),
		PublicKey:       repository.StringToNullable(pubKey),
		IsUsed:          sql.NullBool{Bool: false, Valid: true},
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		return nil, err
	}

	// Update wallet's last derived index
	_, err = s.store.UpdateXpubWalletLastIndex(ctx, repository.UpdateXpubWalletLastIndexParams{
		ID:               walletID,
		LastDerivedIndex: sql.NullInt32{Int32: int32(nextIndex), Valid: true},
		UpdatedAt:        now,
	})
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to update wallet last derived index")
	}

	return entryToDerivedAddress(entry), nil
}

// DeriveAddressBatch derives multiple addresses
func (s *Service) DeriveAddressBatch(ctx context.Context, walletID int64, count int) ([]*DerivedAddress, error) {
	addresses := make([]*DerivedAddress, 0, count)
	for i := 0; i < count; i++ {
		addr, err := s.DeriveAddress(ctx, walletID)
		if err != nil {
			return addresses, err
		}
		addresses = append(addresses, addr)
	}
	return addresses, nil
}

// GetNextUnusedAddress gets the next unused address, deriving if needed
func (s *Service) GetNextUnusedAddress(ctx context.Context, walletID int64) (*DerivedAddress, error) {
	// Try to get existing unused address
	entry, err := s.store.GetNextUnusedAddress(ctx, walletID)
	if err == nil {
		return entryToDerivedAddress(entry), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// No unused address, derive a new one
	return s.DeriveAddress(ctx, walletID)
}

// ListDerivedAddresses lists all derived addresses for a wallet
func (s *Service) ListDerivedAddresses(ctx context.Context, walletID int64) ([]*DerivedAddress, error) {
	entries, err := s.store.ListDerivedAddressesByWalletID(ctx, walletID)
	if err != nil {
		return nil, err
	}

	addresses := make([]*DerivedAddress, len(entries))
	for i, entry := range entries {
		addresses[i] = entryToDerivedAddress(entry)
	}

	return addresses, nil
}

// MarkAddressAsUsed marks an address as used by a payment
func (s *Service) MarkAddressAsUsed(ctx context.Context, addressID int64, paymentID int64) (*DerivedAddress, error) {
	entry, err := s.store.MarkAddressAsUsed(ctx, repository.MarkAddressAsUsedParams{
		ID:        addressID,
		PaymentID: repository.Int64ToNullable(paymentID),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		return nil, err
	}

	return entryToDerivedAddress(entry), nil
}

// GetAddressByAddress finds a derived address by its blockchain address
func (s *Service) GetAddressByAddress(ctx context.Context, blockchain, address string) (*DerivedAddress, error) {
	entry, err := s.store.GetDerivedAddressByAddress(ctx, repository.GetDerivedAddressByAddressParams{
		Blockchain: blockchain,
		Address:    address,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAddressNotFound
	}
	if err != nil {
		return nil, err
	}

	return entryToDerivedAddress(entry), nil
}

// validateXpub validates the xpub format
func (s *Service) validateXpub(xpub string) bool {
	// Basic validation - xpub should start with "xpub" for Bitcoin mainnet
	// or other prefixes for different networks
	if len(xpub) < 111 {
		return false
	}

	// Try to parse it
	_, err := hdwallet.StringWallet(xpub)
	return err == nil
}

// deriveAddressFromXpub derives an address from xpub at given index
func (s *Service) deriveAddressFromXpub(xpub, blockchain string, index int) (string, string, error) {
	// Parse xpub
	key, err := hdwallet.StringWallet(xpub)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to parse xpub")
	}

	// Derive child key at index
	childKey, err := key.Child(uint32(index))
	if err != nil {
		return "", "", errors.Wrap(err, "failed to derive child key")
	}

	// Get address based on blockchain
	var address string
	pubKeyStr := childKey.String()

	switch wallet.Blockchain(blockchain) {
	case wallet.BTC:
		address = childKey.Address()
	case wallet.ETH, wallet.MATIC, wallet.BSC, wallet.ARBITRUM, wallet.AVAX:
		// For EVM chains, derive Ethereum-style address
		// Note: This is a simplified version - proper implementation would use
		// secp256k1 to derive the Ethereum address from the public key
		address = childKey.Address() // This needs proper EVM address derivation
	case wallet.TRON:
		// TRON uses different address format
		address = childKey.Address() // This needs proper TRON address derivation
	default:
		address = childKey.Address()
	}

	return address, pubKeyStr, nil
}

// Helper functions to convert database entries to domain models
func entryToXpubWallet(entry repository.XpubWallet) *XpubWallet {
	lastDerivedIndex := 0
	if entry.LastDerivedIndex.Valid {
		lastDerivedIndex = int(entry.LastDerivedIndex.Int32)
	}
	isActive := true
	if entry.IsActive.Valid {
		isActive = entry.IsActive.Bool
	}

	return &XpubWallet{
		ID:               entry.ID,
		UUID:             entry.Uuid,
		MerchantID:       entry.MerchantID,
		Blockchain:       entry.Blockchain,
		Xpub:             entry.Xpub,
		DerivationPath:   entry.DerivationPath,
		CreatedAt:        entry.CreatedAt,
		UpdatedAt:        entry.UpdatedAt,
		LastDerivedIndex: lastDerivedIndex,
		IsActive:         isActive,
	}
}

func entryToDerivedAddress(entry repository.DerivedAddress) *DerivedAddress {
	isUsed := false
	if entry.IsUsed.Valid {
		isUsed = entry.IsUsed.Bool
	}

	return &DerivedAddress{
		ID:              entry.ID,
		UUID:            entry.Uuid,
		XpubWalletID:    entry.XpubWalletID,
		MerchantID:      entry.MerchantID,
		Blockchain:      entry.Blockchain,
		Address:         entry.Address,
		DerivationPath:  entry.DerivationPath,
		DerivationIndex: int(entry.DerivationIndex),
		PublicKey:       repository.NullableStringToPointer(entry.PublicKey),
		IsUsed:          isUsed,
		PaymentID:       repository.NullableInt64ToPointer(entry.PaymentID),
		CreatedAt:       entry.CreatedAt,
		UpdatedAt:       entry.UpdatedAt,
	}
}
