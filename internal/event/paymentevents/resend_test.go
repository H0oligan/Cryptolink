package paymentevents_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cryptolink/cryptolink/internal/bus"
	"github.com/cryptolink/cryptolink/internal/db/repository"
	"github.com/cryptolink/cryptolink/internal/event/paymentevents"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/service/merchant"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/subscription"
	"github.com/cryptolink/cryptolink/internal/test"
	"github.com/cryptolink/cryptolink/internal/webhook"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func settledPayment(t *testing.T, tc *test.IntegrationTest, merchantID int64, status payment.Status) *payment.Payment {
	t.Helper()

	p, err := tc.Services.Payment.CreatePayment(tc.Context, merchantID, payment.CreatePaymentProps{
		MerchantOrderUUID: uuid.New(),
		Money:             lo.Must(money.USD.MakeAmount("5000")),
	})
	require.NoError(t, err)

	_, err = tc.Repository.UpdatePayment(tc.Context, repository.UpdatePaymentParams{
		ID:         p.ID,
		MerchantID: merchantID,
		Status:     status.String(),
		UpdatedAt:  time.Now(),
	})
	require.NoError(t, err)

	return p
}

func TestResendWebhook(t *testing.T) {
	defer webhook.AllowPrivateDestinationsForTesting()()

	tc, handler, _ := setup(t)

	var received atomic.Int32
	var respond atomic.Int32
	respond.Store(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(int(respond.Load()))
	}))
	defer srv.Close()

	mt, err := tc.Services.Merchants.Create(tc.Context, 1, "site", "site.com", merchant.Settings{
		merchant.PropertySignatureSecret: "abc",
		merchant.PropertyWebhookURL:      srv.URL,
	})
	require.NoError(t, err)

	state := func(p *payment.Payment) *payment.WebhookState {
		s, err := tc.Services.Payment.GetWebhookState(tc.Context, p.PublicID)
		require.NoError(t, err)
		return s
	}

	t.Run("a delivered webhook is sent exactly once more", func(t *testing.T) {
		received.Store(0)
		p := settledPayment(t, tc, mt.ID, payment.StatusSuccess)

		msg := marshal(bus.PaymentStatusUpdateEvent{MerchantID: mt.ID, PaymentID: p.ID})
		require.NoError(t, handler.ProcessPaymentStatusUpdate(tc.Context, msg))
		require.NoError(t, handler.ProcessPaymentStatusUpdate(tc.Context, msg))
		require.EqualValues(t, 1, received.Load(), "normal delivery stays exactly-once")
		require.True(t, state(p).Delivered())

		require.NoError(t, handler.ResendWebhook(tc.Context, mt.ID, p.ID))
		assert.EqualValues(t, 2, received.Load())
		assert.True(t, state(p).Delivered())

		require.NoError(t, handler.ProcessPaymentStatusUpdate(tc.Context, msg))
		assert.EqualValues(t, 2, received.Load(), "resend must not reopen normal delivery")
	})

	t.Run("a failed resend stays undelivered and scheduled to retry", func(t *testing.T) {
		received.Store(0)
		respond.Store(http.StatusInternalServerError)
		defer respond.Store(http.StatusOK)
		p := settledPayment(t, tc, mt.ID, payment.StatusSuccess)

		require.NoError(t, handler.ResendWebhook(tc.Context, mt.ID, p.ID))
		s := state(p)
		assert.False(t, s.Delivered())
		assert.Equal(t, 1, s.Attempts)
		assert.NotNil(t, s.NextAttemptAt)

		listed, err := tc.Services.Payment.ListAwaitingWebhook(tc.Context, 50)
		require.NoError(t, err)
		assert.Contains(t, lo.Map(listed, func(w *payment.WebhookState, _ int) int64 { return w.ID }), p.ID)
	})

	t.Run("an unsettled payment has nothing to resend", func(t *testing.T) {
		received.Store(0)
		p := settledPayment(t, tc, mt.ID, payment.StatusPending)

		err := handler.ResendWebhook(tc.Context, mt.ID, p.ID)
		assert.ErrorIs(t, err, payment.ErrWebhookNotResendable)
		assert.Zero(t, received.Load())
	})

	t.Run("a merchant with no webhook URL is refused", func(t *testing.T) {
		noURL, err := tc.Services.Merchants.Create(tc.Context, 1, "no-url", "no-url.com", nil)
		require.NoError(t, err)
		p := settledPayment(t, tc, noURL.ID, payment.StatusSuccess)

		assert.ErrorIs(t, handler.ResendWebhook(tc.Context, noURL.ID, p.ID), paymentevents.ErrWebhookURLNotSet)
	})
}

// The admin merchant that collects subscription payments has no webhook URL, so a paid
// subscription used to stay pending for ever.
func TestPaidSubscriptionActivatesWithoutWebhookURL(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	logger := zerolog.Nop()
	subs := subscription.New(tc.Database.Conn().Pool, &logger)
	handler := paymentevents.New(tc.Services.Merchants, tc.Services.Processing, tc.Services.Payment, subs, "", tc.Logger)

	adminMerchant, err := tc.Services.Merchants.Create(tc.Context, 1, "admin", "admin.com", nil)
	require.NoError(t, err)
	buyer, err := tc.Services.Merchants.Create(tc.Context, 1, "buyer", "buyer.com", nil)
	require.NoError(t, err)

	_, err = tc.Database.Conn().Pool.Exec(tc.Context, `
		INSERT INTO subscription_plans (id, name, description, price_usd, billing_period, max_merchants, features, is_active)
		VALUES ('resend-test', 'Test', 'Test', 29, 'monthly', 1, '{}', true)`)
	require.NoError(t, err)

	var subID int64
	var subUUID uuid.UUID
	require.NoError(t, tc.Database.Conn().Pool.QueryRow(tc.Context, `
		INSERT INTO merchant_subscriptions (merchant_id, plan_id, status, current_period_start, current_period_end)
		VALUES ($1, 'resend-test', $2, now(), now() + interval '1 month')
		RETURNING id, uuid`, buyer.ID, subscription.StatusPendingPayment).Scan(&subID, &subUUID))

	p := settledPayment(t, tc, adminMerchant.ID, payment.StatusSuccess)
	meta, err := json.Marshal(map[string]any{"subscription_id": subUUID.String()})
	require.NoError(t, err)
	_, err = tc.Database.Conn().Pool.Exec(tc.Context, `UPDATE payments SET metadata = $1 WHERE id = $2`, meta, p.ID)
	require.NoError(t, err)

	msg := marshal(bus.PaymentStatusUpdateEvent{MerchantID: adminMerchant.ID, PaymentID: p.ID})
	require.NoError(t, handler.ProcessPaymentStatusUpdate(tc.Context, msg))

	var status string
	var paymentID *int64
	require.NoError(t, tc.Database.Conn().Pool.QueryRow(tc.Context,
		`SELECT status, payment_id FROM merchant_subscriptions WHERE id = $1`, subID).Scan(&status, &paymentID))
	assert.Equal(t, subscription.StatusActive, status)
	require.NotNil(t, paymentID)
	assert.Equal(t, p.ID, *paymentID)
}
