package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Service struct {
	db     *pgxpool.Pool
	logger *zerolog.Logger
}

type EmailSettings struct {
	ID        int64     `json:"id"`
	SMTPHost  string    `json:"smtp_host"`
	SMTPPort  int       `json:"smtp_port"`
	SMTPUser  string    `json:"smtp_user"`
	SMTPPass  string    `json:"smtp_pass"`
	FromName  string    `json:"from_name"`
	FromEmail string    `json:"from_email"`
	IsActive  bool      `json:"is_active"`
	UpdatedAt time.Time `json:"updated_at"`
}

type EmailLog struct {
	ID           int64     `json:"id"`
	ToEmail      string    `json:"to_email"`
	Subject      string    `json:"subject"`
	Template     string    `json:"template"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}

type SendEmailParams struct {
	To       string
	Subject  string
	Body     string
	Template string
}

func New(db *pgxpool.Pool, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "email_service").Logger()
	return &Service{db: db, logger: &log}
}

// GetSettings returns the current email settings
func (s *Service) GetSettings(ctx context.Context) (*EmailSettings, error) {
	query := `SELECT id, smtp_host, smtp_port, smtp_user, smtp_pass, from_name, from_email, is_active, updated_at
	          FROM email_settings ORDER BY id LIMIT 1`

	var settings EmailSettings
	err := s.db.QueryRow(ctx, query).Scan(
		&settings.ID, &settings.SMTPHost, &settings.SMTPPort, &settings.SMTPUser,
		&settings.SMTPPass, &settings.FromName, &settings.FromEmail,
		&settings.IsActive, &settings.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("no email settings configured")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get email settings")
	}

	return &settings, nil
}

// UpdateSettings updates email settings
func (s *Service) UpdateSettings(ctx context.Context, settings *EmailSettings) (*EmailSettings, error) {
	query := `INSERT INTO email_settings (id, smtp_host, smtp_port, smtp_user, smtp_pass, from_name, from_email, is_active, updated_at)
	          VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8)
	          ON CONFLICT (id) DO UPDATE SET
	            smtp_host = $1, smtp_port = $2, smtp_user = $3, smtp_pass = $4,
	            from_name = $5, from_email = $6, is_active = $7, updated_at = $8
	          RETURNING id, smtp_host, smtp_port, smtp_user, smtp_pass, from_name, from_email, is_active, updated_at`

	var updated EmailSettings
	err := s.db.QueryRow(ctx, query,
		settings.SMTPHost, settings.SMTPPort, settings.SMTPUser, settings.SMTPPass,
		settings.FromName, settings.FromEmail, settings.IsActive, time.Now(),
	).Scan(
		&updated.ID, &updated.SMTPHost, &updated.SMTPPort, &updated.SMTPUser,
		&updated.SMTPPass, &updated.FromName, &updated.FromEmail,
		&updated.IsActive, &updated.UpdatedAt,
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to update email settings")
	}

	return &updated, nil
}

// SendEmail sends an email using the configured SMTP settings
func (s *Service) SendEmail(ctx context.Context, params SendEmailParams) error {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return err
	}

	if !settings.IsActive {
		s.logger.Warn().Msg("email sending disabled - settings not active")
		return errors.New("email sending is disabled")
	}

	from := fmt.Sprintf("%s <%s>", settings.FromName, settings.FromEmail)

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=\"UTF-8\"\r\n"+
		"\r\n"+
		"%s", from, params.To, params.Subject, params.Body)

	addr := fmt.Sprintf("%s:%d", settings.SMTPHost, settings.SMTPPort)

	auth := smtp.PlainAuth("", settings.SMTPUser, settings.SMTPPass, settings.SMTPHost)

	tlsConfig := &tls.Config{
		ServerName: settings.SMTPHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		// Try STARTTLS fallback
		sendErr := smtp.SendMail(addr, auth, settings.FromEmail, []string{params.To}, []byte(msg))
		if sendErr != nil {
			s.logEmail(ctx, params.To, params.Subject, params.Template, "failed", sendErr.Error())
			return errors.Wrap(sendErr, "failed to send email")
		}
		s.logEmail(ctx, params.To, params.Subject, params.Template, "sent", "")
		return nil
	}

	client, err := smtp.NewClient(conn, settings.SMTPHost)
	if err != nil {
		s.logEmail(ctx, params.To, params.Subject, params.Template, "failed", err.Error())
		return errors.Wrap(err, "failed to create SMTP client")
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		s.logEmail(ctx, params.To, params.Subject, params.Template, "failed", err.Error())
		return errors.Wrap(err, "SMTP auth failed")
	}

	if err = client.Mail(settings.FromEmail); err != nil {
		s.logEmail(ctx, params.To, params.Subject, params.Template, "failed", err.Error())
		return err
	}

	if err = client.Rcpt(params.To); err != nil {
		s.logEmail(ctx, params.To, params.Subject, params.Template, "failed", err.Error())
		return err
	}

	w, err := client.Data()
	if err != nil {
		s.logEmail(ctx, params.To, params.Subject, params.Template, "failed", err.Error())
		return err
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		s.logEmail(ctx, params.To, params.Subject, params.Template, "failed", err.Error())
		return err
	}

	err = w.Close()
	if err != nil {
		s.logEmail(ctx, params.To, params.Subject, params.Template, "failed", err.Error())
		return err
	}

	client.Quit()

	s.logEmail(ctx, params.To, params.Subject, params.Template, "sent", "")
	s.logger.Info().Str("to", params.To).Str("subject", params.Subject).Msg("email sent successfully")
	return nil
}

// SendVolumeAlert sends a volume threshold alert email
func (s *Service) SendVolumeAlert(ctx context.Context, toEmail, merchantName string, volumePercent float64, currentVolume, limitVolume string) error {
	var templateName, subject string
	var color string

	if volumePercent >= 100 {
		templateName = "volume_exceeded"
		subject = fmt.Sprintf("[CryptoLink] Volume limit exceeded for %s", merchantName)
		color = "#ff4d4f"
	} else if volumePercent >= 90 {
		templateName = "volume_critical"
		subject = fmt.Sprintf("[CryptoLink] Critical: %s approaching volume limit", merchantName)
		color = "#ff7a45"
	} else if volumePercent >= 80 {
		templateName = "volume_warning"
		subject = fmt.Sprintf("[CryptoLink] Warning: %s approaching volume limit", merchantName)
		color = "#faad14"
	} else {
		return nil
	}

	// Check deduplication: don't send same template more than once per billing period
	dedupQuery := `SELECT COUNT(*) FROM email_log
	               WHERE to_email = $1 AND template = $2 AND status = 'sent'
	               AND created_at >= date_trunc('month', NOW())`
	var count int
	_ = s.db.QueryRow(ctx, dedupQuery, toEmail, templateName).Scan(&count)
	if count > 0 {
		s.logger.Debug().Str("template", templateName).Str("to", toEmail).Msg("dedup: alert already sent this period")
		return nil
	}

	body := renderVolumeAlertTemplate(merchantName, volumePercent, currentVolume, limitVolume, color)

	return s.SendEmail(ctx, SendEmailParams{
		To:       toEmail,
		Subject:  subject,
		Body:     body,
		Template: templateName,
	})
}

// PaymentReceivedParams contains data for a payment received notification email.
type PaymentReceivedParams struct {
	MerchantEmail    string
	MerchantName     string
	TxHash           string
	Amount           string // e.g. "1.234"
	Ticker           string // e.g. "ETH"
	USDAmount        string // e.g. "2345.67"
	SenderAddress    string
	RecipientAddress string
	ExplorerLink     string
	Network          string // e.g. "Ethereum"
	ReceivedAt       time.Time
}

// SendPaymentReceived sends a payment received notification to the merchant.
// This is best-effort: errors are logged but not propagated to avoid blocking payment processing.
func (s *Service) SendPaymentReceived(ctx context.Context, params PaymentReceivedParams) {
	body := renderPaymentReceivedTemplate(params)
	subject := fmt.Sprintf("[CryptoLink] Payment received: %s %s", params.Amount, params.Ticker)

	if err := s.SendEmail(ctx, SendEmailParams{
		To:       params.MerchantEmail,
		Subject:  subject,
		Body:     body,
		Template: "payment_received",
	}); err != nil {
		s.logger.Warn().Err(err).
			Str("merchant_email", params.MerchantEmail).
			Str("tx_hash", params.TxHash).
			Msg("unable to send payment received email")
	}
}

// GetLogs returns email logs
func (s *Service) GetLogs(ctx context.Context, limit, offset int) ([]*EmailLog, int, error) {
	if limit <= 0 {
		limit = 50
	}

	var total int
	err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM email_log").Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count email logs")
	}

	query := `SELECT id, to_email, subject, template, status, COALESCE(error_message, ''), created_at
	          FROM email_log ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := s.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to list email logs")
	}
	defer rows.Close()

	var logs []*EmailLog
	for rows.Next() {
		var log EmailLog
		err := rows.Scan(&log.ID, &log.ToEmail, &log.Subject, &log.Template, &log.Status, &log.ErrorMessage, &log.CreatedAt)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to scan email log")
		}
		logs = append(logs, &log)
	}

	return logs, total, nil
}

