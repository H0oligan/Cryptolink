package processing

import (
	"context"
	"testing"
	"time"

	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/provider/bitcoin"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

type fakeBTCInspector struct {
	info  *bitcoin.AddressInfo
	err   error
	calls int
}

func (f *fakeBTCInspector) GetAddressInfo(_ context.Context, _ string, _ bool) (*bitcoin.AddressInfo, error) {
	f.calls++
	return f.info, f.err
}

func TestService_btcPaymentInMempool(t *testing.T) {
	logger := zerolog.Nop()

	ago := func(d time.Duration) *time.Time {
		at := time.Now().Add(-d)
		return &at
	}

	btcTx := func() *transaction.Transaction {
		return &transaction.Transaction{
			RecipientAddress: "bc1qhwh2axexux3c5d8puajhf0d9zrr4eexlprj0l5",
			Currency:         money.CryptoCurrency{Blockchain: "BTC", Ticker: "BTC"},
		}
	}

	for _, tc := range []struct {
		name      string
		inspector *fakeBTCInspector
		expiresAt *time.Time
		tx        *transaction.Transaction
		want      bool
		wantCalls int
	}{
		{
			name:      "unconfirmed funds in mempool defer expiry",
			inspector: &fakeBTCInspector{info: &bitcoin.AddressInfo{MempoolFunded: 73023}},
			expiresAt: ago(31 * time.Minute),
			tx:        btcTx(),
			want:      true,
			wantCalls: 1,
		},
		{
			name:      "empty mempool lets payment expire",
			inspector: &fakeBTCInspector{info: &bitcoin.AddressInfo{}},
			expiresAt: ago(31 * time.Minute),
			tx:        btcTx(),
			want:      false,
			wantCalls: 1,
		},
		{
			name:      "provider error defers expiry",
			inspector: &fakeBTCInspector{err: errors.New("boom")},
			expiresAt: ago(31 * time.Minute),
			tx:        btcTx(),
			want:      true,
			wantCalls: 1,
		},
		{
			name:      "hard cap reached — expire even if still in mempool",
			inspector: &fakeBTCInspector{info: &bitcoin.AddressInfo{MempoolFunded: 73023}},
			expiresAt: ago(25 * time.Hour),
			tx:        btcTx(),
			want:      false,
		},
		{
			name:      "non-BTC chain is never checked",
			inspector: &fakeBTCInspector{info: &bitcoin.AddressInfo{MempoolFunded: 1}},
			expiresAt: ago(31 * time.Minute),
			tx: &transaction.Transaction{
				RecipientAddress: "0xabc",
				Currency:         money.CryptoCurrency{Blockchain: "ETH", Ticker: "ETH"},
			},
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{btc: tc.inspector, logger: &logger}
			pt := &payment.Payment{ID: 2077, ExpiresAt: tc.expiresAt}

			assert.Equal(t, tc.want, s.btcPaymentInMempool(context.Background(), pt, tc.tx))
			assert.Equal(t, tc.wantCalls, tc.inspector.calls)
		})
	}

	t.Run("nil inspector is a no-op", func(t *testing.T) {
		s := &Service{logger: &logger}
		pt := &payment.Payment{ID: 1, ExpiresAt: ago(31 * time.Minute)}
		assert.False(t, s.btcPaymentInMempool(context.Background(), pt, btcTx()))
	})
}
