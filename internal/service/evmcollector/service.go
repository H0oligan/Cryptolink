// Package evmcollector manages per-merchant smart contract collector wallets for EVM chains.
// Each merchant gets one MerchantCollector contract address per EVM chain.
// Payments accumulate in the contract; the merchant signs withdrawals via MetaMask.
// CryptoLink never holds funds — it monitors addresses via the internal address watcher.
package evmcollector

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// EvmChainConfig holds per-chain RPC and factory configuration.
type EvmChainConfig struct {
	ChainID        int    `yaml:"chain_id"`
	RPCEndpoint    string `yaml:"rpc_endpoint"`
	FactoryAddress string `yaml:"factory_address"`
}

// Config holds all EVM collector configuration.
type Config struct {
	WalletConnectProjectID string                    `yaml:"walletconnect_project_id" env:"EVM_WALLETCONNECT_PROJECT_ID"`
	Chains                 map[string]EvmChainConfig `yaml:"chains"`
}

// Collector represents a merchant's EVM smart contract collector wallet.
type Collector struct {
	ID              int64
	UUID            uuid.UUID
	MerchantID      int64
	Blockchain      string
	ChainID         int
	ContractAddress string
	OwnerAddress    string
	FactoryAddress  string
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TokenBalance holds an on-chain token balance.
type TokenBalance struct {
	ContractAddress string
	Ticker          string
	Amount          string // human-readable, e.g. "1.23"
	Decimals        int
}

// OnChainBalance holds the full on-chain balance for a collector.
type OnChainBalance struct {
	NativeAmount string // human-readable native balance (ETH/MATIC/BNB/etc)
	NativeTicker string
	Tokens       []TokenBalance
}

// Service manages EVM collector wallets.
type Service struct {
	db     *pgxpool.Pool
	config Config
	logger *zerolog.Logger
}

var (
	ErrNotFound      = errors.New("evm collector not found")
	ErrAlreadyExists = errors.New("evm collector already exists for this blockchain")
)

// New constructs an EVM collector service.
func New(db *pgxpool.Pool, config Config, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "evmcollector_service").Logger()
	return &Service{db: db, config: config, logger: &log}
}

// RegisterCollector creates a new collector record.
// The contract_address must be pre-computed by the frontend using CREATE2 prediction,
// or simply the merchant's wallet address for a simpler flow.
func (s *Service) RegisterCollector(
	ctx context.Context,
	merchantID int64,
	blockchain string,
	chainID int,
	contractAddress string,
	ownerAddress string,
	factoryAddress string,
) (*Collector, error) {
	blockchain = strings.ToUpper(blockchain)

	// Only lowercase EVM addresses (hex). TRON uses Base58Check where case matters.
	if blockchain != "TRON" {
		contractAddress = strings.ToLower(contractAddress)
		ownerAddress = strings.ToLower(ownerAddress)
	}

	now := time.Now().UTC().Truncate(time.Second)
	id := uuid.New()

	_, err := s.db.Exec(ctx, `
		INSERT INTO evm_collector_wallets
			(uuid, merchant_id, blockchain, chain_id, contract_address, owner_address, factory_address, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8, $8)
		ON CONFLICT (merchant_id, blockchain)
		DO UPDATE SET
			contract_address = EXCLUDED.contract_address,
			owner_address    = EXCLUDED.owner_address,
			factory_address  = EXCLUDED.factory_address,
			chain_id         = EXCLUDED.chain_id,
			is_active        = true,
			updated_at       = EXCLUDED.updated_at
	`, id, merchantID, blockchain, chainID, contractAddress, ownerAddress, factoryAddress, now)

	if err != nil {
		return nil, errors.Wrap(err, "unable to upsert evm collector")
	}

	return s.GetByMerchantAndBlockchain(ctx, merchantID, blockchain)
}

// GetByUUID retrieves a collector by its UUID.
func (s *Service) GetByUUID(ctx context.Context, id uuid.UUID) (*Collector, error) {
	return s.scanCollector(s.db.QueryRow(ctx, `
		SELECT id, uuid, merchant_id, blockchain, chain_id, contract_address, owner_address,
		       factory_address, is_active, created_at, updated_at
		FROM evm_collector_wallets
		WHERE uuid = $1 AND is_active = true
	`, id))
}

