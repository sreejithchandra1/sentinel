package checker

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/safehost"
)

func (c *Checker) probeSSL(ctx context.Context, m *models.Monitor) *models.CheckResult {
	start := time.Now()
	result := &models.CheckResult{
		MonitorID: m.ID,
		Status:    models.StatusDown,
		CheckedAt: start,
	}

	host := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(m.URL, "https://"), "http://"))
	host = strings.Split(host, "/")[0]
	host = strings.Split(host, ":")[0]
	if host == "" {
		result.Error = "hostname is required for SSL monitors"
		return result
	}
	if err := safehost.ValidateHostname(host); err != nil {
		result.Error = err.Error()
		return result
	}

	port := 443
	if m.Port != nil {
		port = *m.Port
	}

	timeout := time.Duration(m.TimeoutMs) * time.Millisecond
	raw, err := safehost.ControlDialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		result.ResponseTimeMs = int(time.Since(start).Milliseconds())
		result.Error = fmt.Sprintf("TLS connection failed: %v", err)
		return result
	}
	_ = raw.SetDeadline(time.Now().Add(timeout))
	conn := tls.Client(raw, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS13,
	})
	if err := conn.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		result.ResponseTimeMs = int(time.Since(start).Milliseconds())
		result.Error = fmt.Sprintf("TLS connection failed: %v", err)
		return result
	}
	defer conn.Close()
	result.ResponseTimeMs = int(time.Since(start).Milliseconds())

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		result.Error = "no peer certificates"
		return result
	}

	cert := state.PeerCertificates[0]
	now := time.Now()
	daysRemaining := int(cert.NotAfter.Sub(now).Hours() / 24)
	fp := sha256.Sum256(cert.Raw)
	fingerprint := hex.EncodeToString(fp[:])
	sans := append([]string(nil), cert.DNSNames...)

	var issues []string
	if now.After(cert.NotAfter) {
		issues = append(issues, "expired")
	}
	if cert.Issuer.String() == cert.Subject.String() {
		issues = append(issues, "self_signed")
	}
	// VerifyHostname checks CN and SANs (e.g. CN=nxcli.io, SAN includes teqtivity.com).
	if err := cert.VerifyHostname(host); err != nil {
		issues = append(issues, "wrong_hostname")
	}
	if state.CipherSuite != 0 {
		suite := tls.CipherSuiteName(state.CipherSuite)
		if strings.Contains(suite, "RC4") || strings.Contains(suite, "DES") || strings.HasPrefix(suite, "TLS_RSA_") {
			issues = append(issues, "weak_cipher")
		}
	}
	if !sslChainOK(cert, state, host) && !containsIssue(issues, "self_signed") {
		issues = append(issues, "chain_error")
	}

	details := models.SSLDetails{
		ExpiresAt:     cert.NotAfter.UTC().Format(time.RFC3339),
		DaysRemaining: daysRemaining,
		Issuer:        cert.Issuer.CommonName,
		Subject:       cert.Subject.CommonName,
		SANs:          sans,
		Fingerprint:   fingerprint,
		Issues:        issues,
	}
	if b, e := json.Marshal(details); e == nil {
		result.Details = string(b)
	}

	if containsIssue(issues, "expired") {
		result.Status = models.StatusDown
		result.Error = fmt.Sprintf("certificate expired on %s", cert.NotAfter.Format("2006-01-02"))
		return result
	}

	result.Status = sslExpiryStatus(daysRemaining, issues)
	switch result.Status {
	case models.StatusDown:
		result.Error = fmt.Sprintf("certificate expires in %d days", daysRemaining)
	case models.StatusDegraded:
		if len(issues) > 0 {
			result.Error = strings.Join(issues, ", ")
		} else {
			result.Error = fmt.Sprintf("certificate expires in %d days", daysRemaining)
		}
	}

	tlsMs := result.ResponseTimeMs
	result.TLSMs = &tlsMs
	return result
}

// sslExpiryStatus applies:
//
//	>30 days  → healthy (up)
//	8–30 days → warning (degraded)
//	≤7 days   → critical (down)
//
// Non-expiry certificate issues are always warning/degraded.
func sslExpiryStatus(daysRemaining int, issues []string) models.MonitorStatus {
	if len(issues) > 0 {
		return models.StatusDegraded
	}
	switch {
	case daysRemaining <= 7:
		return models.StatusDown
	case daysRemaining <= 30:
		return models.StatusDegraded
	default:
		return models.StatusUp
	}
}

// sslChainOK validates the leaf using intermediates from the handshake.
// A successful TLS handshake already populates VerifiedChains when the OS trust
// store accepted the chain; that counts as OK even if a redundant Verify fails
// on incomplete options (common false positive on shared-hosting certs).
func sslChainOK(leaf *x509.Certificate, state tls.ConnectionState, host string) bool {
	if len(state.VerifiedChains) > 0 {
		return true
	}
	opts := x509.VerifyOptions{
		DNSName:       host,
		Intermediates: x509.NewCertPool(),
		CurrentTime:   time.Now(),
	}
	for _, ic := range state.PeerCertificates[1:] {
		opts.Intermediates.AddCert(ic)
	}
	if roots, err := x509.SystemCertPool(); err == nil && roots != nil {
		opts.Roots = roots
	}
	_, err := leaf.Verify(opts)
	return err == nil
}

func containsIssue(issues []string, target string) bool {
	for _, i := range issues {
		if i == target {
			return true
		}
	}
	return false
}
