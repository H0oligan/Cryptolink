package emailapi

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/service/email"
	"github.com/cryptolink/cryptolink/pkg/api-dashboard/v1/model"
	"github.com/rs/zerolog"
)

type Handler struct {
	emailService *email.Service
	logger       *zerolog.Logger
}

func New(emailService *email.Service, logger *zerolog.Logger) *Handler {
	log := logger.With().Str("channel", "email_api").Logger()
	return &Handler{emailService: emailService, logger: &log}
}

type SettingsResponse struct {
	SMTPHost  string `json:"smtp_host"`
	SMTPPort  int    `json:"smtp_port"`
	SMTPUser  string `json:"smtp_user"`
	FromName  string `json:"from_name"`
	FromEmail string `json:"from_email"`
	IsActive  bool   `json:"is_active"`
}

type UpdateSettingsRequest struct {
	SMTPHost  string `json:"smtp_host"`
	SMTPPort  int    `json:"smtp_port"`
	SMTPUser  string `json:"smtp_user"`
	SMTPPass  string `json:"smtp_pass"`
	FromName  string `json:"from_name"`
	FromEmail string `json:"from_email"`
	IsActive  bool   `json:"is_active"`
}

type SendEmailRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// GetSettings returns the current SMTP settings (passwords masked)
func (h *Handler) GetSettings(c echo.Context) error {
	ctx := c.Request().Context()

	settings, err := h.emailService.GetSettings(ctx)
	if err != nil {
		return c.JSON(http.StatusOK, &SettingsResponse{})
	}

	return c.JSON(200, &SettingsResponse{
		SMTPHost:  settings.SMTPHost,
		SMTPPort:  settings.SMTPPort,
		SMTPUser:  settings.SMTPUser,
		FromName:  settings.FromName,
		FromEmail: settings.FromEmail,
		IsActive:  settings.IsActive,
	})
}

// UpdateSettings updates SMTP settings
func (h *Handler) UpdateSettings(c echo.Context) error {
	ctx := c.Request().Context()

	var req UpdateSettingsRequest
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	settings := &email.EmailSettings{
		SMTPHost:  req.SMTPHost,
		SMTPPort:  req.SMTPPort,
		SMTPUser:  req.SMTPUser,
		SMTPPass:  req.SMTPPass,
		FromName:  req.FromName,
		FromEmail: req.FromEmail,
		IsActive:  req.IsActive,
	}

	updated, err := h.emailService.UpdateSettings(ctx, settings)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{
			Message: "failed to update settings: " + err.Error(),
			Status:  "internal_error",
		})
	}

	return c.JSON(200, &SettingsResponse{
		SMTPHost:  updated.SMTPHost,
		SMTPPort:  updated.SMTPPort,
		SMTPUser:  updated.SMTPUser,
		FromName:  updated.FromName,
		FromEmail: updated.FromEmail,
		IsActive:  updated.IsActive,
	})
}

// SendEmail sends an email
func (h *Handler) SendEmail(c echo.Context) error {
	ctx := c.Request().Context()

	var req SendEmailRequest
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	if req.To == "" || req.Subject == "" || req.Body == "" {
		return common.ValidationErrorResponse(c, "to, subject, and body are required")
	}

	err := h.emailService.SendEmail(ctx, email.SendEmailParams{
		To:       req.To,
		Subject:  req.Subject,
		Body:     req.Body,
		Template: "manual",
	})

	if err != nil {
		h.logger.Error().Err(err).Str("to", req.To).Msg("failed to send email")
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{
			Message: "failed to send email: " + err.Error(),
			Status:  "internal_error",
		})
	}

	return c.JSON(200, map[string]string{"message": "Email sent successfully"})
}

// TestEmail sends a test email to verify SMTP settings
func (h *Handler) TestEmail(c echo.Context) error {
	ctx := c.Request().Context()

	settings, err := h.emailService.GetSettings(ctx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &model.ErrorResponse{
			Message: "no email settings configured",
			Status:  "validation_error",
		})
	}

	err = h.emailService.SendEmail(ctx, email.SendEmailParams{
		To:       settings.FromEmail,
		Subject:  "[CryptoLink] Test Email",
		Body:     "<h2>Test Email</h2><p>This is a test email from CryptoLink. Your SMTP settings are working correctly.</p>",
		Template: "test",
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{
			Message: "test email failed: " + err.Error(),
			Status:  "internal_error",
		})
	}

	return c.JSON(200, map[string]string{"message": "Test email sent to " + settings.FromEmail})
}

// GetLogs returns email log entries
func (h *Handler) GetLogs(c echo.Context) error {
	ctx := c.Request().Context()

	limit := 50
	offset := 0
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	logs, total, err := h.emailService.GetLogs(ctx, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{
			Message: "failed to get email logs",
			Status:  "internal_error",
		})
	}

	return c.JSON(200, map[string]interface{}{
		"results": logs,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}