func (s *Service) logEmail(ctx context.Context, to, subject, tmpl, status, errMsg string) {
	query := `INSERT INTO email_log (to_email, subject, template, status, error_message, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.db.Exec(ctx, query, to, subject, tmpl, status, errMsg, time.Now())
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to log email")
	}
}

func renderPaymentReceivedTemplate(params PaymentReceivedParams) string {
	shortTx := params.TxHash
	if len(shortTx) > 20 {
		shortTx = shortTx[:10] + "..." + shortTx[len(shortTx)-10:]
	}
	shortSender := params.SenderAddress
	if len(shortSender) > 20 {
		shortSender = shortSender[:10] + "..." + shortSender[len(shortSender)-10:]
	}

	explorerBtn := ""
	if params.ExplorerLink != "" {
		explorerBtn = fmt.Sprintf(`<a href="%s" style="display:inline-block;background:#10b981;color:#fff;padding:10px 20px;border-radius:6px;text-decoration:none;margin-top:8px;">View Transaction</a>`, params.ExplorerLink)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;max-width:600px;margin:0 auto;padding:20px;">
  <div style="background:#0f172a;padding:24px;border-radius:8px 8px 0 0;">
    <h1 style="color:#fff;margin:0;font-size:20px;">CryptoLink</h1>
  </div>
  <div style="border:1px solid #e2e8f0;border-top:none;padding:24px;border-radius:0 0 8px 8px;">
    <h2 style="color:#10b981;margin-top:0;">Payment Received!</h2>
    <p>Hello <strong>%s</strong>, you just received a payment on <strong>%s</strong>.</p>
    <div style="background:#f0fdf4;border:1px solid #bbf7d0;padding:16px;border-radius:8px;margin:16px 0;">
      <p style="margin:4px 0;font-size:24px;font-weight:700;color:#059669;">%s %s</p>
      <p style="margin:4px 0;color:#64748b;">≈ $%s USD</p>
    </div>
    <table style="width:100%%;border-collapse:collapse;font-size:14px;">
      <tr><td style="padding:6px 0;color:#64748b;width:40%%;">Network</td><td style="padding:6px 0;font-weight:500;">%s</td></tr>
      <tr><td style="padding:6px 0;color:#64748b;">From</td><td style="padding:6px 0;font-family:monospace;font-size:12px;">%s</td></tr>
      <tr><td style="padding:6px 0;color:#64748b;">To (your wallet)</td><td style="padding:6px 0;font-family:monospace;font-size:12px;">%s</td></tr>
      <tr><td style="padding:6px 0;color:#64748b;">Transaction</td><td style="padding:6px 0;font-family:monospace;font-size:12px;">%s</td></tr>
      <tr><td style="padding:6px 0;color:#64748b;">Date</td><td style="padding:6px 0;">%s</td></tr>
    </table>
    %s
    <hr style="border:none;border-top:1px solid #e2e8f0;margin:24px 0;">
    <p style="color:#94a3b8;font-size:12px;">This is an automated notification from CryptoLink. Manage your notification settings in your dashboard.</p>
  </div>
</body>
</html>`,
		params.MerchantName,
		params.Network,
		params.Amount, params.Ticker,
		params.USDAmount,
		params.Network,
		shortSender,
		params.RecipientAddress,
		shortTx,
		params.ReceivedAt.Format("2006-01-02 15:04:05 UTC"),
		explorerBtn,
	)
}

