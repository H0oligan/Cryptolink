package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/cryptolink/cryptolink/internal/db/repository"
	"github.com/cryptolink/cryptolink/internal/scheduler"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/cryptolink/cryptolink/internal/service/wallet"
	"github.com/cryptolink/cryptolink/internal/test"
	"github.com/cryptolink/cryptolink/internal/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestContext struct {
	*test.IntegrationTest

	Context        context.Context
	ProcessingMock *mock.ProcessingProxyMock
	Scheduler      *scheduler.Handler
}

func setup(t *testing.T) *TestContext {
	tc := test.NewIntegrationTest(t)

	ctx := context.WithValue(context.Background(), scheduler.ContextJobID{}, "job-abc")
	ctx = tc.Logger.WithContext(ctx)

	processingMock := mock.NewProcessingProxyMock(t, tc.Services.Processing)

	return &TestContext{
		IntegrationTest: tc,

		Context:        ctx,
		ProcessingMock: processingMock,
		Scheduler: scheduler.New(
			tc.Services.Payment,
			processingMock,
			tc.Services.Transaction,
			nil, // watcher (not needed in tests)
			tc.Services.JobLogger,
		),
	}
}

func TestHandler_CheckIncomingTransactionsProgress(t *testing.T) {
	// ARRANGE
	tc := setup(t)

	// Given merchant
	mt, _ := tc.Must.CreateMerchant(t, 1)

	// Given outbound wallet
	inboundWallet := tc.Must.CreateWallet(t, "ETH", "0x123", "0x123-pub-key", wallet.TypeInbound)
	outboundWallet := tc.Must.CreateWallet(t, "ETH", "0x1234", "0x1234-pub-key", wallet.TypeOutbound)

	// Given several transactions
	asIncoming := func(p *transaction.CreateTransaction) {
		p.Type = transaction.TypeIncoming
		p.RecipientWallet = inboundWallet
	}

	asInternal := func(p *transaction.CreateTransaction) {
		p.Type = transaction.TypeInternal
		p.EntityID = 0
		p.SenderWallet = inboundWallet
		p.RecipientWallet = outboundWallet
	}

	// Given 2 incoming 'in progress' txs
	tx1 := tc.Must.CreateTransaction(t, mt.ID, asIncoming)
	_, err := tc.Repository.UpdateTransaction(tc.Context, repository.UpdateTransactionParams{
		MerchantID: mt.ID,
		ID:         tx1.ID,
		Status:     string(transaction.StatusInProgress),
		UpdatedAt:  time.Now(),
		FactAmount: pgtype.Numeric{Status: pgtype.Null},
		NetworkFee: pgtype.Numeric{Status: pgtype.Null},
		Metadata:   pgtype.JSONB{Status: pgtype.Null},
	})
	require.NoError(t, err)

	tx2 := tc.Must.CreateTransaction(t, mt.ID, asIncoming)
	_, err = tc.Repository.UpdateTransaction(tc.Context, repository.UpdateTransactionParams{
		MerchantID: mt.ID,
		ID:         tx2.ID,
		Status:     string(transaction.StatusInProgressInvalid),
		UpdatedAt:  time.Now(),
		FactAmount: pgtype.Numeric{Status: pgtype.Null},
		NetworkFee: pgtype.Numeric{Status: pgtype.Null},
		Metadata:   pgtype.JSONB{Status: pgtype.Null},
	})
	require.NoError(t, err)

	// And 1 incoming 'pending' & 2 internal txs
	tc.Must.CreateTransaction(t, mt.ID, asIncoming)
	tc.Must.CreateTransaction(t, 0, asInternal)
	tc.Must.CreateTransaction(t, 0, asInternal)

	// And expected mock
	tc.ProcessingMock.SetupBatchCheckIncomingTransactions([]int64{tx1.ID, tx2.ID}, nil)

	// ACT
	err = tc.Scheduler.CheckIncomingTransactionsProgress(tc.Context)

	// ASSERT
	assert.NoError(t, err)
}

func TestScheduler_CancelExpiredPayments(t *testing.T) {
	// ARRANGE
	tc := setup(t)

	// Given a merchant
	mt, _ := tc.Must.CreateMerchant(t, 1)

	setExpiration := func(pt *payment.Payment, status payment.Status, expiresAt time.Time) {
		_, err := tc.Repository.UpdatePayment(tc.Context, repository.UpdatePaymentParams{
			ID:           pt.ID,
			MerchantID:   pt.MerchantID,
			Status:       string(status),
			UpdatedAt:    time.Now(),
			ExpiresAt:    repository.TimeToNullable(expiresAt),
			SetExpiresAt: true,
		})
		require.NoError(t, err)
	}

	alterCreatedAt := func(dur time.Duration) func(*repository.CreatePaymentParams) {
		return func(create *repository.CreatePaymentParams) {
			create.CreatedAt = time.Now().Add(dur)
		}
	}

	// With several payments
	pt1 := tc.CreateSamplePayment(t, mt.ID)
	pt2 := tc.CreateSamplePayment(t, mt.ID)
	pt3 := tc.CreateSamplePayment(t, mt.ID)
	pt4 := tc.CreateSamplePayment(t, mt.ID)

	// And some payments should be expired
	setExpiration(pt1, payment.StatusPending, time.Now().Add(payment.ExpirationPeriodForLocked))
	setExpiration(pt2, payment.StatusPending, time.Now()) // should expire
	setExpiration(pt3, payment.StatusLocked, time.Now().Add(payment.ExpirationPeriodForLocked/2))
	setExpiration(pt4, payment.StatusLocked, time.Now().Add(-time.Minute)) // should expire

	// And payments that were created a long time ago, but didn't have any interaction from a user.
	// pt5Raw shouldn't be included in the batch
	_ = tc.CreateRawPayment(t, mt.ID, alterCreatedAt(-payment.ExpirationPeriodForNotLocked+time.Minute))

	// should be included in the batch
	pt6Raw := tc.CreateRawPayment(t, mt.ID, alterCreatedAt(-payment.ExpirationPeriodForNotLocked))

	// And expected processing service call
	tc.ProcessingMock.SetupBatchExpirePayments([]int64{pt2.ID, pt4.ID, pt6Raw.ID}, nil)

	// ACT
	err := tc.Scheduler.CancelExpiredPayments(tc.Context)

	// ASSERT
	assert.NoError(t, err)
}
