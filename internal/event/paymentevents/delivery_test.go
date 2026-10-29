package paymentevents_test

import (
	"testing"
	"time"

	"github.com/cryptolink/cryptolink/internal/event/paymentevents"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/stretchr/testify/assert"
)

func TestNextWebhookAttempt(t *testing.T) {
	now := time.Date(2026, 10, 28, 10, 45, 59, 0, time.UTC)

	t.Run("backs off after each failed attempt", func(t *testing.T) {
		var prev time.Duration
		for attempt := 1; attempt < paymentevents.WebhookMaxAttempts; attempt++ {
			next, ok := paymentevents.NextWebhookAttempt(attempt, now)
			assert.True(t, ok, "attempt %d", attempt)

			delay := next.Sub(now)
			assert.GreaterOrEqual(t, delay, prev, "attempt %d", attempt)
			prev = delay
		}
	})

	t.Run("first retry is quick", func(t *testing.T) {
		next, ok := paymentevents.NextWebhookAttempt(1, now)
		assert.True(t, ok)
		assert.Equal(t, time.Minute, next.Sub(now))
	})

	t.Run("gives up at max attempts", func(t *testing.T) {
		_, ok := paymentevents.NextWebhookAttempt(paymentevents.WebhookMaxAttempts, now)
		assert.False(t, ok)

		_, ok = paymentevents.NextWebhookAttempt(0, now)
		assert.False(t, ok)
	})

	t.Run("keeps retrying for days before giving up", func(t *testing.T) {
		var total time.Duration
		for attempt := 1; attempt < paymentevents.WebhookMaxAttempts; attempt++ {
			next, _ := paymentevents.NextWebhookAttempt(attempt, now)
			total += next.Sub(now)
		}

		assert.Greater(t, total, 48*time.Hour)
	})
}

func TestWebhookDeliveryKey(t *testing.T) {
	assert.Equal(t, "success", paymentevents.WebhookDeliveryKey(payment.StatusSuccess, ""))

	// a stray fill key must not make a success webhook look new
	assert.Equal(t, "success", paymentevents.WebhookDeliveryKey(payment.StatusSuccess, "1:0xabc:3"))

	// each partial fill is its own delivery
	assert.Equal(t, "partial:1:0xabc:3", paymentevents.WebhookDeliveryKey(payment.StatusPartial, "1:0xabc:3"))
	assert.NotEqual(t,
		paymentevents.WebhookDeliveryKey(payment.StatusPartial, "1:0xabc:3"),
		paymentevents.WebhookDeliveryKey(payment.StatusPartial, "1:0xdef:0"),
	)
	assert.Equal(t, "partial", paymentevents.WebhookDeliveryKey(payment.StatusPartial, ""))
}
