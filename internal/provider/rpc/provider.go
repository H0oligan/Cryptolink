// Package rpc provides direct EVM blockchain RPC connections,
// replacing the Tatum RPC proxy with configurable free endpoints.
package rpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
)

// Config holds RPC endpoint URLs for each supported EVM chain.
// Each chain has a mainnet and testnet endpoint, with sensible free defaults.
type Config struct {
	ETH           ChainRPC `yaml:"eth"`
	MATIC         ChainRPC `yaml:"matic"`
	BSC           ChainRPC `yaml:"bsc"`
	ARBITRUM      ChainRPC `yaml:"arbitrum"`
	AVAX          ChainRPC `yaml:"avax"`
	ConnTimeout   int      `yaml:"conn_timeout" env:"RPC_CONN_TIMEOUT" env-default:"15" env-description:"RPC connection timeout in seconds"`
}

// ChainRPC holds mainnet/testnet RPC URLs for a single chain.
type ChainRPC struct {
	Mainnet  string `yaml:"mainnet"`
	Testnet  string `yaml:"testnet"`
	Fallback string `yaml:"fallback"`
}

// Provider manages EVM RPC connections with health checking and fallback.
type Provider struct {
	config Config
	logger *zerolog.Logger
	mu     sync.RWMutex
	health map[string]bool // tracks endpoint health
}

// DefaultConfig returns config with free public RPC endpoints.
func DefaultConfig() Config {
	return Config{
		ETH: ChainRPC{
			Mainnet:  "https://eth.llamarpc.com",
			Testnet:  "https://rpc.sepolia.org",
			Fallback: "https://rpc.ankr.com/eth",
		},
		MATIC: ChainRPC{
			Mainnet:  "https://polygon-rpc.com",
			Testnet:  "https://rpc-mumbai.maticvigil.com",
			Fallback: "https://rpc.ankr.com/polygon",
		},
		BSC: ChainRPC{
			Mainnet:  "https://bsc-dataseed.binance.org",
			Testnet:  "https://data-seed-prebsc-1-s1.binance.org:8545",
			Fallback: "https://rpc.ankr.com/bsc",
		},
		ARBITRUM: ChainRPC{
			Mainnet:  "https://arb1.arbitrum.io/rpc",
			Testnet:  "https://sepolia-rollup.arbitrum.io/rpc",
			Fallback: "https://rpc.ankr.com/arbitrum",
		},
		AVAX: ChainRPC{
			Mainnet:  "https://api.avax.network/ext/bc/C/rpc",
			Testnet:  "https://api.avax-test.network/ext/bc/C/rpc",
			Fallback: "https://rpc.ankr.com/avalanche",
		},
		ConnTimeout: 15,
	}
}

func New(config Config, logger *zerolog.Logger) *Provider {
	log := logger.With().Str("channel", "rpc_provider").Logger()

	// Apply defaults for any empty endpoints
	defaults := DefaultConfig()
	applyDefaults(&config, &defaults)

	return &Provider{
		config: config,
		logger: &log,
		health: make(map[string]bool),
	}
}

func applyDefaults(cfg, defaults *Config) {
	applyChainDefaults(&cfg.ETH, &defaults.ETH)
	applyChainDefaults(&cfg.MATIC, &defaults.MATIC)
	applyChainDefaults(&cfg.BSC, &defaults.BSC)
	applyChainDefaults(&cfg.ARBITRUM, &defaults.ARBITRUM)
	applyChainDefaults(&cfg.AVAX, &defaults.AVAX)
	if cfg.ConnTimeout <= 0 {
		cfg.ConnTimeout = defaults.ConnTimeout
	}
}

func applyChainDefaults(cfg, defaults *ChainRPC) {
	if cfg.Mainnet == "" {
		cfg.Mainnet = defaults.Mainnet
	}
	if cfg.Testnet == "" {
		cfg.Testnet = defaults.Testnet
	}
	if cfg.Fallback == "" {
		cfg.Fallback = defaults.Fallback
	}
}

// EthereumRPC returns an Ethereum JSON-RPC client.
func (p *Provider) EthereumRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return p.dial(ctx, p.config.ETH, isTest)
}

// MaticRPC returns a Polygon JSON-RPC client.
func (p *Provider) MaticRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return p.dial(ctx, p.config.MATIC, isTest)
}

// BinanceSmartChainRPC returns a BSC JSON-RPC client.
func (p *Provider) BinanceSmartChainRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return p.dial(ctx, p.config.BSC, isTest)
}

// ArbitrumRPC returns an Arbitrum JSON-RPC client.
func (p *Provider) ArbitrumRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return p.dial(ctx, p.config.ARBITRUM, isTest)
}

// AvalancheRPC returns an Avalanche C-Chain JSON-RPC client.
func (p *Provider) AvalancheRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return p.dial(ctx, p.config.AVAX, isTest)
}

// dial connects to the primary endpoint, falling back if the primary fails.
func (p *Provider) dial(ctx context.Context, chain ChainRPC, isTest bool) (*ethclient.Client, error) {
	timeout := time.Duration(p.config.ConnTimeout) * time.Second
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	primary := chain.Mainnet
	if isTest {
		primary = chain.Testnet
	}

	client, err := ethclient.DialContext(dialCtx, primary)
	if err == nil {
		return client, nil
	}

	p.logger.Warn().Err(err).Str("url", primary).Msg("primary RPC failed, trying fallback")

	// Only try fallback for mainnet (testnet has no fallback)
	if !isTest && chain.Fallback != "" {
		fallbackCtx, fallbackCancel := context.WithTimeout(ctx, timeout)
		defer fallbackCancel()

		client, err = ethclient.DialContext(fallbackCtx, chain.Fallback)
		if err == nil {
			p.markUnhealthy(primary)
			return client, nil
		}

		p.logger.Error().Err(err).Str("url", chain.Fallback).Msg("fallback RPC also failed")
	}

	return nil, fmt.Errorf("all RPC endpoints failed for chain (primary=%s): %w", primary, err)
}

func (p *Provider) markUnhealthy(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health[url] = false
	p.logger.Warn().Str("url", url).Msg("RPC endpoint marked unhealthy")
}
