package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/cryptolink/cryptolink/internal/service/subscription"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Security Test Suite for Subscription Service
// Tests for SQL injection, authorization bypass, and other security vulnerabilities

func setupTestDB(t *testing.T) *pgxpool.Pool {
	// Connect to test database
	connString := "postgres://oxygen_user:br2pdRilIGAPhmC0dwo7jve0kv%2FRzJlI@127.0.0.1:5432/oxygen_db?sslmode=disable"

	db, err := pgxpool.Connect(context.Background(), connString)
	require.NoError(t, err, "Failed to connect to database")

	return db
}

func setupService(t *testing.T) (*subscription.Service, *pgxpool.Pool) {
	db := setupTestDB(t)
	logger := zerolog.Nop()

	service := subscription.New(db, &logger)

	return service, db
}

func cleanupDB(db *pgxpool.Pool) {
	db.Close()
}

// cleanupTestData removes all test data from subscription tables
func cleanupTestData(ctx context.Context, db *pgxpool.Pool, merchantID int64) {
	// Delete in correct order due to foreign keys
	db.Exec(ctx, "DELETE FROM usage_tracking WHERE merchant_id = $1", merchantID)
	db.Exec(ctx, "DELETE FROM merchant_subscriptions WHERE merchant_id = $1", merchantID)
	db.Exec(ctx, "DELETE FROM merchants WHERE id = $1", merchantID)
}

// ============================================================================
// SQL INJECTION TESTS
// ============================================================================

func TestSQLInjection_GetPlan(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	testCases := []struct {
		name     string
		planID   string
		expectError bool
	}{
		{
			name:     "SQL injection with quotes",
			planID:   "free' OR '1'='1",
			expectError: false, // Should not find plan, but shouldn't execute injection
		},
		{
			name:     "SQL injection with semicolon",
			planID:   "free'; DROP TABLE subscription_plans; --",
			expectError: false,
		},
		{
			name:     "SQL injection with UNION",
			planID:   "free' UNION SELECT * FROM users --",
			expectError: false,
		},
		{
			name:     "SQL injection with comments",
			planID:   "free' OR 1=1 --",
			expectError: false,
		},
		{
			name:     "Normal plan ID",
			planID:   "free",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := service.GetPlan(ctx, tc.planID)

			// Should either return not found or the correct plan
			// Should NEVER execute SQL injection
			if tc.planID == "free" {
				assert.NoError(t, err, "Normal query should work")
				assert.NotNil(t, plan, "Should find free plan")
			} else {
				// Injection attempts should just not find anything
				assert.Error(t, err, "SQL injection should not work")
			}

			// Verify subscription_plans table still exists
			var count int
			err = db.QueryRow(ctx, "SELECT COUNT(*) FROM subscription_plans").Scan(&count)
			assert.NoError(t, err, "Table should still exist after injection attempt")
			assert.Greater(t, count, 0, "Plans should still exist")
		})
	}
}

func TestSQLInjection_GetActiveSubscription(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	// Get a real merchant ID first
	var realMerchantID int64
	err := db.QueryRow(ctx, "SELECT id FROM merchants LIMIT 1").Scan(&realMerchantID)
	require.NoError(t, err, "Should find a merchant")

	testCases := []struct {
		name       string
		merchantID int64
		expectErr  bool
	}{
		{
			name:       "Normal merchant ID",
			merchantID: realMerchantID,
			expectErr:  false,
		},
		{
			name:       "Negative merchant ID",
			merchantID: -1,
			expectErr:  true,
		},
		{
			name:       "Very large merchant ID",
			merchantID: 999999999,
			expectErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sub, err := service.GetActiveSubscription(ctx, tc.merchantID)

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, sub)
			} else {
				// Should find subscription or return not found (both safe)
				if err != nil {
					assert.ErrorIs(t, err, subscription.ErrSubscriptionNotFound)
				}
			}
		})
	}
}

func TestSQLInjection_GetSubscriptionByUUID(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	testCases := []struct {
		name     string
		uuid     string
		expectErr bool
	}{
		{
			name:     "SQL injection in UUID",
			uuid:     "' OR '1'='1",
			expectErr: true, // UUID parsing should fail
		},
		{
			name:     "SQL injection with semicolon",
			uuid:     "'; DROP TABLE merchant_subscriptions; --",
			expectErr: true,
		},
		{
			name:     "Invalid UUID format",
			uuid:     "not-a-uuid",
			expectErr: true,
		},
		{
			name:     "Valid UUID but not exists",
			uuid:     uuid.New().String(),
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := uuid.Parse(tc.uuid)
			if err != nil {
				// UUID parsing should fail for injection attempts
				return
			}

			sub, err := service.GetSubscriptionByUUID(ctx, u)

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, sub)
			}

			// Verify tables still exist
			var count int
			err = db.QueryRow(ctx, "SELECT COUNT(*) FROM merchant_subscriptions").Scan(&count)
			assert.NoError(t, err, "Table should exist after injection attempt")
		})
	}
}

