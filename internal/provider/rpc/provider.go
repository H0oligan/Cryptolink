// Package rpc provides direct EVM blockchain RPC connections
// with multi-endpoint failover and health tracking.
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
// Each chain supports multiple endpoints for failover.
type Config struct {
	ETH         ChainRPC `yaml:"eth"`
	MATIC       ChainRPC `yaml:"matic"`
	BSC         ChainRPC `yaml:"bsc"`
	ARBITRUM    ChainRPC `yaml:"arbitrum"`
	AVAX        ChainRPC `yaml:"avax"`
	ConnTimeout int      `yaml:"conn_timeout" env:"RPC_CONN_TIMEOUT" env-default:"15" env-description:"RPC connection timeout in seconds"`
}

// ChainRPC holds mainnet/testnet RPC URLs for a single chain.
// Mainnet and Fallback are tried in order; Extra provides additional failover URLs.
type ChainRPC struct {
	Mainnet  string   `yaml:"mainnet"`
	Testnet  string   `yaml:"testnet"`
	Fallback string   `yaml:"fallback"`
	Extra    []string `yaml:"extra"`
}

// Provider manages EVM RPC connections with health checking and multi-endpoint failover.
type Provider struct {
	config  Config
	logger  *zerolog.Logger
	mu      sync.RWMutex
	health  map[string]endpointHealth
}

type endpointHealth struct {
	healthy   bool
	failedAt  time.Time
	failCount int
}

const healthRecoveryInterval = 2 * time.Minute

// DefaultConfig returns config with free public RPC endpoints and failover alternatives.
func DefaultConfig() Config {
	return Config{
		ETH: ChainRPC{
			Mainnet:  "https://ethereum-rpc.publicnode.com",
			Testnet:  "https://rpc.sepolia.org",
			Fallback: "https://1rpc.io/eth",
			Extra:    []string{"https://eth.llamarpc.com", "https://rpc.ankr.com/eth"},
		},
		MATIC: ChainRPC{
			Mainnet:  "https://polygon-bor-rpc.publicnode.com",
			Testnet:  "https://rpc-mumbai.maticvigil.com",
			Fallback: "https://1rpc.io/matic",
			Extra:    []string{"https://polygon-rpc.com"},
		},
		BSC: ChainRPC{
			Mainnet:  "https://bsc-rpc.publicnode.com",
			Testnet:  "https://data-seed-prebsc-1-s1.binance.org:8545",
			Fallback: "https://1rpc.io/bnb",
			Extra:    []string{"https://bsc-dataseed.binance.org", "https://bsc-dataseed1.defibit.io"},
		},
		ARBITRUM: ChainRPC{
			Mainnet:  "https://arb1.arbitrum.io/rpc",
			Testnet:  "https://sepolia-rollup.arbitrum.io/rpc",
			Fallback: "https://rpc.ankr.com/arbitrum",
			Extra:    []string{"https://arbitrum-one-rpc.publicnode.com", "https://1rpc.io/arb"},
		},
		AVAX: ChainRPC{
			Mainnet:  "https://api.avax.network/ext/bc/C/rpc",
			Testnet:  "https://api.avax-test.network/ext/bc/C/rpc",
			Fallback: "https://rpc.ankr.com/avalanche",
			Extra:    []string{"https://avalanche-c-chain-rpc.publicnode.com", "https://1rpc.io/avax/c"},
		},
		ConnTimeout: 15,
	}
}

func New(config Config, logger *zerolog.Logger) *Provider {
	log := logger.With().Str("channel", "rpc_provider").Logger()

	defaults := DefaultConfig()
	applyDefaults(&config, &defaults)

	return &Provider{
		config: config,
		logger: &log,
		health: make(map[string]endpointHealth),
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
	if len(cfg.Extra) == 0 {
		cfg.Extra = defaults.Extra
	}
}

// EthereumRPC returns an Ethereum JSON-RPC client.
func (p *Provider) EthereumRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return p.dialWithFailover(ctx, p.config.ETH, isTest)
}

// MaticRPC returns a Polygon JSON-RPC client.
func (p *Provider) MaticRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return p.dialWithFailover(ctx, p.config.MATIC, isTest)
}

// BinanceSmartChainRPC returns a BSC JSON-RPC client.
func (p *Provider) BinanceSmartChainRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return p.dialWithFailover(ctx, p.config.BSC, isTest)
}

// ArbitrumRPC returns an Arbitrum JSON-RPC client.
func (p *Provider) ArbitrumRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return p.dialWithFailover(ctx, p.config.ARBITRUM, isTest)
}

// AvalancheRPC returns an Avalanche C-Chain JSON-RPC client.
func (p *Provider) AvalancheRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return p.dialWithFailover(ctx, p.config.AVAX, isTest)
}

// dialWithFailover tries all endpoints in order: primary, fallback, then extras.
// Endpoints marked unhealthy are skipped unless enough time has passed for recovery.
// After dialing, a BlockNumber health-check call is made to detect rate-limiting
// or other HTTP-level errors that only surface after a successful TCP connection.
func (p *Provider) dialWithFailover(ctx context.Context, chain ChainRPC, isTest bool) (*ethclient.Client, error) {
	timeout := time.Duration(p.config.ConnTimeout) * time.Second

	// Build ordered endpoint list
	var endpoints []string
	if isTest {
		endpoints = []string{chain.Testnet}
	} else {
		endpoints = []string{chain.Mainnet}
		if chain.Fallback != "" {
			endpoints = append(endpoints, chain.Fallback)
		}
		endpoints = append(endpoints, chain.Extra...)
	}

	var lastErr error
	for _, url := range endpoints {
		if url == "" {
			continue
		}

		if !p.isHealthy(url) {
			continue
		}

		dialCtx, cancel := context.WithTimeout(ctx, timeout)
		client, err := ethclient.DialContext(dialCtx, url)
		cancel()

		if err != nil {
			lastErr = err
			p.markUnhealthy(url)
			p.logger.Warn().Err(err).Str("url", url).Msg("RPC dial failed, trying next")
			continue
		}

		// Verify the endpoint actually works (catches 429 rate limits, auth errors, etc.)
		checkCtx, checkCancel := context.WithTimeout(ctx, timeout)
		_, err = client.BlockNumber(checkCtx)
		checkCancel()

		if err != nil {
			client.Close()
			lastErr = err
			p.markUnhealthy(url)
			p.logger.Warn().Err(err).Str("url", url).Msg("RPC health check failed, trying next")
			continue
		}

		p.markHealthy(url)
		return client, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints available")
	}

	return nil, fmt.Errorf("all RPC endpoints exhausted for chain: %w", lastErr)
}

// isHealthy returns true if the endpoint can be tried.
// Unhealthy endpoints recover after healthRecoveryInterval.
func (p *Provider) isHealthy(url string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	h, exists := p.health[url]
	if !exists {
		return true
	}
	if h.healthy {
		return true
	}

	return time.Since(h.failedAt) >= healthRecoveryInterval
}

func (p *Provider) markHealthy(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health[url] = endpointHealth{healthy: true}
}

func (p *Provider) markUnhealthy(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	h := p.health[url]
	h.healthy = false
	h.failedAt = time.Now()
	h.failCount++
	p.health[url] = h

	p.logger.Warn().Str("url", url).Int("fail_count", h.failCount).Msg("RPC endpoint marked unhealthy")
}
