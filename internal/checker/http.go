package checker

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/safehost"
)

// defaultHTTPUserAgent identifies Sentinel probes in customer access logs.
const defaultHTTPUserAgent = "Sentinel/1.0 (compatible; Health-Check)"

type timingInfo struct {
	dnsStart, dnsDone         time.Time
	connectStart, connectDone time.Time
	tlsStart, tlsDone         time.Time
	gotFirstResponseByte      time.Time
}

func (t timingInfo) reachedHost() bool {
	return !t.connectDone.IsZero() || !t.tlsDone.IsZero() || !t.gotFirstResponseByte.IsZero()
}

func (c *Checker) probeHTTP(ctx context.Context, m *models.Monitor) *models.CheckResult {
	start := time.Now()
	result := &models.CheckResult{
		MonitorID: m.ID,
		Status:    models.StatusDown,
		CheckedAt: start,
	}

	var timings timingInfo
	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) { timings.dnsStart = time.Now() },
		DNSDone:  func(_ httptrace.DNSDoneInfo) { timings.dnsDone = time.Now() },
		ConnectStart: func(_, _ string) {
			if timings.connectStart.IsZero() {
				timings.connectStart = time.Now()
			}
		},
		ConnectDone: func(_, _ string, err error) {
			if err == nil && timings.connectDone.IsZero() {
				timings.connectDone = time.Now()
			}
		},
		TLSHandshakeStart: func() { timings.tlsStart = time.Now() },
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if err == nil {
				timings.tlsDone = time.Now()
			}
		},
		GotFirstResponseByte: func() {
			if timings.gotFirstResponseByte.IsZero() {
				timings.gotFirstResponseByte = time.Now()
			}
		},
	}

	timeout := time.Duration(m.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	// Keyword rules need the response body. Give slow pages extra budget so we
	// can report a real keyword match instead of a verification timeout.
	reqTimeout := timeout
	if hasKeywordRules(m) && reqTimeout < 30*time.Second {
		reqTimeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, reqTimeout)
	defer cancel()
	reqCtx = httptrace.WithClientTrace(reqCtx, trace)

	if err := safehost.ValidateHTTPURL(m.URL); err != nil {
		result.Error = err.Error()
		result.ResponseTimeMs = int(time.Since(start).Milliseconds())
		return result
	}

	method := m.Method
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if m.RequestBody != "" {
		body = strings.NewReader(m.RequestBody)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, m.URL, body)
	if err != nil {
		result.Error = fmt.Sprintf("invalid request: %v", err)
		result.ResponseTimeMs = int(time.Since(start).Milliseconds())
		return result
	}

	applyHTTPRequestHeaders(req, m.RequestHeaders)
	applyHTTPBasicAuth(req, m)

	// Rely on request context for deadline — Client.Timeout doubles the same budget
	// and produces opaque "context deadline exceeded" failures on slow-but-up sites.
	client := &http.Client{
		Transport: &http.Transport{
			DialContext:           safehost.ControlDialContext,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS13,
				InsecureSkipVerify: false,
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !m.FollowRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if err := safehost.ValidateHTTPURL(req.URL.String()); err != nil {
				return err
			}
			applyHTTPBasicAuth(req, m)
			return nil
		},
	}

	resp, err := client.Do(req)
	result.ResponseTimeMs = int(time.Since(start).Milliseconds())
	fillTimings(result, timings, start)

	keywordsRequired := hasKeywordRules(m)
	if err != nil {
		applyHTTPProbeError(result, err, timings.reachedHost(), m)
		return result
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	result.StatusCode = &code

	if !statusMatches(m, code) {
		if hasHTTPBasicAuth(m) && code == http.StatusUnauthorized {
			result.Error = "HTTP basic authentication failed (401 Unauthorized)"
			return result
		}
		result.Error = fmt.Sprintf("expected status %s, got %d", expectedStatusLabel(m), code)
		return result
	}

	bodyStr, forbiddenHit, err := readBodyForKeywords(resp.Body, m, 1<<20)
	if forbiddenHit {
		result.Error = fmt.Sprintf("reported down because keyword matched (must not exist): %q", m.KeywordMustNotExist)
		return result
	}
	if err != nil {
		if isTimeoutErr(err) {
			// Host answered, but we couldn't finish the body. Reachability alone is UP unless
			// keyword rules need a full body — those must not silently pass on timeout.
			if keywordsRequired {
				result.Error = keywordTimeoutError(m)
				return result
			}
			result.Status = models.StatusUp
			result.Error = ""
			return result
		}
		result.Error = fmt.Sprintf("read body: %v", err)
		return result
	}

	if failed := applyKeywordChecks(result, m, bodyStr); failed {
		return result
	}

	// Uptime monitors are up/down only. Latency warnings belong to performance targets.
	result.Status = models.StatusUp
	return result
}

// applyHTTPRequestHeaders sets the branded default User-Agent, then applies
// optional monitor custom headers (which may override User-Agent).
func applyHTTPRequestHeaders(req *http.Request, custom string) {
	req.Header.Set("User-Agent", defaultHTTPUserAgent)
	if custom == "" {
		return
	}
	for _, line := range strings.Split(custom, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

func hasHTTPBasicAuth(m *models.Monitor) bool {
	return m != nil && strings.TrimSpace(m.HTTPUsername) != ""
}

func applyHTTPBasicAuth(req *http.Request, m *models.Monitor) {
	if req == nil || !hasHTTPBasicAuth(m) {
		return
	}
	req.SetBasicAuth(m.HTTPUsername, m.HTTPPassword)
}

func hasKeywordRules(m *models.Monitor) bool {
	return m.KeywordMustExist != "" || m.KeywordMustNotExist != ""
}

func keywordTimeoutError(m *models.Monitor) string {
	switch {
	case m.KeywordMustNotExist != "" && m.KeywordMustExist != "":
		return fmt.Sprintf("timed out before keyword rules could be verified (must exist %q / must not exist %q)", m.KeywordMustExist, m.KeywordMustNotExist)
	case m.KeywordMustNotExist != "":
		return fmt.Sprintf("timed out before keyword rules could be verified (must not exist: %q)", m.KeywordMustNotExist)
	default:
		return fmt.Sprintf("timed out before keyword rules could be verified (must exist: %q)", m.KeywordMustExist)
	}
}

// readBodyForKeywords streams the body and fails early when a must-not-exist keyword appears.
func readBodyForKeywords(r io.Reader, m *models.Monitor, limit int64) (body string, forbiddenHit bool, err error) {
	var b strings.Builder
	buf := make([]byte, 32*1024)
	var total int64
	needle := m.KeywordMustNotExist
	overlap := 0
	if needle != "" {
		overlap = len(needle) - 1
		if overlap < 0 {
			overlap = 0
		}
	}
	var window string

	for {
		n, readErr := r.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if total+int64(n) > limit {
				chunk = buf[:limit-total]
				n = len(chunk)
			}
			total += int64(n)
			b.Write(chunk)
			if needle != "" {
				search := window + string(chunk)
				if strings.Contains(search, needle) {
					return b.String(), true, nil
				}
				if overlap > 0 {
					if len(search) > overlap {
						window = search[len(search)-overlap:]
					} else {
						window = search
					}
				}
			}
			if total >= limit {
				return b.String(), false, nil
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return b.String(), false, nil
			}
			return b.String(), false, readErr
		}
	}
}

// applyKeywordChecks returns true when the probe should stay DOWN.
func applyKeywordChecks(result *models.CheckResult, m *models.Monitor, bodyStr string) bool {
	if m.KeywordMustExist != "" && !strings.Contains(bodyStr, m.KeywordMustExist) {
		result.Error = fmt.Sprintf("reported down because required keyword was missing: %q", m.KeywordMustExist)
		return true
	}
	if m.KeywordMustNotExist != "" && strings.Contains(bodyStr, m.KeywordMustNotExist) {
		result.Error = fmt.Sprintf("reported down because keyword matched (must not exist): %q", m.KeywordMustNotExist)
		return true
	}
	return false
}

// applyHTTPProbeError maps transport errors to uptime status.
// Timeouts after the host was reached mean the site is slow, not down — unless
// keyword rules were configured and never validated.
func applyHTTPProbeError(result *models.CheckResult, err error, reachedHost bool, m *models.Monitor) {
	if isTimeoutErr(err) && reachedHost {
		if m != nil && hasKeywordRules(m) {
			result.Status = models.StatusDown
			result.Error = keywordTimeoutError(m)
			return
		}
		result.Status = models.StatusUp
		result.Error = ""
		return
	}
	result.Status = models.StatusDown
	result.Error = formatHTTPProbeError(err)
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "client.timeout") ||
		strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "i/o timeout")
}