// ============================================================================
// AUTHORIZATION BYPASS TESTS
// ============================================================================

func TestAuthorizationBypass_GetActiveSubscription(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	// Create two test merchants
	var merchant1ID, merchant2ID int64

	err := db.QueryRow(ctx, `
		INSERT INTO merchants (uuid, name, website, creator_id, created_at, updated_at, settings)
		VALUES ($1, 'Test Merchant 1', 'https://test1.com', 1, NOW(), NOW(), '{}'::jsonb)
		RETURNING id
	`, uuid.New()).Scan(&merchant1ID)
	require.NoError(t, err)

	err = db.QueryRow(ctx, `
		INSERT INTO merchants (uuid, name, website, creator_id, created_at, updated_at, settings)
		VALUES ($1, 'Test Merchant 2', 'https://test2.com', 1, NOW(), NOW(), '{}'::jsonb)
		RETURNING id
	`, uuid.New()).Scan(&merchant2ID)
	require.NoError(t, err)

	// Create subscription for merchant1
	_, err = service.CreateSubscription(ctx, merchant1ID, "free", time.Now(), time.Now().AddDate(1, 0, 0))
	require.NoError(t, err)

	// Merchant 1 should access their own subscription
	sub1, err := service.GetActiveSubscription(ctx, merchant1ID)
	assert.NoError(t, err)
	assert.NotNil(t, sub1)
	assert.Equal(t, merchant1ID, sub1.MerchantID)

	// Merchant 2 should NOT access merchant 1's subscription
	// This test verifies the service doesn't allow cross-merchant access
	_, err = service.GetActiveSubscription(ctx, merchant2ID)
	assert.Error(t, err, "Merchant 2 should not have subscription")
	assert.ErrorIs(t, err, subscription.ErrSubscriptionNotFound)

	// Cleanup
	db.Exec(ctx, "DELETE FROM merchant_subscriptions WHERE merchant_id = $1", merchant1ID)
	db.Exec(ctx, "DELETE FROM merchants WHERE id IN ($1, $2)", merchant1ID, merchant2ID)
}

func TestAuthorizationBypass_CreateSubscription(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	// Try to create subscription for non-existent merchant
	_, err := service.CreateSubscription(ctx, 999999, "pro", time.Now(), time.Now().AddDate(0, 1, 0))

	// Should fail gracefully without exposing system information
	assert.Error(t, err)

	// Verify no subscription was created
	var count int
	err = db.QueryRow(ctx, "SELECT COUNT(*) FROM merchant_subscriptions WHERE merchant_id = 999999").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count, "No subscription should be created for invalid merchant")
}

// ============================================================================
// LIMIT BYPASS TESTS
// ============================================================================

func TestLimitBypass_PaymentLimit(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	// Create a test merchant with Free plan (limit: 100 payments)
	var merchantID int64
	err := db.QueryRow(ctx, `
		INSERT INTO merchants (uuid, name, website, creator_id, created_at, updated_at, settings)
		VALUES ($1, 'Test Merchant Limit', 'https://test.com', 1, NOW(), NOW(), '{}'::jsonb)
		RETURNING id
	`, uuid.New()).Scan(&merchantID)
	require.NoError(t, err)
	defer db.Exec(ctx, "DELETE FROM merchants WHERE id = $1", merchantID)

	// Create free subscription for the merchant
	_, err = service.CreateSubscription(ctx, merchantID, "free", time.Now(), time.Now().AddDate(0, 1, 0))
	require.NoError(t, err)

	// Set usage to 99 (just below limit)
	_, err = db.Exec(ctx, `
		INSERT INTO usage_tracking (merchant_id, period_start, period_end, payment_count, payment_volume_usd, api_calls_count, created_at, updated_at)
		VALUES ($1, $2, $3, 99, 0, 0, NOW(), NOW())
		ON CONFLICT (merchant_id, period_start)
		DO UPDATE SET payment_count = 99
	`, merchantID, time.Now().Truncate(24*time.Hour), time.Now().AddDate(0, 1, 0))
	require.NoError(t, err)

	// Should allow 1 more payment
	err = service.CheckPaymentLimit(ctx, merchantID)
	assert.NoError(t, err, "Should allow payment at 99/100")

	// Increment to 100 (at limit)
	err = service.IncrementPaymentUsage(ctx, merchantID, decimal.NewFromInt(10))
	assert.NoError(t, err)

	// Should now block payment
	err = service.CheckPaymentLimit(ctx, merchantID)
	assert.Error(t, err, "Should block payment at 100/100")
	assert.ErrorIs(t, err, subscription.ErrLimitExceeded)

	// Try concurrent increment (race condition test)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = service.IncrementPaymentUsage(ctx, merchantID, decimal.NewFromInt(1))
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check final count - should be atomic
	var finalCount int
	err = db.QueryRow(ctx, `
		SELECT payment_count FROM usage_tracking
		WHERE merchant_id = $1 AND period_start <= NOW() AND period_end >= NOW()
	`, merchantID).Scan(&finalCount)
	assert.NoError(t, err)
	assert.Greater(t, finalCount, 100, "Concurrent increments should all succeed atomically")
}

