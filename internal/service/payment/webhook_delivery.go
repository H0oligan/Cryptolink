package payment

import (
	"context"
	"strings"
	"time"

	"github.com/cryptolink/cryptolink/internal/db/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

// Webhook delivery bookkeeping. A merchant webhook for a given delivery key
// (status, or "partial:<fill key>") is sent by whoever wins ClaimWebhook, and
// never again once MarkWebhookDelivered has recorded it.

// ErrWebhookNotClaimed means another sender owns the delivery, it was already
// delivered, or the payment status changed since it was read.
var ErrWebhookNotClaimed = errors.New("webhook delivery not claimed")

// ErrWebhookNotResendable means the payment has not settled, so there is nothing to send.
var ErrWebhookNotResendable = errors.New("payment has no webhook to resend")

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
// due for an attempt now: oldest-due first, at most perMerchant per merchant.
func (s *Service) ListUndeliveredWebhooks(
	ctx context.Context,
	updatedAfter, updatedBefore time.Time,
	maxAttempts, perMerchant, limit int,
) ([]UndeliveredWebhook, error) {
	rows, err := s.repo.ListPaymentsWithUndeliveredWebhook(ctx, repository.ListPaymentsWithUndeliveredWebhookParams{
		UpdatedAfter:  updatedAfter,
		UpdatedBefore: updatedBefore,
		Now:           time.Now(),
		MaxAttempts:   int32(maxAttempts),
		PerMerchant:   int32(perMerchant),
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

// WebhookState is a payment's webhook delivery as shown to support staff.
type WebhookState struct {
	ID              int64
	PublicID        uuid.UUID
	MerchantID      int64
	Status          string
	Attempts        int
	SentAt          *time.Time
	DeliveredStatus *string
	NextAttemptAt   *time.Time
	UpdatedAt       time.Time
}

// Delivered reports whether the merchant acknowledged the current status. Partial
// payments use "partial:<fill key>" delivery keys, so only the status part counts.
func (w *WebhookState) Delivered() bool {
	if w.DeliveredStatus == nil {
		return false
	}

	status, _, _ := strings.Cut(*w.DeliveredStatus, ":")

	return status == w.Status
}

func webhookStateFromEntry(e repository.PaymentWebhookState) *WebhookState {
	return &WebhookState{
		ID:              e.ID,
		PublicID:        e.PublicID,
		MerchantID:      e.MerchantID,
		Status:          e.Status,
		Attempts:        int(e.WebhookAttempts),
		SentAt:          repository.NullTimeToPointer(e.WebhookSentAt),
		DeliveredStatus: repository.NullableStringToPointer(e.WebhookDeliveredStatus),
		NextAttemptAt:   repository.NullTimeToPointer(e.WebhookNextAttemptAt),
		UpdatedAt:       e.UpdatedAt,
	}
}

// GetWebhookState looks a payment up across all merchants, for support staff only.
func (s *Service) GetWebhookState(ctx context.Context, publicID uuid.UUID) (*WebhookState, error) {
	e, err := s.repo.GetPaymentWebhookStateByPublicID(ctx, publicID)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, errors.Wrap(err, "unable to get payment webhook state")
	}

	return webhookStateFromEntry(e), nil
}

// ListAwaitingWebhook returns payments the merchant has not acknowledged, newest first.
func (s *Service) ListAwaitingWebhook(ctx context.Context, limit int) ([]*WebhookState, error) {
	rows, err := s.repo.ListPaymentsAwaitingWebhook(ctx, int32(limit))
	if err != nil {
		return nil, errors.Wrap(err, "unable to list payments awaiting webhook")
	}

	out := make([]*WebhookState, len(rows))
	for i, r := range rows {
		out[i] = webhookStateFromEntry(r)
	}

	return out, nil
}

// ResetWebhook makes a settled payment's webhook sendable again, with a fresh retry budget.
func (s *Service) ResetWebhook(ctx context.Context, merchantID, paymentID int64) error {
	n, err := s.repo.ResetPaymentWebhook(ctx, merchantID, paymentID)
	if err != nil {
		return errors.Wrap(err, "unable to reset webhook")
	}
	if n == 0 {
		return ErrWebhookNotResendable
	}

	return nil
}
