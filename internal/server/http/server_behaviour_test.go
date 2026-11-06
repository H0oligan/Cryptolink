package http_test

import (
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cryptolink/cryptolink/internal/server/http"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/labstack/echo/v5"
	mw "github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serve(s *http.Server, r *nethttp.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.Echo().ServeHTTP(w, r)

	return w
}

func TestUnmatchedRoutesKeepTheirStatusCodes(t *testing.T) {
	s := http.New(http.Config{})
	s.Echo().GET("/api/thing", func(c *echo.Context) error { return c.NoContent(nethttp.StatusOK) })

	assert.Equal(t, nethttp.StatusNotFound, serve(s, httptest.NewRequest(nethttp.MethodGet, "/api/missing", nil)).Code)
	assert.Equal(t, nethttp.StatusMethodNotAllowed, serve(s, httptest.NewRequest(nethttp.MethodPost, "/api/thing", nil)).Code)
}

func TestCSRF(t *testing.T) {
	s := http.New(http.Config{})
	g := s.Echo().Group("/api", middleware.CSRF(middleware.CSRFConfig{CookiePath: "/", CookieMaxAge: 60}))
	g.GET("/csrf-cookie", func(c *echo.Context) error {
		c.Response().Header().Set(echo.HeaderXCSRFToken, c.Get("csrf").(string))
		return c.NoContent(nethttp.StatusOK)
	})
	g.POST("/write", func(c *echo.Context) error { return c.NoContent(nethttp.StatusOK) })

	post := func(secFetchSite, cookie, header string) int {
		r := httptest.NewRequest(nethttp.MethodPost, "/api/write", nil)
		if secFetchSite != "" {
			r.Header.Set(echo.HeaderSecFetchSite, secFetchSite)
		}
		if cookie != "" {
			r.AddCookie(&nethttp.Cookie{Name: "_csrf", Value: cookie})
		}
		if header != "" {
			r.Header.Set(middleware.CSRFTokenHeader, header)
		}

		return serve(s, r).Code
	}

	issued := serve(s, httptest.NewRequest(nethttp.MethodGet, "/api/csrf-cookie", nil))
	require.Equal(t, nethttp.StatusOK, issued.Code)
	token := issued.Header().Get(echo.HeaderXCSRFToken)
	require.NotEmpty(t, token)
	require.Contains(t, issued.Header().Get("Set-Cookie"), "_csrf="+token)

	// Same-origin browser requests are trusted through Sec-Fetch-Site.
	assert.Equal(t, nethttp.StatusOK, post("same-origin", "", ""))

	// Everything else must prove it holds the token.
	for _, site := range []string{"", "cross-site", "same-site"} {
		assert.Equal(t, nethttp.StatusBadRequest, post(site, "", ""), "no token, site %q", site)
		assert.Equal(t, nethttp.StatusForbidden, post(site, token, "wrong"), "bad token, site %q", site)
		assert.Equal(t, nethttp.StatusOK, post(site, token, token), "good token, site %q", site)
	}
}

func TestCORS(t *testing.T) {
	preflight := func(s *http.Server, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(nethttp.MethodOptions, "/api/x", nil)
		r.Header.Set(echo.HeaderOrigin, origin)
		r.Header.Set(echo.HeaderAccessControlRequestMethod, nethttp.MethodPost)

		return serve(s, r)
	}
	build := func(cfg middleware.CORSConfig) *http.Server {
		s := http.New(http.Config{})
		s.Echo().Group("/api", middleware.CORS(cfg)).POST("/x", func(c *echo.Context) error { return nil })

		return s
	}
	const allowOrigin = echo.HeaderAccessControlAllowOrigin

	s := build(middleware.CORSConfig{AllowCredentials: true, AllowOrigins: []string{"https://pay.example.com"}})
	allowed := preflight(s, "https://pay.example.com")
	assert.Equal(t, "https://pay.example.com", allowed.Header().Get(allowOrigin))
	assert.Equal(t, "true", allowed.Header().Get(echo.HeaderAccessControlAllowCredentials))
	assert.Empty(t, preflight(s, "https://evil.example.com").Header().Get(allowOrigin))

	assert.Empty(t, preflight(build(middleware.CORSConfig{AllowCredentials: true}), "https://evil.example.com").Header().Get(allowOrigin))

	wildcard := preflight(build(middleware.CORSConfig{AllowCredentials: true, AllowOrigins: []string{"*"}}), "https://any.example.com")
	assert.Empty(t, wildcard.Header().Get(echo.HeaderAccessControlAllowCredentials))
}

