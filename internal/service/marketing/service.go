package marketing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/cryptolink/cryptolink/internal/service/email"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Daily marketing email limit (reserve 100 of Brevo's 300 for transactional)
const DailyMarketingLimit = 200

// Madrid timezone for quota reset at 17:00
var madridTZ *time.Location

func init() {
	var err error
	madridTZ, err = time.LoadLocation("Europe/Madrid")
	if err != nil {
		madridTZ = time.FixedZone("CET", 3600)
	}
}

type Service struct {
	db           *pgxpool.Pool
	emailService *email.Service
	logger       *zerolog.Logger
	stopCh       chan struct{}
}

func New(db *pgxpool.Pool, emailService *email.Service, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "marketing_service").Logger()
	return &Service{
		db:           db,
		emailService: emailService,
		logger:       &log,
		stopCh:       make(chan struct{}),
	}
}

// Campaign model
type Campaign struct {
	ID              int64   `json:"id"`
	UUID            string  `json:"uuid"`
	Name            string  `json:"name"`
	Subject         string  `json:"subject"`
	BodyHTML        string  `json:"body_html"`
	TemplateID      *string `json:"template_id"`
	Audience        string  `json:"audience"`
	Status          string  `json:"status"`
	TotalRecipients int     `json:"total_recipients"`
	SentCount       int     `json:"sent_count"`
	FailedCount     int     `json:"failed_count"`
	PendingCount    int     `json:"pending_count"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	StartedAt       *string `json:"started_at"`
	CompletedAt     *string `json:"completed_at"`
}

type Recipient struct {
	ID        int64   `json:"id"`
	Email     string  `json:"email"`
	Status    string  `json:"status"`
	SentAt    *string `json:"sent_at"`
	ErrorMsg  *string `json:"error_message"`
}

type CreateCampaignParams struct {
	Name       string `json:"name"`
	Subject    string `json:"subject"`
	BodyHTML   string `json:"body_html"`
	TemplateID string `json:"template_id"`
	Audience   string `json:"audience"`
}

// --- Campaign CRUD ---

func (s *Service) CreateCampaign(ctx context.Context, params CreateCampaignParams) (*Campaign, error) {
	if params.Name == "" || params.Subject == "" || params.BodyHTML == "" {
		return nil, errors.New("name, subject, and body are required")
	}
	if params.Audience == "" {
		params.Audience = "contacts_opted_in"
	}
	validAudiences := map[string]bool{"merchants": true, "contacts_opted_in": true, "all": true}
	if !validAudiences[params.Audience] {
		return nil, errors.New("audience must be merchants, contacts_opted_in, or all")
	}

	id := uuid.New()
	var campaign Campaign
	var createdAt, updatedAt time.Time
	var templateID sql.NullString
	if params.TemplateID != "" {
		templateID = sql.NullString{String: params.TemplateID, Valid: true}
	}

	err := s.db.QueryRow(ctx,
		`INSERT INTO marketing_campaigns (uuid, name, subject, body_html, template_id, audience)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, uuid, name, subject, body_html, template_id, audience, status,
		           total_recipients, sent_count, failed_count, pending_count, created_at, updated_at`,
		id.String(), params.Name, params.Subject, params.BodyHTML, templateID, params.Audience,
	).Scan(&campaign.ID, &campaign.UUID, &campaign.Name, &campaign.Subject, &campaign.BodyHTML,
		&templateID, &campaign.Audience, &campaign.Status,
		&campaign.TotalRecipients, &campaign.SentCount, &campaign.FailedCount, &campaign.PendingCount,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create campaign")
	}

	campaign.CreatedAt = createdAt.Format(time.RFC3339)
	campaign.UpdatedAt = updatedAt.Format(time.RFC3339)
	if templateID.Valid {
		campaign.TemplateID = &templateID.String
	}
	return &campaign, nil
}

func (s *Service) ListCampaigns(ctx context.Context, limit, offset int) ([]*Campaign, int, error) {
	if limit <= 0 {
		limit = 20
	}
	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM marketing_campaigns`).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, "failed to count campaigns")
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, uuid, name, subject, template_id, audience, status,
		        total_recipients, sent_count, failed_count, pending_count,
		        created_at, updated_at, started_at, completed_at
		 FROM marketing_campaigns ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to list campaigns")
	}
	defer rows.Close()

	var campaigns []*Campaign
	for rows.Next() {
		c, err := scanCampaignRow(rows)
		if err != nil {
			return nil, 0, err
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, total, nil
}

func (s *Service) GetCampaign(ctx context.Context, campaignUUID string) (*Campaign, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, uuid, name, subject, template_id, audience, status,
		        total_recipients, sent_count, failed_count, pending_count,
		        created_at, updated_at, started_at, completed_at
		 FROM marketing_campaigns WHERE uuid = $1`, campaignUUID)

	c, err := scanCampaignSingle(row)
	if err != nil {
		return nil, errors.Wrap(err, "campaign not found")
	}
	return c, nil
}

func (s *Service) GetCampaignRecipients(ctx context.Context, campaignUUID string, limit, offset int) ([]*Recipient, int, error) {
	if limit <= 0 {
		limit = 50
	}

	var campaignID int64
	if err := s.db.QueryRow(ctx, `SELECT id FROM marketing_campaigns WHERE uuid = $1`, campaignUUID).Scan(&campaignID); err != nil {
		return nil, 0, errors.Wrap(err, "campaign not found")
	}

	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM marketing_campaign_recipients WHERE campaign_id = $1`, campaignID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, email, status, sent_at, error_message FROM marketing_campaign_recipients
		 WHERE campaign_id = $1 ORDER BY id LIMIT $2 OFFSET $3`, campaignID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var recipients []*Recipient
	for rows.Next() {
		var r Recipient
		var sentAt sql.NullTime
		var errMsg sql.NullString
		if err := rows.Scan(&r.ID, &r.Email, &r.Status, &sentAt, &errMsg); err != nil {
			return nil, 0, err
		}
		if sentAt.Valid {
			t := sentAt.Time.Format(time.RFC3339)
			r.SentAt = &t
		}
		if errMsg.Valid {
			r.ErrorMsg = &errMsg.String
		}
		recipients = append(recipients, &r)
	}
	return recipients, total, nil
}

// --- Send Campaign ---

func (s *Service) SendCampaign(ctx context.Context, campaignUUID string) error {
	campaign, err := s.GetCampaign(ctx, campaignUUID)
	if err != nil {
		return err
	}
	if campaign.Status != "draft" && campaign.Status != "paused" {
		return errors.Errorf("campaign is %s, cannot send", campaign.Status)
	}

	// 1. Resolve audience emails
	emails, err := s.resolveAudience(ctx, campaign.Audience)
	if err != nil {
		return errors.Wrap(err, "failed to resolve audience")
	}

	// 2. Filter out unsubscribed
	emails, err = s.filterUnsubscribed(ctx, emails)
	if err != nil {
		return errors.Wrap(err, "failed to filter unsubscribed")
	}

	if len(emails) == 0 {
		return errors.New("no eligible recipients after filtering unsubscribes")
	}

	// 3. Insert recipients
	for _, e := range emails {
		_, err := s.db.Exec(ctx,
			`INSERT INTO marketing_campaign_recipients (campaign_id, email)
			 VALUES ($1, $2) ON CONFLICT (campaign_id, email) DO NOTHING`,
			campaign.ID, e)
		if err != nil {
			s.logger.Warn().Err(err).Str("email", e).Msg("failed to insert recipient")
		}
	}

	// 4. Update campaign status and counts
	now := time.Now()
	_, err = s.db.Exec(ctx,
		`UPDATE marketing_campaigns SET status = 'sending', total_recipients = $2, pending_count = $2,
		        started_at = $3, updated_at = $3 WHERE id = $1`,
		campaign.ID, len(emails), now)
	if err != nil {
		return errors.Wrap(err, "failed to update campaign status")
	}

	s.logger.Info().Str("campaign", campaign.Name).Int("recipients", len(emails)).Msg("campaign queued for sending")
	return nil
}

// --- Queue Processor (background worker) ---

func (s *Service) StartQueueProcessor(ctx context.Context) {
	s.logger.Info().Msg("starting marketing queue processor")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info().Msg("stopping marketing queue processor")
			return
		case <-s.stopCh:
			s.logger.Info().Msg("stopping marketing queue processor")
			return
		case <-ticker.C:
			s.processQueue(ctx)
		}
	}
}

func (s *Service) Stop() {
	close(s.stopCh)
}

func (s *Service) processQueue(ctx context.Context) {
	// Check how many we can send today
	remaining, err := s.getRemainingQuota(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to check daily quota")
		return
	}
	if remaining <= 0 {
		return // quota exhausted, wait for reset
	}

	// Find campaigns that are "sending"
	rows, err := s.db.Query(ctx,
		`SELECT id, uuid, subject, body_html FROM marketing_campaigns WHERE status = 'sending' ORDER BY started_at ASC`)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to query sending campaigns")
		return
	}
	defer rows.Close()

	type activeCampaign struct {
		ID       int64
		UUID     string
		Subject  string
		BodyHTML string
	}
	var campaigns []activeCampaign
	for rows.Next() {
		var c activeCampaign
		if err := rows.Scan(&c.ID, &c.UUID, &c.Subject, &c.BodyHTML); err != nil {
			continue
		}
		campaigns = append(campaigns, c)
	}
	rows.Close()

	for _, c := range campaigns {
		if remaining <= 0 {
			break
		}

		// Get pending recipients for this campaign (batch)
		batch := remaining
		if batch > 10 {
			batch = 10 // process in small batches per tick
		}

		recipientRows, err := s.db.Query(ctx,
			`SELECT id, email FROM marketing_campaign_recipients
			 WHERE campaign_id = $1 AND status = 'pending'
			 ORDER BY id LIMIT $2`, c.ID, batch)
		if err != nil {
			s.logger.Error().Err(err).Int64("campaign_id", c.ID).Msg("failed to query pending recipients")
			continue
		}

		type recipientItem struct {
			ID    int64
			Email string
		}
		var recipients []recipientItem
		for recipientRows.Next() {
			var r recipientItem
			if err := recipientRows.Scan(&r.ID, &r.Email); err != nil {
				continue
			}
			recipients = append(recipients, r)
		}
		recipientRows.Close()

		if len(recipients) == 0 {
			// No more pending — mark campaign completed
			s.completeCampaign(ctx, c.ID)
			continue
		}

		for _, r := range recipients {
			if remaining <= 0 {
				break
			}

			// Generate unsubscribe token and link
			unsubToken := generateToken()
			unsubLink := fmt.Sprintf("https://cryptolink.cc/api/dashboard/v1/marketing/unsubscribe?token=%s", unsubToken)

			// Store unsubscribe token (upsert — ignore if already exists)
			_, _ = s.db.Exec(ctx,
				`INSERT INTO marketing_unsubscribes (email, token) VALUES ($1, $2)
				 ON CONFLICT (email) DO UPDATE SET token = $2`,
				r.Email, unsubToken)

			// Inject unsubscribe footer into the email body
			bodyWithUnsub := injectUnsubscribeFooter(c.BodyHTML, unsubLink)

			// Send email
			sendErr := s.emailService.SendEmail(ctx, email.SendEmailParams{
				To:       r.Email,
				Subject:  c.Subject,
				Body:     bodyWithUnsub,
				Template: "marketing_campaign",
			})

			now := time.Now()
			if sendErr != nil {
				_, _ = s.db.Exec(ctx,
					`UPDATE marketing_campaign_recipients SET status = 'failed', error_message = $2, sent_at = $3 WHERE id = $1`,
					r.ID, sendErr.Error(), now)
				_, _ = s.db.Exec(ctx,
					`UPDATE marketing_campaigns SET failed_count = failed_count + 1, pending_count = pending_count - 1, updated_at = $2 WHERE id = $1`,
					c.ID, now)
				s.logger.Warn().Err(sendErr).Str("email", r.Email).Msg("marketing email failed")
			} else {
				_, _ = s.db.Exec(ctx,
					`UPDATE marketing_campaign_recipients SET status = 'sent', sent_at = $2 WHERE id = $1`,
					r.ID, now)
				_, _ = s.db.Exec(ctx,
					`UPDATE marketing_campaigns SET sent_count = sent_count + 1, pending_count = pending_count - 1, updated_at = $2 WHERE id = $1`,
					c.ID, now)
				s.incrementQuota(ctx)
				remaining--
			}
		}
	}
}

func (s *Service) completeCampaign(ctx context.Context, campaignID int64) {
	now := time.Now()
	_, _ = s.db.Exec(ctx,
		`UPDATE marketing_campaigns SET status = 'completed', completed_at = $2, updated_at = $2 WHERE id = $1`,
		campaignID, now)
	s.logger.Info().Int64("campaign_id", campaignID).Msg("campaign completed")
}

// --- Daily Quota ---

// quotaDate returns the current quota date based on Madrid 17:00 reset.
// Before 17:00 Madrid = previous calendar day's quota. After 17:00 = today's quota.
func quotaDate() time.Time {
	now := time.Now().In(madridTZ)
	if now.Hour() < 17 {
		now = now.AddDate(0, 0, -1)
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *Service) getRemainingQuota(ctx context.Context) (int, error) {
	qd := quotaDate()
	var sentCount, dailyLimit int
	err := s.db.QueryRow(ctx,
		`SELECT sent_count, daily_limit FROM marketing_email_quota WHERE quota_date = $1`, qd,
	).Scan(&sentCount, &dailyLimit)

	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			// First query of the day — create row
			_, err = s.db.Exec(ctx,
				`INSERT INTO marketing_email_quota (quota_date, sent_count, daily_limit) VALUES ($1, 0, $2)
				 ON CONFLICT (quota_date) DO NOTHING`, qd, DailyMarketingLimit)
			if err != nil {
				return 0, err
			}
			return DailyMarketingLimit, nil
		}
		return 0, err
	}
	return dailyLimit - sentCount, nil
}

func (s *Service) incrementQuota(ctx context.Context) {
	qd := quotaDate()
	_, _ = s.db.Exec(ctx,
		`INSERT INTO marketing_email_quota (quota_date, sent_count, daily_limit, updated_at)
		 VALUES ($1, 1, $2, NOW())
		 ON CONFLICT (quota_date) DO UPDATE SET sent_count = marketing_email_quota.sent_count + 1, updated_at = NOW()`,
		qd, DailyMarketingLimit)
}

// GetQuotaStatus returns current day's remaining quota for the API
func (s *Service) GetQuotaStatus(ctx context.Context) (sent int, limit int, resetAt string, err error) {
	qd := quotaDate()
	err = s.db.QueryRow(ctx,
		`SELECT sent_count, daily_limit FROM marketing_email_quota WHERE quota_date = $1`, qd,
	).Scan(&sent, &limit)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return 0, DailyMarketingLimit, nextResetTime(), nil
		}
		return
	}
	resetAt = nextResetTime()
	return
}

func nextResetTime() string {
	now := time.Now().In(madridTZ)
	reset := time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, madridTZ)
	if now.After(reset) {
		reset = reset.AddDate(0, 0, 1)
	}
	return reset.UTC().Format(time.RFC3339)
}

// --- Audience Resolution ---

func (s *Service) resolveAudience(ctx context.Context, audience string) ([]string, error) {
	emailSet := make(map[string]struct{})

	if audience == "merchants" || audience == "all" {
		rows, err := s.db.Query(ctx,
			`SELECT DISTINCT u.email FROM users u
			 JOIN merchants m ON m.creator_id = u.id
			 WHERE u.email IS NOT NULL AND u.email != ''`)
		if err != nil {
			return nil, errors.Wrap(err, "failed to query merchant emails")
		}
		for rows.Next() {
			var e string
			if err := rows.Scan(&e); err == nil && e != "" {
				emailSet[strings.ToLower(e)] = struct{}{}
			}
		}
		rows.Close()
	}

	if audience == "contacts_opted_in" || audience == "all" {
		rows, err := s.db.Query(ctx,
			`SELECT email FROM contacts WHERE marketing_consent = true AND email IS NOT NULL AND email != ''`)
		if err != nil {
			return nil, errors.Wrap(err, "failed to query contact emails")
		}
		for rows.Next() {
			var e string
			if err := rows.Scan(&e); err == nil && e != "" {
				emailSet[strings.ToLower(e)] = struct{}{}
			}
		}
		rows.Close()
	}

	emails := make([]string, 0, len(emailSet))
	for e := range emailSet {
		emails = append(emails, e)
	}
	return emails, nil
}

func (s *Service) filterUnsubscribed(ctx context.Context, emails []string) ([]string, error) {
	unsubSet := make(map[string]struct{})
	rows, err := s.db.Query(ctx, `SELECT email FROM marketing_unsubscribes`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err == nil {
			unsubSet[strings.ToLower(e)] = struct{}{}
		}
	}
	rows.Close()

	var filtered []string
	for _, e := range emails {
		if _, unsub := unsubSet[strings.ToLower(e)]; !unsub {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// --- Unsubscribe ---

func (s *Service) Unsubscribe(ctx context.Context, token string) (string, error) {
	var emailAddr string
	err := s.db.QueryRow(ctx,
		`SELECT email FROM marketing_unsubscribes WHERE token = $1`, token,
	).Scan(&emailAddr)
	if err != nil {
		return "", errors.New("invalid or expired unsubscribe link")
	}

	// Also remove marketing consent from contacts table
	_, _ = s.db.Exec(ctx,
		`UPDATE contacts SET marketing_consent = false, updated_at = NOW() WHERE LOWER(email) = LOWER($1)`, emailAddr)

	return emailAddr, nil
}

// --- Helpers ---

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func injectUnsubscribeFooter(bodyHTML, unsubLink string) string {
	footer := fmt.Sprintf(`<div style="margin-top:32px;padding-top:16px;border-top:1px solid #e2e8f0;text-align:center;">
  <p style="color:#94a3b8;font-size:12px;">You are receiving this email because you opted in to marketing communications from CryptoLink.</p>
  <p style="color:#94a3b8;font-size:12px;"><a href="%s" style="color:#10b981;">Unsubscribe</a> from future marketing emails.</p>
</div>`, unsubLink)

	// Try to inject before </body>, otherwise append
	if idx := strings.LastIndex(strings.ToLower(bodyHTML), "</body>"); idx != -1 {
		return bodyHTML[:idx] + footer + bodyHTML[idx:]
	}
	return bodyHTML + footer
}

// scanCampaignRow scans a campaign from rows.
func scanCampaignRow(rows interface{ Scan(dest ...interface{}) error }) (*Campaign, error) {
	var c Campaign
	var templateID sql.NullString
	var createdAt, updatedAt time.Time
	var startedAt, completedAt sql.NullTime

	err := rows.Scan(&c.ID, &c.UUID, &c.Name, &c.Subject, &templateID, &c.Audience, &c.Status,
		&c.TotalRecipients, &c.SentCount, &c.FailedCount, &c.PendingCount,
		&createdAt, &updatedAt, &startedAt, &completedAt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to scan campaign")
	}

	c.CreatedAt = createdAt.Format(time.RFC3339)
	c.UpdatedAt = updatedAt.Format(time.RFC3339)
	if templateID.Valid {
		c.TemplateID = &templateID.String
	}
	if startedAt.Valid {
		t := startedAt.Time.Format(time.RFC3339)
		c.StartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time.Format(time.RFC3339)
		c.CompletedAt = &t
	}
	return &c, nil
}

func scanCampaignSingle(row interface{ Scan(dest ...interface{}) error }) (*Campaign, error) {
	return scanCampaignRow(row)
}
