package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// Handoff carries a logged-in dashboard session into a wallet app's built-in
// browser. A completed handoff is a full session (it can change the xpub and
// collectors), so:
//   - Claim never creates a session; only Complete does, exactly once.
//   - "qr" (PC -> phone): the phone shows a 6-digit code and the PC user must
//     type it to approve. A device label alone is attacker-controlled, and a
//     QR can be scanned by someone else.
//   - "direct" (same phone, native-scheme link): approved on claim, but the
//     phone shows "Continue as <masked email>" before Complete, so a link sent
//     by someone else can't silently sign the victim into a foreign account.
//   - Complete refuses to replace a session that belongs to another user.
//
// The token travels in the URL fragment, so nginx never logs it. In memory: a
// restart just means "generate a new code".
const (
	handoffTTL          = 2 * time.Minute
	handoffDirect       = "direct"
	handoffQR           = "qr"
	handoffMaxCodeTries = 3
)

var (
	errHandoffInvalid     = errors.New("invalid handoff request")
	errHandoffGone        = errors.New("handoff expired or already used")
	errHandoffBadCode     = errors.New("wrong confirmation code")
	errHandoffOtherUser   = errors.New("browser is signed in as another user")
	handoffAllowedReturns = map[string]bool{"/balance": true, "/wallet-setup": true}
)

type handoffEntry struct {
	id         string
	userID     int64
	email      string
	merchantID string
	returnPath string
	mode       string
	state      string // pending | claimed | approved | denied | done
	device     string
	matchCode  string
	codeTries  int
	expires    time.Time
}

type claimResult struct {
	claimSecret string
	matchCode   string // qr mode: shown on the phone, typed on the PC
	email       string
	mode        string
}

type handoffStore struct {
	mu       sync.Mutex
	byToken  map[string]*handoffEntry // key: hex(sha256(token))
	bySecret map[string]*handoffEntry // key: hex(sha256(claimSecret))
	byID     map[string]*handoffEntry
}

func newHandoffStore() *handoffStore {
	return &handoffStore{
		byToken:  map[string]*handoffEntry{},
		bySecret: map[string]*handoffEntry{},
		byID:     map[string]*handoffEntry{},
	}
}

func randomB64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashKey(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func maskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return "***"
	}
	keep := 2
	if at < keep {
		keep = at
	}
	return email[:keep] + "***" + email[at:]
}

func (s *handoffStore) gc(now time.Time) {
	for id, e := range s.byID {
		if now.Sub(e.expires) > handoffTTL { // keep final states visible to the PC for one extra TTL
			delete(s.byID, id)
		}
	}
	for k, e := range s.byToken {
		if now.After(e.expires) {
			delete(s.byToken, k)
		}
	}
	for k, e := range s.bySecret {
		if now.After(e.expires) {
			delete(s.bySecret, k)
		}
	}
}

