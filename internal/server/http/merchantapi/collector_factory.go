package merchantapi

import (
	"net/http"
	"strings"

	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/service/evmcollector"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// ────────────────────────────────────────────────────────────────────────────
// Request / Response types
// ────────────────────────────────────────────────────────────────────────────

type upsertFactoryRequest struct {
	Blockchain            string `json:"blockchain"`
	ImplementationAddress string `json:"implementationAddress"`
	FactoryAddress        string `json:"factoryAddress"`
}

type factoryResponse struct {
	Blockchain            string `json:"blockchain"`
	ImplementationAddress string `json:"implementationAddress"`
	FactoryAddress        string `json:"factoryAddress"`
	CreatedAt             string `json:"createdAt"`
	UpdatedAt             string `json:"updatedAt"`
}

// ────────────────────────────────────────────────────────────────────────────
// Admin Handlers
// ────────────────────────────────────────────────────────────────────────────

// ListCollectorFactories returns all collector factory configs (admin only).
func (h *Handler) ListCollectorFactories(c echo.Context) error {
	ctx := c.Request().Context()

	factories, err := h.evmCollector.ListFactories(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("unable to list collector factories")
		return err
	}

	result := make([]factoryResponse, 0, len(factories))
	for _, f := range factories {
		result = append(result, toFactoryResponse(f))
	}

	return c.JSON(http.StatusOK, result)
}

// GetCollectorFactory returns the factory config for a specific blockchain (admin only).
func (h *Handler) GetCollectorFactory(c echo.Context) error {
	ctx := c.Request().Context()
	blockchain := strings.ToUpper(c.Param("blockchain"))

	f, err := h.evmCollector.GetFactoryByBlockchain(ctx, blockchain)
	switch {
	case errors.Is(err, evmcollector.ErrFactoryNotFound):
		return common.NotFoundResponse(c, "collector factory not found for "+blockchain)
	case err != nil:
		h.logger.Error().Err(err).Str("blockchain", blockchain).Msg("unable to get collector factory")
		return err
	}

	return c.JSON(http.StatusOK, toFactoryResponse(f))
}

// UpsertCollectorFactory creates or updates a collector factory config (admin only).
func (h *Handler) UpsertCollectorFactory(c echo.Context) error {
	var req upsertFactoryRequest
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	if req.Blockchain == "" {
		return common.ValidationErrorItemResponse(c, "blockchain", "blockchain is required")
	}
	if req.ImplementationAddress == "" {
		return common.ValidationErrorItemResponse(c, "implementationAddress", "implementation address is required")
	}
	if req.FactoryAddress == "" {
		return common.ValidationErrorItemResponse(c, "factoryAddress", "factory address is required")
	}

	ctx := c.Request().Context()

	factory := &evmcollector.CollectorFactory{
		Blockchain:            req.Blockchain,
		ImplementationAddress: req.ImplementationAddress,
		FactoryAddress:        req.FactoryAddress,
	}

	result, err := h.evmCollector.UpsertFactory(ctx, factory)
	if err != nil {
		h.logger.Error().Err(err).Str("blockchain", req.Blockchain).Msg("unable to upsert collector factory")
		return err
	}

	return c.JSON(http.StatusOK, toFactoryResponse(result))
}

// ────────────────────────────────────────────────────────────────────────────
// Merchant Handler
// ────────────────────────────────────────────────────────────────────────────

// GetMerchantCollectorFactory returns the factory address for a blockchain so
// the merchant frontend can call the factory contract to deploy a collector.
func (h *Handler) GetMerchantCollectorFactory(c echo.Context) error {
	ctx := c.Request().Context()
	_ = middleware.ResolveMerchant(c) // ensures merchant context is valid
	blockchain := strings.ToUpper(c.Param("blockchain"))

	f, err := h.evmCollector.GetFactoryByBlockchain(ctx, blockchain)
	switch {
	case errors.Is(err, evmcollector.ErrFactoryNotFound):
		return common.NotFoundResponse(c, "no collector factory configured for "+blockchain)
	case err != nil:
		h.logger.Error().Err(err).Str("blockchain", blockchain).Msg("unable to get collector factory for merchant")
		return err
	}

	return c.JSON(http.StatusOK, toFactoryResponse(f))
}

// ────────────────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────────────────

func toFactoryResponse(f *evmcollector.CollectorFactory) factoryResponse {
	return factoryResponse{
		Blockchain:            f.Blockchain,
		ImplementationAddress: f.ImplementationAddress,
		FactoryAddress:        f.FactoryAddress,
		CreatedAt:             f.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:             f.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
