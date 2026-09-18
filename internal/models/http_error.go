package models

import "strings"

const (
	HTTPErrorSourceShopware   = "shopware_maintenance"
	HTTPErrorSourceNginx      = "nginx"
	HTTPErrorSourcePHPFPM     = "php_fpm"
	HTTPErrorSourceCloudflare = "cloudflare"
	HTTPErrorSourceUnknown    = "unknown"
)

// HTTPErrorPage is a captured HTTP error response used to tell Shopware
// maintenance pages apart from nginx / PHP-FPM / Cloudflare errors.
type HTTPErrorPage struct {
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers,omitempty"`
	Source      string            `json:"source,omitempty"`
	SourceLabel string            `json:"source_label,omitempty"`
	Excerpt     string            `json:"excerpt,omitempty"`
	BodyHTML    string            `json:"body_html,omitempty"`
	PageURL     string            `json:"page_url,omitempty"`
	ViewToken   string            `json:"view_token,omitempty"`
	ViewURL     string            `json:"view_url,omitempty"`
}

func HTTPErrorSourceLabel(source string) string {
	switch source {
	case HTTPErrorSourceShopware:
		return "Shopware maintenance"
	case HTTPErrorSourceNginx:
		return "nginx error page"
	case HTTPErrorSourcePHPFPM:
		return "PHP-FPM / upstream"
	case HTTPErrorSourceCloudflare:
		return "Cloudflare"
	default:
		return "Unknown error page"
	}
}

// StripHTTPErrorSourceSuffix removes guessed labels previously appended to
// probe errors, e.g. "expected status 200, got 404 (nginx error page)".
func StripHTTPErrorSourceSuffix(msg string) string {
	for _, source := range []string{
		HTTPErrorSourceShopware,
		HTTPErrorSourceNginx,
		HTTPErrorSourcePHPFPM,
		HTTPErrorSourceCloudflare,
		HTTPErrorSourceUnknown,
	} {
		suffix := " (" + HTTPErrorSourceLabel(source) + ")"
		msg = strings.TrimSuffix(msg, suffix)
	}
	return msg
}

func (p *HTTPErrorPage) Clone() *HTTPErrorPage {
	if p == nil {
		return nil
	}
	cp := *p
	if p.Headers != nil {
		cp.Headers = make(map[string]string, len(p.Headers))
		for k, v := range p.Headers {
			cp.Headers[k] = v
		}
	}
	return &cp
}

// ForCheckResult returns a compact copy without the HTML body or view token
// so check_results.details stays small during a long outage.
func (p *HTTPErrorPage) ForCheckResult() *HTTPErrorPage {
	if p == nil {
		return nil
	}
	return &HTTPErrorPage{
		StatusCode:  p.StatusCode,
		Headers:     p.Headers,
		Source:      p.Source,
		SourceLabel: p.SourceLabel,
		Excerpt:     p.Excerpt,
		PageURL:     p.PageURL,
	}
}

func HTMLExcerpt(html string, n int) string {
	if n <= 0 {
		n = 240
	}
	var b strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	s := strings.Join(strings.Fields(b.String()), " ")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return strings.TrimSpace(string(runes[:n])) + "…"
}
