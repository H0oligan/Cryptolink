package merchantapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/service/merchant"
)

// GetFeeSettings returns the merchant's fee/currency configuration.
func (h *Handler) GetFeeSettings(c echo.Context) error {
	mt := middleware.ResolveMerchant(c)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"preferredCurrency":  mt.Settings().FiatCurrency(),
		"globalFeePercentage": mt.Settings()["fee.global"],
	})
}

// UpdateFeeSettings updates the merchant's fee/currency configuration.
func (h *Handler) UpdateFeeSettings(c echo.Context) error {
	var req struct {
		PreferredCurrency  string            `json:"preferredCurrency"`
		GlobalFeePercentage string           `json:"globalFeePercentage"`
		PerCurrencyFees    map[string]string `json:"perCurrencyFees"`
	}
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	// Validate the chosen fiat currency against the canonical list
	if req.PreferredCurrency != "" {
		if !money.IsFiatCurrency(req.PreferredCurrency) {
			return common.ValidationErrorItemResponse(c, "preferredCurrency", "unsupported fiat currency: %s", req.PreferredCurrency)
		}
	}

	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	settings := merchant.Settings{}
	if req.PreferredCurrency != "" {
		settings[merchant.PropertyFiatCurrency] = req.PreferredCurrency
	}
	if req.GlobalFeePercentage != "" {
		settings[merchant.Property("fee.global")] = req.GlobalFeePercentage
	} else {
		settings[merchant.Property("fee.global")] = ""
	}

	if err := h.merchants.UpsertSettings(ctx, mt, settings); err != nil {
		h.logger.Error().Err(err).Msg("failed to update fee settings")
		return common.ErrorResponse(c, "internal_error")
	}

	return c.NoContent(http.StatusNoContent)
}

// ListFiatCurrencies returns all supported fiat currencies with their symbols.
func (h *Handler) ListFiatCurrencies(c echo.Context) error {
	currencies := money.SupportedFiatCurrencies()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"currencies": currencies,
	})
}
