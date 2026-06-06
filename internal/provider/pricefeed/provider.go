// Package pricefeed provides cryptocurrency exchange rates from free public APIs.
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

	"github.com/cryptolink/cryptolink/internal/money"
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
	"TRON":      "TRXUSDT", // currencies.json uses "TRON" as the native coin ticker
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
	"TRON":  "tron", // currencies.json uses "TRON" as the native coin ticker
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
// Example: GetExchangeRate("USD", "ETH") returns rate = 1823.45
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

	// Stablecoin identity is exactly 1.0 by definition (USDT->USDT, USDC->USDC,
	// ETH_USDT->USDT, ...). Note we deliberately do NOT short-circuit every
	// stablecoin->USD to 1.0: a stablecoin's real value must be fetched from the
	// market so a de-peg (e.g. USDC trading at 0.98) is reflected in conversions
	// and in cross-currency payment matching. A peg fallback is applied at the
	// end only if every market source is unreachable.
	if isStablecoin(selected) && stableBase(selected) == stableBase(desired) {
		rate := ExchangeRate{Value: "1.0", Timestamp: float64(time.Now().UnixMilli())}
		p.cache.set(cacheKey, rate)
		return rate, nil
	}

	// Non-reference stablecoin (USDC, DAI, ...) -> non-USD fiat (EUR, GBP, ...):
	// Binance lists EURUSDT but not EURUSDC, so the direct path would value
	// every stablecoin via the USDT pair and mask a de-peg. Compose the real
	// stablecoin/USDT rate with the USDT/fiat rate so EUR (and every other
	// supported fiat) sees the *specific* stablecoin's true value.
	if isStablecoin(selected) && stableBase(selected) != "USDT" && isNonUSDFiat(desired) {
		if composed, ok := p.composeStablecoinFiatRate(ctx, selected, desired); ok {
			p.cache.set(cacheKey, composed)
			return composed, nil
		}
		// fall through to the existing best-effort path on composition failure
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
		p.logger.Warn().Err(binanceErr).
			Str("selected", selected).Str("desired", desired).
			Msg("Binance rate fetch failed")
	} else if !binanceOK {
		p.logger.Warn().
			Str("selected", selected).Str("desired", desired).
			Str("rate", binanceRate.Value).
			Msg("Binance rate failed validation")
	}

	// Try CoinGecko (for cross-validation or as fallback)
	geckoRate, geckoErr := p.getCoinGeckoRate(ctx, selected, desired)
	geckoOK := geckoErr == nil && validateRate(geckoRate)
	if geckoErr != nil {
		// WARN, not DEBUG: when CoinGecko fails silently it masks the second
		// half of our dual-source pricing. A run where Binance is also down
		// becomes a hard 500 with no log trail — promoting this keeps the
		// fallback path observable.
		p.logger.Warn().Err(geckoErr).
			Str("selected", selected).Str("desired", desired).
			Msg("CoinGecko rate fetch failed")
	} else if !geckoOK {
		p.logger.Warn().
			Str("selected", selected).Str("desired", desired).
			Str("rate", geckoRate.Value).
			Msg("CoinGecko rate failed validation")
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

	// Peg fallback: a stablecoin priced in USD is ~1.0. If every market source
	// is unreachable, assume the peg rather than failing the whole conversion.
	// De-peg detection above still applies whenever any source responds, so this
	// only degrades to 1.0 during a full pricing outage.
	if isStablecoin(selected) && (desired == "USD" || desired == "USDT" || desired == "USDC") {
		rate := ExchangeRate{Value: "1.0", Timestamp: float64(time.Now().UnixMilli())}
		p.cache.set(cacheKey, rate)
		return rate, nil
	}

	return ExchangeRate{}, errors.Errorf("unable to get exchange rate for %s/%s from any source", selected, desired)
}

// stableBase strips an optional chain prefix and upper-cases a ticker so that
// "ETH_USDT" and "USDT" compare equal. Used to detect stablecoin identity.
func stableBase(ticker string) string {
	t := strings.ToUpper(ticker)
	if parts := strings.Split(t, "_"); len(parts) == 2 {
		return parts[1]
	}
	return t
}

// isNonUSDFiat reports whether desired is a supported fiat other than the
// USD-equivalent units (USD/USDT/USDC are treated as ~1 USD here).
func isNonUSDFiat(desired string) bool {
	d := strings.ToUpper(desired)
	return d != "" && d != "USD" && d != "USDT" && d != "USDC" && isFiat(d)
}

