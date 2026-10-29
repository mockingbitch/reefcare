package httpserver

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const (
	clientIPKey ctxKey = requestIDKey + 1 + iota
	trustedPeerKey
)

// ClientIP resolves the client address. X-Forwarded-For is honored only when the
// direct peer is a trusted proxy; the right-most untrusted hop is used so clients
// cannot spoof their address by sending the header themselves.
func ClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := remoteAddr(r)
			peerTrusted := ip.IsValid() && isTrusted(ip)
			if peerTrusted {
				hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
				for i := len(hops) - 1; i >= 0; i-- {
					a, ok := parseHop(hops[i])
					if !ok {
						break
					}
					ip = a
					if !isTrusted(ip) {
						break
					}
				}
			}
			ctx := context.WithValue(r.Context(), clientIPKey, ip)
			ctx = context.WithValue(ctx, trustedPeerKey, peerTrusted)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// parseHop accepts "ip", "ip:port", "[v6]" and "[v6]:port" forms some proxies emit.
func parseHop(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap(), true
	}
	s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, false
	}
	return a.Unmap(), true
}

func remoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

// ClientIPFrom returns the resolved client address (invalid if unknown).
func ClientIPFrom(ctx context.Context) netip.Addr {
	a, _ := ctx.Value(clientIPKey).(netip.Addr)
	return a
}

func trustedPeerFrom(ctx context.Context) bool {
	ok, _ := ctx.Value(trustedPeerKey).(bool)
	return ok
}

// RateLimitKey returns the key to use for per-client rate limits: the full address for
// IPv4 and the /64 for IPv6, since one IPv6 client typically controls a whole /64 and
// could otherwise rotate addresses to evade limits.
func RateLimitKey(a netip.Addr) string {
	if !a.IsValid() {
		return "unknown"
	}
	if a.Is4() {
		return a.String()
	}
	p, _ := a.Prefix(64)
	return p.String()
}

// RateLimitNetKey returns a coarser key for a second, more generous limit tier: the /48
// for IPv6, which a single end site commonly receives (so 65,536 /64 keys can't each get
// a fresh budget). IPv4 has no coarser tier ("") because carrier-grade NAT puts many
// unrelated users behind one /24.
func RateLimitNetKey(a netip.Addr) string {
	if !a.IsValid() || a.Is4() {
		return ""
	}
	p, _ := a.Prefix(48)
	return p.String()
}

// TruncatedIP returns the client network for privacy-preserving storage:
// /24 for IPv4 and /48 for IPv6 (docs/security.md §8). Nil when unknown.
func TruncatedIP(a netip.Addr) *netip.Prefix {
	if !a.IsValid() {
		return nil
	}
	bits := 48
	if a.Is4() {
		bits = 24
	}
	p, err := a.Prefix(bits)
	if err != nil {
		return nil
	}
	return &p
}
