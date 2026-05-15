package marketing

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/cryptolink/cryptolink/internal/service/email"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type Sequence struct {
	ID              int64          `json:"id"`
	UUID            string         `json:"uuid"`
	Name            string         `json:"name"`
	Audience        string         `json:"audience"`
	Status          string         `json:"status"`
	SkipIfConverted bool           `json:"skip_if_converted"`
	StartAt         *string        `json:"start_at"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
	StartedAt       *string        `json:"started_at"`
	CompletedAt     *string        `json:"completed_at"`
	Steps           []SequenceStep `json:"steps"`
}

type SequenceStep struct {
	ID              int64   `json:"id"`
	SequenceID      int64   `json:"sequence_id"`
	StepIndex       int     `json:"step_index"`
	TemplateID      string  `json:"template_id"`
	SubjectOverride *string `json:"subject_override"`
	OffsetHours     int     `json:"offset_hours"`
}

type SequenceStats struct {
	Sequence      *Sequence  `json:"sequence"`
	TotalEnrolled int        `json:"total_enrolled"`
	Active        int        `json:"active"`
	Converted     int        `json:"converted"`
	Unsubscribed  int        `json:"unsubscribed"`
	Completed     int        `json:"completed"`
	Failed        int        `json:"failed"`
	StepBreakdown []StepStat `json:"step_breakdown"`
}

type StepStat struct {
	StepIndex int `json:"step_index"`
	Sent      int `json:"sent"`
	Pending   int `json:"pending"`
}

type CreateSequenceStepInput struct {
	TemplateID      string `json:"template_id"`
	SubjectOverride string `json:"subject_override"`
	OffsetHours     int    `json:"offset_hours"`
}

type CreateSequenceParams struct {
	Name            string
	Audience        string
	SkipIfConverted bool
	StartAt         *time.Time
	Steps           []CreateSequenceStepInput
}

func (s *Service) CreateSequence(ctx context.Context, params CreateSequenceParams) (*Sequence, error) {
	if params.Name == "" {
		return nil, errors.New("name is required")
	}
	if params.Audience == "" {
		params.Audience = "contacts_opted_in"
	}
	validAudiences := map[string]bool{"merchants": true, "contacts_opted_in": true, "all": true}
	if !validAudiences[params.Audience] {
		return nil, errors.New("audience must be merchants, contacts_opted_in, or all")
	}
	if len(params.Steps) < 1 {
		return nil, errors.New("at least one step is required")
	}
	if len(params.Steps) > 10 {
		return nil, errors.New("at most 10 steps per sequence")
	}
	for i, step := range params.Steps {
		if step.TemplateID == "" {
			return nil, errors.Errorf("step %d: template_id is required", i+1)
		}
		if GetTemplateByID(step.TemplateID) == nil {
			return nil, errors.Errorf("step %d: template %q does not exist", i+1, step.TemplateID)
		}
		if step.OffsetHours < 0 {
			return nil, errors.Errorf("step %d: offset_hours must be >= 0", i+1)
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "begin tx")
	}
	defer tx.Rollback(ctx)

	seqUUID := uuid.New().String()
	var seqID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO marketing_sequences (uuid, name, audience, skip_if_converted, start_at, status)
		 VALUES ($1, $2, $3, $4, $5, 'draft') RETURNING id`,
		seqUUID, params.Name, params.Audience, params.SkipIfConverted, params.StartAt,
	).Scan(&seqID)
	if err != nil {
		return nil, errors.Wrap(err, "insert sequence")
	}

	for i, step := range params.Steps {
		var subjectOverride sql.NullString
		if strings.TrimSpace(step.SubjectOverride) != "" {
			subjectOverride = sql.NullString{String: step.SubjectOverride, Valid: true}
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO marketing_sequence_steps (sequence_id, step_index, template_id, subject_override, offset_hours)
			 VALUES ($1, $2, $3, $4, $5)`,
			seqID, i+1, step.TemplateID, subjectOverride, step.OffsetHours,
		)
		if err != nil {
			return nil, errors.Wrap(err, "insert step")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "commit")
	}
	return s.GetSequence(ctx, seqUUID)
}

func (s *Service) GetSequence(ctx context.Context, sequenceUUID string) (*Sequence, error) {
	var seq Sequence
	var startAt, startedAt, completedAt sql.NullTime
	var createdAt, updatedAt time.Time

	err := s.db.QueryRow(ctx,
		`SELECT id, uuid, name, audience, status, skip_if_converted,
		        start_at, created_at, updated_at, started_at, completed_at
		 FROM marketing_sequences WHERE uuid = $1`, sequenceUUID,
	).Scan(&seq.ID, &seq.UUID, &seq.Name, &seq.Audience, &seq.Status, &seq.SkipIfConverted,
		&startAt, &createdAt, &updatedAt, &startedAt, &completedAt)
	if err != nil {
		return nil, errors.Wrap(err, "sequence not found")
	}
	seq.CreatedAt = createdAt.Format(time.RFC3339)
	seq.UpdatedAt = updatedAt.Format(time.RFC3339)
	if startAt.Valid {
		t := startAt.Time.Format(time.RFC3339)
		seq.StartAt = &t
	}
	if startedAt.Valid {
		t := startedAt.Time.Format(time.RFC3339)
		seq.StartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time.Format(time.RFC3339)
		seq.CompletedAt = &t
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, sequence_id, step_index, COALESCE(template_id, ''), subject_override, offset_hours
		 FROM marketing_sequence_steps WHERE sequence_id = $1 ORDER BY step_index`, seq.ID)
	if err != nil {
		return nil, errors.Wrap(err, "load steps")
	}
	defer rows.Close()
	for rows.Next() {
		var step SequenceStep
		var subjectOverride sql.NullString
		if err := rows.Scan(&step.ID, &step.SequenceID, &step.StepIndex, &step.TemplateID, &subjectOverride, &step.OffsetHours); err != nil {
			return nil, err
		}
		if subjectOverride.Valid {
			step.SubjectOverride = &subjectOverride.String
		}
		seq.Steps = append(seq.Steps, step)
	}
	return &seq, nil
}

