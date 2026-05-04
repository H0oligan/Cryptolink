package watcher

import (
	"math/big"
	"testing"

	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
)

// makeTx is a tiny helper that builds a transaction record carrying just an
// amount in BTC satoshis — enough surface for bestMatchByAmount.
func makeTx(t *testing.T, sat int64) *transaction.Transaction {
	t.Helper()
	currency := money.CryptoCurrency{
		Ticker:     "BTC",
		Blockchain: money.Blockchain("BTC"),
		Type:       money.Coin,
		Decimals:   8,
	}
	amount, err := money.NewFromBigInt(money.Crypto, currency.Ticker, big.NewInt(sat), currency.Decimals)
	if err != nil {
		t.Fatalf("NewFromBigInt: %v", err)
	}
	return &transaction.Transaction{
		ID:       int64(sat), // unique-ish id
		Amount:   amount,
		Currency: currency,
	}
}

// TestBestMatch_ExactBeatsUnderpayment verifies the existing rule that an
// invoice receiving its exact expected wins over any concurrent invoice that
// happens to be a numerical-distance match for an underpayment. Regression
// guard for the 50000 bp underpayment penalty in bestMatchByAmount.
func TestBestMatch_ExactBeatsUnderpayment(t *testing.T) {
	pending := []pendingInfo{
		{tx: makeTx(t, 100_000)}, // invoice A: 100k
		{tx: makeTx(t, 80_000)},  // invoice B: 80k
	}
	// On-chain transfer of 80k could be EITHER an exact match for B or a 20%
	// underpayment for A. Must pick B.
	idx, _, ok := bestMatchByAmount(pending, big.NewInt(80_000))
	if !ok {
		t.Fatal("expected ok=true for exact match")
	}
	if idx != 1 {
		t.Fatalf("expected exact-match invoice B (idx=1), got idx=%d", idx)
	}
}

// TestBestMatch_PartialUsesRemaining verifies that an invoice already half-
// paid via the partial-fill flow is matched against its REMAINING amount,
// not its original expected. Without this, a 0.3 BTC top-up to a partially
// paid 1 BTC invoice (0.3 BTC remaining) would look like a 70% underpayment
// and would lose to any concurrent invoice asking for ~0.3 BTC.
func TestBestMatch_PartialUsesRemaining(t *testing.T) {
	partial := pendingInfo{
		tx:        makeTx(t, 100_000_000), // 1 BTC original
		remaining: big.NewInt(30_000_000), // 0.3 BTC remaining after partial fill
	}
	fresh := pendingInfo{
		tx: makeTx(t, 30_000_000), // a fresh 0.3 BTC invoice
	}
	pending := []pendingInfo{partial, fresh}

	// On-chain transfer of 30M sats matches BOTH effective expecteds exactly.
	// Tie-break: bestMatchByAmount picks the first-seen with same score, so
	// the partial invoice wins. The critical assertion is that the partial
	// invoice IS considered competitive (not heavily-penalized as a 70%
	// underpayment of its original 1 BTC amount).
	idx, info, ok := bestMatchByAmount(pending, big.NewInt(30_000_000))
	if !ok {
		t.Fatal("expected ok=true")
	}
	if idx != 0 {
		t.Fatalf("expected partial invoice (idx=0) to win, got idx=%d", idx)
	}
	if info.effectiveExpected().Cmp(big.NewInt(30_000_000)) != 0 {
		t.Fatalf("effectiveExpected should equal remaining for partial; got %s", info.effectiveExpected())
	}
}

// TestBestMatch_DustRejected confirms that a transfer below 20% of the
// effective expected is rejected as dust-spam. Using `remaining` (not
// original) means the dust floor scales with what's actually outstanding.
func TestBestMatch_DustRejected(t *testing.T) {
	pending := []pendingInfo{
		{tx: makeTx(t, 100_000)}, // expected 100k, dust threshold = 20k
	}
	_, _, ok := bestMatchByAmount(pending, big.NewInt(10_000)) // 10k < 20k floor
	if ok {
		t.Fatal("expected ok=false for dust transfer below 20% floor")
	}
}

// TestBestMatch_DustFloorScalesWithRemaining is the partial-fill version of
// the dust-rejection test: a 5k transfer against a 100k invoice with 20k
// remaining is still 25% of remaining, so it should be ACCEPTED rather
// than scored against the original 100k (which would make it look like 5%
// dust). Protects partial-fill top-ups from being mistaken for spam.
func TestBestMatch_DustFloorScalesWithRemaining(t *testing.T) {
	pending := []pendingInfo{
		{
			tx:        makeTx(t, 100_000),
			remaining: big.NewInt(20_000), // 80k already received
		},
	}
	_, _, ok := bestMatchByAmount(pending, big.NewInt(5_000)) // 25% of remaining
	if !ok {
		t.Fatal("expected ok=true: 5k is 25% of remaining 20k, well above 20% floor")
	}
}
