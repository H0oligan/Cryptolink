package marketingapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/service/marketing"
	"github.com/rs/zerolog"
)

type Handler struct {
	service *marketing.Service
	logger  *zerolog.Logger
}

func New(service *marketing.Service, logger *zerolog.Logger) *Handler {
	log := logger.With().Str("channel", "marketing_api").Logger()
	return &Handler{service: service, logger: &log}
}

// ListTemplates returns the 5 predefined email templates
func (h *Handler) ListTemplates(c echo.Context) error {
	return c.JSON(http.StatusOK, marketing.GetTemplates())
}

// GetTemplate returns a single template by ID (for preview)
func (h *Handler) GetTemplate(c echo.Context) error {
	id := c.Param("templateId")
	t := marketing.GetTemplateByID(id)
	if t == nil {
		return common.ErrorResponse(c, "template not found")
	}
	return c.JSON(http.StatusOK, t)
}

// ListCampaigns returns paginated campaigns
func (h *Handler) ListCampaigns(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	campaigns, total, err := h.service.ListCampaigns(c.Request().Context(), limit, offset)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list campaigns")
		return common.ErrorResponse(c, "internal_error")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"results": campaigns,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// GetCampaign returns a single campaign with details
func (h *Handler) GetCampaign(c echo.Context) error {
	campaignUUID := c.Param("campaignId")
	campaign, err := h.service.GetCampaign(c.Request().Context(), campaignUUID)
	if err != nil {
		return common.ErrorResponse(c, "campaign not found")
	}
	return c.JSON(http.StatusOK, campaign)
}

// GetCampaignRecipients returns paginated recipients for a campaign
func (h *Handler) GetCampaignRecipients(c echo.Context) error {
	campaignUUID := c.Param("campaignId")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	recipients, total, err := h.service.GetCampaignRecipients(c.Request().Context(), campaignUUID, limit, offset)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get recipients")
		return common.ErrorResponse(c, "internal_error")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"results": recipients,
		"total":   total,
	})
}

type createCampaignRequest struct {
	Name       string `json:"name"`
	Subject    string `json:"subject"`
	BodyHTML   string `json:"body_html"`
	TemplateID string `json:"template_id"`
	Audience   string `json:"audience"`
}

// CreateCampaign creates a new campaign (draft status)
func (h *Handler) CreateCampaign(c echo.Context) error {
	var req createCampaignRequest
	if err := c.Bind(&req); err != nil {
		return common.ErrorResponse(c, "invalid request")
	}

	// If template_id is provided, pre-fill subject and body from template
	if req.TemplateID != "" && req.BodyHTML == "" {
		t := marketing.GetTemplateByID(req.TemplateID)
		if t != nil {
			req.BodyHTML = t.BodyHTML
			if req.Subject == "" {
				req.Subject = t.Subject
			}
		}
	}

	campaign, err := h.service.CreateCampaign(c.Request().Context(), marketing.CreateCampaignParams{
		Name:       req.Name,
		Subject:    req.Subject,
		BodyHTML:   req.BodyHTML,
		TemplateID: req.TemplateID,
		Audience:   req.Audience,
	})
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to create campaign")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, campaign)
}

// SendCampaign triggers sending (queues recipients)
func (h *Handler) SendCampaign(c echo.Context) error {
	campaignUUID := c.Param("campaignId")
	err := h.service.SendCampaign(c.Request().Context(), campaignUUID)
	if err != nil {
		h.logger.Error().Err(err).Str("campaign", campaignUUID).Msg("failed to send campaign")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "campaign queued for sending"})
}

// GetQuota returns current daily email quota status
func (h *Handler) GetQuota(c echo.Context) error {
	sent, limit, resetAt, err := h.service.GetQuotaStatus(c.Request().Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get quota")
		return common.ErrorResponse(c, "internal_error")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"sent":      sent,
		"limit":     limit,
		"remaining": limit - sent,
		"reset_at":  resetAt,
	})
}

// Unsubscribe handles the public unsubscribe link (no auth required)
func (h *Handler) Unsubscribe(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return c.HTML(http.StatusBadRequest, unsubPage("Invalid Link", "The unsubscribe link is invalid or expired.", "#ef4444"))
	}

	email, err := h.service.Unsubscribe(c.Request().Context(), token)
	if err != nil {
		return c.HTML(http.StatusBadRequest, unsubPage("Invalid Link", "The unsubscribe link is invalid or expired.", "#ef4444"))
	}

	return c.HTML(http.StatusOK, unsubPage("Unsubscribed", fmt.Sprintf("You (%s) have been unsubscribed from CryptoLink marketing emails.", email), "#10b981"))
}

func unsubPage(title, message, color string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s — CryptoLink</title></head>
<body style="margin:0;padding:0;background:#050505;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;">
<div style="background:#111;border:1px solid #1e1e1e;border-radius:12px;padding:40px;max-width:480px;text-align:center;">
  <h1 style="color:%s;font-size:24px;margin-bottom:12px;">%s</h1>
  <p style="color:#94a3b8;font-size:15px;line-height:1.6;">%s</p>
  <a href="https://cryptolink.cc" style="display:inline-block;margin-top:20px;color:#10b981;text-decoration:none;font-size:14px;">← Back to CryptoLink</a>
</div>
</body>
</html>`, title, color, title, message)
}
