package paymentevents

import (
	"context"
	"fmt"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/cryptolink/cryptolink/internal/bus"
	"github.com/cryptolink/cryptolink/internal/service/merchant"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/processing"
	"github.com/cryptolink/cryptolink/internal/service/subscription"
	"github.com/cryptolink/cryptolink/internal/slack"
	"github.com/cryptolink/cryptolink/internal/util"
	"github.com/cryptolink/cryptolink/internal/webhook"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Handler struct {
	merchants       *merchant.Service
	processing      *processing.Service
	payments        *payment.Service
	subscriptions   *subscription.Service
	slackWebhookURL string
	logger          *zerolog.Logger

	// retrying prevents overlapping RetryUndeliveredWebhooks runs.
	retrying atomic.Bool
}

func New(
	merchants *merchant.Service,
	processingService *processing.Service,
	payments *payment.Service,
	subscriptions *subscription.Service,
	slackWebhookURL string,
	logger *zerolog.Logger,
) *Handler {
	log := logger.With().Str("channel", "payment_events_consumer").Logger()

	return &Handler{
		merchants:       merchants,
		processing:      processingService,
		payments:        payments,
		subscriptions:   subscriptions,
		slackWebhookURL: slackWebhookURL,
		logger:          &log,
	}
}

func (h *Handler) Consumers() map[bus.Topic][]bus.Consumer {
	return map[bus.Topic][]bus.Consumer{
		bus.TopicPaymentStatusUpdate: {
			h.ProcessPaymentStatusUpdate,
			h.SendSuccessfulPaymentNotification,
		},
	}
}

type PaymentWebhook struct {
	ID     string `json:"id"`
	Status string `json:"status"`

	CustomerEmail string `json:"customerEmail"`

	SelectedBlockchain string `json:"selectedBlockchain"`
	SelectedCurrency   string `json:"selectedCurrency"`

	IsTest bool `json:"isTest"`

	LinkID *string `json:"paymentLinkId"`

	// Partial-fill payload — populated only when Status == "partial".
	// Merchants should treat the `idempotencyKey` as a dedup token: it's
	// "<network_id>:<tx_hash>:<vout_or_logidx>" of the most recent
	// confirmed fill, so retried webhook deliveries for the same fill
	// arrive with the same key.
	ReceivedAmount  string `json:"receivedAmount,omitempty"`
	RemainingAmount string `json:"remainingAmount,omitempty"`
	IdempotencyKey  string `json:"idempotencyKey,omitempty"`
}

func (h *Handler) ProcessPaymentStatusUpdate(ctx context.Context, message bus.Message) error {
	req, err := bus.Bind[bus.PaymentStatusUpdateEvent](message)
	if err != nil {
		return err
	}

	return h.deliverWebhook(ctx, req.MerchantID, req.PaymentID)
}

// Webhook retry policy. A failed delivery is retried by RetryUndeliveredWebhooks
// with WebhookBackoff between attempts; after WebhookMaxAttempts it gives up
// and raises an alert.
const (
	WebhookMaxAttempts = 12

	// webhookLease must comfortably exceed webhook.Timeout: while it holds,
	// no other sender may deliver the same key.
	webhookLease = 2 * time.Minute

	// Retry job scope: payments changed within the window, and settled long
	// enough for the live event to have had its chance.
	webhookRetryWindow = 7 * 24 * time.Hour
	webhookRetrySettle = time.Minute
	webhookRetryBatch  = 50
)

var WebhookBackoff = []time.Duration{
	time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
	time.Hour,
	2 * time.Hour,
	6 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
	24 * time.Hour,
}

// NextWebhookAttempt returns when to retry after failed attempt number
// `attempt` (1-based), or false when the delivery should give up.
func NextWebhookAttempt(attempt int, now time.Time) (time.Time, bool) {
	if attempt < 1 || attempt >= WebhookMaxAttempts {
		return time.Time{}, false
	}

	i := attempt - 1
	if i >= len(WebhookBackoff) {
		i = len(WebhookBackoff) - 1
	}

	return now.Add(WebhookBackoff[i]), true
}

// WebhookDeliveryKey identifies one webhook the merchant must receive exactly
// once: the status, or the status plus fill key for partial payments, which
// get one webhook per fill.
func WebhookDeliveryKey(status payment.Status, fillKey string) string {
	if status == payment.StatusPartial && fillKey != "" {
		return status.String() + ":" + fillKey
	}

	return status.String()
}

// RetryUndeliveredWebhooks is a scheduler job that re-sends webhooks that
// failed, or whose status event never reached the live sender.
func (h *Handler) RetryUndeliveredWebhooks(ctx context.Context) error {
	if !h.retrying.CompareAndSwap(false, true) {
		return nil
	}
	defer h.retrying.Store(false)

	now := time.Now()

	due, err := h.payments.ListUndeliveredWebhooks(
		ctx,
		now.Add(-webhookRetryWindow),
		now.Add(-webhookRetrySettle),
		WebhookMaxAttempts,
		webhookRetryBatch,
	)
	if err != nil {
		return err
	}

	for _, d := range due {
		if err := h.deliverWebhook(ctx, d.MerchantID, d.PaymentID); err != nil {
			h.logger.Error().Err(err).
				Int64("merchant_id", d.MerchantID).
				Int64("payment_id", d.PaymentID).
				Msg("webhook retry failed")
		}
	}

	if len(due) > 0 {
		h.logger.Info().Int("count", len(due)).Msg("retried undelivered webhooks")
	}

	return nil
}

// deliverWebhook sends the payment's current status to the merchant at most
// once per delivery key. The DB claim makes the live event and the retry job
// mutually exclusive, so concurrent or repeated triggers never double-send.
func (h *Handler) deliverWebhook(ctx context.Context, merchantID, paymentID int64) error {
	mt, err := h.merchants.GetByID(ctx, merchantID, false)
	if err != nil {
		return errors.Wrap(err, "unable to get merchant")
	}

	webhookURL := mt.Settings().WebhookURL()
	signatureSecret := mt.Settings().WebhookSignatureSecret()

	p, err := h.processing.GetDetailedPayment(ctx, merchantID, paymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get detailed payment")
	}

	// omit "locked" event
	if p.Payment.Status == payment.StatusLocked {
		return nil
	}

	wh := PaymentWebhook{
		ID:     p.Payment.MerchantOrderUUID.String(),
		Status: p.Payment.Status.String(),
		IsTest: p.Payment.IsTest,
	}
	if p.Customer != nil {
		wh.CustomerEmail = p.Customer.Email
	}
	if p.PaymentMethod != nil {
		wh.SelectedBlockchain = p.PaymentMethod.Currency.Blockchain.String()
		wh.SelectedCurrency = p.PaymentMethod.Currency.Ticker
	}
	// Partial-fill enrichment: include received/remaining amounts and a
	// per-fill idempotency key so merchants can dedup retried deliveries.
	if p.Payment.Status == payment.StatusPartial && p.PaymentInfo != nil {
		wh.ReceivedAmount = p.PaymentInfo.ReceivedAmount
		wh.RemainingAmount = p.PaymentInfo.RemainingAmount
		if key, keyErr := h.processing.LatestFillIdempotencyKey(ctx, merchantID, paymentID); keyErr == nil {
			wh.IdempotencyKey = key
		}
	}
	if webhookURL != "" && p.Payment.LinkID() != 0 {
		link, err := h.payments.GetPaymentLinkByID(ctx, mt.ID, p.Payment.LinkID())
		if err != nil {
			return errors.Wrap(err, "unable to get payment link")
		}

		wh.LinkID = util.Ptr(link.PublicID.String())
	}

	key := WebhookDeliveryKey(p.Payment.Status, wh.IdempotencyKey)
	now := time.Now()

	attempt, err := h.payments.ClaimWebhook(ctx, merchantID, paymentID, p.Payment.Status, key, now.Add(webhookLease), WebhookMaxAttempts)
	switch {
	case errors.Is(err, payment.ErrWebhookNotClaimed):
		h.logger.Debug().
			Int64("merchant_id", merchantID).Int64("payment_id", paymentID).Str("delivery_key", key).
			Msg("webhook already delivered or in flight; skipping")

		return nil
	case err != nil:
		return err
	}

	if webhookURL == "" {
		h.logger.Warn().
			Int64("merchant_id", merchantID).Int64("payment_id", paymentID).
			Msg("webhook not set; skipping sending")

		// Nothing to deliver; record it so a webhook URL set later doesn't
		// replay old statuses.
		return h.payments.MarkWebhookDelivered(ctx, merchantID, paymentID, key, nil)
	}

	if err := webhook.Send(ctx, webhookURL, signatureSecret, wh); err != nil {
		return h.handleWebhookFailure(ctx, mt, p.Payment.ID, key, attempt, webhookURL, wh, err)
	}

	sentAt := time.Now()
	if err := h.payments.MarkWebhookDelivered(ctx, merchantID, paymentID, key, &sentAt); err != nil {
		return errors.Wrap(err, "unable to mark webhook delivered")
	}

	h.logger.Info().
		Int64("merchant_id", merchantID).
		Int64("payment_id", paymentID).
		Str("delivery_key", key).
		Int("attempt", attempt).
		Str("webhook_url", webhookURL).
		Msg("sent webhook to merchant")

	// Handle subscription activation if payment is for a subscription
	if p.Payment.Status == payment.StatusSuccess {
		if err := h.handleSubscriptionPayment(ctx, p.Payment.ID); err != nil {
			h.logger.Error().Err(err).
				Int64("payment_id", p.Payment.ID).
				Msg("failed to handle subscription payment")
			// Don't fail the whole webhook processing if subscription activation fails
		}
	}

	return nil
}

func (h *Handler) handleWebhookFailure(
	ctx context.Context,
	mt *merchant.Merchant,
	paymentID int64,
	key string,
	attempt int,
	webhookURL string,
	wh PaymentWebhook,
	sendErr error,
) error {
	next, retry := NextWebhookAttempt(attempt, time.Now())

	if retry {
		h.logger.Warn().Err(sendErr).
			Int64("merchant_id", mt.ID).
			Int64("payment_id", paymentID).
			Str("delivery_key", key).
			Int("attempt", attempt).
			Time("next_attempt_at", next).
			Interface("webhook", wh).
			Str("webhook_url", webhookURL).
			Msg("unable to send webhook; will retry")

		return h.payments.MarkWebhookFailed(ctx, mt.ID, paymentID, key, &next)
	}

	h.logger.Error().Err(sendErr).
		Int64("merchant_id", mt.ID).
		Int64("payment_id", paymentID).
		Str("delivery_key", key).
		Int("attempt", attempt).
		Interface("webhook", wh).
		Str("webhook_url", webhookURL).
		Msg("unable to send webhook; giving up")

	if err := h.payments.MarkWebhookFailed(ctx, mt.ID, paymentID, key, nil); err != nil {
		return errors.Wrap(err, "unable to mark webhook failed")
	}

	alert := fmt.Sprintf(
		"⚠️ webhook delivery gave up after %d attempts: payment #%d (%s) for merchant %q id#%d → %s",
		attempt, paymentID, wh.Status, mt.Name, mt.ID, webhookURL,
	)
	if err := h.sendSlackMessage(alert); err != nil {
		h.logger.Error().Err(err).Int64("payment_id", paymentID).Msg("unable to send webhook give-up alert")
	}

	return nil
}

func (h *Handler) handleSubscriptionPayment(ctx context.Context, paymentID int64) error {
	if h.subscriptions == nil {
		return nil
	}

	// Try to activate subscription webhook - it will check if payment has subscription metadata
	if err := h.subscriptions.HandlePaymentWebhook(ctx, paymentID, ""); err != nil {
		// Only log if it's a real error, not just "no subscription" case
		if !errors.Is(err, subscription.ErrSubscriptionNotFound) {
			return errors.Wrap(err, "failed to handle payment webhook")
		}
		// Not a subscription payment, skip silently
		return nil
	}

	h.logger.Info().
		Int64("payment_id", paymentID).
		Msg("subscription activated successfully from payment webhook")

	return nil
}

func (h *Handler) SendSuccessfulPaymentNotification(ctx context.Context, message bus.Message) error {
	req, err := bus.Bind[bus.PaymentStatusUpdateEvent](message)
	if err != nil {
		return err
	}

	p, err := h.processing.GetDetailedPayment(ctx, req.MerchantID, req.PaymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get detailed payment")
	}

	if p.Payment.Status != payment.StatusSuccess {
		// skip
		return nil
	}

	content := fmt.Sprintf(
		"[%s] 💰processed payment #%d: %s %s for merchant %q id#%d (isTest=%t) ",
		extractHost(p.Payment.PaymentURL),
		p.Payment.ID,
		p.Payment.Price.String(),
		p.Payment.Price.Ticker(),
		p.Merchant.Name,
		p.Merchant.ID,
		p.Payment.IsTest,
	)

	return h.sendSlackMessage(content)
}

func (h *Handler) sendSlackMessage(messages ...string) error {
	if h.slackWebhookURL == "" {
		h.logger.Warn().Msg("Skipping slack notification send due to empty webhook url")
		return nil
	}

	return slack.SendWebhook(h.slackWebhookURL, messages...)
}

func extractHost(u string) string {
	link, _ := url.Parse(u)

	return link.Host
}
