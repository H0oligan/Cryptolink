package auth

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/service/user"
	"github.com/cryptolink/cryptolink/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
)

func (h *Handler) PostLogin(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.LoginRequest
	if !common.BindAndValidateRequest(c, &req) {
		return nil
	}

	// already logged in
	if u := middleware.ResolveUser(c); u != nil {
		return c.NoContent(http.StatusNoContent)
	}

	person, err := h.users.GetByEmailWithPasswordCheck(ctx, req.Email.String(), req.Password)
	switch {
	case errors.Is(err, user.ErrNotFound), errors.Is(err, user.ErrWrongPassword):
		return common.ValidationErrorItemResponse(c, "email", "User with provided email or password not found")
	case err != nil:
		return errors.Wrap(err, "unable to resolve user")
	}

	setSession := map[string]any{middleware.UserIDContextKey: person.ID}
	if err := h.persistSession(c, "email", setSession); err != nil {
		return common.ErrorResponse(c, "internal error")
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) PostRegister(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.LoginRequest
	if !common.BindAndValidateRequest(c, &req) {
		return nil
	}

	// already logged in
	if u := middleware.ResolveUser(c); u != nil {
		return c.NoContent(http.StatusNoContent)
	}

	// GDPR: terms must be accepted
	if !req.TermsAccepted {
		return common.ValidationErrorItemResponse(c, "termsAccepted", "You must accept the Terms of Service")
	}

	person, err := h.users.Register(ctx, user.RegisterParams{
		Email:            req.Email.String(),
		Password:         req.Password,
		Name:             req.Name,
		CompanyName:      req.CompanyName,
		Address:          req.Address,
		Website:          req.Website,
		Phone:            req.Phone,
		MarketingConsent: req.MarketingConsent,
	})
	switch {
	case errors.Is(err, user.ErrAlreadyExists):
		return common.ValidationErrorItemResponse(c, "email", "User with this email already exists")
	case err != nil:
		return errors.Wrap(err, "unable to register user")
	}

	// Generate verification token and send email (best-effort)
	if h.emailService != nil {
		token, err := h.users.GenerateVerificationToken(ctx, person.ID)
		if err != nil {
			h.logger.Error().Err(err).Msg("failed to generate verification token")
		} else {
			go h.emailService.SendVerificationEmail(context.Background(), person.Email, person.Name, token)
		}
	}

	// Auto-login after registration
	setSession := map[string]any{middleware.UserIDContextKey: person.ID}
	if err := h.persistSession(c, "email", setSession); err != nil {
		return common.ErrorResponse(c, "internal error")
	}

	return c.NoContent(http.StatusCreated)
}
