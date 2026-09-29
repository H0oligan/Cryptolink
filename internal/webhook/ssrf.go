package webhook

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/pkg/errors"
)

// SSRF protection for merchant-controlled webhook URLs.
//
// Merchants choose where our server sends HTTP requests, so without these
// checks a merchant can make the server call internal services (HestiaCP on
// :8083, Postgres, Redis, cloud metadata on 169.254.169.254, ...). This was
// actively exploited on 2026-09-29.
//
// The authoritative check runs in the dialer's Control hook, i.e. on the IP
// actually being connected to. That covers DNS names resolving to internal
// IPs, DNS rebinding, and redirects to internal addresses. ValidateDestination
// is an early check used when a merchant saves a webhook URL.

var ErrForbiddenDestination = errors.New("webhook destination is not allowed")

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",       // "this" network
	"10.0.0.0/8",      // private
	"100.64.0.0/10",   // carrier-grade NAT
	"127.0.0.0/8",     // loopback
	"169.254.0.0/16",  // link-local, cloud metadata
	"172.16.0.0/12",   // private
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // TEST-NET-1
	"192.168.0.0/16",  // private
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"224.0.0.0/4",     // multicast
	"240.0.0.0/4",     // reserved, broadcast
	"::/128",          // unspecified
	"::1/128",         // loopback
	"64:ff9b::/96",    // NAT64 (can embed internal IPv4)
	"64:ff9b:1::/48",  // local-use NAT64
	"2001:db8::/32",   // documentation
	"fc00::/7",        // unique local
	"fe80::/10",       // link-local
	"ff00::/8",        // multicast
)

// allowPrivate is only ever flipped by tests that post to httptest servers.
var allowPrivate atomic.Bool

// AllowPrivateDestinationsForTesting disables the SSRF guard so tests can
// target httptest servers on 127.0.0.1. Never call it from production code.
func AllowPrivateDestinationsForTesting() (restore func()) {
	prev := allowPrivate.Swap(true)
	return func() { allowPrivate.Store(prev) }
}

func mustPrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		out = append(out, netip.MustParsePrefix(c))
	}

	return out
}

// isForbiddenIP reports whether connecting to ip must be refused.
func isForbiddenIP(ip netip.Addr) bool {
	if allowPrivate.Load() {
		return false
	}

	ip = ip.Unmap() // ::ffff:127.0.0.1 -> 127.0.0.1

	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}

	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}

	// Our own public IPs: reaching e.g. https://<server-ip>:8083 is as bad as
	// reaching 127.0.0.1:8083.
	return isLocalInterfaceIP(ip)
}

func isLocalInterfaceIP(ip netip.Addr) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		// Fail closed: if we cannot tell, refuse.
		return true
	}

	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}

		local, ok := netip.AddrFromSlice(ipNet.IP)
		if ok && local.Unmap() == ip {
			return true
		}
	}

	return false
}

// dialControl runs after DNS resolution, right before connect(2).
func dialControl(_ string, address string, _ syscall.RawConn) error {
	addrPort, err := netip.ParseAddrPort(address)
	if err != nil {
		return errors.Wrap(ErrForbiddenDestination, "unparseable address")
	}

	if isForbiddenIP(addrPort.Addr()) {
		return errors.Wrapf(ErrForbiddenDestination, "address %s is internal", addrPort.Addr())
	}

	return nil
}

func validateScheme(u *url.URL) error {
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return errors.Wrapf(ErrForbiddenDestination, "scheme %q is not allowed", u.Scheme)
	}

	if u.Hostname() == "" {
		return errors.Wrap(ErrForbiddenDestination, "missing hostname")
	}

	if u.User != nil {
		return errors.Wrap(ErrForbiddenDestination, "credentials in URL are not allowed")
	}

	return nil
}

// ValidateDestination checks a merchant-supplied webhook URL before it is
// saved. The dialer re-checks on every request, so this is not the only guard.
func ValidateDestination(ctx context.Context, raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return errors.Wrap(ErrForbiddenDestination, "invalid url")
	}

	if err := validateScheme(u); err != nil {
		return err
	}

	host := u.Hostname()
	if ip, err := netip.ParseAddr(host); err == nil {
		if isForbiddenIP(ip) {
			return errors.Wrap(ErrForbiddenDestination, "internal address")
		}

		return nil
	}

	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return errors.Wrap(ErrForbiddenDestination, "internal address")
	}

	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return errors.Wrap(ErrForbiddenDestination, "hostname does not resolve")
	}

	for _, ip := range ips {
		if isForbiddenIP(ip) {
			return errors.Wrap(ErrForbiddenDestination, "hostname resolves to an internal address")
		}
	}

	return nil
}
