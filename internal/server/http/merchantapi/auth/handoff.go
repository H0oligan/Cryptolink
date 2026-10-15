package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// Handoff carries a logged-in dashboard session into a wallet app's built-in
// browser. "direct" (same phone, native-scheme deep link) completes on claim.
// "qr" (PC -> phone) needs the PC user to approve the claiming device, because
// a QR can be scanned by someone else and https wallet links pass through the
// vendor's servers. In memory: a restart just means "generate a new code".
const (
	handoffTTL    = 2 * time.Minute
	handoffDirect = "direct"
	handoffQR     = "qr"
)

var (
	errHandoffInvalid     = errors.New("invalid handoff request")
	errHandoffGone        = errors.New("handoff expired or already used")
	handoffAllowedReturns = map[string]bool{"/balance": true, "/wallet-setup": true}
)

type handoffEntry struct {
	id         string
	userID     int64
	merchantID string
	returnPath string
	mode       string
	state      string // pending | claimed | approved | denied | done
	device     string
	expires    time.Time
}

type claimResult struct {
	entry       handoffEntry
	complete    bool   // direct mode: create the session now
	claimSecret string // qr mode: phone polls Complete with this
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

func hashKey(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
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

func (s *handoffStore) Create(userID int64, merchantID, returnPath, mode string, now time.Time) (id, token string, expires time.Time, err error) {
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

	e := &handoffEntry{id: id, userID: userID, merchantID: merchantID, returnPath: returnPath, mode: mode, state: "pending", expires: now.Add(handoffTTL)}
	s.byToken[hashKey(token)] = e
	s.byID[id] = e

	return id, token, e.expires, nil
}

// Claim consumes the token under the lock, so concurrent claims cannot both win.
func (s *handoffStore) Claim(token, device string, now time.Time) (claimResult, error) {
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

	if e.mode == handoffDirect {
		e.state = "done"
		return claimResult{entry: *e, complete: true}, nil
	}

	secret, err := randomB64(32)
	if err != nil {
		return claimResult{}, err
	}
	e.state = "claimed"
	e.expires = now.Add(handoffTTL) // approval window
	s.bySecret[hashKey(secret)] = e

	return claimResult{entry: *e, claimSecret: secret}, nil
}

func (s *handoffStore) Decide(id string, userID int64, approve bool, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.byID[id]
	if !ok || e.userID != userID || e.state != "claimed" || now.After(e.expires) {
		return errHandoffGone
	}
	if approve {
		e.state = "approved"
	} else {
		e.state = "denied"
	}

	return nil
}

// Complete returns done=true exactly once, after approval.
func (s *handoffStore) Complete(claimSecret string, now time.Time) (handoffEntry, bool, error) {
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
	id, token, expires, err := h.handoffs.Create(u.ID, req.MerchantID, req.ReturnPath, req.Mode, time.Now())
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
		Approve bool `json:"approve"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "Invalid JSON"})
	}

	u := middleware.ResolveUser(c)
	if err := h.handoffs.Decide(c.Param("id"), u.ID, req.Approve, time.Now()); err != nil {
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

	if !res.complete {
		return c.JSON(http.StatusAccepted, echo.Map{"claimSecret": res.claimSecret})
	}

	return h.finishHandoff(c, res.entry)
}

func (h *Handler) PostHandoffComplete(c echo.Context) error {
	var req struct {
		ClaimSecret string `json:"claimSecret"`
	}
	if err := c.Bind(&req); err != nil || req.ClaimSecret == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"message": "Invalid JSON"})
	}

	e, done, err := h.handoffs.Complete(req.ClaimSecret, time.Now())
	switch {
	case err != nil:
		return c.JSON(http.StatusBadRequest, handoffGoneResponse)
	case !done:
		return c.JSON(http.StatusAccepted, echo.Map{"status": "waiting"})
	}

	return h.finishHandoff(c, e)
}

func (h *Handler) finishHandoff(c echo.Context, e handoffEntry) error {
	if err := h.persistSession(c, "handoff", map[string]any{middleware.UserIDContextKey: e.userID}); err != nil {
		return errors.Wrap(err, "unable to persist session")
	}

	h.logger.Info().Int64("user_id", e.userID).Str("mode", e.mode).Str("ip", c.RealIP()).Msg("wallet handoff completed")

	return c.JSON(http.StatusOK, echo.Map{"returnPath": e.returnPath, "merchantId": e.merchantID})
}
