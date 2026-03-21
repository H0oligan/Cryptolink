package contact

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Service struct {
	db     *pgxpool.Pool
	logger *zerolog.Logger
}

type Contact struct {
	ID               int64     `json:"id"`
	UUID             string    `json:"uuid"`
	Email            string    `json:"email"`
	MarketingConsent bool      `json:"marketing_consent"`
	TermsAcceptedAt  *string   `json:"terms_accepted_at"`
	SourceMerchantID *int64    `json:"source_merchant_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type AdminContact struct {
	ID                 int64   `json:"id"`
	UUID               string  `json:"uuid"`
	Email              string  `json:"email"`
	MarketingConsent   bool    `json:"marketing_consent"`
	TermsAcceptedAt    *string `json:"terms_accepted_at"`
	SourceMerchantName string  `json:"source_merchant_name"`
	CreatedAt          string  `json:"created_at"`
}

func New(db *pgxpool.Pool, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "contact_service").Logger()
	return &Service{db: db, logger: &log}
}

// ResolveContact upserts a contact by email. If already exists, upgrades consent (never downgrades).
func (s *Service) ResolveContact(ctx context.Context, email string, merchantID int64, marketingConsent bool, termsAccepted bool) (*Contact, error) {
	// Try to get existing
	var contact Contact
	var termsAt sql.NullTime
	var sourceMerchantID sql.NullInt64

	err := s.db.QueryRow(ctx,
		`SELECT id, uuid, email, marketing_consent, terms_accepted_at, source_merchant_id, created_at, updated_at
		 FROM contacts WHERE email = $1`, email,
	).Scan(&contact.ID, &contact.UUID, &contact.Email, &contact.MarketingConsent,
		&termsAt, &sourceMerchantID, &contact.CreatedAt, &contact.UpdatedAt)

	if err != nil && err.Error() == "no rows in result set" {
		// Not found — create new
		newUUID := uuid.New()
		now := time.Now()

		var termsAcceptedAt sql.NullTime
		if termsAccepted {
			termsAcceptedAt = sql.NullTime{Time: now, Valid: true}
		}

		err = s.db.QueryRow(ctx,
			`INSERT INTO contacts (uuid, email, marketing_consent, terms_accepted_at, source_merchant_id, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (email) DO UPDATE SET
			   marketing_consent = CASE WHEN contacts.marketing_consent THEN TRUE ELSE EXCLUDED.marketing_consent END,
			   terms_accepted_at = COALESCE(contacts.terms_accepted_at, EXCLUDED.terms_accepted_at),
			   updated_at = $7
			 RETURNING id, uuid, email, marketing_consent, terms_accepted_at, source_merchant_id, created_at, updated_at`,
			newUUID.String(), email, marketingConsent, termsAcceptedAt,
			sql.NullInt64{Int64: merchantID, Valid: true}, now, now,
		).Scan(&contact.ID, &contact.UUID, &contact.Email, &contact.MarketingConsent,
			&termsAt, &sourceMerchantID, &contact.CreatedAt, &contact.UpdatedAt)

		if err != nil {
			return nil, errors.Wrap(err, "failed to create/upsert contact")
		}
	} else if err == nil {
		// Exists — upgrade consent only (never downgrade)
		newMarketing := contact.MarketingConsent || marketingConsent
		now := time.Now()

		if !termsAt.Valid && termsAccepted {
			termsAt = sql.NullTime{Time: now, Valid: true}
		}

		_, err = s.db.Exec(ctx,
			`UPDATE contacts SET marketing_consent = $2, terms_accepted_at = COALESCE(terms_accepted_at, $3), updated_at = $4 WHERE id = $1`,
			contact.ID, newMarketing, termsAt, now,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to update contact consent")
		}
		contact.MarketingConsent = newMarketing
	} else {
		return nil, errors.Wrap(err, "failed to look up contact")
	}

	if termsAt.Valid {
		t := termsAt.Time.Format(time.RFC3339)
		contact.TermsAcceptedAt = &t
	}
	if sourceMerchantID.Valid {
		contact.SourceMerchantID = &sourceMerchantID.Int64
	}

	return &contact, nil
}

// ListAllContacts returns a paginated list of all contacts for the admin panel
func (s *Service) ListAllContacts(ctx context.Context, limit, offset int, search string) ([]*AdminContact, int, error) {
	if limit <= 0 {
		limit = 20
	}

	// Count total
	var total int
	var countQuery string
	var countArgs []interface{}

	if search != "" {
		countQuery = `SELECT COUNT(*) FROM contacts WHERE email ILIKE $1`
		countArgs = []interface{}{"%" + search + "%"}
	} else {
		countQuery = `SELECT COUNT(*) FROM contacts`
	}

	if err := s.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, "failed to count contacts")
	}

	// Query with merchant name join
	var query string
	var args []interface{}

	if search != "" {
		query = `SELECT c.id, c.uuid, c.email, c.marketing_consent, c.terms_accepted_at,
		         COALESCE(m.name, '') as source_merchant_name, c.created_at
		         FROM contacts c
		         LEFT JOIN merchants m ON m.id = c.source_merchant_id
		         WHERE c.email ILIKE $1
		         ORDER BY c.created_at DESC LIMIT $2 OFFSET $3`
		args = []interface{}{"%" + search + "%", limit, offset}
	} else {
		query = `SELECT c.id, c.uuid, c.email, c.marketing_consent, c.terms_accepted_at,
		         COALESCE(m.name, '') as source_merchant_name, c.created_at
		         FROM contacts c
		         LEFT JOIN merchants m ON m.id = c.source_merchant_id
		         ORDER BY c.created_at DESC LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to list contacts")
	}
	defer rows.Close()

	var contacts []*AdminContact
	for rows.Next() {
		var c AdminContact
		var termsAt sql.NullTime
		var createdAt time.Time

		if err := rows.Scan(&c.ID, &c.UUID, &c.Email, &c.MarketingConsent, &termsAt, &c.SourceMerchantName, &createdAt); err != nil {
			return nil, 0, errors.Wrap(err, "failed to scan contact")
		}

		c.CreatedAt = createdAt.Format(time.RFC3339)
		if termsAt.Valid {
			t := termsAt.Time.Format(time.RFC3339)
			c.TermsAcceptedAt = &t
		}

		contacts = append(contacts, &c)
	}

	return contacts, total, nil
}

// ExportContacts returns all contacts for CSV export
func (s *Service) ExportContacts(ctx context.Context, marketingOnly bool) ([]*AdminContact, error) {
	query := `SELECT c.id, c.uuid, c.email, c.marketing_consent, c.terms_accepted_at,
	          COALESCE(m.name, '') as source_merchant_name, c.created_at
	          FROM contacts c
	          LEFT JOIN merchants m ON m.id = c.source_merchant_id`

	if marketingOnly {
		query += ` WHERE c.marketing_consent = true`
	}
	query += ` ORDER BY c.created_at DESC`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to export contacts")
	}
	defer rows.Close()

	var contacts []*AdminContact
	for rows.Next() {
		var c AdminContact
		var termsAt sql.NullTime
		var createdAt time.Time

		if err := rows.Scan(&c.ID, &c.UUID, &c.Email, &c.MarketingConsent, &termsAt, &c.SourceMerchantName, &createdAt); err != nil {
			return nil, errors.Wrap(err, "failed to scan contact")
		}

		c.CreatedAt = createdAt.Format(time.RFC3339)
		if termsAt.Valid {
			t := termsAt.Time.Format(time.RFC3339)
			c.TermsAcceptedAt = &t
		}

		contacts = append(contacts, &c)
	}

	return contacts, nil
}