// GetByMerchantAndBlockchain retrieves a collector for a specific merchant and chain.
func (s *Service) GetByMerchantAndBlockchain(ctx context.Context, merchantID int64, blockchain string) (*Collector, error) {
	return s.scanCollector(s.db.QueryRow(ctx, `
		SELECT id, uuid, merchant_id, blockchain, chain_id, contract_address, owner_address,
		       factory_address, is_active, created_at, updated_at
		FROM evm_collector_wallets
		WHERE merchant_id = $1 AND blockchain = $2 AND is_active = true
	`, merchantID, strings.ToUpper(blockchain)))
}

// GetByContractAddress retrieves a collector by its contract address.
func (s *Service) GetByContractAddress(ctx context.Context, contractAddress string) (*Collector, error) {
	return s.scanCollector(s.db.QueryRow(ctx, `
		SELECT id, uuid, merchant_id, blockchain, chain_id, contract_address, owner_address,
		       factory_address, is_active, created_at, updated_at
		FROM evm_collector_wallets
		WHERE contract_address = $1 AND is_active = true
	`, strings.ToLower(contractAddress)))
}

// ListByMerchantID returns all active collectors for a merchant.
func (s *Service) ListByMerchantID(ctx context.Context, merchantID int64) ([]*Collector, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, uuid, merchant_id, blockchain, chain_id, contract_address, owner_address,
		       factory_address, is_active, created_at, updated_at
		FROM evm_collector_wallets
		WHERE merchant_id = $1 AND is_active = true
		ORDER BY blockchain
	`, merchantID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to list evm collectors")
	}
	defer rows.Close()

	var collectors []*Collector
	for rows.Next() {
		c := &Collector{}
		if err := rows.Scan(
			&c.ID, &c.UUID, &c.MerchantID, &c.Blockchain, &c.ChainID,
			&c.ContractAddress, &c.OwnerAddress, &c.FactoryAddress,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "unable to scan evm collector")
		}
		collectors = append(collectors, c)
	}
	return collectors, nil
}

// Delete soft-deletes a collector (sets is_active=false).
func (s *Service) Delete(ctx context.Context, merchantID int64, blockchain string) error {
	result, err := s.db.Exec(ctx, `
		UPDATE evm_collector_wallets SET is_active = false, updated_at = $1
		WHERE merchant_id = $2 AND blockchain = $3
	`, time.Now().UTC().Truncate(time.Second), merchantID, strings.ToUpper(blockchain))

	if err != nil {
		return errors.Wrap(err, "unable to delete evm collector")
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetChainConfig returns the chain config for a given blockchain name (e.g. "ETH").
func (s *Service) GetChainConfig(blockchain string) (EvmChainConfig, bool) {
	cfg, ok := s.config.Chains[strings.ToUpper(blockchain)]
	return cfg, ok
}

// GetMerchantEmail returns the email of the user who created the given merchant.
// Used for payment notification emails.
func (s *Service) GetMerchantEmail(ctx context.Context, merchantID int64) (string, error) {
	var email string
	err := s.db.QueryRow(ctx, `
		SELECT u.email
		FROM merchants m
		JOIN users u ON u.id = m.creator_id
		WHERE m.id = $1 AND u.deleted_at IS NULL
	`, merchantID).Scan(&email)
	if err != nil {
		return "", errors.Wrap(err, "unable to get merchant user email")
	}
	return email, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...interface{}) error
}

func (s *Service) scanCollector(row scanner) (*Collector, error) {
	c := &Collector{}
	err := row.Scan(
		&c.ID, &c.UUID, &c.MerchantID, &c.Blockchain, &c.ChainID,
		&c.ContractAddress, &c.OwnerAddress, &c.FactoryAddress,
		&c.IsActive, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, ErrNotFound
		}
		return nil, errors.Wrap(err, "unable to scan evm collector")
	}
	return c, nil
}
