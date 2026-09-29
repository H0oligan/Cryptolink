package auth

import (
	_ "embed"
	"strings"
	"sync"
	"time"
)

// Brute-force protection keyed by the target account (email), because
// attackers rotate IPs (Tor exits on 2026-09-29), which defeats per-IP limits.
//
// Trade-off: someone hammering a known email can lock its owner out for up to
// loginLockWindow. That is preferable to unlimited password guessing.
const (
	loginMaxFailures = 10
	loginLockWindow  = 15 * time.Minute
)

type loginAttempts struct {
	failures int
	first    time.Time
}

type loginThrottle struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempts
	lastGC   time.Time
}

func newLoginThrottle() *loginThrottle {
	return &loginThrottle{attempts: make(map[string]*loginAttempts)}
}

func throttleKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Blocked reports whether the email has too many recent failed logins.
func (t *loginThrottle) Blocked(email string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	a, ok := t.attempts[throttleKey(email)]
	if !ok {
		return false
	}

	if now.Sub(a.first) > loginLockWindow {
		delete(t.attempts, throttleKey(email))
		return false
	}

	return a.failures >= loginMaxFailures
}

func (t *loginThrottle) Fail(email string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.gc(now)

	key := throttleKey(email)
	a, ok := t.attempts[key]
	if !ok || now.Sub(a.first) > loginLockWindow {
		t.attempts[key] = &loginAttempts{failures: 1, first: now}
		return
	}

	a.failures++
}

func (t *loginThrottle) Reset(email string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.attempts, throttleKey(email))
}

// gc drops expired entries at most once a minute so the map cannot grow
// without bound under a spray of random emails. Caller holds t.mu.
func (t *loginThrottle) gc(now time.Time) {
	if now.Sub(t.lastGC) < time.Minute {
		return
	}

	t.lastGC = now

	for k, a := range t.attempts {
		if now.Sub(a.first) > loginLockWindow {
			delete(t.attempts, k)
		}
	}
}

// disposableDomainsList is the community-maintained blocklist of throwaway
// inbox providers (CC0). The 2026-09-29 attacker cycled through mailinator.com
// and uberip.com, which is why a short hand-written list was not enough.
//
//go:embed disposable_domains.txt
var disposableDomainsList string

// extraDisposableEmailDomains are seen in attacks but missing upstream.
var extraDisposableEmailDomains = []string{}

var disposableEmailDomains = func() map[string]struct{} {
	m := make(map[string]struct{}, 10000)
	for _, line := range strings.Split(disposableDomainsList, "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		m[line] = struct{}{}
	}

	for _, d := range extraDisposableEmailDomains {
		m[d] = struct{}{}
	}

	return m
}()

// reservedEmailDomains are our own domains: nobody may self-register with them.
var reservedEmailDomains = map[string]struct{}{
	"cryptolink.cc": {},
}

// registrationEmailBlocked returns a user-facing reason, or "" if allowed.
func registrationEmailBlocked(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}

	domain := strings.ToLower(strings.TrimSpace(email[at+1:]))

	if _, ok := reservedEmailDomains[domain]; ok {
		return "This email domain cannot be used for registration"
	}

	for d := domain; d != ""; {
		if _, ok := disposableEmailDomains[d]; ok {
			return "Disposable email addresses are not allowed"
		}

		dot := strings.Index(d, ".")
		if dot < 0 {
			break
		}

		d = d[dot+1:] // also match subdomains, e.g. x.mailinator.com
	}

	return ""
}
