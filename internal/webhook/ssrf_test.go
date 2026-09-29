package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsForbiddenIP(t *testing.T) {
	for _, ip := range []string{
		"127.0.0.1", "127.8.8.8", "10.1.2.3", "172.16.0.1", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "0.0.0.0", "224.0.0.1", "255.255.255.255",
		"::1", "::", "fe80::1", "fd00::1", "::ffff:127.0.0.1", "::ffff:169.254.169.254",
		"64:ff9b::a9fe:a9fe",
	} {
		assert.True(t, isForbiddenIP(netip.MustParseAddr(ip)), ip)
	}

	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		assert.False(t, isForbiddenIP(netip.MustParseAddr(ip)), ip)
	}
}

func TestValidateDestination(t *testing.T) {
	ctx := context.Background()

	for _, u := range []string{
		"http://127.0.0.1:3000/internal/v1/router",
		"http://localhost:8083/api/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]:6379/",
		"http://[::ffff:127.0.0.1]/",
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/_x",
		"https://user:pass@example.com/hook",
		"not a url",
	} {
		assert.ErrorIs(t, ValidateDestination(ctx, u), ErrForbiddenDestination, u)
	}

	assert.NoError(t, ValidateDestination(ctx, "https://1.1.1.1/hook"))
}

func TestSendBlocksInternalDestinations(t *testing.T) {
	ctx := context.Background()

	var hit bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	// Direct loopback target.
	assert.ErrorIs(t, Send(ctx, s.URL, "", map[string]string{}), ErrInvalidInput)

	// Loopback reached through a redirect from an (allowed) external-looking hop
	// is covered by the same dialer check; simulate with a redirect server.
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, s.URL, http.StatusTemporaryRedirect)
	}))
	defer redir.Close()
	assert.Error(t, Send(ctx, redir.URL, "", map[string]string{}))

	assert.False(t, hit, "internal server must never be reached")
}