// composeStablecoinFiatRate values a non-USDT stablecoin in a non-USD fiat as
// (real stablecoin/USDT) × (USDT/fiat), so a de-peg on the specific stablecoin
// is reflected for every fiat. Returns ok=false if either leg can't be fetched.
func (p *Provider) composeStablecoinFiatRate(ctx context.Context, selected, desired string) (ExchangeRate, bool) {
	// USDT per 1 <stablecoin> — real market (e.g. USDCUSDT ≈ 0.998).
	stableToUSDT, err1 := p.GetExchangeRate(ctx, "USDT", selected)
	// <fiat> per 1 USDT — existing inverted EURUSDT/GBPUSDT/... path.
	usdtToFiat, err2 := p.GetExchangeRate(ctx, desired, "USDT")
	if err1 != nil || err2 != nil {
		return ExchangeRate{}, false
	}

	a, errA := strconv.ParseFloat(stableToUSDT.Value, 64)
	b, errB := strconv.ParseFloat(usdtToFiat.Value, 64)
	if errA != nil || errB != nil || a <= 0 || b <= 0 {
		return ExchangeRate{}, false
	}

	return ExchangeRate{
		Value:     strconv.FormatFloat(a*b, 'f', -1, 64),
		Timestamp: float64(time.Now().UnixMilli()),
	}, true
}

// getBinanceRate fetches price from Binance public ticker.
// GET /api/v3/ticker/price?symbol=ETHUSDT
//
// For pairs where Binance only lists the inverse (notably stablecoin / non-USD
// fiat like USDTEUR — which is unlisted, while EURUSDT exists), the resolver
// returns Invert=true and we 1/x the reported price so the caller always gets
// "<desired> per 1 <selected>".
func (p *Provider) getBinanceRate(ctx context.Context, selected, desired string) (ExchangeRate, error) {
	pair, ok := resolveBinanceSymbol(selected, desired)
	if !ok {
		return ExchangeRate{}, errors.Errorf("no Binance symbol mapping for %s/%s", selected, desired)
	}

	url := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", p.config.BinanceBaseURL, pair.Symbol)

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

	value := result.Price
	if pair.Invert {
		priceFloat, err := strconv.ParseFloat(result.Price, 64)
		if err != nil {
			return ExchangeRate{}, errors.Wrapf(err, "binance price parse failed for %s", pair.Symbol)
		}
		if priceFloat == 0 {
			return ExchangeRate{}, errors.Errorf("binance returned zero price for %s", pair.Symbol)
		}
		value = strconv.FormatFloat(1/priceFloat, 'f', -1, 64)
	}

	return ExchangeRate{
		Value:     value,
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

// binanceTickerAliases maps internal ticker names to their Binance symbol prefix.
// currencies.json uses "TRON" for the native coin but Binance trades it as "TRX".
var binanceTickerAliases = map[string]string{
	"TRON": "TRX",
}

// binancePair is the result of mapping a (selected, desired) pair to a
// concrete Binance symbol. When Invert is true, the rate reported by Binance
// is "selected per desired" and the caller must take 1/rate to obtain the
// requested "desired per selected" quote.
type binancePair struct {
	Symbol string
	Invert bool
}

func resolveBinanceSymbol(selected, desired string) (binancePair, bool) {
	// Resolve the base crypto ticker (e.g. "ETH_USDT" -> "USDT", "ETH" -> "ETH")
	baseTicker := selected
	if parts := strings.Split(selected, "_"); len(parts) == 2 {
		baseTicker = parts[1] // e.g. "USDT" from "ETH_USDT"
	}

	// Normalize internal ticker names to Binance symbol prefixes (e.g. "TRON" → "TRX")
	if alias, ok := binanceTickerAliases[baseTicker]; ok {
		baseTicker = alias
	}

	nonUSDFiat := desired != "" && desired != "USD" && desired != "USDT" && desired != "USDC" && isFiat(desired)

	// Stablecoin → non-USD fiat: Binance does not list USDTEUR/USDCEUR/etc.,
	// but lists EURUSDT, GBPUSDT, TRYUSDT and so on. Since USDC ≈ USDT ≈ 1 USD
	// on Binance's own books, both stablecoins use the USDT-quoted pair and we
	// invert the rate to express "fiat per stablecoin".
	if nonUSDFiat && isStablecoin(baseTicker) {
		return binancePair{Symbol: desired + "USDT", Invert: true}, true
	}

	// Crypto → non-USD fiat: direct pair (e.g. ETHEUR, BTCEUR).
	if nonUSDFiat {
		return binancePair{Symbol: baseTicker + desired}, true
	}

	// Direct USD/USDT lookup from the table.
	if sym, ok := binanceSymbols[selected]; ok {
		return binancePair{Symbol: sym}, true
	}

	// Fallback: pair the resolved base against USDT (e.g. ETH + USDT = ETHUSDT).
	if desired == "USD" || desired == "USDT" {
		return binancePair{Symbol: baseTicker + "USDT"}, true
	}

	return binancePair{}, false
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
	return money.IsFiatCurrency(strings.ToUpper(ticker))
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