func formatHTTPProbeError(err error) string {
	msg := err.Error()
	// Strip Go's verbose "Get \"url\": " prefix for clearer alerts.
	if i := strings.Index(msg, ": "); i >= 0 && (strings.HasPrefix(msg, "Get ") || strings.HasPrefix(msg, "Post ") || strings.HasPrefix(msg, "Head ")) {
		msg = strings.TrimSpace(msg[i+2:])
	}
	if isTimeoutErr(err) {
		return "connection timed out (host unreachable)"
	}
	return msg
}

func fillTimings(result *models.CheckResult, t timingInfo, start time.Time) {
	if !t.dnsStart.IsZero() && !t.dnsDone.IsZero() {
		v := int(t.dnsDone.Sub(t.dnsStart).Milliseconds())
		result.DNSMs = &v
	}
	if !t.connectStart.IsZero() && !t.connectDone.IsZero() {
		v := int(t.connectDone.Sub(t.connectStart).Milliseconds())
		result.TCPMs = &v
	}
	if !t.tlsStart.IsZero() && !t.tlsDone.IsZero() {
		v := int(t.tlsDone.Sub(t.tlsStart).Milliseconds())
		result.TLSMs = &v
	}
	if !t.gotFirstResponseByte.IsZero() {
		v := int(t.gotFirstResponseByte.Sub(start).Milliseconds())
		result.TTFBMs = &v
	}
}

func statusMatches(m *models.Monitor, code int) bool {
	if m.ExpectedStatusMin != nil && m.ExpectedStatusMax != nil {
		return code >= *m.ExpectedStatusMin && code <= *m.ExpectedStatusMax
	}
	if m.ExpectedStatus == 0 {
		return code >= 200 && code < 300
	}
	if m.ExpectedStatus == 2 {
		return code >= 200 && code < 300
	}
	return code == m.ExpectedStatus
}

func expectedStatusLabel(m *models.Monitor) string {
	if m.ExpectedStatusMin != nil && m.ExpectedStatusMax != nil {
		return fmt.Sprintf("%d-%d", *m.ExpectedStatusMin, *m.ExpectedStatusMax)
	}
	if m.ExpectedStatus == 2 {
		return "2xx"
	}
	return fmt.Sprintf("%d", m.ExpectedStatus)
}
