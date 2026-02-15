package subscriptionapi

import (
	"context"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/subscription"
	"github.com/pkg/errors"
)

// PaymentServiceAdapter adapts payment.Service to subscription.PaymentService interface
type PaymentServiceAdapter struct {
	paymentService  *payment.Service
	merchantService *merchant.Service
}

func (a *PaymentServiceAdapter) CreatePayment(ctx context.Context, params subscription.PaymentParams) (*subscription.PaymentResult, error) {
	// Convert amount to money.Money
	amount, err := money.USD.MakeAmount(params.Amount.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create money amount")
	}

	// Create payment using the payment service
	// Note: metadata will be stored as part of the payment record
	// The subscription_id in metadata will be used by the webhook handler
	p, err := a.paymentService.CreatePayment(ctx, params.MerchantID, payment.CreatePaymentProps{
		MerchantOrderUUID: uuid.New(),
		Money:             amount,
		Description:       &params.Description,
		RedirectURL:       &params.RedirectURL,
		IsTest:            false,
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to create payment")
	}

	// TODO: Store metadata (subscription_id) separately or update payment record
	// For now, we'll need to handle this at the database level

	// Return result
	return &subscription.PaymentResult{
		ID:       p.ID,
		PublicID: p.PublicID,
		URL:      p.PaymentURL,
	}, nil
}