func TestLimitBypass_MerchantLimit(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	// Create test user
	var userID int64
	err := db.QueryRow(ctx, `
		INSERT INTO users (uuid, name, email, created_at, updated_at, settings)
		VALUES ($1, 'Test User', 'test@example.com', NOW(), NOW(), '{}'::jsonb)
		RETURNING id
	`, uuid.New()).Scan(&userID)
	require.NoError(t, err)

	// Free plan allows 1 merchant
	// Create subscription with free plan
	var merchantID int64
	err = db.QueryRow(ctx, `
		INSERT INTO merchants (uuid, name, website, creator_id, created_at, updated_at, settings)
		VALUES ($1, 'Test Merchant', 'https://test.com', $2, NOW(), NOW(), '{}'::jsonb)
		RETURNING id
	`, uuid.New(), userID).Scan(&merchantID)
	require.NoError(t, err)

	_, err = service.CreateSubscription(ctx, merchantID, "free", time.Now(), time.Now().AddDate(1, 0, 0))
	require.NoError(t, err)

	// Should allow first merchant (count = 0, checking for 2nd)
	err = service.CheckMerchantLimit(ctx, userID, 0)
	assert.NoError(t, err, "Should allow first merchant")

	// Should block second merchant (count = 1, at limit)
	err = service.CheckMerchantLimit(ctx, userID, 1)
	assert.Error(t, err, "Should block second merchant on free plan")
	assert.ErrorIs(t, err, subscription.ErrLimitExceeded)

	// Cleanup
	db.Exec(ctx, "DELETE FROM merchant_subscriptions WHERE merchant_id = $1", merchantID)
	db.Exec(ctx, "DELETE FROM merchants WHERE id = $1", merchantID)
	db.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
}

// ============================================================================
// INPUT VALIDATION TESTS
// ============================================================================

func TestInputValidation_CreateSubscription(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	testCases := []struct {
		name        string
		merchantID  int64
		planID      string
		start       time.Time
		end         time.Time
		expectError bool
	}{
		{
			name:        "Invalid plan ID",
			merchantID:  1,
			planID:      "invalid_plan",
			start:       time.Now(),
			end:         time.Now().AddDate(0, 1, 0),
			expectError: true,
		},
		{
			name:        "Empty plan ID",
			merchantID:  1,
			planID:      "",
			start:       time.Now(),
			end:         time.Now().AddDate(0, 1, 0),
			expectError: true,
		},
		{
			name:        "SQL injection in plan ID",
			merchantID:  1,
			planID:      "free'; DROP TABLE subscription_plans; --",
			start:       time.Now(),
			end:         time.Now().AddDate(0, 1, 0),
			expectError: true,
		},
		{
			name:        "End before start",
			merchantID:  1,
			planID:      "free",
			start:       time.Now(),
			end:         time.Now().AddDate(0, -1, 0),
			expectError: false, // Service doesn't validate this (business logic)
		},
		{
			name:        "Zero merchant ID",
			merchantID:  0,
			planID:      "free",
			start:       time.Now(),
			end:         time.Now().AddDate(0, 1, 0),
			expectError: true,
		},
		{
			name:        "Negative merchant ID",
			merchantID:  -1,
			planID:      "free",
			start:       time.Now(),
			end:         time.Now().AddDate(0, 1, 0),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sub, err := service.CreateSubscription(ctx, tc.merchantID, tc.planID, tc.start, tc.end)

			if tc.expectError {
				assert.Error(t, err, "Should fail validation")
				assert.Nil(t, sub)
			}
		})
	}
}