func (s *handoffStore) Create(userID int64, email, merchantID, returnPath, mode string, now time.Time) (id, token string, expires time.Time, err error) {
	if !handoffAllowedReturns[returnPath] || (mode != handoffDirect && mode != handoffQR) {
		return "", "", time.Time{}, errHandoffInvalid
	}
	if merchantID != "" {
		if _, err := uuid.Parse(merchantID); err != nil {
			return "", "", time.Time{}, errHandoffInvalid
		}
	}
	if token, err = randomB64(32); err != nil {
		return "", "", time.Time{}, err
	}
	if id, err = randomB64(16); err != nil {
		return "", "", time.Time{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.gc(now)

	e := &handoffEntry{
		id: id, userID: userID, email: email, merchantID: merchantID, returnPath: returnPath,
		mode: mode, state: "pending", expires: now.Add(handoffTTL),
	}
	s.byToken[hashKey(token)] = e
	s.byID[id] = e

	return id, token, e.expires, nil
}

// Claim consumes the token under the lock, so concurrent claims cannot both win.
// It never creates a session: the phone gets a claim secret for Complete.
func (s *handoffStore) Claim(token, device string, now time.Time) (claimResult, error) {
	secret, err := randomB64(32)
	if err != nil {
		return claimResult{}, err
	}
	code, err := randomCode()
	if err != nil {
		return claimResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	k := hashKey(token)
	e, ok := s.byToken[k]
	if !ok {
		return claimResult{}, errHandoffGone
	}
	delete(s.byToken, k)
	if now.After(e.expires) || e.state != "pending" {
		return claimResult{}, errHandoffGone
	}

	if len(device) > 80 {
		device = device[:80]
	}
	e.device = device
	e.expires = now.Add(handoffTTL) // approval / confirmation window
	s.bySecret[hashKey(secret)] = e

	res := claimResult{claimSecret: secret, email: e.email, mode: e.mode}
	if e.mode == handoffDirect {
		e.state = "approved"
	} else {
		e.state = "claimed"
		e.matchCode = code
		res.matchCode = code
	}

	return res, nil
}

// Decide approves (with the code shown on the phone) or denies a QR claim.
func (s *handoffStore) Decide(id string, userID int64, approve bool, code string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.byID[id]
	if !ok || e.userID != userID || e.state != "claimed" || now.After(e.expires) {
		return errHandoffGone
	}

	if !approve {
		e.state = "denied"
		return nil
	}

	if subtle.ConstantTimeCompare([]byte(code), []byte(e.matchCode)) != 1 {
		e.codeTries++
		if e.codeTries >= handoffMaxCodeTries {
			e.state = "denied"
		}
		return errHandoffBadCode
	}

	e.state = "approved"

	return nil
}

// Complete returns done=true exactly once, after approval. currentUserID is
// the user already signed in on the calling browser (0 if none); a different
// user is refused rather than silently replaced.
func (s *handoffStore) Complete(claimSecret string, currentUserID int64, now time.Time) (handoffEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := hashKey(claimSecret)
	e, ok := s.bySecret[k]
	if !ok || now.After(e.expires) {
		return handoffEntry{}, false, errHandoffGone
	}

	switch e.state {
	case "claimed":
		return handoffEntry{}, false, nil
	case "approved":
		if currentUserID != 0 && currentUserID != e.userID {
			return handoffEntry{}, false, errHandoffOtherUser
		}
		delete(s.bySecret, k)
		e.state = "done"
		return *e, true, nil
	default:
		delete(s.bySecret, k)
		return handoffEntry{}, false, errHandoffGone
	}
}

func (s *handoffStore) Status(id string, userID int64, now time.Time) (status, device string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.byID[id]
	switch {
	case !ok || e.userID != userID:
		return "expired", ""
	case e.state == "done":
		return "approved", e.device
	case e.state == "denied":
		return "denied", e.device
	case now.After(e.expires):
		return "expired", ""
	default:
		return e.state, e.device
	}
}

var handoffGoneResponse = echo.Map{"message": "This link has expired. Generate a new one on your computer."}

func (h *Handler) PostHandoff(c echo.Context) error {
	var req struct {
		ReturnPath string `json:"returnPath"`
		MerchantID string `json:"merchantId"`
		Mode       string `json:"mode"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "Invalid JSON"})
	}

	u := middleware.ResolveUser(c)
	id, token, expires, err := h.handoffs.Create(u.ID, u.Email, req.MerchantID, req.ReturnPath, req.Mode, time.Now())
	switch {
	case errors.Is(err, errHandoffInvalid):
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "Invalid handoff request"})
	case err != nil:
		return errors.Wrap(err, "unable to create handoff")
	}

	return c.JSON(http.StatusCreated, echo.Map{"id": id, "token": token, "expiresAt": expires.UTC().Format(time.RFC3339)})
}

func (h *Handler) GetHandoffStatus(c echo.Context) error {
	u := middleware.ResolveUser(c)
	status, device := h.handoffs.Status(c.Param("id"), u.ID, time.Now())
	return c.JSON(http.StatusOK, echo.Map{"status": status, "device": device})
}

func (h *Handler) PostHandoffDecision(c echo.Context) error {
	var req struct {
		Approve bool   `json:"approve"`
		Code    string `json:"code"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "Invalid JSON"})
	}

	u := middleware.ResolveUser(c)
	err := h.handoffs.Decide(c.Param("id"), u.ID, req.Approve, strings.TrimSpace(req.Code), time.Now())
	switch {
	case errors.Is(err, errHandoffBadCode):
		return c.JSON(http.StatusUnprocessableEntity, echo.Map{"message": "Wrong code. Check the code shown on your phone."})
	case err != nil:
		return c.JSON(http.StatusBadRequest, handoffGoneResponse)
	}

	h.logger.Info().Int64("user_id", u.ID).Bool("approve", req.Approve).Msg("wallet handoff decision")

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) PostHandoffClaim(c echo.Context) error {
	var req struct {
		Token  string `json:"token"`
		Device string `json:"device"`
	}
	if err := c.Bind(&req); err != nil || req.Token == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "Invalid JSON"})
	}

	res, err := h.handoffs.Claim(req.Token, req.Device, time.Now())
	if err != nil {
		return c.JSON(http.StatusBadRequest, handoffGoneResponse)
	}

	return c.JSON(http.StatusOK, echo.Map{
		"claimSecret": res.claimSecret,
		"matchCode":   res.matchCode,
		"mode":        res.mode,
		"account":     maskEmail(res.email),
	})
}

func (h *Handler) PostHandoffComplete(c echo.Context) error {
	var req struct {
		ClaimSecret string `json:"claimSecret"`
	}
	if err := c.Bind(&req); err != nil || req.ClaimSecret == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "Invalid JSON"})
	}

	var currentUserID int64
	if u := middleware.ResolveUser(c); u != nil {
		currentUserID = u.ID
	}

	e, done, err := h.handoffs.Complete(req.ClaimSecret, currentUserID, time.Now())
	switch {
	case errors.Is(err, errHandoffOtherUser):
		return c.JSON(http.StatusConflict, echo.Map{"message": "This browser is signed in to another account. Log out first, then try again."})
	case err != nil:
		return c.JSON(http.StatusBadRequest, handoffGoneResponse)
	case !done:
		return c.JSON(http.StatusAccepted, echo.Map{"status": "waiting"})
	}

	if err := h.persistSession(c, "handoff", map[string]any{middleware.UserIDContextKey: e.userID}); err != nil {
		return errors.Wrap(err, "unable to persist session")
	}

	h.logger.Info().Int64("user_id", e.userID).Str("mode", e.mode).Str("ip", c.RealIP()).Msg("wallet handoff completed")

	return c.JSON(http.StatusOK, echo.Map{"returnPath": e.returnPath, "merchantId": e.merchantID})
}
