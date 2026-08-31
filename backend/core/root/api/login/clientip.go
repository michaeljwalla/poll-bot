package login

import (
	"log"
	"net/http"
	"net/netip"
	"strings"
	"sync"
)

// Behind a Cloudflare tunnel the origin never sees a visitor directly:
// cloudflared runs on the host and dials the container, so RemoteAddr is the
// bridge gateway for every request in the world. Counting failures against
// that would let three bad passwords lock out every user at once, so the real
// address has to come from the headers the edge attaches.
//
// Those headers are only worth reading when the peer is entitled to set them.
// Cloudflare overwrites CF-Connecting-IP at the edge, so it cannot be forged
// through the tunnel; it can be forged by anything that reaches the origin
// directly, which is why the peer must be a trusted hop first.
const CF_CLIENT_IP_HEADER = "CF-Connecting-IP"

// Anything reaching this server from off-network is a visitor, not a proxy.
// The tunnel, a sidecar, or a reverse proxy on the same host all land inside
// these ranges; a real client never does. Tighten this if the port is ever
// published to a LAN where an untrusted machine could pose as the proxy.
var TRUSTED_PROXY_CIDRS = []string{
	"127.0.0.0/8",    // loopback
	"::1/128",        //
	"10.0.0.0/8",     // rfc1918
	"172.16.0.0/12",  // (docker bridge lives here)
	"192.168.0.0/16", //
	"fc00::/7",       // unique local
	"fe80::/10",      // link local
}

// A single client is normally handed a whole IPv6 /64, so counting failures
// against one full address would let it walk to a fresh one after every third
// guess. IPv4 has no such slack and is counted exactly.
const IPV6_BUCKET_BITS = 64

var trustedProxies = func() []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(TRUSTED_PROXY_CIDRS))
	for _, cidr := range TRUSTED_PROXY_CIDRS {
		prefixes = append(prefixes, netip.MustParsePrefix(cidr))
	}
	return prefixes
}()

var warnMissingClientIP sync.Once

func isTrustedProxy(addr netip.Addr) bool {
	for _, prefix := range trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// parseAddr accepts a bare address, with or without a port, and normalises the
// 4-in-6 form so ::ffff:1.2.3.4 and 1.2.3.4 cannot occupy separate buckets.
func parseAddr(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Addr{}, false
	}
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr.Unmap(), true
	}
	if hostPort, err := netip.ParseAddrPort(s); err == nil {
		return hostPort.Addr().Unmap(), true
	}
	return netip.Addr{}, false
}

// forwardedFor reads the last entry, not the first: everything to the left of
// it is whatever the client chose to send, while the rightmost hop is the one
// the trusted proxy appended itself.
func forwardedFor(r *http.Request) (netip.Addr, bool) {
	for _, header := range r.Header.Values("X-Forwarded-For") {
		hops := strings.Split(header, ",")
		for i := len(hops) - 1; i >= 0; i-- {
			if addr, ok := parseAddr(hops[i]); ok {
				return addr, true
			}
		}
	}
	return netip.Addr{}, false
}

// requestIP is the key a caller is rate limited under.
func requestIP(r *http.Request) string {
	peer, ok := parseAddr(r.RemoteAddr)
	if !ok {
		return r.RemoteAddr
	}
	if !isTrustedProxy(peer) {
		return bucket(peer)
	}

	if addr, ok := parseAddr(r.Header.Get(CF_CLIENT_IP_HEADER)); ok {
		return bucket(addr)
	}
	if addr, ok := forwardedFor(r); ok {
		return bucket(addr)
	}

	// The peer claims to be a proxy but named no one, so every caller behind
	// it shares a bucket and one of them can lock out the rest. That is a
	// deployment fault worth saying out loud, once -- unless the peer is
	// loopback, which is the developer browsing the server directly rather
	// than a proxy that forgot to say who it was speaking for.
	if !peer.IsLoopback() {
		warnMissingClientIP.Do(func() {
			log.Printf(
				"login: request from proxy %s carries no %s or X-Forwarded-For; rate limiting all callers behind it as one",
				peer, CF_CLIENT_IP_HEADER,
			)
		})
	}
	return bucket(peer)
}

func bucket(addr netip.Addr) string {
	if addr.Is6() {
		if prefix, err := addr.Prefix(IPV6_BUCKET_BITS); err == nil {
			return prefix.String()
		}
	}
	return addr.String()
}
