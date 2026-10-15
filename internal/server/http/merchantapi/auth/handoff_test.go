package auth

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMerchant = "6f1c9b8e-2d3a-4f5b-9c7d-1e2f3a4b5c6d"

func TestHandoffStore(t *testing.T) {
	now := time.Now()

	t.Run("direct: claim creates the session once, never twice", func(t *testing.T) {
		s := newHandoffStore()
		_, token, _, err := s.Create(7, testMerchant, "/balance", handoffDirect, now)
		require.NoError(t, err)

		res, err := s.Claim(token, "MetaMask · iPhone", now)
		require.NoError(t, err)
		assert.True(t, res.complete)
		assert.Equal(t, int64(7), res.entry.userID)

		_, err = s.Claim(token, "x", now)
		assert.ErrorIs(t, err, errHandoffGone)
	})

	t.Run("qr: no session until the creator approves", func(t *testing.T) {
		s := newHandoffStore()
		id, token, _, _ := s.Create(7, "", "/wallet-setup", handoffQR, now)

		res, err := s.Claim(token, "Trust · Android", now)
		require.NoError(t, err)
		assert.False(t, res.complete)
		require.NotEmpty(t, res.claimSecret)

		st, dev := s.Status(id, 7, now)
		assert.Equal(t, "claimed", st)
		assert.Equal(t, "Trust · Android", dev)

		_, done, err := s.Complete(res.claimSecret, now)
		require.NoError(t, err)
		assert.False(t, done, "waiting: not approved yet")

		assert.ErrorIs(t, s.Decide(id, 8, true, now), errHandoffGone, "only the creator can approve")
		require.NoError(t, s.Decide(id, 7, true, now))

		e, done, err := s.Complete(res.claimSecret, now)
		require.NoError(t, err)
		assert.True(t, done)
		assert.Equal(t, "/wallet-setup", e.returnPath)

		_, _, err = s.Complete(res.claimSecret, now)
		assert.ErrorIs(t, err, errHandoffGone, "complete is single-use")
	})

	t.Run("qr: deny kills the claim", func(t *testing.T) {
		s := newHandoffStore()
		id, token, _, _ := s.Create(7, "", "/balance", handoffQR, now)
		res, _ := s.Claim(token, "x", now)
		require.NoError(t, s.Decide(id, 7, false, now))
		_, _, err := s.Complete(res.claimSecret, now)
		assert.ErrorIs(t, err, errHandoffGone)
		st, _ := s.Status(id, 7, now)
		assert.Equal(t, "denied", st)
	})

	t.Run("expiry: unclaimed token and unapproved claim both expire", func(t *testing.T) {
		s := newHandoffStore()
		_, token, _, _ := s.Create(7, "", "/balance", handoffQR, now)
		_, err := s.Claim(token, "x", now.Add(handoffTTL+time.Second))
		assert.ErrorIs(t, err, errHandoffGone)

		id, token2, _, _ := s.Create(7, "", "/balance", handoffQR, now)
		res, _ := s.Claim(token2, "x", now)
		later := now.Add(handoffTTL + time.Second)
		assert.ErrorIs(t, s.Decide(id, 7, true, later), errHandoffGone)
		_, _, err = s.Complete(res.claimSecret, later)
		assert.ErrorIs(t, err, errHandoffGone)
	})

	t.Run("status hides other users' handoffs", func(t *testing.T) {
		s := newHandoffStore()
		id, _, _, _ := s.Create(7, "", "/balance", handoffQR, now)
		st, _ := s.Status(id, 7, now)
		assert.Equal(t, "pending", st)
		st, _ = s.Status(id, 8, now)
		assert.Equal(t, "expired", st)
		st, _ = s.Status("unknown", 7, now)
		assert.Equal(t, "expired", st)
	})

	t.Run("validates path, merchant id and mode", func(t *testing.T) {
		s := newHandoffStore()
		for _, p := range []string{"", "/", "//evil.com", "https://evil.com", "/balance/../admin", "/settings"} {
			_, _, _, err := s.Create(7, "", p, handoffQR, now)
			assert.ErrorIs(t, err, errHandoffInvalid, p)
		}
		_, _, _, err := s.Create(7, "../../x", "/balance", handoffQR, now)
		assert.ErrorIs(t, err, errHandoffInvalid)
		_, _, _, err = s.Create(7, "", "/balance", "magic", now)
		assert.ErrorIs(t, err, errHandoffInvalid)
	})

	t.Run("concurrent claim succeeds exactly once", func(t *testing.T) {
		s := newHandoffStore()
		_, token, _, _ := s.Create(7, "", "/balance", handoffDirect, now)
		var wins int32
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := s.Claim(token, "x", now); err == nil {
					atomic.AddInt32(&wins, 1)
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, int32(1), wins)
	})

	t.Run("tokens are unique 43-char base64url", func(t *testing.T) {
		s := newHandoffStore()
		_, a, _, _ := s.Create(7, "", "/balance", handoffQR, now)
		_, b, _, _ := s.Create(7, "", "/balance", handoffQR, now)
		assert.NotEqual(t, a, b)
		assert.Len(t, a, 43)
	})
}
