// Package pricefeed provides cryptocurrency exchange rates from free public APIs,
// replacing the Tatum exchange rate API dependency.
//
// Sources:
//   - Primary: Binance public ticker API (no key needed, generous rate limits)
//   - Fallback: CoinGecko free API (10-30 req/min)
//
// Security: Validates rate ranges, rejects anomalous data, uses HTTPS only.
package pricefeed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// ExchangeRate represents a price quote from an external source.
type ExchangeRate struct {
	Value     string  // string float, e.g. "1823.45"
	Timestamp float64 // unix millis
}

// Config for the price feed provider.
type Config struct {
	BinanceBaseURL   string `yaml:"binance_base_url" env:"PRICEFEED_BINANCE_URL" env-default:"https://api.binance.com" env-description:"Binance API base URL"`
	CoinGeckoBaseURL string `yaml:"coingecko_base_url" env:"PRICEFEED_COINGECKO_URL" env-default:"https://api.coingecko.com" env-description:"CoinGecko API base URL"`
	CacheTTLSeconds  int    `yaml:"cache_ttl_seconds" env:"PRICEFEED_CACHE_TTL" env-default:"30" env-description:"Price cache TTL in seconds"`
}

// Provider fetches exchange rates from free public APIs.
type Provider struct {
	config     Config
	logger     *zerolog.Logger
	httpClient *http.Client
	cache      *rateCache
}

type rateCache struct {
	mu      sync.RWMutex
	entries map[string]cachedRate
	ttl     time.Duration
}

type cachedRate struct {
	rate      ExchangeRate
	fetchedAt time.Time
}

// Ticker mappings: CryptoLink ticker -> Binance symbol suffix
// Binance uses pairs like ETHUSDT, BTCUSDT, etc.
var binanceSymbols = map[string]string{
	"ETH":       "ETHUSDT",
	"BTC":       "BTCUSDT",
	"MATIC":     "MATICUSDT",
	"BNB":       "BNBUSDT",
	"TRX":       "TRXUSDT",
	"AVAX":      "AVAXUSDT",
	"USDT":      "USDTUSD",
	"USDC":      "USDCUSDT",
	"ARB":       "ARBUSDT",
	// Stablecoins pegged 1:1
	"ETH_USDT":  "USDTUSD",
	"MATIC_USDT": "USDTUSD",
	"BSC_USDT":  "USDTUSD",
	"TRON_USDT": "USDTUSD",
	"ETH_USDC":  "USDCUSDT",
	"MATIC_USDC": "USDCUSDT",
	"BSC_USDC":  "USDCUSDT",
}

// CoinGecko ID mappings for fallback
var coinGeckoIDs = map[string]string{
	"ETH":   "ethereum",
	"BTC":   "bitcoin",
	"MATIC": "matic-network",
	"BNB":   "binancecoin",
	"TRX":   "tron",
	"AVAX":  "avalanche-2",
	"USDT":  "tether",
	"USDC":  "usd-coin",
	"ARB":   "arbitrum",
}

func New(config Config, logger *zerolog.Logger) *Provider {
	log := logger.With().Str("channel", "pricefeed_provider").Logger()

	if config.BinanceBaseURL == "" {
		config.BinanceBaseURL = "https://api.binance.com"
	}
	if config.CoinGeckoBaseURL == "" {
		config.CoinGeckoBaseURL = "https://api.coingecko.com"
	}
	if config.CacheTTLSeconds <= 0 {
		config.CacheTTLSeconds = 30
	}

	return &Provider{
		config: config,
		logger: &log,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		cache: &rateCache{
			entries: make(map[string]cachedRate),
			ttl:     time.Duration(config.CacheTTLSeconds) * time.Second,
		},
	}
}