func (s *Service) ListSequences(ctx context.Context, limit, offset int) ([]*Sequence, int, error) {
	if limit <= 0 {
		limit = 20
	}
	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM marketing_sequences`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(ctx,
		`SELECT uuid FROM marketing_sequences ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var uuids []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, 0, err
		}
		uuids = append(uuids, u)
	}
	rows.Close()
	var sequences []*Sequence
	for _, u := range uuids {
		seq, err := s.GetSequence(ctx, u)
		if err != nil {
			continue
		}
		sequences = append(sequences, seq)
	}
	return sequences, total, nil
}

func (s *Service) LaunchSequence(ctx context.Context, sequenceUUID string) error {
	seq, err := s.GetSequence(ctx, sequenceUUID)
	if err != nil {
		return err
	}
	if seq.Status != "draft" && seq.Status != "paused" {
		return errors.Errorf("cannot launch sequence in status %q", seq.Status)
	}
	if len(seq.Steps) == 0 {
		return errors.New("sequence has no steps")
	}

	emails, err := s.resolveAudience(ctx, seq.Audience)
	if err != nil {
		return errors.Wrap(err, "resolve audience")
	}
	emails, err = s.filterUnsubscribed(ctx, emails)
	if err != nil {
		return errors.Wrap(err, "filter unsubscribes")
	}
	if len(emails) == 0 {
		return errors.New("no eligible recipients after filtering")
	}

	startAt := time.Now()
	if seq.StartAt != nil {
		if parsed, err := time.Parse(time.RFC3339, *seq.StartAt); err == nil {
			if parsed.After(startAt) {
				startAt = parsed
			}
		}
	}
	firstOffset := time.Duration(seq.Steps[0].OffsetHours) * time.Hour
	nextSendAt := startAt.Add(firstOffset)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, e := range emails {
		_, err := tx.Exec(ctx,
			`INSERT INTO marketing_sequence_enrollments (sequence_id, email, current_step, next_send_at, status)
			 VALUES ($1, $2, 0, $3, 'active')
			 ON CONFLICT (sequence_id, email) DO NOTHING`,
			seq.ID, e, nextSendAt)
		if err != nil {
			return errors.Wrap(err, "enroll recipient")
		}
	}

	_, err = tx.Exec(ctx,
		`UPDATE marketing_sequences SET status = 'running', started_at = COALESCE(started_at, NOW()), updated_at = NOW() WHERE id = $1`,
		seq.ID)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.logger.Info().Str("sequence", seq.Name).Int("enrolled", len(emails)).Msg("sequence launched")
	return nil
}

func (s *Service) setSequenceStatus(ctx context.Context, sequenceUUID, newStatus string, allowedFrom ...string) error {
	seq, err := s.GetSequence(ctx, sequenceUUID)
	if err != nil {
		return err
	}
	ok := false
	for _, allowed := range allowedFrom {
		if seq.Status == allowed {
			ok = true
			break
		}
	}
	if !ok {
		return errors.Errorf("cannot transition from %q to %q", seq.Status, newStatus)
	}
	_, err = s.db.Exec(ctx,
		`UPDATE marketing_sequences SET status = $2, updated_at = NOW() WHERE id = $1`, seq.ID, newStatus)
	return err
}

func (s *Service) PauseSequence(ctx context.Context, sequenceUUID string) error {
	return s.setSequenceStatus(ctx, sequenceUUID, "paused", "running")
}

