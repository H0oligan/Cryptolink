package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
)

// Every per-IP rate limit keys on RealIP, so a client must not be able to
// choose its own value with a forged X-Forwarded-For.
func TestServer_RealIPIgnoresForgedForwardedFor(t *testing.T) {
	srv := New(Config{Address: "127.0.0.1", Port: "0"})

	for _, tc := range []struct {
		name       string
		remoteAddr string
		realIP     string
		expected   string
	}{
		{name: "behind nginx uses X-Real-IP", remoteAddr: "127.0.0.1:50000", realIP: "203.0.113.7", expected: "203.0.113.7"},
		{name: "direct peer cannot claim an address", remoteAddr: "198.51.100.9:50000", realIP: "203.0.113.7", expected: "198.51.100.9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			req.Header.Set(echo.HeaderXForwardedFor, "6.6.6.6")
			req.Header.Set(echo.HeaderXRealIP, tc.realIP)

			c := srv.echo.NewContext(req, httptest.NewRecorder())

			assert.Equal(t, tc.expected, c.RealIP())
		})
	}
}
