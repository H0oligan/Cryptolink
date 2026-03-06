package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/auth"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/service/user"
	"github.com/cryptolink/cryptolink/internal/util"
	"github.com/cryptolink/cryptolink/pkg/api-dashboard/v1/model"
	"github.com/rs/zerolog"
)

// Handler user session auth handler. Uses Google OAuth.
type Handler struct {
	googleAuth       *auth.GoogleOAuthManager
	users            *user.Service
	enabledProviders []auth.ProviderType
	logger           *zerolog.Logger
}

func NewHandler(
	googleAuth *auth.GoogleOAuthManager,
	users *user.Service,
	enabledProviders []auth.ProviderType,
	logger *zerolog.Logger,
) *Handler {
	log := logger.With().Str("channel", "auth_handler").Logger()

	return &Handler{
		googleAuth:       googleAuth,
		users:            users,
		enabledProviders: enabledProviders,
		logger:           &log,
	}
}

// GetCookie returns csrf cookie & csrf header in the same response.
func (h *Handler) GetCookie(c echo.Context) error {
	tokenRaw := c.Get("csrf")
	token, ok := tokenRaw.(string)
	if !ok {
		return common.ErrorResponse(c, "internal_error")
	}

	c.Response().Header().Set(echo.HeaderXCSRFToken, token)
	c.Response().Header().Set(echo.HeaderAccessControlExposeHeaders, middleware.CSRFTokenHeader)

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ListAvailableProviders(c echo.Context) error {
	providers := util.MapSlice(h.enabledProviders, func(t auth.ProviderType) *model.Provider {
		return &model.Provider{Name: string(t)}
	})

	return c.JSON(http.StatusOK, &model.AvailableProvidersResponse{Providers: providers})
}

func (h *Handler) GetMe(c echo.Context) error {
	person := middleware.ResolveUser(c)

	return c.JSON(http.StatusOK, &model.User{
		UUID:            person.UUID.String(),
		Email:           person.Email,
		Name:            person.Name,
		ProfileImageURL: person.ProfileImageURL,
		IsSuperAdmin:    person.IsSuperAdmin,
	})
}

func (h *Handler) PostLogout(c echo.Context) error {
	userSession := middleware.ResolveSession(c)
	userSession.Values["user_id"] = nil
	if err := userSession.Save(c.Request(), c.Response()); err != nil {
		h.logger.Error().Err(err).Msg("unable to persist user session")
	}

	return c.NoContent(http.StatusNoContent)
}

// UpdateProfile handles PUT /auth/profile
func (h *Handler) UpdateProfile(c echo.Context) error {
	person := middleware.ResolveUser(c)

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request")
	}

	updated, err := h.users.UpdateProfile(c.Request().Context(), person.ID, req.Name, req.Email)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to update profile")
		return common.ErrorResponse(c, err.Error())
	}

	return c.JSON(http.StatusOK, &model.User{
		UUID:            updated.UUID.String(),
		Email:           updated.Email,
		Name:            updated.Name,
		ProfileImageURL: updated.ProfileImageURL,
		IsSuperAdmin:    updated.IsSuperAdmin,
	})
}

// UpdatePassword handles PUT /auth/password
func (h *Handler) UpdatePassword(c echo.Context) error {
	person := middleware.ResolveUser(c)

	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request")
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		return common.ValidationErrorResponse(c, "currentPassword and newPassword are required")
	}

	// Verify current password
	_, err := h.users.GetByEmailWithPasswordCheck(c.Request().Context(), person.Email, req.CurrentPassword)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &model.ErrorResponse{
			Message: "Current password is incorrect",
			Status:  "validation_error",
		})
	}

	// Update password
	_, err = h.users.UpdatePassword(c.Request().Context(), person.ID, req.NewPassword)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to update password")
		return common.ErrorResponse(c, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Password updated successfully"})
}

func (h *Handler) persistSession(c echo.Context, source string, values map[string]any) error {
	s := middleware.ResolveSession(c)

	for k, v := range values {
		s.Values[k] = v
	}

	if err := s.Save(c.Request(), c.Response()); err != nil {
		h.logger.Error().Err(err).Str("source", source).Msg("unable to persist user session")
		return err
	}

	return nil
}
