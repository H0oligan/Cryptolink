package pricefeed

import "testing"

func TestStableBase(t *testing.T) {
	cases := map[string]string{
		"USDT":      "USDT",
		"usdc":      "USDC",
		"ETH_USDT":  "USDT",
		"MATIC_USDC": "USDC",
		"ETH":       "ETH",
	}
	for in, want := range cases {
		if got := stableBase(in); got != want {
			t.Errorf("stableBase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStablecoinIdentity(t *testing.T) {
	// Same stablecoin (incl. chain-prefixed) must be treated as 1:1...
	identity := [][2]string{{"USDT", "USDT"}, {"USDC", "USDC"}, {"ETH_USDT", "USDT"}}
	for _, p := range identity {
		if !(isStablecoin(p[0]) && stableBase(p[0]) == stableBase(p[1])) {
			t.Errorf("expected identity for %v", p)
		}
	}
	// ...but different stablecoins must NOT be identity (so a de-peg is fetched).
	nonIdentity := [][2]string{{"USDC", "USDT"}, {"USDC", "USD"}, {"USDT", "USD"}}
	for _, p := range nonIdentity {
		if isStablecoin(p[0]) && stableBase(p[0]) == stableBase(p[1]) {
			t.Errorf("expected NON-identity (real rate fetch) for %v", p)
		}
	}
}

func TestIsNonUSDFiat(t *testing.T) {
	for _, f := range []string{"EUR", "GBP", "JPY", "TRY", "BRL"} {
		if !isNonUSDFiat(f) {
			t.Errorf("isNonUSDFiat(%q) = false, want true", f)
		}
	}
	for _, f := range []string{"USD", "USDT", "USDC", "ETH", ""} {
		if isNonUSDFiat(f) {
			t.Errorf("isNonUSDFiat(%q) = true, want false", f)
		}
	}
}
