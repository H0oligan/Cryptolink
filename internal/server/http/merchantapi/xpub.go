package merchantapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/service/xpub"
	"github.com/pkg/errors"
)

// XpubWalletRequest represents the request to create an xpub wallet
type XpubWalletRequest struct {
	Blockchain     string `json:"blockchain" validate:"required"`
	Xpub           string `json:"xpub" validate:"required"`
	DerivationPath string `json:"derivationPath" validate:"required"`
}

// XpubWalletResponse represents an xpub wallet response
type XpubWalletResponse struct {
	UUID             string `json:"uuid"`
	Blockchain       string `json:"blockchain"`
	DerivationPath   string `json:"derivationPath"`
	LastDerivedIndex int    `json:"lastDerivedIndex"`
	CreatedAt        string `json:"createdAt"`
}

// DerivedAddressResponse represents a derived address response
type DerivedAddressResponse struct {
	UUID            string  `json:"uuid"`
	Address         string  `json:"address"`
	DerivationPath  string  `json:"derivationPath"`
	DerivationIndex int     `json:"derivationIndex"`
	IsUsed          bool    `json:"isUsed"`
	CreatedAt       string  `json:"createdAt"`
}

// CreateXpubWallet creates a new xpub wallet for a merchant
func (h *Handler) CreateXpubWallet(c echo.Context) error {
	ctx := c.Request().Context()
	merchant := middleware.ResolveMerchant(c)

	var req XpubWalletRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Basic validation
	if req.Blockchain == "" || req.Xpub == "" || req.DerivationPath == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "blockchain, xpub, and derivationPath are required"})
	}

	wallet, err := h.xpubService.CreateXpubWallet(ctx, merchant.ID, req.Blockchain, req.Xpub, req.DerivationPath)
	switch {
	case errors.Is(err, xpub.ErrAlreadyExists):
		return common.ValidationErrorItemResponse(c, "blockchain", "Xpub wallet already exists for this blockchain")
	case errors.Is(err, xpub.ErrInvalidXpub):
		return common.ValidationErrorItemResponse(c, "xpub", "Invalid xpub format")
	case err != nil:
		return errors.Wrap(err, "unable to create xpub wallet")
	}

	return c.JSON(http.StatusCreated, &XpubWalletResponse{
		UUID:             wallet.UUID.String(),
		Blockchain:       wallet.Blockchain,
		DerivationPath:   wallet.DerivationPath,
		LastDerivedIndex: wallet.LastDerivedIndex,
		CreatedAt:        wallet.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// ListXpubWallets lists all xpub wallets for a merchant
func (h *Handler) ListXpubWallets(c echo.Context) error {
	ctx := c.Request().Context()
	merchant := middleware.ResolveMerchant(c)

	wallets, err := h.xpubService.ListByMerchantID(ctx, merchant.ID)
	if err != nil {
		return errors.Wrap(err, "unable to list xpub wallets")
	}

	response := make([]*XpubWalletResponse, len(wallets))
	for i, wallet := range wallets {
		response[i] = &XpubWalletResponse{
			UUID:             wallet.UUID.String(),
			Blockchain:       wallet.Blockchain,
			DerivationPath:   wallet.DerivationPath,
			LastDerivedIndex: wallet.LastDerivedIndex,
			CreatedAt:        wallet.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return c.JSON(http.StatusOK, response)
}

// GetXpubWallet gets an xpub wallet by UUID
func (h *Handler) GetXpubWallet(c echo.Context) error {
	ctx := c.Request().Context()

	walletUUID, err := uuid.Parse(c.Param("walletId"))
	if err != nil {
		return common.ValidationErrorItemResponse(c, "walletId", "Invalid wallet UUID")
	}

	wallet, err := h.xpubService.GetByUUID(ctx, walletUUID)
	switch {
	case errors.Is(err, xpub.ErrNotFound):
		return common.ErrorResponse(c, "Xpub wallet not found")
	case err != nil:
		return errors.Wrap(err, "unable to get xpub wallet")
	}

	return c.JSON(http.StatusOK, &XpubWalletResponse{
		UUID:             wallet.UUID.String(),
		Blockchain:       wallet.Blockchain,
		DerivationPath:   wallet.DerivationPath,
		LastDerivedIndex: wallet.LastDerivedIndex,
		CreatedAt:        wallet.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// DeleteXpubWallet deactivates an xpub wallet
func (h *Handler) DeleteXpubWallet(c echo.Context) error {
	ctx := c.Request().Context()
	merchant := middleware.ResolveMerchant(c)

	walletUUID, err := uuid.Parse(c.Param("walletId"))
	if err != nil {
		return common.ValidationErrorItemResponse(c, "walletId", "Invalid wallet UUID")
	}

	err = h.xpubService.DeactivateWallet(ctx, walletUUID, merchant.ID)
	switch {
	case errors.Is(err, xpub.ErrNotFound):
		return common.ErrorResponse(c, "Xpub wallet not found")
	case err != nil:
		return errors.Wrap(err, "unable to delete xpub wallet")
	}

	return c.NoContent(http.StatusNoContent)
}

// DeriveAddress derives a new address for an xpub wallet
func (h *Handler) DeriveAddress(c echo.Context) error {
	ctx := c.Request().Context()

	walletUUID, err := uuid.Parse(c.Param("walletId"))
	if err != nil {
		return common.ValidationErrorItemResponse(c, "walletId", "Invalid wallet UUID")
	}

	wallet, err := h.xpubService.GetByUUID(ctx, walletUUID)
	switch {
	case errors.Is(err, xpub.ErrNotFound):
		return common.ErrorResponse(c, "Xpub wallet not found")
	case err != nil:
		return errors.Wrap(err, "unable to get xpub wallet")
	}

	address, err := h.xpubService.DeriveAddress(ctx, wallet.ID)
	if err != nil {
		return errors.Wrap(err, "unable to derive address")
	}

	return c.JSON(http.StatusCreated, &DerivedAddressResponse{
		UUID:            address.UUID.String(),
		Address:         address.Address,
		DerivationPath:  address.DerivationPath,
		DerivationIndex: address.DerivationIndex,
		IsUsed:          address.IsUsed,
		CreatedAt:       address.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// GetNextAddress gets the next unused address for an xpub wallet
func (h *Handler) GetNextAddress(c echo.Context) error {
	ctx := c.Request().Context()

	walletUUID, err := uuid.Parse(c.Param("walletId"))
	if err != nil {
		return common.ValidationErrorItemResponse(c, "walletId", "Invalid wallet UUID")
	}

	wallet, err := h.xpubService.GetByUUID(ctx, walletUUID)
	switch {
	case errors.Is(err, xpub.ErrNotFound):
		return common.ErrorResponse(c, "Xpub wallet not found")
	case err != nil:
		return errors.Wrap(err, "unable to get xpub wallet")
	}

	address, err := h.xpubService.GetNextUnusedAddress(ctx, wallet.ID)
	if err != nil {
		return errors.Wrap(err, "unable to get next address")
	}

	return c.JSON(http.StatusOK, &DerivedAddressResponse{
		UUID:            address.UUID.String(),
		Address:         address.Address,
		DerivationPath:  address.DerivationPath,
		DerivationIndex: address.DerivationIndex,
		IsUsed:          address.IsUsed,
		CreatedAt:       address.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// ListDerivedAddresses lists all derived addresses for an xpub wallet
func (h *Handler) ListDerivedAddresses(c echo.Context) error {
	ctx := c.Request().Context()

	walletUUID, err := uuid.Parse(c.Param("walletId"))
	if err != nil {
		return common.ValidationErrorItemResponse(c, "walletId", "Invalid wallet UUID")
	}

	wallet, err := h.xpubService.GetByUUID(ctx, walletUUID)
	switch {
	case errors.Is(err, xpub.ErrNotFound):
		return common.ErrorResponse(c, "Xpub wallet not found")
	case err != nil:
		return errors.Wrap(err, "unable to get xpub wallet")
	}

	addresses, err := h.xpubService.ListDerivedAddresses(ctx, wallet.ID)
	if err != nil {
		return errors.Wrap(err, "unable to list derived addresses")
	}

	response := make([]*DerivedAddressResponse, len(addresses))
	for i, addr := range addresses {
		response[i] = &DerivedAddressResponse{
			UUID:            addr.UUID.String(),
			Address:         addr.Address,
			DerivationPath:  addr.DerivationPath,
			DerivationIndex: addr.DerivationIndex,
			IsUsed:          addr.IsUsed,
			CreatedAt:       addr.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return c.JSON(http.StatusOK, response)
}
