package webhook

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/log"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/service/processing"
	"github.com/pkg/errors"
)

// ReceiveWebhook handles incoming payment notification webhooks.
// The route URL is kept at /api/webhook/v1/tatum/:networkId/:walletId for backwards
// compatibility with the address watcher, but the handler is provider-agnostic.
func (h *Handler) ReceiveWebhook(c echo.Context) error {
	ctx := c.Request().Context()

	// 1. Parse request params
	networkID := c.Param(paramNetworkID)

	walletID, err := common.UUID(c, paramWalletID)
	if err != nil {
		return err
	}

	// 2. Verify signature (currently a no-op; the internal watcher is trusted)
	signature := c.Request().Header.Get(headerWebhookHMAC)
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}

	if err := h.processing.ValidateWebhookSignature(body, signature); err != nil {
		h.logger.Error().Err(err).
			EmbedObject(log.Ctx(ctx)).
			Int("body_bytes", len(body)).
			Msg("invalid webhook signature")

		return common.ValidationErrorResponse(c, errors.New("invalid signature"))
	}

	// 3. Parse request
	var req processing.IncomingWebhook
	if err := json.Unmarshal(body, &req); err != nil {
		return err
	}

	// 4. Process incoming webhook
	if err := h.processing.ProcessIncomingWebhook(ctx, walletID, networkID, req); err != nil {
		h.logger.Error().Err(err).
			Str("wallet_id", walletID.String()).Interface("webhook", req).
			Msg("unable to process incoming webhook")

		return c.JSON(http.StatusBadRequest, "unable to process incoming webhook")
	}

	h.logger.Info().
		Str("wallet_id", walletID.String()).
		Interface("webhook", req).
		Msg("processed incoming webhook")

	return c.NoContent(http.StatusNoContent)
}
