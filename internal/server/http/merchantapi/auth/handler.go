package auth

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/auth"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/service/email"
	"github.com/cryptolink/cryptolink/internal/service/user"
	"github.com/cryptolink/cryptolink/internal/util"
	"github.com/cryptolink/cryptolink/pkg/api-dashboard/v1/model"
	"github.com/rs/zerolog"
)

// Handler user session auth handler. Uses Google OAuth.
type Handler struct {
	googleAuth       *auth.GoogleOAuthManager
	users            *user.Service
	emailService     *email.Service
	enabledProviders []auth.ProviderType
	logger           *zerolog.Logger
}

func NewHandler(
	googleAuth *auth.GoogleOAuthManager,
	users *user.Service,
	emailService *email.Service,
	enabledProviders []auth.ProviderType,
	logger *zerolog.Logger,
) *Handler {
	log := logger.With().Str("channel", "auth_handler").Logger()

	return &Handler{
		googleAuth:       googleAuth,
		users:            users,
		emailService:     emailService,
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
		UUID:             person.UUID.String(),
		Email:            person.Email,
		Name:             person.Name,
		ProfileImageURL:  person.ProfileImageURL,
		IsSuperAdmin:     person.IsSuperAdmin,
		CompanyName:      person.CompanyName,
		Address:          person.Address,
		Website:          person.Website,
		Phone:            person.Phone,
		EmailVerified:    person.EmailVerified,
		MarketingConsent: person.MarketingConsent,
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
		Name        string `json:"name"`
		Email       string `json:"email"`
		CompanyName string `json:"companyName"`
		Address     string `json:"address"`
		Website     string `json:"website"`
		Phone       string `json:"phone"`
	}
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request")
	}

	updated, err := h.users.UpdateProfile(c.Request().Context(), person.ID, user.UpdateProfileParams{
		Name:        req.Name,
		Email:       req.Email,
		CompanyName: req.CompanyName,
		Address:     req.Address,
		Website:     req.Website,
		Phone:       req.Phone,
	})
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to update profile")
		return common.ErrorResponse(c, err.Error())
	}

	return c.JSON(http.StatusOK, &model.User{
		UUID:             updated.UUID.String(),
		Email:            updated.Email,
		Name:             updated.Name,
		ProfileImageURL:  updated.ProfileImageURL,
		IsSuperAdmin:     updated.IsSuperAdmin,
		CompanyName:      updated.CompanyName,
		Address:          updated.Address,
		Website:          updated.Website,
		Phone:            updated.Phone,
		EmailVerified:    updated.EmailVerified,
		MarketingConsent: updated.MarketingConsent,
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

// VerifyEmail handles GET /auth/verify-email?token=xxx
func (h *Handler) VerifyEmail(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return c.Redirect(http.StatusTemporaryRedirect, "/merchants/login?error=missing_token")
	}

	_, err := h.users.VerifyEmail(c.Request().Context(), token)
	if err != nil {
		h.logger.Error().Err(err).Msg("email verification failed")
		return c.Redirect(http.StatusTemporaryRedirect, "/merchants/login?error=invalid_token")
	}

	return c.Redirect(http.StatusTemporaryRedirect, "/merchants/?verified=true")
}

// ResendVerification handles POST /auth/resend-verification
func (h *Handler) ResendVerification(c echo.Context) error {
	person := middleware.ResolveUser(c)
	if person == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "not authenticated"})
	}

	if person.EmailVerified {
		return c.JSON(http.StatusOK, map[string]string{"message": "Email already verified"})
	}

	if h.emailService == nil {
		return common.ErrorResponse(c, "email service not available")
	}

	token, err := h.users.GenerateVerificationToken(c.Request().Context(), person.ID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to generate verification token")
		return common.ErrorResponse(c, "failed to generate verification token")
	}

	go h.emailService.SendVerificationEmail(context.Background(), person.Email, person.Name, token)

	return c.JSON(http.StatusOK, map[string]string{"message": "Verification email sent"})
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