func TestClientIPIgnoresSpoofedForwardedFor(t *testing.T) {
	s := http.New(http.Config{})
	s.Echo().GET("/ip", func(c *echo.Context) error { return c.String(nethttp.StatusOK, c.RealIP()) })

	request := func(peer string) string {
		r := httptest.NewRequest(nethttp.MethodGet, "/ip", nil)
		r.RemoteAddr = peer
		r.Header.Set(echo.HeaderXRealIP, "203.0.113.7")
		r.Header.Set(echo.HeaderXForwardedFor, "198.51.100.1")

		return serve(s, r).Body.String()
	}

	assert.Equal(t, "203.0.113.7", request("127.0.0.1:40000"), "nginx on loopback is trusted")
	assert.Equal(t, "198.18.0.9", request("198.18.0.9:40000"), "a direct client cannot spoof its address")
}

func TestRateLimiterRejectsOverBurst(t *testing.T) {
	s := http.New(http.Config{})
	store := mw.NewRateLimiterMemoryStoreWithConfig(mw.RateLimiterMemoryStoreConfig{Rate: 0.001, Burst: 2})
	s.Echo().GET("/limited", func(c *echo.Context) error { return c.NoContent(nethttp.StatusOK) }, mw.RateLimiter(store))

	request := func(ip string) int {
		r := httptest.NewRequest(nethttp.MethodGet, "/limited", nil)
		r.RemoteAddr = "127.0.0.1:40000"
		r.Header.Set(echo.HeaderXRealIP, ip)

		return serve(s, r).Code
	}

	assert.Equal(t, nethttp.StatusOK, request("203.0.113.1"))
	assert.Equal(t, nethttp.StatusOK, request("203.0.113.1"))
	assert.Equal(t, nethttp.StatusTooManyRequests, request("203.0.113.1"))
	assert.Equal(t, nethttp.StatusOK, request("203.0.113.2"), "limits are per client")
}

func TestRecoverReturnsGenericErrorAndLogsPanic(t *testing.T) {
	var logs strings.Builder
	logger := zerolog.New(&logs)

	s := http.New(http.Config{}, http.WithLogger(&logger), http.WithRecover())
	s.Echo().GET("/boom", func(c *echo.Context) error { panic("secret detail") })

	res := serve(s, httptest.NewRequest(nethttp.MethodGet, "/boom", nil))

	assert.Equal(t, nethttp.StatusInternalServerError, res.Code)
	assert.NotContains(t, res.Body.String(), "secret detail")

	var body map[string]any
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	assert.Contains(t, logs.String(), "PANIC RECOVERED")
	assert.Contains(t, logs.String(), "secret detail")
}

func TestEmbeddedFrontend(t *testing.T) {
	files := fstest.MapFS{
		"index.html":    {Data: []byte("<html>spa</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	s := http.New(http.Config{}, http.WithEmbeddedFrontend(files, files))

	get := func(path string) *httptest.ResponseRecorder {
		return serve(s, httptest.NewRequest(nethttp.MethodGet, path, nil))
	}

	for _, prefix := range []string{"/dashboard", "/p"} {
		spaRoute := get(prefix + "/pay/abc")
		assert.Equal(t, nethttp.StatusOK, spaRoute.Code, prefix)
		assert.Contains(t, spaRoute.Body.String(), "spa", "client routes fall back to index.html")
		assert.Contains(t, spaRoute.Header().Get("Cache-Control"), "no-store", "HTML must not be cached")

		asset := get(prefix + "/assets/app.js")
		assert.Equal(t, nethttp.StatusOK, asset.Code)
		assert.Contains(t, asset.Body.String(), "console.log")

		for _, attack := range []string{"/../../../etc/passwd", "/..%2f..%2f..%2fetc%2fpasswd", "/%2e%2e/%2e%2e/etc/passwd"} {
			assert.NotContains(t, get(prefix+attack).Body.String(), "root:", attack)
		}
	}
}
