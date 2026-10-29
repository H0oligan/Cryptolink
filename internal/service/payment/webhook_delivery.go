package payment

import (
	"context"
	"time"

	"github.com/cryptolink/cryptolink/internal/db/repository"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
)

// Webhook delivery bookkeeping. A merchant webhook for a given delivery key
// (status, or "partial:<fill key>") is sent by whoever wins ClaimWebhook, and
// never again once MarkWebhookDelivered has recorded it.

// ErrWebhookNotClaimed means another sender owns the delivery, it was already
// delivered, or the payment status changed since it was read.
var ErrWebhookNotClaimed = errors.New("webhook delivery not claimed")

type UndeliveredWebhook struct {
	MerchantID int64
	PaymentID  int64
}

// ClaimWebhook takes the delivery lease for key until leaseUntil and returns
// the attempt number for that key.
func (s *Service) ClaimWebhook(
	ctx context.Context,
	merchantID, paymentID int64,
	status Status,
	key string,
	leaseUntil time.Time,
	maxAttempts int,
) (int, error) {
	attempts, err := s.repo.ClaimPaymentWebhook(ctx, repository.ClaimPaymentWebhookParams{
		ID:          paymentID,
		MerchantID:  merchantID,
		Status:      status.String(),
		DeliveryKey: key,
		LeaseUntil:  leaseUntil,
		Now:         time.Now(),
		MaxAttempts: int32(maxAttempts),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return 0, ErrWebhookNotClaimed
	case err != nil:
		return 0, errors.Wrap(err, "unable to claim webhook")
	}

	return int(attempts), nil
}

// MarkWebhookDelivered records key as delivered. A nil sentAt keeps the
// previous webhook_sent_at (used when there was nothing to send).
func (s *Service) MarkWebhookDelivered(ctx context.Context, merchantID, paymentID int64, key string, sentAt *time.Time) error {
	return s.repo.MarkPaymentWebhookDelivered(ctx, repository.MarkPaymentWebhookDeliveredParams{
		ID:          paymentID,
		MerchantID:  merchantID,
		DeliveryKey: key,
		SentAt:      repository.PointerTimeToNullable(sentAt),
	})
}

// MarkWebhookFailed schedules the next attempt; nil nextAttemptAt means the
// delivery gave up.
func (s *Service) MarkWebhookFailed(ctx context.Context, merchantID, paymentID int64, key string, nextAttemptAt *time.Time) error {
	return s.repo.MarkPaymentWebhookFailed(ctx, repository.MarkPaymentWebhookFailedParams{
		ID:            paymentID,
		MerchantID:    merchantID,
		DeliveryKey:   key,
		NextAttemptAt: repository.PointerTimeToNullable(nextAttemptAt),
	})
}

// ListUndeliveredWebhooks returns payments whose current status was never
// acknowledged by the merchant, or whose last attempt failed, and that are
// due for an attempt now.
func (s *Service) ListUndeliveredWebhooks(
	ctx context.Context,
	updatedAfter, updatedBefore time.Time,
	maxAttempts, limit int,
) ([]UndeliveredWebhook, error) {
	rows, err := s.repo.ListPaymentsWithUndeliveredWebhook(ctx, repository.ListPaymentsWithUndeliveredWebhookParams{
		UpdatedAfter:  updatedAfter,
		UpdatedBefore: updatedBefore,
		Now:           time.Now(),
		MaxAttempts:   int32(maxAttempts),
		RowLimit:      int32(limit),
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to list undelivered webhooks")
	}

	out := make([]UndeliveredWebhook, len(rows))
	for i, r := range rows {
		out[i] = UndeliveredWebhook{MerchantID: r.MerchantID, PaymentID: r.ID}
	}

	return out, nil
}
