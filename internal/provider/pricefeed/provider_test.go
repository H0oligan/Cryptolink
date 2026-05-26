package pricefeed

import "testing"

func TestResolveBinanceSymbol(t *testing.T) {
	tests := []struct {
		name       string
		selected   string
		desired    string
		wantSymbol string
		wantInvert bool
		wantOK     bool
	}{
		// Stablecoin → non-USD fiat: Binance does not list USDT<FIAT> / USDC<FIAT>,
		// so we route through the inverted FIAT-USDT pair. Both stablecoins use
		// USDT as the proxy because Binance treats USDC ≈ USDT ≈ 1 USD on-book.
		{"USDT to EUR uses inverted EURUSDT", "USDT", "EUR", "EURUSDT", true, true},
		{"USDC to EUR uses inverted EURUSDT", "USDC", "EUR", "EURUSDT", true, true},
		{"USDT to GBP uses inverted GBPUSDT", "USDT", "GBP", "GBPUSDT", true, true},
		{"USDC to TRY uses inverted TRYUSDT", "USDC", "TRY", "TRYUSDT", true, true},

		// Chain-prefixed stablecoin tickers normalize through the same inverted path.
		{"ETH_USDT to EUR uses inverted EURUSDT", "ETH_USDT", "EUR", "EURUSDT", true, true},
		{"TRON_USDT to JPY uses inverted JPYUSDT", "TRON_USDT", "JPY", "JPYUSDT", true, true},
		{"BSC_USDC to BRL uses inverted BRLUSDT", "BSC_USDC", "BRL", "BRLUSDT", true, true},

		// Crypto → non-USD fiat: direct pair, no inversion.
		{"ETH to EUR direct ETHEUR", "ETH", "EUR", "ETHEUR", false, true},
		{"BTC to GBP direct BTCGBP", "BTC", "GBP", "BTCGBP", false, true},

		// TRON ticker alias must be applied before pairing.
		{"TRON to EUR uses TRX alias", "TRON", "EUR", "TRXEUR", false, true},

		// USD/USDT/USDC desired: existing behavior preserved via lookup table.
		{"ETH to USD via table", "ETH", "USD", "ETHUSDT", false, true},
		{"BTC to USDT via table", "BTC", "USDT", "BTCUSDT", false, true},
		{"ETH_USDT to USD via table", "ETH_USDT", "USD", "USDTUSD", false, true},

		// Unmapped + non-fiat desired returns ok=false.
		{"unknown ticker to non-USD non-fiat", "XYZ", "DOGE", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveBinanceSymbol(tt.selected, tt.desired)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got.Symbol != tt.wantSymbol {
				t.Errorf("Symbol = %q, want %q", got.Symbol, tt.wantSymbol)
			}
			if got.Invert != tt.wantInvert {
				t.Errorf("Invert = %v, want %v", got.Invert, tt.wantInvert)
			}
		})
	}
}
