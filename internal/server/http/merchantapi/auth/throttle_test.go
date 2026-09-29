package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoginThrottle(t *testing.T) {
	th := newLoginThrottle()
	now := time.Now()

	for i := 0; i < loginMaxFailures-1; i++ {
		th.Fail("Admin@Example.com", now)
	}
	assert.False(t, th.Blocked("admin@example.com", now))

	th.Fail("admin@example.com ", now)
	assert.True(t, th.Blocked("ADMIN@example.com", now), "case/space-insensitive key")
	assert.False(t, th.Blocked("other@example.com", now))

	assert.False(t, th.Blocked("admin@example.com", now.Add(loginLockWindow+time.Second)), "window expires")

	th.Fail("x@example.com", now)
	th.Reset("x@example.com")
	assert.False(t, th.Blocked("x@example.com", now))
}

func TestRegistrationEmailBlocked(t *testing.T) {
	for _, e := range []string{
		"admin+ctf@cryptolink.cc", "root@CryptoLink.cc", "clctf1@mailinator.com",
		"a@sub.mailinator.com", "x@yopmail.com",
	} {
		assert.NotEmpty(t, registrationEmailBlocked(e), e)
	}

	for _, e := range []string{"john@gmail.com", "ops@smsmobile.io", "me@notcryptolink.cc"} {
		assert.Empty(t, registrationEmailBlocked(e), e)
	}
}
