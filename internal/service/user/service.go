package user

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/cryptolink/cryptolink/internal/bus"
	"github.com/cryptolink/cryptolink/internal/db/repository"
	"github.com/cryptolink/cryptolink/internal/service/registry"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

const (
	// bcryptCost defines the computational cost for password hashing.
	// Cost of 12 provides strong security while remaining performant on modern hardware.
	bcryptCost = 12
)

type Service struct {
	store     repository.Storage
	publisher bus.Publisher
	registry  *registry.Service
	logger    *zerolog.Logger
}

type User struct {
	ID               int64
	Name             string
	Email            string
	UUID             uuid.UUID
	GoogleID         *string
	ProfileImageURL  *string
	IsSuperAdmin     bool
	CompanyName      string
	Address          string
	Website          string
	Phone            string
	EmailVerified    bool
	MarketingConsent bool
	TermsAcceptedAt  *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
	Settings         []byte
}

type RegisterParams struct {
	Email            string
	Password         string
	Name             string
	CompanyName      string
	Address          string
	Website          string
	Phone            string
	MarketingConsent bool
}

var (
	ErrNotFound      = errors.New("user not found")
	ErrWrongPassword = errors.New("wrong password provided")
	ErrAlreadyExists = errors.New("user already exists")
	ErrRestricted    = errors.New("access restricted")
)

const (
	registryRegistrationWhitelistOnly = "registration.is_whitelist_only"
	registryRegistrationWhitelist     = "registration.whitelist"
)

func New(store repository.Storage, pub bus.Publisher, registryService *registry.Service, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "user_service").Logger()

	return &Service{
		store:     store,
		publisher: pub,
		registry:  registryService,
		logger:    &log,
	}
}

func (s *Service) GetByID(ctx context.Context, id int64) (*User, error) {
	entry, err := s.store.GetUserByID(ctx, id)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToUser(entry)
}

func (s *Service) GetByEmail(ctx context.Context, email string) (*User, error) {
	entry, err := s.store.GetUserByEmail(ctx, email)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToUser(entry)
}

func (s *Service) GetByEmailWithPasswordCheck(ctx context.Context, email, password string) (*User, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}

	entry, err := s.store.GetUserByEmail(ctx, email)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	if !checkPass(entry.Password.String, password) {
		return nil, ErrWrongPassword
	}

	return entryToUser(entry)
}

// Register registers user via email. If user already exists, return User and ErrAlreadyExists
func (s *Service) Register(ctx context.Context, params RegisterParams) (*User, error) {
	if err := validateEmail(params.Email); err != nil {
		return nil, err
	}

	if len(params.Password) < 8 {
		return nil, errors.New("password should have minimum length of 8")
	}

	// check if exists
	u, err := s.GetByEmail(ctx, params.Email)
	switch {
	case err == nil:
		return u, ErrAlreadyExists
	case errors.Is(err, ErrNotFound):
		// do nothing
	case err != nil:
		return nil, err
	}

	hashedPass, err := hashPass(params.Password)
	if err != nil {
		return nil, err
	}

	displayName := strings.TrimSpace(params.Name)
	if displayName == "" {
		displayName = params.Email[:strings.IndexByte(params.Email, '@')]
	}

	now := time.Now()

	var termsAcceptedAt sql.NullTime
	termsAcceptedAt = sql.NullTime{Time: now, Valid: true}

	entry, err := s.store.CreateUser(ctx, repository.CreateUserParams{
		Name:             displayName,
		Email:            params.Email,
		Password:         repository.StringToNullable(hashedPass),
		Uuid:             uuid.New(),
		GoogleID:         sql.NullString{},
		ProfileImageUrl:  sql.NullString{},
		IsSuperAdmin:     sql.NullBool{},
		CreatedAt:        now,
		UpdatedAt:        now,
		DeletedAt:        sql.NullTime{},
		Settings:         pgtype.JSONB{Status: pgtype.Null},
		CompanyName:      repository.StringToNullable(params.CompanyName),
		Address:          repository.StringToNullable(params.Address),
		Website:          repository.StringToNullable(params.Website),
		Phone:            repository.StringToNullable(params.Phone),
		EmailVerified:    sql.NullBool{Bool: false, Valid: true},
		VerificationToken:        sql.NullString{},
		VerificationTokenExpires: sql.NullTime{},
		MarketingConsent:         sql.NullBool{Bool: params.MarketingConsent, Valid: true},
		TermsAcceptedAt:          termsAcceptedAt,
	})
	if err != nil {
		return nil, err
	}

	event := bus.UserRegisteredEvent{UserID: entry.ID}
	if err = s.publisher.Publish(bus.TopicUserRegistered, event); err != nil {
		s.logger.Error().Err(err).Msg("unable to publish event")
	}

	return entryToUser(entry)
}

// GenerateVerificationToken creates a crypto-random token for email verification
func (s *Service) GenerateVerificationToken(ctx context.Context, userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Wrap(err, "failed to generate random token")
	}

	token := hex.EncodeToString(b)
	expires := time.Now().Add(24 * time.Hour)

	err := s.store.SetVerificationToken(ctx, userID,
		sql.NullString{String: token, Valid: true},
		sql.NullTime{Time: expires, Valid: true},
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to save verification token")
	}

	return token, nil
}

