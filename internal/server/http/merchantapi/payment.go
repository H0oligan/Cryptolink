package merchantapi

import (
	"fmt"
	"net/http"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/subscription"
	"github.com/cryptolink/cryptolink/internal/util"
	"github.com/cryptolink/cryptolink/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	paramPaymentID = "paymentId"
	queryParamType = "type"
)

func (h *Handler) ListPayments(c echo.Context) error {
	ctx := c.Request().Context()

	mt := middleware.ResolveMerchant(c)

	pagination, err := common.QueryPagination(c)
	if err != nil {
		return common.ValidationErrorResponse(c, err)
	}

	var filterByType []payment.Type

	ptType := payment.Type(c.QueryParam(queryParamType))
	if ptType != "" {
		if ptType == payment.TypePayment || ptType == payment.TypeWithdrawal {
			filterByType = append(filterByType, ptType)
		} else {
			return common.ValidationErrorItemResponse(c, "type", "unknown type %q", ptType)
		}
	}

	payments, nextCursor, err := h.payments.ListWithRelations(ctx, mt.ID, payment.ListOptions{
		Limit:        pagination.Limit,
		Cursor:       pagination.Cursor,
		ReverseOrder: pagination.ReverseSort,
		FilterByType: filterByType,
	})

	switch {
	case errors.Is(err, payment.ErrValidation):
		return common.ValidationErrorResponse(c, "invalid query")
	case err != nil:
		return err
	}

	feePercent := mt.Settings().GlobalFeePercent()
	return c.JSON(http.StatusOK, &model.PaymentsPagination{
		Cursor:  nextCursor,
		Limit:   int64(pagination.Limit),
		Results: util.MapSlice(payments, func(pr payment.PaymentWithRelations) *model.Payment {
			return paymentToResponse(pr, feePercent)
		}),
	})
}

func (h *Handler) GetPayment(c echo.Context) error {
	ctx := c.Request().Context()

	paymentID, err := uuid.Parse(c.Param(paramPaymentID))
	if err != nil {
		return common.ValidationErrorResponse(c, "invalid payment id")
	}

	mt := middleware.ResolveMerchant(c)

	pt, err := h.payments.GetByMerchantOrderIDWithRelations(ctx, mt.ID, paymentID)

	switch {
	case errors.Is(err, payment.ErrNotFound):
		return common.NotFoundResponse(c, "payment not found")
	case err != nil:
		h.logger.Error().Err(err).
			Int64("merchant_id", mt.ID).Str("payment_uuid", paymentID.String()).
			Msg("unable to get payment")

		return err
	}

	return c.JSON(http.StatusOK, paymentToResponse(pt, mt.Settings().GlobalFeePercent()))
}

func (h *Handler) CreatePayment(c echo.Context) error {
	var req model.CreatePaymentRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	merchantOrderUUID, err := uuid.Parse(req.ID.String())
	if err != nil {
		return common.ValidationErrorResponse(c, "order id is invalid")
	}

	currency, err := money.MakeFiatCurrency(req.Currency)
	if err != nil {
		return common.ValidationErrorResponse(c, err)
	}

	if req.Price <= 0 {
		return common.ValidationErrorResponse(c, errors.New("price should be positive"))
	}

	price, err := money.FiatFromFloat64(currency, req.Price)
	if err != nil {
		return common.ValidationErrorItemResponse(c, "price", "price should be between %.2f and %.0f", money.FiatMin, money.FiatMax)
	}

	// Enforce subscription limits before creating payment
	if h.subscriptions != nil {
		if err := h.subscriptions.CheckPaymentLimit(ctx, mt.ID); err != nil {
			if errors.Is(err, subscription.ErrLimitExceeded) {
				return common.ErrorResponseWithStatus(c, http.StatusPaymentRequired, err.Error())
			}
			// If subscription not found, allow payment (graceful degradation)
			if !errors.Is(err, subscription.ErrSubscriptionNotFound) {
				h.logger.Warn().Err(err).Int64("merchant_id", mt.ID).Msg("failed to check payment limit")
			}
		}

		priceUSD, _ := decimal.NewFromString(fmt.Sprintf("%.2f", req.Price))
		if err := h.subscriptions.CheckVolumeLimit(ctx, mt.ID, priceUSD); err != nil {
			if errors.Is(err, subscription.ErrLimitExceeded) {
				return common.ErrorResponseWithStatus(c, http.StatusPaymentRequired, err.Error())
			}
			if !errors.Is(err, subscription.ErrSubscriptionNotFound) {
				h.logger.Warn().Err(err).Int64("merchant_id", mt.ID).Msg("failed to check volume limit")
			}
		}
	}

	pt, err := h.payments.CreatePayment(ctx, mt.ID, payment.CreatePaymentProps{
		MerchantOrderUUID: merchantOrderUUID,
		MerchantOrderID:   req.OrderID,
		Money:             price,
		Description:       req.Description,
		RedirectURL:       req.RedirectURL,
		IsTest:            req.IsTest,
	})

	switch {
	case errors.Is(err, payment.ErrValidation):
		return common.ValidationErrorResponse(c, err)
	case errors.Is(err, payment.ErrAlreadyExists):
		return common.ValidationErrorResponse(c, err)
	case err != nil:
		h.logger.Err(err).Msg("unable to create payment")
		return common.ErrorResponse(c, "internal_error")
	}

	return c.JSON(http.StatusCreated, paymentToResponse(
		payment.PaymentWithRelations{Payment: pt}, mt.Settings().GlobalFeePercent()),
	)
}

