package merchant

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/cryptolink/cryptolink/internal/kms/wallet"
)

type Merchant struct {
	ID        int64
	UUID      uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Website   string
	CreatorID int64
	settings  Settings
}

const (
	PropertyWebhookURL      = "webhook.url"
	PropertySignatureSecret = "webhook.secret"
	PropertyPaymentMethods  = "payment.methods"
	PropertyFiatCurrency    = "fiat.currency"
)

func (m *Merchant) Settings() Settings {
	return m.settings
}

type Property string
type Settings map[Property]string

func (s Settings) WebhookURL() string {
	return s[PropertyWebhookURL]
}

func (s Settings) WebhookSignatureSecret() string {
	return s[PropertySignatureSecret]
}

func (s Settings) PaymentMethods() []string {
	raw := s[PropertyPaymentMethods]
	if raw == "" {
		return nil
	}

	return strings.Split(raw, ",")
}

// FiatCurrency returns the merchant's chosen billing fiat currency code. Defaults to "USD".
func (s Settings) FiatCurrency() string {
	if v := s[PropertyFiatCurrency]; v != "" {
		return v
	}
	return "USD"
}

// GlobalFeePercent returns the merchant's volatility buffer fee as a float (e.g. 2.5 for 2.5%).
// Returns 0 if not set or invalid.
func (s Settings) GlobalFeePercent() float64 {
	raw := s[Property("fee.global")]
	if raw == "" {
		return 0
	}
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil || val <= 0 || val > 100 {
		return 0
	}
	return val
}

func (s Settings) toJSONB() pgtype.JSONB {
	if len(s) == 0 {
		return pgtype.JSONB{Status: pgtype.Null}
	}

	raw, _ := json.Marshal(s)

	return pgtype.JSONB{Bytes: raw, Status: pgtype.Present}
}

type Address struct {
	ID             int64
	UUID           uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Name           string
	MerchantID     int64
	Blockchain     wallet.Blockchain
	BlockchainName string
	Address        string
}