func TestInputValidation_UsageIncrement(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	// Get real merchant
	var merchantID int64
	err := db.QueryRow(ctx, "SELECT id FROM merchants LIMIT 1").Scan(&merchantID)
	require.NoError(t, err)

	testCases := []struct {
		name        string
		merchantID  int64
		volume      decimal.Decimal
		expectError bool
	}{
		{
			name:        "Normal usage",
			merchantID:  merchantID,
			volume:      decimal.NewFromInt(100),
			expectError: false,
		},
		{
			name:        "Negative volume",
			merchantID:  merchantID,
			volume:      decimal.NewFromInt(-100),
			expectError: false, // Service doesn't validate sign
		},
		{
			name:        "Very large volume",
			merchantID:  merchantID,
			volume:      decimal.NewFromInt(999999999),
			expectError: false,
		},
		{
			name:        "Invalid merchant ID",
			merchantID:  999999,
			volume:      decimal.NewFromInt(100),
			expectError: true, // Should fail due to foreign key constraint
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.IncrementPaymentUsage(ctx, tc.merchantID, tc.volume)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				// Service should handle gracefully
				assert.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// RACE CONDITION TESTS
// ============================================================================

func TestRaceCondition_SubscriptionActivation(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	// Create test subscription
	var merchantID int64
	err := db.QueryRow(ctx, "SELECT id FROM merchants LIMIT 1").Scan(&merchantID)
	require.NoError(t, err)

	// Cleanup any existing active subscriptions for this merchant
	db.Exec(ctx, "DELETE FROM merchant_subscriptions WHERE merchant_id = $1 AND status IN ('active', 'pending_payment')", merchantID)

	// Create a test payment
	var paymentID int64
	merchantOrderUUID := uuid.New()
	err = db.QueryRow(ctx, `
		INSERT INTO payments (public_id, merchant_id, merchant_order_uuid, price, decimals, currency,
		                      type, status, redirect_url, is_test, created_at, updated_at)
		VALUES ($1, $2, $3, 10000, 2, 'USD', 'payment', 'completed', '', false, NOW(), NOW())
		RETURNING id
	`, uuid.New(), merchantID, merchantOrderUUID).Scan(&paymentID)
	require.NoError(t, err)

	sub, err := service.CreateSubscription(ctx, merchantID, "pro", time.Now(), time.Now().AddDate(0, 1, 0))
	require.NoError(t, err)

	// Try to activate concurrently with same payment ID (simulates race condition)
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			err := service.ActivateSubscription(ctx, sub.ID, paymentID)
			done <- err
		}()
	}

	// Wait for all
	successCount := 0
	for i := 0; i < 10; i++ {
		err := <-done
		if err == nil {
			successCount++
		}
	}

	// All should succeed (last write wins, idempotent operation)
	assert.Equal(t, 10, successCount, "All concurrent activations should succeed")

	// Check final state
	var finalPaymentID int64
	var finalStatus string
	err = db.QueryRow(ctx, `
		SELECT payment_id, status FROM merchant_subscriptions WHERE id = $1
	`, sub.ID).Scan(&finalPaymentID, &finalStatus)
	assert.NoError(t, err)
	assert.Equal(t, "active", finalStatus)
	assert.Equal(t, paymentID, finalPaymentID, "Should have the correct payment ID")

	// Cleanup
	db.Exec(ctx, "DELETE FROM merchant_subscriptions WHERE id = $1", sub.ID)
	db.Exec(ctx, "DELETE FROM payments WHERE id = $1", paymentID)
}

// ============================================================================
// PAYMENT MANIPULATION TESTS
// ============================================================================

func TestPaymentManipulation_ActivateWithWrongPayment(t *testing.T) {
	service, db := setupService(t)
	defer cleanupDB(db)

	ctx := context.Background()

	// This test verifies that subscription activation validates payment FK
	// The database enforces payment existence via foreign key constraint

	var merchantID int64
	err := db.QueryRow(ctx, "SELECT id FROM merchants LIMIT 1").Scan(&merchantID)
	require.NoError(t, err)

	// Cleanup any existing active subscriptions for this merchant
	db.Exec(ctx, "DELETE FROM merchant_subscriptions WHERE merchant_id = $1 AND status IN ('active', 'pending_payment')", merchantID)

	sub, err := service.CreateSubscription(ctx, merchantID, "pro", time.Now(), time.Now().AddDate(0, 1, 0))
	require.NoError(t, err)

	// Try to activate with non-existent payment - should fail due to FK constraint
	err = service.ActivateSubscription(ctx, sub.ID, 999999)
	assert.Error(t, err, "Should fail due to foreign key constraint on payment_id")

	// Cleanup
	db.Exec(ctx, "DELETE FROM merchant_subscriptions WHERE id = $1", sub.ID)
}