func renderVolumeAlertTemplate(merchantName string, percent float64, current, limit, color string) string {
	tmplStr := `<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <div style="background: #0f172a; padding: 24px; border-radius: 8px 8px 0 0;">
    <h1 style="color: #fff; margin: 0; font-size: 20px;">CryptoLink</h1>
  </div>
  <div style="border: 1px solid #e2e8f0; border-top: none; padding: 24px; border-radius: 0 0 8px 8px;">
    <h2 style="color: {{.Color}}; margin-top: 0;">Volume Alert for {{.MerchantName}}</h2>
    <p>Your merchant <strong>{{.MerchantName}}</strong> has used <strong style="color: {{.Color}};">{{printf "%.0f" .Percent}}%</strong> of its monthly volume limit.</p>
    <div style="background: #f1f5f9; padding: 16px; border-radius: 8px; margin: 16px 0;">
      <p style="margin: 4px 0;"><strong>Current Volume:</strong> ${{.Current}}</p>
      <p style="margin: 4px 0;"><strong>Monthly Limit:</strong> ${{.Limit}}</p>
    </div>
    <p>To avoid service interruption, please consider upgrading your subscription plan.</p>
    <a href="https://cryptolink.cc/dashboard/subscription" style="display: inline-block; background: #6366f1; color: #fff; padding: 12px 24px; border-radius: 6px; text-decoration: none; margin-top: 8px;">Upgrade Plan</a>
    <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 24px 0;">
    <p style="color: #94a3b8; font-size: 12px;">This is an automated message from CryptoLink. Do not reply to this email.</p>
  </div>
</body>
</html>`

	data := struct {
		MerchantName string
		Percent      float64
		Current      string
		Limit        string
		Color        string
	}{merchantName, percent, current, limit, color}

	tmpl, err := template.New("volume_alert").Parse(tmplStr)
	if err != nil {
		return fmt.Sprintf("<p>Volume alert: %s is at %.0f%% of limit ($%s / $%s)</p>", merchantName, percent, current, limit)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("<p>Volume alert: %s is at %.0f%% of limit ($%s / $%s)</p>", merchantName, percent, current, limit)
	}

	return buf.String()
}
