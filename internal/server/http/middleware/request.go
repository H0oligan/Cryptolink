package middleware

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	mw "github.com/labstack/echo/v5/middleware"
)

func BodyDump() echo.MiddlewareFunc {
	return mw.BodyDump(func(c *echo.Context, req, res []byte, _ error) {
		tpl := "%s %s. Response: %s"
		args := []any{
			c.Request().Method,
			c.Request().URL.Path,
		}

		if len(req) > 0 {
			tpl = "%s %s with body %s. Response: %s"
			args = append(args, string(req))
		}

		args = append(args, string(res))

		c.Logger().Info(fmt.Sprintf(tpl, args...))
	})
}

const RequestIDKey = "request_id"

type ctxRequestID struct{}

func RequestID() echo.MiddlewareFunc {
	return mw.RequestIDWithConfig(mw.RequestIDConfig{
		Generator: func() string { return uuid.New().String() },
		RequestIDHandler: func(c *echo.Context, requestID string) {
			c.Set(RequestIDKey, requestID)

			ctx := context.WithValue(c.Request().Context(), ctxRequestID{}, requestID)

			c.SetRequest(c.Request().WithContext(ctx))
		},
	})
}

func RequestIDFromCtx(ctx context.Context) string {
	id := ctx.Value(ctxRequestID{})
	if id == nil {
		return ""
	}

	return id.(string)
}
