package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cryptolink/cryptolink/internal/service/subscription"
	"github.com/cryptolink/cryptolink/internal/test"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Plan prices and usage volumes are fractional numerics read and written through
// raw SQL, so they depend on how the driver maps decimal.Decimal.
func TestDecimalColumnsThroughDriver(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	ctx := context.Background()
	logger := zerolog.Nop()
	service := subscription.New(tc.Database.Conn().Pool, &logger)

	dec := func(v string) decimal.Decimal { return decimal.RequireFromString(v) }

	created, err := service.CreatePlan(ctx, &subscription.SubscriptionPlan{
		ID:                  "pgx-test",
		Name:                "Driver test",
		Description:         "Driver test",
		PriceUSD:            dec("29.99"),
		BillingPeriod:       "monthly",
		MaxPaymentsMonthly:  sql.NullInt32{Int32: 10, Valid: true},
		MaxMerchants:        1,
		MaxVolumeMonthlyUSD: decimal.NullDecimal{Decimal: dec("1.00"), Valid: true},
		Features:            json.RawMessage(`{"support": "email"}`),
		IsActive:            true,
	})
	require.NoError(t, err)
	assert.True(t, dec("29.99").Equal(created.PriceUSD), created.PriceUSD.String())
	assert.True(t, created.MaxVolumeMonthlyUSD.Valid)
	assert.True(t, dec("1").Equal(created.MaxVolumeMonthlyUSD.Decimal))
	assert.Equal(t, sql.NullInt32{Int32: 10, Valid: true}, created.MaxPaymentsMonthly)
	assert.False(t, created.MaxAPICallsMonthly.Valid)
	assert.JSONEq(t, `{"support": "email"}`, string(created.Features))

	u, _ := tc.Must.CreateUserViaEmail(t, "pgx-test@example.com", "qwerty123")
	mt, _ := tc.Must.CreateMerchant(t, u.ID)
	require.NoError(t, service.AssignMerchantPlan(ctx, mt.ID, created.ID))

	sub, err := service.GetActiveSubscription(ctx, mt.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, sub.PlanID)
	assert.NotEmpty(t, sub.UUID.String())
	assert.False(t, sub.CancelledAt.Valid)
	require.NotNil(t, sub.Plan)
	assert.True(t, dec("29.99").Equal(sub.Plan.PriceUSD))

	// 0.10 three times must be exactly 0.30, with no float drift.
	_, err = service.GetCurrentUsage(ctx, mt.ID)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		require.NoError(t, service.IncrementPaymentUsage(ctx, mt.ID, dec("0.10")))
	}

	usage, err := service.GetCurrentUsage(ctx, mt.ID)
	require.NoError(t, err)
	assert.True(t, dec("0.30").Equal(usage.PaymentVolumeUSD), usage.PaymentVolumeUSD.String())
	assert.Equal(t, int32(3), usage.PaymentCount)

	pct, err := service.GetVolumePercentage(ctx, mt.ID)
	require.NoError(t, err)
	assert.InDelta(t, 30.0, pct, 0.0001)

	// The volume limit is the billing gate, so test both sides of the boundary.
	assert.NoError(t, service.CheckVolumeLimit(ctx, mt.ID, dec("0.70")))
	assert.True(t, errors.Is(service.CheckVolumeLimit(ctx, mt.ID, dec("0.71")), subscription.ErrLimitExceeded))

	// A NULL volume limit means unlimited.
	created.MaxVolumeMonthlyUSD = decimal.NullDecimal{}
	updated, err := service.UpdatePlan(ctx, created.ID, created)
	require.NoError(t, err)
	assert.False(t, updated.MaxVolumeMonthlyUSD.Valid)
	assert.NoError(t, service.CheckVolumeLimit(ctx, mt.ID, dec("100000")))
}
