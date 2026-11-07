package subscriptionapi

import (
	"net/http"
	"time"

	"github.com/cryptolink/cryptolink/internal/event/paymentevents"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/pkg/api-dashboard/v1/model"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/pkg/errors"
)

const adminPendingWebhooksLimit = 200

type adminPaymentResponse struct {
	ID               int64      `json:"id"`
	PublicID         string     `json:"public_id"`
	MerchantID       int64      `json:"merchant_id"`
	Status           string     `json:"status"`
	WebhookAttempts  int        `json:"webhook_attempts"`
	WebhookSentAt    *time.Time `json:"webhook_sent_at"`
	WebhookDelivered bool       `json:"webhook_delivered"`
	NextAttemptAt    *time.Time `json:"next_attempt_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func newAdminPaymentResponse(w *payment.WebhookState) adminPaymentResponse {
	return adminPaymentResponse{
		ID:               w.ID,
		PublicID:         w.PublicID.String(),
		MerchantID:       w.MerchantID,
		Status:           w.Status,
		WebhookAttempts:  w.Attempts,
		WebhookSentAt:    w.SentAt,
		WebhookDelivered: w.Delivered(),
		NextAttemptAt:    w.NextAttemptAt,
		UpdatedAt:        w.UpdatedAt,
	}
}

func adminError(c *echo.Context, code int, status, message string) error {
	return c.JSON(code, &model.ErrorResponse{Message: message, Status: status})
}

// AdminListPendingWebhookPayments lists payments their merchant has not acknowledged.
// GET /api/dashboard/v1/admin/payments/pending-webhooks
func (h *Handler) AdminListPendingWebhookPayments(c *echo.Context) error {
	rows, err := h.paymentService.ListAwaitingWebhook(c.Request().Context(), adminPendingWebhooksLimit)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list payments awaiting webhook")
		return adminError(c, http.StatusInternalServerError, "internal_error", "unable to list payments")
	}

	results := make([]adminPaymentResponse, len(rows))
	for i, r := range rows {
		results[i] = newAdminPaymentResponse(r)
	}

	return c.JSON(http.StatusOK, map[string]any{"results": results})
}

// lookupAdminPayment writes the error response itself and returns nil when there is no payment.
func (h *Handler) lookupAdminPayment(c *echo.Context) (*payment.WebhookState, error) {
	publicID, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		return nil, adminError(c, http.StatusBadRequest, "invalid_request", "Payment ID must be a UUID")
	}

	state, err := h.paymentService.GetWebhookState(c.Request().Context(), publicID)

	switch {
	case errors.Is(err, payment.ErrNotFound):
		return nil, adminError(c, http.StatusNotFound, "not_found", "Payment not found")
	case err != nil:
		h.logger.Error().Err(err).Msg("failed to get payment webhook state")
		return nil, adminError(c, http.StatusInternalServerError, "internal_error", "unable to get payment")
	}

	return state, nil
}

// AdminGetPayment finds a payment by UUID across all merchants.
// GET /api/dashboard/v1/admin/payments/:paymentId
func (h *Handler) AdminGetPayment(c *echo.Context) error {
	state, err := h.lookupAdminPayment(c)
	if state == nil {
		return err
	}

	return c.JSON(http.StatusOK, newAdminPaymentResponse(state))
}

// AdminResendWebhook sends a payment's current status to its merchant again.
// POST /api/dashboard/v1/admin/payments/:paymentId/resend-webhook
func (h *Handler) AdminResendWebhook(c *echo.Context) error {
	state, err := h.lookupAdminPayment(c)
	if state == nil {
		return err
	}

	ctx := c.Request().Context()
	result := func(success bool, message string) error {
		return c.JSON(http.StatusOK, map[string]any{"success": success, "message": message})
	}

	err = h.resendWebhook(ctx, state.MerchantID, state.ID)

	switch {
	case errors.Is(err, paymentevents.ErrWebhookURLNotSet):
		return result(false, "This merchant has no webhook URL set")
	case errors.Is(err, payment.ErrWebhookNotResendable):
		return result(false, "This payment has not settled yet, so there is nothing to send")
	case err != nil:
		h.logger.Error().Err(err).Int64("payment_id", state.ID).Msg("failed to resend webhook")
		return adminError(c, http.StatusInternalServerError, "internal_error", "unable to resend webhook")
	}

	updated, err := h.paymentService.GetWebhookState(ctx, state.PublicID)
	if err != nil {
		h.logger.Error().Err(err).Int64("payment_id", state.ID).Msg("failed to read webhook state after resend")
		return adminError(c, http.StatusInternalServerError, "internal_error", "webhook sent, but its result could not be read")
	}

	if updated.Delivered() {
		return result(true, "The merchant acknowledged the webhook")
	}

	return result(false, "The merchant's endpoint did not acknowledge the webhook. It will keep retrying automatically")
}