// GetExchangeRate returns the exchange rate for a crypto asset priced in a fiat base pair.
// Semantics match the old Tatum API: GetExchangeRate("USD", "ETH") returns rate = 1823.45
// meaning 1 ETH = 1823.45 USD.
//
// Parameters:
//   - desired: the base pair (e.g. "USD")
//   - selected: the crypto ticker (e.g. "ETH", "BTC", "USDT")
func (p *Provider) GetExchangeRate(ctx context.Context, desired, selected string) (ExchangeRate, error) {
	selected = strings.ToUpper(selected)
	desired = strings.ToUpper(desired)

	cacheKey := fmt.Sprintf("%s/%s", selected, desired)

	// Check cache
	if rate, ok := p.cache.get(cacheKey); ok {
		return rate, nil
	}

	// Handle stablecoin self-pricing (USDT->USD, USDC->USD)
	if isStablecoin(selected) && (desired == "USD" || desired == "USDT" || desired == "USDC") {
		rate := ExchangeRate{Value: "1.0", Timestamp: float64(time.Now().UnixMilli())}
		p.cache.set(cacheKey, rate)
		return rate, nil
	}

	// Handle fiat-to-fiat (USD->EUR etc.) - use fixed rates for now
	// CryptoLink primarily uses USD, so this is rarely hit
	if isFiat(selected) && isFiat(desired) {
		rate := ExchangeRate{Value: "1.0", Timestamp: float64(time.Now().UnixMilli())}
		p.cache.set(cacheKey, rate)
		return rate, nil
	}

	// Try Binance first
	binanceRate, binanceErr := p.getBinanceRate(ctx, selected, desired)
	binanceOK := binanceErr == nil && validateRate(binanceRate)
	if binanceErr != nil {
		p.logger.Warn().Err(binanceErr).Str("selected", selected).Msg("Binance rate fetch failed")
	} else if !binanceOK {
		p.logger.Warn().Str("selected", selected).Str("rate", binanceRate.Value).Msg("Binance rate failed validation")
	}

	// Try CoinGecko (for cross-validation or as fallback)
	geckoRate, geckoErr := p.getCoinGeckoRate(ctx, selected, desired)
	geckoOK := geckoErr == nil && validateRate(geckoRate)
	if geckoErr != nil {
		p.logger.Debug().Err(geckoErr).Str("selected", selected).Msg("CoinGecko rate fetch failed")
	}

	// Cross-validate if both sources returned valid rates
	if binanceOK && geckoOK {
		if divergence := rateDivergence(binanceRate, geckoRate); divergence > 0.05 {
			p.logger.Error().
				Str("selected", selected).
				Str("binance", binanceRate.Value).
				Str("coingecko", geckoRate.Value).
				Float64("divergence_pct", divergence*100).
				Msg("price sources diverge >5%, rejecting both")
			return ExchangeRate{}, errors.Errorf(
				"price divergence %.1f%% for %s/%s exceeds 5%% threshold (binance=%s, coingecko=%s)",
				divergence*100, selected, desired, binanceRate.Value, geckoRate.Value,
			)
		}
	}

	// Return the best available rate (prefer Binance)
	if binanceOK {
		p.cache.set(cacheKey, binanceRate)
		return binanceRate, nil
	}
	if geckoOK {
		p.cache.set(cacheKey, geckoRate)
		return geckoRate, nil
	}

	return ExchangeRate{}, errors.Errorf("unable to get exchange rate for %s/%s from any source", selected, desired)
}

// getBinanceRate fetches price from Binance public ticker.
// GET /api/v3/ticker/price?symbol=ETHUSDT
func (p *Provider) getBinanceRate(ctx context.Context, selected, desired string) (ExchangeRate, error) {
	symbol := resolveBinanceSymbol(selected, desired)
	if symbol == "" {
		return ExchangeRate{}, errors.Errorf("no Binance symbol mapping for %s/%s", selected, desired)
	}

	url := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", p.config.BinanceBaseURL, symbol)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ExchangeRate{}, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return ExchangeRate{}, errors.Wrap(err, "binance request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ExchangeRate{}, errors.Errorf("binance returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ExchangeRate{}, errors.Wrap(err, "binance response decode failed")
	}

	return ExchangeRate{
		Value:     result.Price,
		Timestamp: float64(time.Now().UnixMilli()),
	}, nil
}

