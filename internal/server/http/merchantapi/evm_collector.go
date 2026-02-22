package merchantapi

import (
	"net/http"
	"strings"

	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/provider/tatum"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/service/evmcollector"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// ────────────────────────────────────────────────────────────────────────────
// Request / Response types
// ────────────────────────────────────────────────────────────────────────────

type setupCollectorRequest struct {
	Blockchain      string `json:"blockchain"`
	ChainID         int    `json:"chainId"`
	OwnerAddress    string `json:"ownerAddress"`
	ContractAddress string `json:"contractAddress"`
	FactoryAddress  string `json:"factoryAddress"`
}

type collectorResponse struct {
	Blockchain      string `json:"blockchain"`
	ChainID         int    `json:"chainId"`
	ContractAddress string `json:"contractAddress"`
	OwnerAddress    string `json:"ownerAddress"`
	FactoryAddress  string `json:"factoryAddress"`
	IsActive        bool   `json:"isActive"`
	CreatedAt       string `json:"createdAt"`
}

type nativeBalanceResponse struct {
	Amount    string `json:"amount"`
	Ticker    string `json:"ticker"`
	UsdAmount string `json:"usdAmount"`
}

type tokenBalanceResponse struct {
	Contract  string `json:"contract"`
	Ticker    string `json:"ticker"`
	Amount    string `json:"amount"`
	UsdAmount string `json:"usdAmount"`
}

type collectorBalanceResponse struct {
	Native nativeBalanceResponse  `json:"native"`
	Tokens []tokenBalanceResponse `json:"tokens"`
}

// ────────────────────────────────────────────────────────────────────────────
// Handlers
// ────────────────────────────────────────────────────────────────────────────

// ListEvmCollectors returns all active EVM collector wallets for the merchant.
func (h *Handler) ListEvmCollectors(c echo.Context) error {
	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	collectors, err := h.evmCollector.ListByMerchantID(ctx, mt.ID)
	if err != nil {
		return err
	}

	result := make([]collectorResponse, 0, len(collectors))
	for _, col := range collectors {
		result = append(result, toCollectorResponse(col))
	}

	return c.JSON(http.StatusOK, result)
}

// GetEvmCollector returns a single collector by blockchain name.
func (h *Handler) GetEvmCollector(c echo.Context) error {
	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)
	blockchain := strings.ToUpper(c.Param("blockchain"))

	col, err := h.evmCollector.GetByMerchantAndBlockchain(ctx, mt.ID, blockchain)
	switch {
	case errors.Is(err, evmcollector.ErrNotFound):
		return common.NotFoundResponse(c, "evm collector not found")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, toCollectorResponse(col))
}

// SetupEvmCollector registers a new EVM smart contract collector for the merchant.
// The contract_address should be pre-computed by the frontend using CREATE2 prediction
// (via wagmi/viem calling factory.predictAddress) or set to the merchant's wallet address.
func (h *Handler) SetupEvmCollector(c echo.Context) error {
	var req setupCollectorRequest
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	if req.Blockchain == "" {
		return common.ValidationErrorItemResponse(c, "blockchain", "blockchain is required")
	}
	if req.OwnerAddress == "" {
		return common.ValidationErrorItemResponse(c, "owner_address", "owner_address is required")
	}
	if req.ContractAddress == "" {
		return common.ValidationErrorItemResponse(c, "contract_address", "contract_address is required")
	}

	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	// Get chain config from app config
	chainCfg, ok := h.evmCollector.GetChainConfig(req.Blockchain)
	chainID := req.ChainID
	factoryAddress := req.FactoryAddress
	if ok {
		if chainCfg.ChainID != 0 {
			chainID = chainCfg.ChainID
		}
		if chainCfg.FactoryAddress != "" {
			factoryAddress = chainCfg.FactoryAddress
		}
	}

	col, err := h.evmCollector.RegisterCollector(
		ctx,
		mt.ID,
		req.Blockchain,
		chainID,
		req.ContractAddress,
		req.OwnerAddress,
		factoryAddress,
	)

	switch {
	case errors.Is(err, evmcollector.ErrAlreadyExists):
		return common.ValidationErrorResponse(c, "collector already exists for this blockchain")
	case err != nil:
		h.logger.Error().Err(err).Msg("unable to create evm collector")
		return err
	}

	// Subscribe to Tatum webhooks for the contract address
	webhookURL := h.evmCollector.WebhookURL(h.webhookBasePath, req.Blockchain, col.ChainID, col.UUID)
	subID, subErr := h.tatumProvider.SubscribeToWebhook(ctx, tatum.SubscriptionParams{
		Blockchain: money.Blockchain(strings.ToUpper(req.Blockchain)),
		Address:    col.ContractAddress,
		WebhookURL: webhookURL,
		IsTest:     false,
	})
	if subErr != nil {
		h.logger.Warn().Err(subErr).
			Str("contract_address", col.ContractAddress).
			Msg("unable to subscribe evm collector to tatum webhook (non-fatal)")
	} else if subID != "" {
		_ = h.evmCollector.UpdateSubscriptionID(ctx, col.ID, subID)
	}

	return c.JSON(http.StatusCreated, toCollectorResponse(col))
}

// DeleteEvmCollector soft-deletes an EVM collector for the merchant.
func (h *Handler) DeleteEvmCollector(c echo.Context) error {
	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)
	blockchain := strings.ToUpper(c.Param("blockchain"))

	err := h.evmCollector.Delete(ctx, mt.ID, blockchain)
	switch {
	case errors.Is(err, evmcollector.ErrNotFound):
		return common.NotFoundResponse(c, "evm collector not found")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// GetEvmCollectorBalance returns the collector's on-chain balance info.
// The actual balance query requires an RPC endpoint — currently returns the contract address
// so the frontend can query it directly via wagmi/viem.
func (h *Handler) GetEvmCollectorBalance(c echo.Context) error {
	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)
	blockchain := strings.ToUpper(c.Param("blockchain"))

	col, err := h.evmCollector.GetByMerchantAndBlockchain(ctx, mt.ID, blockchain)
	switch {
	case errors.Is(err, evmcollector.ErrNotFound):
		return common.NotFoundResponse(c, "evm collector not found")
	case err != nil:
		return err
	}

	bal, err := h.evmCollector.FetchBalance(ctx, col.Blockchain, col.ContractAddress)
	if err != nil {
		h.logger.Warn().Err(err).Str("blockchain", col.Blockchain).Msg("failed to fetch on-chain balance, returning zeros")
		bal = &evmcollector.OnChainBalance{NativeAmount: "0", NativeTicker: col.Blockchain, Tokens: nil}
	}

	tokens := make([]tokenBalanceResponse, 0, len(bal.Tokens))
	for _, t := range bal.Tokens {
		tokens = append(tokens, tokenBalanceResponse{
			Contract:  t.ContractAddress,
			Ticker:    t.Ticker,
			Amount:    t.Amount,
			UsdAmount: "0",
		})
	}

	return c.JSON(http.StatusOK, collectorBalanceResponse{
		Native: nativeBalanceResponse{
			Amount:    bal.NativeAmount,
			Ticker:    bal.NativeTicker,
			UsdAmount: "0",
		},
		Tokens: tokens,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────────────────

func toCollectorResponse(col *evmcollector.Collector) collectorResponse {
	return collectorResponse{
		Blockchain:      col.Blockchain,
		ChainID:         col.ChainID,
		ContractAddress: col.ContractAddress,
		OwnerAddress:    col.OwnerAddress,
		FactoryAddress:  col.FactoryAddress,
		IsActive:        col.IsActive,
		CreatedAt:       col.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

