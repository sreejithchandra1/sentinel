package safehost

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ErrBlockedTarget is returned when a host resolves to a non-public address.
var ErrBlockedTarget = fmt.Errorf("target host is not allowed (private, loopback, or link-local)")

// testAllowNonPublic lets unit tests dial loopback httptest servers.
var testAllowNonPublic atomic.Bool

// AllowNonPublicForTest temporarily permits private/loopback destinations.
func AllowNonPublicForTest(t testing.TB) {
	t.Helper()
	testAllowNonPublic.Store(true)
	t.Cleanup(func() { testAllowNonPublic.Store(false) })
}

// AllowNonPublicForPackageTests enables private dials for a package TestMain.
func AllowNonPublicForPackageTests() {
	testAllowNonPublic.Store(true)
}

// IsBlockedIP reports whether ip is unsuitable for outbound probes (SSRF risk).
func IsBlockedIP(ip net.IP) bool {
	if testAllowNonPublic.Load() {
		return false
	}
	if ip == nil {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsMulticast() || ip.IsUnspecified() || !ip.IsGlobalUnicast() {
		return true
	}
	// Carrier-grade NAT
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}

// ValidateHostname resolves host and rejects blocked addresses.
func ValidateHostname(host string) error {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if host == "" {
		return fmt.Errorf("hostname required")
	}
	if strings.Contains(host, "/") || strings.Contains(host, " ") {
		return fmt.Errorf("invalid hostname")
	}
	// Strip unexpected port if present (JoinHostPort hosts shouldn't include it twice)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return ErrBlockedTarget
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("host has no addresses")
	}
	for _, addr := range ips {
		if IsBlockedIP(addr.IP) {
			return ErrBlockedTarget
		}
	}
	return nil
}

// ValidateHTTPURL parses an HTTP(S) URL and validates its hostname.
func ValidateHTTPURL(raw string) error {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("url host required")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("url host required")
	}
	return ValidateHostname(host)
}

// ValidateMonitorTarget validates probe destination by monitor type.
func ValidateMonitorTarget(monitorType, rawURL string, port *int) error {
	switch monitorType {
	case "http", "":
		return ValidateHTTPURL(rawURL)
	case "port":
		host := strings.TrimSpace(rawURL)
		if host == "" || port == nil || *port <= 0 || *port > 65535 {
			return fmt.Errorf("host and valid port required")
		}
		return ValidateHostname(host)
	case "ssl":
		host := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://"))
		host = strings.Split(host, "/")[0]
		host = strings.Split(host, ":")[0]
		return ValidateHostname(host)
	case "dns", "heartbeat":
		return nil
	default:
		return nil
	}
}

// ControlDialContext dials only after verifying the destination IP is public.
func ControlDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if err := ValidateHostname(host); err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: 30 * time.Second}
	// Re-resolve and dial per-IP with a final IP check to reduce DNS rebinding risk.
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, addr := range ips {
		if IsBlockedIP(addr.IP) {
			lastErr = ErrBlockedTarget
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
		if err != nil {
			lastErr = err
			continue
		}
		return conn, nil
	}
	if lastErr == nil {
		lastErr = ErrBlockedTarget
	}
	return nil, lastErr
}
