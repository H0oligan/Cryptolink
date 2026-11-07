package subscriptionapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cryptolink/cryptolink/internal/db/repository"
	"github.com/cryptolink/cryptolink/internal/event/paymentevents"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/server/http/subscriptionapi"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/user"
	"github.com/cryptolink/cryptolink/internal/test"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminPaymentRoutes(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	u, _ := tc.Must.CreateUserViaEmail(t, "support@example.com", "qwerty123")
	mt, _ := tc.Must.CreateMerchant(t, u.ID)

	p, err := tc.Services.Payment.CreatePayment(tc.Context, mt.ID, payment.CreatePaymentProps{
		MerchantOrderUUID: uuid.New(),
		Money:             lo.Must(money.USD.MakeAmount("5000")),
	})
	require.NoError(t, err)
	_, err = tc.Repository.UpdatePayment(tc.Context, repository.UpdatePaymentParams{
		ID: p.ID, MerchantID: mt.ID, Status: payment.StatusSuccess.String(), UpdatedAt: time.Now(),
	})
	require.NoError(t, err)

	var resendErr error
	var resent []int64
	resend := func(_ context.Context, _, paymentID int64) error {
		resent = append(resent, paymentID)
		return resendErr
	}

	logger := zerolog.Nop()
	h := subscriptionapi.New(nil, tc.Services.Payment, nil, nil, nil, 0, resend, &logger)

	e := echo.New()
	admin := e.Group("/admin", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			isAdmin := c.Request().Header.Get("X-Test-Admin") == "1"
			c.Set(middleware.UserContextKey, &user.User{ID: u.ID, IsSuperAdmin: isAdmin})
			return next(c)
		}
	}, middleware.GuardsSuperAdmin())
	admin.GET("/payments/pending-webhooks", h.AdminListPendingWebhookPayments)
	admin.GET("/payments/:paymentId", h.AdminGetPayment)
	admin.POST("/payments/:paymentId/resend-webhook", h.AdminResendWebhook)

	do := func(method, path string, isAdmin bool) (int, map[string]any) {
		req := httptest.NewRequest(method, path, nil)
		if isAdmin {
			req.Header.Set("X-Test-Admin", "1")
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}

	byID := "/admin/payments/" + p.PublicID.String()

	t.Run("non-admins are refused on every route", func(t *testing.T) {
		for _, r := range [][2]string{
			{http.MethodGet, "/admin/payments/pending-webhooks"},
			{http.MethodGet, byID},
			{http.MethodPost, byID + "/resend-webhook"},
		} {
			code, _ := do(r[0], r[1], false)
			assert.Equal(t, http.StatusForbidden, code, r[1])
		}
		assert.Empty(t, resent)
	})

	t.Run("finds a payment across merchants", func(t *testing.T) {
		code, body := do(http.MethodGet, byID, true)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, p.PublicID.String(), body["public_id"])
		assert.Equal(t, "success", body["status"])
		assert.Equal(t, false, body["webhook_delivered"])
	})

	t.Run("bad and unknown ids", func(t *testing.T) {
		code, _ := do(http.MethodGet, "/admin/payments/not-a-uuid", true)
		assert.Equal(t, http.StatusBadRequest, code)

		code, _ = do(http.MethodGet, "/admin/payments/"+uuid.New().String(), true)
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("lists a payment the merchant has not acknowledged only if it has a webhook URL", func(t *testing.T) {
		code, body := do(http.MethodGet, "/admin/payments/pending-webhooks", true)
		require.Equal(t, http.StatusOK, code)
		assert.Empty(t, body["results"], "no webhook URL, so nothing is owed")
	})

	t.Run("resend reports the real outcome", func(t *testing.T) {
		_, body := do(http.MethodPost, byID+"/resend-webhook", true)
		assert.Equal(t, false, body["success"], "nothing was acknowledged")
		assert.Contains(t, resent, p.ID)

		resendErr = paymentevents.ErrWebhookURLNotSet
		_, body = do(http.MethodPost, byID+"/resend-webhook", true)
		assert.Equal(t, false, body["success"])
		assert.Contains(t, body["message"], "no webhook URL")

		resendErr = payment.ErrWebhookNotResendable
		_, body = do(http.MethodPost, byID+"/resend-webhook", true)
		assert.Equal(t, false, body["success"])
		assert.Contains(t, body["message"], "not settled")
	})
}
