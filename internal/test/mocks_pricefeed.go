package test

import (
	"fmt"
	"sync"

	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/provider/pricefeed"
	"github.com/cryptolink/cryptolink/internal/service/blockchain"
	"github.com/rs/zerolog"
)

// PriceFeedMock is a test double for the pricefeed.Provider.
// It serves pre-configured exchange rates for use in integration tests.
type PriceFeedMock struct {
	mu       sync.Mutex
	rates    map[string]map[string]float64
	Provider *pricefeed.Provider
}

func NewPriceFeedMock(logger *zerolog.Logger) *PriceFeedMock {
	cfg := pricefeed.Config{
		CacheTTLSeconds: 0, // no caching in tests
	}

	return &PriceFeedMock{
		mu:       sync.Mutex{},
		rates:    map[string]map[string]float64{},
		Provider: pricefeed.New(cfg, logger),
	}
}

// SetupRates configures a mock exchange rate. This matches the old TatumMock.SetupRates signature.
func (m *PriceFeedMock) SetupRates(from string, to money.FiatCurrency, rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	from = blockchain.NormalizeTicker(from)

	if m.rates[from] == nil {
		m.rates[from] = make(map[string]float64)
	}

	m.rates[from][string(to)] = rate
}

// GetRate returns the configured rate, or 0 if not set.
func (m *PriceFeedMock) GetRate(from string, to string) (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	from = blockchain.NormalizeTicker(from)
	rates, ok := m.rates[from]
	if !ok {
		return 0, false
	}
	rate, ok := rates[to]
	return rate, ok
}

func (m *PriceFeedMock) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rates = map[string]map[string]float64{}
}

func (m *PriceFeedMock) String() string {
	return fmt.Sprintf("PriceFeedMock{rates: %v}", m.rates)
}
