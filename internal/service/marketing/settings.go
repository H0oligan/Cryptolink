package marketing

import (
	"context"

	"github.com/pkg/errors"
)

type Settings struct {
	DailyLimit int    `json:"daily_limit"`
	UpdatedAt  string `json:"updated_at"`
}

const (
	settingsMinDailyLimit = 50
	settingsMaxDailyLimit = 250
)

func (s *Service) GetSettings(ctx context.Context) (*Settings, error) {
	var out Settings
	err := s.db.QueryRow(ctx,
		`SELECT daily_limit, COALESCE(to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')
		 FROM marketing_settings WHERE id = 1`,
	).Scan(&out.DailyLimit, &out.UpdatedAt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load marketing settings")
	}
	return &out, nil
}

func (s *Service) UpdateSettings(ctx context.Context, dailyLimit int) (*Settings, error) {
	if dailyLimit < settingsMinDailyLimit || dailyLimit > settingsMaxDailyLimit {
		return nil, errors.Errorf("daily_limit must be between %d and %d", settingsMinDailyLimit, settingsMaxDailyLimit)
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO marketing_settings (id, daily_limit, updated_at)
		 VALUES (1, $1, NOW())
		 ON CONFLICT (id) DO UPDATE SET daily_limit = EXCLUDED.daily_limit, updated_at = NOW()`,
		dailyLimit,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update marketing settings")
	}
	return s.GetSettings(ctx)
}

// effectiveDailyLimit returns the configured limit, falling back to the compile-time
// default when the settings row is unavailable.
func (s *Service) effectiveDailyLimit(ctx context.Context) int {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return DailyMarketingLimit
	}
	return settings.DailyLimit
}
