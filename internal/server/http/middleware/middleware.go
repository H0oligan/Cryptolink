package middleware

import (
	"runtime"
	"slices"
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/v5/session"
	"github.com/labstack/echo/v5"
	mw "github.com/labstack/echo/v5/middleware"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/util"
	"github.com/rs/zerolog"
)

type SessionConfig struct {
	FilesystemPath string `yaml:"path" env:"SESSION_FS_PATH" env-description:"Path to sessions directory"`
	Secret         string `yaml:"secret" env:"SESSION_SECRET" env-description:"Sessions secret, use random string with 8+ characters"`
	CookieDomain   string `yaml:"cookie_domain" env:"SESSION_COOKIE_DOMAIN" env-description:"Cookie domain"`
	CookiePath     string `yaml:"cookie_path" env:"SESSION_COOKIE_PATH" env-default:"/" env-description:"Cookie path"`
	CookieMaxAge   int    `yaml:"cookie_max_age" env:"SESSION_COOKIE_MAX_AGE" env-default:"86400" env-description:"Cookie max-age"` // 60*60*24 = 24h
	CookieSecure   bool   `yaml:"cookie_secure" env:"SESSION_COOKIE_SECURE" env-description:"Allow secure cookies only"`
	CookieHTTPOnly bool   `yaml:"cookie_http_only" env:"SESSION_COOKIE_HTTP_ONLY" env-default:"true" env-description:"Allow HTTP cookies only"`
	CookieSameSite string `yaml:"cookie_same_site" env:"SESSION_COOKIE_SAME_SITE" env-default:"default" env-description:"Cookie same site"`
}

const sessionOptionsKey = "session_options"

const chmodReadWrite = 0660

func Session(cfg SessionConfig) echo.MiddlewareFunc {
	if err := util.EnsureDirectory(cfg.FilesystemPath, chmodReadWrite); err != nil {
		panic(fmt.Sprintf("unable to ensure session directory %q", cfg.FilesystemPath))
	}

	sessionOptions := sessions.Options{
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		MaxAge:   cfg.CookieMaxAge,
		HttpOnly: cfg.CookieHTTPOnly,
		Secure:   cfg.CookieSecure,
		SameSite: parseSameSite(cfg.CookieSameSite),
	}

	applySessionOptions := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(sessionOptionsKey, sessionOptions)
			return next(c)
		}
	}

	sessionMW := session.Middleware(sessions.NewFilesystemStore(cfg.FilesystemPath, []byte(cfg.Secret)))

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return sessionMW(applySessionOptions(next))
	}
}

// CSRFConfig Cross Site Request Forgery config.
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie
type CSRFConfig struct {
	// CookieDomain Defines the host to which the cookie will be sent.
	// If omitted, this attribute defaults to the host of the current document URL, not including subdomains.
	// Multiple host/domain values are not allowed, but if a domain is specified, then subdomains are always included.
	CookieDomain   string `yaml:"cookie_domain" env:"CSRF_COOKIE_DOMAIN" env-description:"CSRF (Cross Site Request Forgery) cookie domain"`
	CookiePath     string `yaml:"cookie_path" env:"CSRF_COOKIE_PATH" env-default:"/" env-description:"CSRF cookie path"`
	CookieMaxAge   int    `yaml:"cookie_max_age" env:"CSRF_COOKIE_MAX_AGE" env-default:"7200" env-description:"CSRF cookie max-age"` // 2h
	CookieSecure   bool   `yaml:"cookie_secure" env:"CSRF_COOKIE_SECURE" env-description:"CSRF secure cookie"`
	CookieHTTPOnly bool   `yaml:"cookie_http_only" env:"CSRF_COOKIE_HTTP_ONLY" env-default:"true" env-description:"CSRF HTTP only cookie"`
	CookieSameSite string `yaml:"cookie_same_site" env:"CSRF_COOKIE_SAME_SITE" env-default:"default" env-description:"CSRF cookie same site"`
}