// VerifyEmail validates the token and marks the user's email as verified
func (s *Service) VerifyEmail(ctx context.Context, token string) (*User, error) {
	entry, err := s.store.GetUserByVerificationToken(ctx, token)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, errors.New("invalid or expired verification token")
	case err != nil:
		return nil, errors.Wrap(err, "failed to look up verification token")
	}

	if err := s.store.UpdateEmailVerified(ctx, entry.ID); err != nil {
		return nil, errors.Wrap(err, "failed to verify email")
	}

	entry.EmailVerified = sql.NullBool{Bool: true, Valid: true}
	entry.VerificationToken = sql.NullString{}
	entry.VerificationTokenExpires = sql.NullTime{}

	return entryToUser(entry)
}

func (s *Service) UpdatePassword(ctx context.Context, id int64, pass string) (*User, error) {
	if len(pass) < 8 {
		return nil, errors.New("password should have minimum length of 8")
	}

	hashedPass, err := hashPass(pass)
	if err != nil {
		return nil, err
	}

	entry, err := s.store.UpdateUserPassword(ctx, repository.UpdateUserPasswordParams{
		ID:        id,
		Password:  repository.StringToNullable(hashedPass),
		UpdatedAt: time.Now(),
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToUser(entry)
}

// UpdateProfileParams contains fields for updating a user's profile.
type UpdateProfileParams struct {
	Name        string
	Email       string
	CompanyName string
	Address     string
	Website     string
	Phone       string
}

// UpdateProfile updates user profile fields.
func (s *Service) UpdateProfile(ctx context.Context, id int64, params UpdateProfileParams) (*User, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	newName := strings.TrimSpace(params.Name)
	if newName == "" {
		newName = existing.Name
	}

	newEmail := strings.TrimSpace(params.Email)
	if newEmail == "" {
		newEmail = existing.Email
	} else {
		if err := validateEmail(newEmail); err != nil {
			return nil, errors.Wrap(err, "invalid email")
		}
	}

	entry, err := s.store.UpdateUser(ctx, repository.UpdateUserParams{
		ID:              id,
		Name:            newName,
		ProfileImageUrl: sql.NullString{},
		GoogleID:        sql.NullString{},
		UpdatedAt:       time.Now(),
		SetGoogleID:     false,
		CompanyName:     sql.NullString{String: params.CompanyName, Valid: true},
		Address:         sql.NullString{String: params.Address, Valid: true},
		Website:         sql.NullString{String: params.Website, Valid: true},
		Phone:           sql.NullString{String: params.Phone, Valid: true},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to update user")
	}

	return entryToUser(entry)
}

// guardRegistration restricts user from registration is registration by whitelist is enabled.
func (s *Service) guardRegistration(ctx context.Context, email string) error {
	bouncer := s.registry.GetBoolSafe(ctx, registryRegistrationWhitelistOnly, false)
	if !bouncer {
		return nil
	}

	var matched bool

	whitelist := s.registry.GetStringsSafe(ctx, registryRegistrationWhitelist)
	for _, e := range whitelist {
		if e == email {
			matched = true
			break
		}
	}

	if !matched {
		s.logger.Error().Str("email", email).Msg("Restricted user registration due to enabled whitelist")
		return ErrRestricted
	}

	return nil
}

func entryToUser(entry repository.User) (*User, error) {
	isSuperAdmin := false
	if entry.IsSuperAdmin.Valid {
		isSuperAdmin = entry.IsSuperAdmin.Bool
	}

	emailVerified := false
	if entry.EmailVerified.Valid {
		emailVerified = entry.EmailVerified.Bool
	}

	marketingConsent := false
	if entry.MarketingConsent.Valid {
		marketingConsent = entry.MarketingConsent.Bool
	}

	var termsAcceptedAt *time.Time
	if entry.TermsAcceptedAt.Valid {
		termsAcceptedAt = &entry.TermsAcceptedAt.Time
	}

	return &User{
		ID:               entry.ID,
		Name:             entry.Name,
		Email:            entry.Email,
		UUID:             entry.Uuid,
		GoogleID:         repository.NullableStringToPointer(entry.GoogleID),
		ProfileImageURL:  repository.NullableStringToPointer(entry.ProfileImageUrl),
		IsSuperAdmin:     isSuperAdmin,
		CompanyName:      entry.CompanyName.String,
		Address:          entry.Address.String,
		Website:          entry.Website.String,
		Phone:            entry.Phone.String,
		EmailVerified:    emailVerified,
		MarketingConsent: marketingConsent,
		TermsAcceptedAt:  termsAcceptedAt,
		CreatedAt:        entry.CreatedAt,
		UpdatedAt:        entry.UpdatedAt,
		DeletedAt:        nil,
		Settings:         nil,
	}, nil
}

func validateEmail(email string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return err
	}

	return nil
}

func hashPass(pass string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(pass), bcryptCost)
	if err != nil {
		return "", err
	}

	return string(hashed), nil
}

func checkPass(hashed, pass string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(pass)) == nil
}