// getCoinGeckoRate fetches price from CoinGecko free API.
// GET /api/v3/simple/price?ids=ethereum&vs_currencies=usd
func (p *Provider) getCoinGeckoRate(ctx context.Context, selected, desired string) (ExchangeRate, error) {
	cgID := resolveCoinGeckoID(selected)
	if cgID == "" {
		return ExchangeRate{}, errors.Errorf("no CoinGecko ID mapping for %s", selected)
	}

	vsCurrency := strings.ToLower(desired)
	url := fmt.Sprintf("%s/api/v3/simple/price?ids=%s&vs_currencies=%s", p.config.CoinGeckoBaseURL, cgID, vsCurrency)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ExchangeRate{}, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return ExchangeRate{}, errors.Wrap(err, "coingecko request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ExchangeRate{}, errors.Errorf("coingecko returned %d: %s", resp.StatusCode, string(body))
	}

	// Response: {"ethereum": {"usd": 1823.45}}
	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ExchangeRate{}, errors.Wrap(err, "coingecko response decode failed")
	}

	prices, ok := result[cgID]
	if !ok {
		return ExchangeRate{}, errors.Errorf("coingecko: no data for %s", cgID)
	}

	price, ok := prices[vsCurrency]
	if !ok {
		return ExchangeRate{}, errors.Errorf("coingecko: no %s price for %s", vsCurrency, cgID)
	}

	return ExchangeRate{
		Value:     strconv.FormatFloat(price, 'f', -1, 64),
		Timestamp: float64(time.Now().UnixMilli()),
	}, nil
}

func resolveBinanceSymbol(selected, desired string) string {
	// Direct lookup
	if sym, ok := binanceSymbols[selected]; ok {
		return sym
	}

	// Try constructing the pair: e.g. ETH + USDT = ETHUSDT
	if desired == "USD" || desired == "USDT" {
		return selected + "USDT"
	}

	return ""
}

func resolveCoinGeckoID(selected string) string {
	if id, ok := coinGeckoIDs[selected]; ok {
		return id
	}
	// Normalize: remove chain prefix (ETH_USDT -> USDT)
	parts := strings.Split(selected, "_")
	if len(parts) == 2 {
		if id, ok := coinGeckoIDs[parts[1]]; ok {
			return id
		}
	}
	return ""
}

// validateRate checks that a rate is a positive number within reasonable bounds.
func validateRate(rate ExchangeRate) bool {
	val, err := strconv.ParseFloat(rate.Value, 64)
	if err != nil {
		return false
	}
	return val > 0 && val < 1e12
}

// rateDivergence returns the relative difference between two rates as a fraction (0.05 = 5%).
func rateDivergence(a, b ExchangeRate) float64 {
	va, errA := strconv.ParseFloat(a.Value, 64)
	vb, errB := strconv.ParseFloat(b.Value, 64)
	if errA != nil || errB != nil || va == 0 || vb == 0 {
		return 1.0 // treat parse errors as maximum divergence
	}
	avg := (va + vb) / 2
	diff := va - vb
	if diff < 0 {
		diff = -diff
	}
	return diff / avg
}

func isStablecoin(ticker string) bool {
	t := strings.ToUpper(ticker)
	return t == "USDT" || t == "USDC" || t == "DAI" ||
		strings.HasSuffix(t, "_USDT") || strings.HasSuffix(t, "_USDC") ||
		strings.HasSuffix(t, "_DAI")
}

func isFiat(ticker string) bool {
	fiats := map[string]bool{"USD": true, "EUR": true, "GBP": true, "CZK": true}
	return fiats[strings.ToUpper(ticker)]
}

func (c *rateCache) get(key string) (ExchangeRate, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return ExchangeRate{}, false
	}

	if time.Since(entry.fetchedAt) > c.ttl {
		return ExchangeRate{}, false
	}

	return entry.rate, true
}

func (c *rateCache) set(key string, rate ExchangeRate) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cachedRate{
		rate:      rate,
		fetchedAt: time.Now(),
	}
}