func (s *Service) ResumeSequence(ctx context.Context, sequenceUUID string) error {
	return s.setSequenceStatus(ctx, sequenceUUID, "running", "paused")
}

func (s *Service) CancelSequence(ctx context.Context, sequenceUUID string) error {
	return s.setSequenceStatus(ctx, sequenceUUID, "cancelled", "running", "paused", "draft")
}

func (s *Service) GetSequenceStats(ctx context.Context, sequenceUUID string) (*SequenceStats, error) {
	seq, err := s.GetSequence(ctx, sequenceUUID)
	if err != nil {
		return nil, err
	}
	stats := &SequenceStats{Sequence: seq}
	rows, err := s.db.Query(ctx,
		`SELECT status, COUNT(*) FROM marketing_sequence_enrollments WHERE sequence_id = $1 GROUP BY status`,
		seq.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return nil, err
		}
		stats.TotalEnrolled += count
		switch status {
		case "active":
			stats.Active = count
		case "converted":
			stats.Converted = count
		case "unsubscribed":
			stats.Unsubscribed = count
		case "completed":
			stats.Completed = count
		case "failed":
			stats.Failed = count
		}
	}
	rows.Close()

	for _, step := range seq.Steps {
		var sent, pending int
		_ = s.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM marketing_sequence_enrollments WHERE sequence_id = $1 AND current_step >= $2`,
			seq.ID, step.StepIndex).Scan(&sent)
		_ = s.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM marketing_sequence_enrollments WHERE sequence_id = $1 AND current_step = $2 AND status = 'active'`,
			seq.ID, step.StepIndex-1).Scan(&pending)
		stats.StepBreakdown = append(stats.StepBreakdown, StepStat{StepIndex: step.StepIndex, Sent: sent, Pending: pending})
	}
	return stats, nil
}

// sendStepEmail invokes the email service for a sequence step.
func (s *Service) sendStepEmail(ctx context.Context, to, subject, body string) error {
	return s.emailService.SendEmail(ctx, email.SendEmailParams{
		To:       to,
		Subject:  subject,
		Body:     body,
		Template: "marketing_sequence",
	})
}

// processSequenceTick fires due enrollments while respecting the daily quota.
func (s *Service) processSequenceTick(ctx context.Context) {
	remaining, err := s.getRemainingQuota(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("seq: failed to read quota")
		return
	}
	if remaining <= 0 {
		return
	}
	const batchPerTick = 10
	if remaining > batchPerTick {
		remaining = batchPerTick
	}

	rows, err := s.db.Query(ctx,
		`SELECT e.id, e.sequence_id, e.email, e.current_step, e.attempt_count, s.skip_if_converted
		 FROM marketing_sequence_enrollments e
		 JOIN marketing_sequences s ON s.id = e.sequence_id
		 WHERE e.status = 'active' AND s.status = 'running' AND e.next_send_at <= NOW()
		 ORDER BY e.next_send_at ASC LIMIT $1`, remaining)
	if err != nil {
		s.logger.Error().Err(err).Msg("seq: query due enrollments failed")
		return
	}
	type dueRow struct {
		enrollID        int64
		sequenceID      int64
		recipientEmail  string
		currentStep     int
		attempts        int
		skipIfConverted bool
	}
	var due []dueRow
	for rows.Next() {
		var r dueRow
		if err := rows.Scan(&r.enrollID, &r.sequenceID, &r.recipientEmail, &r.currentStep, &r.attempts, &r.skipIfConverted); err != nil {
			continue
		}
		due = append(due, r)
	}
	rows.Close()

	for _, d := range due {
		s.processOneEnrollment(ctx, d.enrollID, d.sequenceID, d.recipientEmail, d.currentStep, d.attempts, d.skipIfConverted)
	}
}