func (h *Handler) ResolvePayment(c echo.Context) error {
	ctx := c.Request().Context()

	paymentUUID, err := uuid.Parse(c.Param(paramPaymentID))
	if err != nil {
		return common.ValidationErrorResponse(c, "invalid payment id")
	}

	mt := middleware.ResolveMerchant(c)

	var req struct {
		Notes  string `json:"notes"`
		TxHash string `json:"txHash"`
	}
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	pt, err := h.payments.GetByMerchantOrderID(ctx, mt.ID, paymentUUID)
	if err != nil {
		if errors.Is(err, payment.ErrNotFound) {
			return common.NotFoundResponse(c, "payment not found")
		}
		return err
	}

	resolved, err := h.payments.ResolvePayment(ctx, mt.ID, pt.ID, req.Notes, req.TxHash)

	switch {
	case errors.Is(err, payment.ErrValidation):
		return common.ValidationErrorResponse(c, err)
	case err != nil:
		h.logger.Error().Err(err).Int64("payment_id", pt.ID).Msg("unable to resolve payment")
		return common.ErrorResponse(c, "internal_error")
	}

	return c.JSON(http.StatusOK, paymentToResponse(
		payment.PaymentWithRelations{Payment: resolved}, mt.Settings().GlobalFeePercent(),
	))
}

func (h *Handler) DeclinePayment(c echo.Context) error {
	ctx := c.Request().Context()

	paymentUUID, err := uuid.Parse(c.Param(paramPaymentID))
	if err != nil {
		return common.ValidationErrorResponse(c, "invalid payment id")
	}

	mt := middleware.ResolveMerchant(c)

	var req struct {
		Notes string `json:"notes"`
	}
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	pt, err := h.payments.GetByMerchantOrderID(ctx, mt.ID, paymentUUID)
	if err != nil {
		if errors.Is(err, payment.ErrNotFound) {
			return common.NotFoundResponse(c, "payment not found")
		}
		return err
	}

	declined, err := h.payments.DeclinePayment(ctx, mt.ID, pt.ID, req.Notes)

	switch {
	case errors.Is(err, payment.ErrValidation):
		return common.ValidationErrorResponse(c, err)
	case err != nil:
		h.logger.Error().Err(err).Int64("payment_id", pt.ID).Msg("unable to decline payment")
		return common.ErrorResponse(c, "internal_error")
	}

	return c.JSON(http.StatusOK, paymentToResponse(
		payment.PaymentWithRelations{Payment: declined}, mt.Settings().GlobalFeePercent(),
	))
}

func paymentToResponse(pr payment.PaymentWithRelations, feePercent float64) *model.Payment {
	pt := pr.Payment
	tx := pr.Transaction
	customer := pr.Customer
	balance := pr.Balance

	// Apply merchant's volatility fee to the displayed fiat price so the
	// merchant sees the actual value received in their wallet.
	displayPrice := pt.Price.String()
	if feePercent > 0 && pt.Type == payment.TypePayment {
		if adjusted, err := pt.Price.MultiplyFloat64(1 + feePercent/100); err == nil {
			displayPrice = adjusted.String()
		}
	}

	res := &model.Payment{
		ID:      pt.MerchantOrderUUID.String(),
		OrderID: pt.MerchantOrderID,

		CreatedAt: strfmt.DateTime(pt.CreatedAt),

		Price:    displayPrice,
		Currency: pt.Price.Ticker(),

		Status: pt.PublicStatus().String(),
		Type:   pt.Type.String(),

		PaymentURL:  pt.PaymentURL,
		RedirectURL: pt.RedirectURL,

		Description: pt.Description,
		IsTest:      pt.IsTest,
	}

	if pt.Type == payment.TypePayment {
		info := &model.AdditionalPaymentInfo{}

		if tx != nil {
			info.SelectedCurrency = util.Ptr(tx.Currency.DisplayName())
			info.ServiceFee = util.Ptr(tx.ServiceFee.String())

			// Crypto amount received (use FactAmount if available, else expected Amount)
			cryptoAmt := tx.Amount.String()
			if tx.FactAmount != nil {
				cryptoAmt = tx.FactAmount.String()
			}
			info.CryptoAmount = util.Ptr(cryptoAmt)
			info.CryptoTicker = util.Ptr(tx.Currency.Ticker)

			// Blockchain transaction details for enterprise tracking
			if tx.HashID != nil && *tx.HashID != "" {
				info.TransactionHash = tx.HashID
				link, _ := tx.ExplorerLink()
				info.ExplorerLink = &link
			}
			if tx.SenderAddress != nil && *tx.SenderAddress != "" {
				info.SenderAddress = tx.SenderAddress
			}
			if tx.NetworkFee != nil {
				info.NetworkFee = util.Ptr(tx.NetworkFee.String())
			}
		}

		if customer != nil {
			info.CustomerEmail = &customer.Email
		}

		res.AdditionalInfo = &model.PaymentAdditionalInfo{Payment: info}
	}

	if pt.Type == payment.TypeWithdrawal {
		info := &model.AdditionalWithdrawalInfo{}

		if balance != nil {
			info.BalanceID = balance.UUID.String()
		}
		if tx != nil {
			info.ServiceFee = tx.ServiceFee.String()

			if tx.HashID != nil {
				info.TransactionHash = tx.HashID

				link, _ := tx.ExplorerLink()
				info.ExplorerLink = &link
			}
		}

		res.AdditionalInfo = &model.PaymentAdditionalInfo{Withdrawal: info}
	}

	return res
}
