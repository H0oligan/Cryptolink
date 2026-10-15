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

	t.Run("direct: claim never creates a session; complete does, once", func(t *testing.T) {
		s := newHandoffStore()
		_, token, _, err := s.Create(7, "owner@example.com", testMerchant, "/balance", handoffDirect, now)
		require.NoError(t, err)

		res, err := s.Claim(token, "MetaMask · iPhone", now)
		require.NoError(t, err)
		require.NotEmpty(t, res.claimSecret)
		assert.Equal(t, "owner@example.com", res.email)
		assert.Empty(t, res.matchCode, "direct mode has no PC approval")

		_, err = s.Claim(token, "x", now)
		assert.ErrorIs(t, err, errHandoffGone, "token is single-use")

		e, done, err := s.Complete(res.claimSecret, 0, now)
		require.NoError(t, err)
		assert.True(t, done)
		assert.Equal(t, int64(7), e.userID)

		_, _, err = s.Complete(res.claimSecret, 0, now)
		assert.ErrorIs(t, err, errHandoffGone, "complete is single-use")
	})

	t.Run("qr: approval requires the code shown on the phone", func(t *testing.T) {
		s := newHandoffStore()
		id, token, _, _ := s.Create(7, "a@b.c", "", "/wallet-setup", handoffQR, now)

		res, err := s.Claim(token, "Trust · Android", now)
		require.NoError(t, err)
		require.Len(t, res.matchCode, 6)

		st, dev := s.Status(id, 7, now)
		assert.Equal(t, "claimed", st)
		assert.Equal(t, "Trust · Android", dev)

		_, done, err := s.Complete(res.claimSecret, 0, now)
		require.NoError(t, err)
		assert.False(t, done, "waiting: not approved yet")

		assert.ErrorIs(t, s.Decide(id, 8, true, res.matchCode, now), errHandoffGone, "only the creator can approve")
		assert.ErrorIs(t, s.Decide(id, 7, true, "000000x", now), errHandoffBadCode)
		require.NoError(t, s.Decide(id, 7, true, res.matchCode, now))

		e, done, err := s.Complete(res.claimSecret, 0, now)
		require.NoError(t, err)
		assert.True(t, done)
		assert.Equal(t, "/wallet-setup", e.returnPath)
	})

	t.Run("qr: three wrong codes deny the claim", func(t *testing.T) {
		s := newHandoffStore()
		id, token, _, _ := s.Create(7, "a@b.c", "", "/balance", handoffQR, now)
		res, _ := s.Claim(token, "x", now)
		for i := 0; i < handoffMaxCodeTries; i++ {
			assert.ErrorIs(t, s.Decide(id, 7, true, "wrong", now), errHandoffBadCode)
		}
		assert.ErrorIs(t, s.Decide(id, 7, true, res.matchCode, now), errHandoffGone)
		_, _, err := s.Complete(res.claimSecret, 0, now)
		assert.ErrorIs(t, err, errHandoffGone)
	})

	t.Run("qr: deny kills the claim without a code", func(t *testing.T) {
		s := newHandoffStore()
		id, token, _, _ := s.Create(7, "a@b.c", "", "/balance", handoffQR, now)
		res, _ := s.Claim(token, "x", now)
		require.NoError(t, s.Decide(id, 7, false, "", now))
		_, _, err := s.Complete(res.claimSecret, 0, now)
		assert.ErrorIs(t, err, errHandoffGone)
		st, _ := s.Status(id, 7, now)
		assert.Equal(t, "denied", st)
	})

	t.Run("complete refuses to swap a browser signed in as another user", func(t *testing.T) {
		s := newHandoffStore()
		_, token, _, _ := s.Create(7, "a@b.c", "", "/balance", handoffDirect, now)
		res, _ := s.Claim(token, "x", now)

		_, _, err := s.Complete(res.claimSecret, 99, now)
		assert.ErrorIs(t, err, errHandoffOtherUser)

		e, done, err := s.Complete(res.claimSecret, 7, now)
		require.NoError(t, err, "same user may continue")
		assert.True(t, done)
		assert.Equal(t, int64(7), e.userID)
	})

	t.Run("expiry: unclaimed token and unapproved claim both expire", func(t *testing.T) {
		s := newHandoffStore()
		_, token, _, _ := s.Create(7, "a@b.c", "", "/balance", handoffQR, now)
		_, err := s.Claim(token, "x", now.Add(handoffTTL+time.Second))
		assert.ErrorIs(t, err, errHandoffGone)

		id, token2, _, _ := s.Create(7, "a@b.c", "", "/balance", handoffQR, now)
		res, _ := s.Claim(token2, "x", now)
		later := now.Add(handoffTTL + time.Second)
		assert.ErrorIs(t, s.Decide(id, 7, true, res.matchCode, later), errHandoffGone)
		_, _, err = s.Complete(res.claimSecret, 0, later)
		assert.ErrorIs(t, err, errHandoffGone)
	})

	t.Run("status hides other users' handoffs", func(t *testing.T) {
		s := newHandoffStore()
		id, _, _, _ := s.Create(7, "a@b.c", "", "/balance", handoffQR, now)
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
			_, _, _, err := s.Create(7, "a@b.c", "", p, handoffQR, now)
			assert.ErrorIs(t, err, errHandoffInvalid, p)
		}
		_, _, _, err := s.Create(7, "a@b.c", "../../x", "/balance", handoffQR, now)
		assert.ErrorIs(t, err, errHandoffInvalid)
		_, _, _, err = s.Create(7, "a@b.c", "", "/balance", "magic", now)
		assert.ErrorIs(t, err, errHandoffInvalid)
	})

	t.Run("concurrent claim succeeds exactly once", func(t *testing.T) {
		s := newHandoffStore()
		_, token, _, _ := s.Create(7, "a@b.c", "", "/balance", handoffDirect, now)
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
		_, a, _, _ := s.Create(7, "a@b.c", "", "/balance", handoffQR, now)
		_, b, _, _ := s.Create(7, "a@b.c", "", "/balance", handoffQR, now)
		assert.NotEqual(t, a, b)
		assert.Len(t, a, 43)
	})
}

func TestMaskEmail(t *testing.T) {
	assert.Equal(t, "ow***@example.com", maskEmail("owner@example.com"))
	assert.Equal(t, "a***@x.io", maskEmail("a@x.io"))
	assert.Equal(t, "***", maskEmail("broken"))
}
