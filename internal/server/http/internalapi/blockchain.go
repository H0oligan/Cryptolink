package internalapi

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	admin "github.com/cryptolink/cryptolink/pkg/api-admin/v1/model"
)

func (h *Handler) CalculateTransactionFee(c echo.Context) error {
	ctx := c.Request().Context()

	req := &admin.EstimateFeesRequest{}
	if !common.BindAndValidateRequest(c, req) {
		return nil
	}

	currency, err := h.blockchain.GetCurrencyByTicker(req.Currency)
	if err != nil {
		return common.ErrorResponse(c, err.Error())
	}

	baseCurrency, err := h.blockchain.GetNativeCoin(currency.Blockchain)
	if err != nil {
		return common.ErrorResponse(c, err.Error())
	}

	fee, err := h.blockchain.CalculateFee(ctx, baseCurrency, currency, req.IsTest)
	if err != nil {
		return common.ErrorResponse(c, err.Error())
	}

	return c.JSON(http.StatusOK, fee)
}

func (h *Handler) BroadcastTransaction(c echo.Context) error {
	ctx := c.Request().Context()

	req := &admin.BroadcastTransactionRequest{}
	if !common.BindAndValidateRequest(c, req) {
		return nil
	}

	txHashID, err := h.blockchain.BroadcastTransaction(ctx, money.Blockchain(req.Blockchain), req.Hex, req.IsTest)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &admin.ErrorResponse{
			Message: err.Error(),
			Status:  "broadcast_error",
		})
	}

	return c.JSON(http.StatusOK, &admin.BroadcastTransactionResponse{
		TransactionHashID: txHashID,
	})
}

func (h *Handler) GetTransactionReceipt(c echo.Context) error {
	ctx := c.Request().Context()

	blockchain := money.Blockchain(c.QueryParam("blockchain"))
	if blockchain == "" {
		return common.ValidationErrorItemResponse(c, "blockchain", "required")
	}

	transactionID := c.QueryParam("txId")
	if transactionID == "" {
		return common.ValidationErrorItemResponse(c, "txId", "required")
	}

	var isTest bool
	if isTestRaw := c.QueryParam("isTest"); isTestRaw != "" {
		b, err := strconv.ParseBool(isTestRaw)
		if err != nil {
			return common.ValidationErrorItemResponse(c, "isTest", "invalid value")
		}
		isTest = b
	}

	receipt, err := h.blockchain.GetTransactionReceipt(ctx, blockchain, transactionID, isTest)
	if err != nil {
		return common.ErrorResponse(c, err.Error())
	}

	return c.JSON(http.StatusOK, &admin.TransactionReceiptResponse{
		Blockchain:      receipt.Blockchain.String(),
		TransactionHash: receipt.Hash,
		Nonce:           int64(receipt.Nonce),

		Recipient: receipt.Recipient,
		Sender:    receipt.Sender,

		NetworkFee:          receipt.NetworkFee.StringRaw(),
		NetworkFeeFormatted: receipt.NetworkFee.String(),

		Confirmations: receipt.Confirmations,
		IsConfirmed:   receipt.IsConfirmed,

		Success: receipt.Success,
		IsTest:  receipt.IsTest,
	})
}
