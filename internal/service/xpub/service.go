package xpub

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/cryptolink/cryptolink/internal/db/repository"
	"github.com/cryptolink/cryptolink/internal/kms/wallet"
	"github.com/cryptolink/cryptolink/internal/util"
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
	TatumMainnetSubscriptionID string
	TatumTestnetSubscriptionID string
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

// CreateXpubWallet creates a new xpub wallet for a merchant.
// If a deactivated wallet exists for this blockchain, it reactivates it with the new xpub.
func (s *Service) CreateXpubWallet(ctx context.Context, merchantID int64, blockchain, xpubKey, derivationPath string) (*XpubWallet, error) {
	// Validate xpub format
	if !s.validateXpub(xpubKey) {
		return nil, ErrInvalidXpub
	}

	// Check if an active wallet already exists for this merchant/blockchain
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

	// Check if a deactivated wallet exists (unique constraint on merchant_id + blockchain).
	// If so, reactivate it with the new xpub instead of inserting a new row.
	existingEntry, findErr := s.store.GetXpubWalletByMerchantAndBlockchainAny(ctx, repository.GetXpubWalletByMerchantAndBlockchainParams{
		MerchantID: merchantID,
		Blockchain: blockchain,
	})
	if findErr == nil {
		// Deactivated wallet found — reactivate with new xpub
		reactivated, reErr := s.store.ReactivateXpubWallet(ctx, repository.ReactivateXpubWalletParams{
			ID:             existingEntry.ID,
			Xpub:           xpubKey,
			DerivationPath: derivationPath,
			UpdatedAt:      now,
		})
		if reErr != nil {
			return nil, reErr
		}
		return entryToXpubWallet(reactivated), nil
	}

	// No existing wallet at all — create new
	entry, err := s.store.CreateXpubWallet(ctx, repository.CreateXpubWalletParams{
		Uuid:             uuid.New(),
		MerchantID:       merchantID,
		Blockchain:       blockchain,
		Xpub:             xpubKey,
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

// DeactivateWallet deactivates an xpub wallet (soft delete)
func (s *Service) DeactivateWallet(ctx context.Context, walletUUID uuid.UUID, merchantID int64) error {
	entry, err := s.store.GetXpubWalletByUUID(ctx, walletUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	// Ensure the wallet belongs to this merchant
	if entry.MerchantID != merchantID {
		return ErrNotFound
	}

	return s.store.DeactivateXpubWallet(ctx, repository.DeactivateXpubWalletParams{
		ID:        entry.ID,
		UpdatedAt: time.Now(),
	})
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

// GetDerivedAddressByUUID finds a derived address by its UUID
func (s *Service) GetDerivedAddressByUUID(ctx context.Context, addrUUID uuid.UUID) (*DerivedAddress, error) {
	entry, err := s.store.GetDerivedAddressByUUID(ctx, addrUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAddressNotFound
	}
	if err != nil {
		return nil, err
	}

	return entryToDerivedAddress(entry), nil
}

// UpdateDerivedAddressTatumSubscription updates the Tatum subscription IDs for a derived address
func (s *Service) UpdateDerivedAddressTatumSubscription(ctx context.Context, addressID int64, mainnetSubID, testnetSubID string) (*DerivedAddress, error) {
	entry, err := s.store.UpdateDerivedAddressTatumSubscription(ctx, repository.UpdateDerivedAddressTatumSubscriptionParams{
		ID:                         addressID,
		TatumMainnetSubscriptionID: repository.StringToNullable(mainnetSubID),
		TatumTestnetSubscriptionID: repository.StringToNullable(testnetSubID),
		UpdatedAt:                  time.Now(),
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

	// childKey.Key is the 33-byte compressed public key
	compressedPubKey := childKey.Key
	pubKeyHex := hex.EncodeToString(compressedPubKey)

	switch wallet.Blockchain(blockchain) {
	case wallet.BTC:
		// Bitcoin P2PKH address (go-hdwallet handles this correctly)
		return childKey.Address(), pubKeyHex, nil

	case wallet.ETH, wallet.MATIC, wallet.BSC, wallet.ARBITRUM, wallet.AVAX:
		// Decompress the public key and compute keccak256-based ETH address
		ecdsaPubKey, err := crypto.DecompressPubkey(compressedPubKey)
		if err != nil {
			return "", "", errors.Wrap(err, "failed to decompress public key for ETH")
		}
		address := crypto.PubkeyToAddress(*ecdsaPubKey).Hex()
		return address, pubKeyHex, nil

	case wallet.TRON:
		// TRON uses same derivation as ETH but with 0x41 prefix + base58check
		ecdsaPubKey, err := crypto.DecompressPubkey(compressedPubKey)
		if err != nil {
			return "", "", errors.Wrap(err, "failed to decompress public key for TRON")
		}
		ethAddr := crypto.PubkeyToAddress(*ecdsaPubKey).Hex()
		tronHex := "41" + ethAddr[2:]
		address := util.TronHexToBase58(tronHex)
		return address, pubKeyHex, nil

	default:
		return childKey.Address(), pubKeyHex, nil
	}
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

	mainnetSubID := ""
	if entry.TatumMainnetSubscriptionID.Valid {
		mainnetSubID = entry.TatumMainnetSubscriptionID.String
	}
	testnetSubID := ""
	if entry.TatumTestnetSubscriptionID.Valid {
		testnetSubID = entry.TatumTestnetSubscriptionID.String
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
		TatumMainnetSubscriptionID: mainnetSubID,
		TatumTestnetSubscriptionID: testnetSubID,
		CreatedAt:       entry.CreatedAt,
		UpdatedAt:       entry.UpdatedAt,
	}
}