func (s *Service) processOneEnrollment(ctx context.Context, enrollID, sequenceID int64, recipientEmail string, currentStep, attempts int, skipIfConverted bool) {
	// 1. Unsubscribe re-check
	var unsubCount int
	_ = s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM marketing_unsubscribes WHERE LOWER(email) = LOWER($1)`,
		recipientEmail).Scan(&unsubCount)
	if unsubCount > 0 {
		_, _ = s.db.Exec(ctx,
			`UPDATE marketing_sequence_enrollments SET status = 'unsubscribed', updated_at = NOW() WHERE id = $1`,
			enrollID)
		return
	}

	// 2. Conversion check
	if skipIfConverted {
		var convertedCount int
		_ = s.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM users u
			 JOIN merchants m ON m.creator_id = u.id
			 WHERE LOWER(u.email) = LOWER($1)`,
			recipientEmail).Scan(&convertedCount)
		if convertedCount > 0 {
			_, _ = s.db.Exec(ctx,
				`UPDATE marketing_sequence_enrollments SET status = 'converted', updated_at = NOW() WHERE id = $1`,
				enrollID)
			return
		}
	}

	// 3. Load next step
	nextStepIndex := currentStep + 1
	var templateID string
	var subjectOverride sql.NullString
	var stepOffsetHours int
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(template_id, ''), subject_override, offset_hours
		 FROM marketing_sequence_steps WHERE sequence_id = $1 AND step_index = $2`,
		sequenceID, nextStepIndex,
	).Scan(&templateID, &subjectOverride, &stepOffsetHours)
	if err != nil {
		_, _ = s.db.Exec(ctx,
			`UPDATE marketing_sequence_enrollments SET status = 'completed', updated_at = NOW() WHERE id = $1`,
			enrollID)
		_ = s.maybeCompleteSequence(ctx, sequenceID)
		return
	}

	tmpl := GetTemplateByID(templateID)
	if tmpl == nil {
		s.markEnrollmentFailed(ctx, enrollID, attempts, errors.Errorf("template %q missing", templateID))
		return
	}
	subject := tmpl.Subject
	if subjectOverride.Valid && subjectOverride.String != "" {
		subject = subjectOverride.String
	}

	// 4. Inject unsubscribe footer (reuse existing per-campaign helpers)
	unsubToken := generateToken()
	unsubLink := "https://cryptolink.cc/api/dashboard/v1/marketing/unsubscribe?token=" + unsubToken
	_, _ = s.db.Exec(ctx,
		`INSERT INTO marketing_unsubscribes (email, token) VALUES ($1, $2)
		 ON CONFLICT (email) DO UPDATE SET token = $2`,
		recipientEmail, unsubToken)
	body := injectUnsubscribeFooter(tmpl.BodyHTML, unsubLink)

	// 5. Send
	if err := s.sendStepEmail(ctx, recipientEmail, subject, body); err != nil {
		s.markEnrollmentFailed(ctx, enrollID, attempts, err)
		return
	}

	// 6. Advance or complete
	var nextOffsetHours int
	err = s.db.QueryRow(ctx,
		`SELECT offset_hours FROM marketing_sequence_steps WHERE sequence_id = $1 AND step_index = $2`,
		sequenceID, nextStepIndex+1).Scan(&nextOffsetHours)
	if err != nil {
		_, _ = s.db.Exec(ctx,
			`UPDATE marketing_sequence_enrollments
			 SET current_step = $2, status = 'completed', updated_at = NOW(), last_error = NULL
			 WHERE id = $1`,
			enrollID, nextStepIndex)
		_ = s.maybeCompleteSequence(ctx, sequenceID)
		s.incrementQuota(ctx)
		return
	}

	var startedAt sql.NullTime
	_ = s.db.QueryRow(ctx,
		`SELECT started_at FROM marketing_sequences WHERE id = $1`, sequenceID).Scan(&startedAt)
	base := time.Now()
	if startedAt.Valid {
		base = startedAt.Time
	}
	newNextSendAt := base.Add(time.Duration(nextOffsetHours) * time.Hour)

	_, _ = s.db.Exec(ctx,
		`UPDATE marketing_sequence_enrollments
		 SET current_step = $2, next_send_at = $3, updated_at = NOW(), last_error = NULL
		 WHERE id = $1`,
		enrollID, nextStepIndex, newNextSendAt)
	s.incrementQuota(ctx)
}

func (s *Service) markEnrollmentFailed(ctx context.Context, enrollID int64, attempts int, err error) {
	const maxAttempts = 3
	newAttempts := attempts + 1
	if newAttempts >= maxAttempts {
		_, _ = s.db.Exec(ctx,
			`UPDATE marketing_sequence_enrollments
			 SET attempt_count = $2, last_error = $3, status = 'failed', updated_at = NOW()
			 WHERE id = $1`,
			enrollID, newAttempts, err.Error())
	} else {
		_, _ = s.db.Exec(ctx,
			`UPDATE marketing_sequence_enrollments
			 SET attempt_count = $2, last_error = $3, next_send_at = NOW() + INTERVAL '1 hour', updated_at = NOW()
			 WHERE id = $1`,
			enrollID, newAttempts, err.Error())
	}
	s.logger.Warn().Err(err).Int64("enrollment", enrollID).Int("attempt", newAttempts).Msg("seq: enrollment send failed")
}

func (s *Service) maybeCompleteSequence(ctx context.Context, sequenceID int64) error {
	var activeCount int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM marketing_sequence_enrollments WHERE sequence_id = $1 AND status = 'active'`,
		sequenceID).Scan(&activeCount); err != nil {
		return err
	}
	if activeCount > 0 {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`UPDATE marketing_sequences SET status = 'completed', completed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		sequenceID)
	return err
}
