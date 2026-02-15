package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
)

// GuardsSuperAdmin ensures the user is a super admin
func GuardsSuperAdmin() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			u := ResolveUser(c)
			if u == nil {
				return c.JSON(http.StatusUnauthorized, &model.ErrorResponse{
					Errors:  nil,
					Message: "Authentication required",
					Status:  "unauthorized",
				})
			}

			// Check if user is super admin
			if !u.IsSuperAdmin {
				return c.JSON(http.StatusForbidden, &model.ErrorResponse{
					Errors:  nil,
					Message: "Super admin access required",
					Status:  "forbidden",
				})
			}

			return next(c)
		}
	}
}