func CSRF(cfg CSRFConfig) echo.MiddlewareFunc {
	return mw.CSRFWithConfig(mw.CSRFConfig{
		CookieDomain:   cfg.CookieDomain,
		CookiePath:     cfg.CookiePath,
		CookieHTTPOnly: cfg.CookieHTTPOnly,
		CookieSecure:   cfg.CookieSecure,
		CookieMaxAge:   cfg.CookieMaxAge,
		CookieSameSite: parseSameSite(cfg.CookieSameSite),

		// Echo v5 rejects every cross-site write outright. Fall back to the token
		// check instead, so origins allowed by CORS (e.g. a self-hosted payment
		// domain) keep working. Same-origin browser requests are trusted through
		// Sec-Fetch-Site, which page scripts cannot forge.
		AllowSecFetchSiteFunc: func(_ *echo.Context) (bool, error) {
			return false, nil
		},

		Skipper: func(c *echo.Context) bool {
			// if token-based auth -> skip

			tokenAuthRaw := c.Get(IsTokenAuthContextKey)
			isTokenAuth, ok := tokenAuthRaw.(bool)

			return ok && isTokenAuth
		},
	})
}

type CORSConfig struct {
	AllowCredentials bool     `yaml:"allow_credentials" env:"CORS_ALLOW_CREDENTIALS" env-default:"true" env-description:"CORS (Cross Origin Resource Sharing) allow credentials"`
	AllowOrigins     []string `yaml:"allow_origins" env:"CORS_ALLOW_ORIGINS" env-description:"Comma separated list (without spaces) of origins (with schema). Example: https://pay.site.com"`
}

func CORS(cfg CORSConfig) echo.MiddlewareFunc {
	// Echo v5 refuses to start without origins, so no origins means same-origin only.
	if len(cfg.AllowOrigins) == 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}

	// Browsers never send credentials to a wildcard origin, and v5 refuses that combination.
	allowCredentials := cfg.AllowCredentials && !slices.Contains(cfg.AllowOrigins, "*")

	return mw.CORSWithConfig(mw.CORSConfig{
		AllowCredentials: allowCredentials,
		AllowOrigins:     cfg.AllowOrigins,
		AllowHeaders: []string{
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderCacheControl,
			CSRFTokenHeader,
		},
	})
}

const recoverStackSize = 4 << 10 // 4 KB

func Recover(logger *zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			defer func() {
				r := recover()
				if r == nil {
					return
				}

				if r == http.ErrAbortHandler {
					panic(r)
				}

				panicErr, ok := r.(error)
				if !ok {
					panicErr = fmt.Errorf("%v", r)
				}

				stack := make([]byte, recoverStackSize)
				stack = stack[:runtime.Stack(stack, true)]

				req := c.Request()

				evt := logger.Error().Err(panicErr)

				evt.Str("stack", string(stack))
				evt.Str("remote_ip", c.RealIP())
				evt.Str("host", req.Host)
				evt.Str("method", req.Method)
				evt.Str("uri", req.RequestURI)
				evt.Str("user_agent", req.UserAgent())
				evt.Str("referer", req.Referer())

				cl := req.Header.Get(echo.HeaderContentLength)
				if cl == "" {
					cl = "0"
				}
				evt.Str("bytes_in", cl)

				evt.Msg("PANIC RECOVERED")

				err = common.ErrorResponse(c, "server error")
			}()

			return next(c)
		}
	}
}

// SecurityHeaders adds hardening headers to every response.
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			h := c.Response().Header()
			h.Set("X-Frame-Options", "SAMEORIGIN")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-XSS-Protection", "1; mode=block")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			return next(c)
		}
	}
}

// VersionHeader stamps every response with the public CryptoLink release
// version (single-decimal scheme: 1.0, 2.0, 3.0, …). Self-hosted operators
// can verify what they're running with `curl -I https://example.com/`
// without shelling into the box.
func VersionHeader(release string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Response().Header().Set("X-CryptoLink-Version", release)
			return next(c)
		}
	}
}

func parseSameSite(v string) http.SameSite {
	switch v {
	case "lax":
		return http.SameSiteLaxMode
	case "none":
		return http.SameSiteNoneMode
	case "strict":
		return http.SameSiteStrictMode
	default:
		return http.SameSiteDefaultMode
	}
}
