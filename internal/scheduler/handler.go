package scheduler

import (
	"context"

	"github.com/cryptolink/cryptolink/internal/log"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/cryptolink/cryptolink/internal/util"
	"github.com/pkg/errors"
)

// Handler scheduler handler. Be aware that each ctx has zerolog.Logger instance!
type Handler struct {
	payments     *payment.Service
	processing   ProcessingService
	transactions *transaction.Service
	tableLogger  *log.JobLogger
}

type ContextJobID struct{}
type ContextJobEnableTableLogger struct{}

type ProcessingService interface {
	BatchCheckIncomingTransactions(ctx context.Context, transactionIDs []int64) error
	BatchExpirePayments(ctx context.Context, paymentsIDs []int64) error
}

func New(
	payments *payment.Service,
	processingService ProcessingService,
	transactions *transaction.Service,
	jobLogger *log.JobLogger,
) *Handler {
	return &Handler{
		payments:     payments,
		processing:   processingService,
		transactions: transactions,
		tableLogger:  jobLogger,
	}
}

func (h *Handler) JobLogger() *log.JobLogger {
	return h.tableLogger
}

func (h *Handler) CheckIncomingTransactionsProgress(ctx context.Context) error {
	const limit = 200

	filter := transaction.Filter{
		Types:    []transaction.Type{transaction.TypeIncoming},
		Statuses: []transaction.Status{transaction.StatusInProgress, transaction.StatusInProgressInvalid},
	}

	txs, err := h.transactions.ListByFilter(ctx, filter, limit)
	if err != nil {
		return errors.Wrap(err, "unable to list incoming transactions")
	}

	ids := util.MapSlice(txs, func(t *transaction.Transaction) int64 { return t.ID })

	if err := h.processing.BatchCheckIncomingTransactions(ctx, ids); err != nil {
		return errors.Wrap(err, "unable to batch check incoming transactions")
	}

	return nil
}

func (h *Handler) CancelExpiredPayments(ctx context.Context) error {
	const limit = 200

	payments, err := h.payments.GetBatchExpired(ctx, limit)
	if err != nil {
		return errors.Wrap(err, "unable to get batch expired payments")
	}

	ids := util.MapSlice(payments, func(pt *payment.Payment) int64 { return pt.ID })

	if err := h.processing.BatchExpirePayments(ctx, ids); err != nil {
		return errors.Wrap(err, "unable to batch expire payments")
	}

	return nil
}
