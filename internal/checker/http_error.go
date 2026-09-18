package checker

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

const (
	errorPageBodyLimit    = 64 << 10
	errorPageExcerptLimit = 240
)

var errorPageHeaderKeys = []string{
	"Server",
	"X-Powered-By",
	"Via",
	"CF-Ray",
	"Content-Type",
	"Retry-After",
}

func attachHTTPErrorPage(result *models.CheckResult, pageURL string, hdr http.Header, body string) {
	if result == nil || result.StatusCode == nil {
		return
	}
	page := buildHTTPErrorPage(pageURL, *result.StatusCode, hdr, body)
	result.ErrorPage = page
	if compact, err := json.Marshal(page.ForCheckResult()); err == nil {
		result.Details = string(compact)
	}
}

func buildHTTPErrorPage(pageURL string, status int, hdr http.Header, body string) *models.HTTPErrorPage {
	if len(body) > errorPageBodyLimit {
		body = body[:errorPageBodyLimit]
	}
	source := classifyHTTPError(status, hdr, body)
	return &models.HTTPErrorPage{
		StatusCode:  status,
		Headers:     captureErrorHeaders(hdr),
		Source:      source,
		SourceLabel: models.HTTPErrorSourceLabel(source),
		Excerpt:     models.HTMLExcerpt(body, errorPageExcerptLimit),
		BodyHTML:    body,
		PageURL:     pageURL,
	}
}

func captureErrorHeaders(h http.Header) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string)
	for _, k := range errorPageHeaderKeys {
		if v := strings.TrimSpace(h.Get(k)); v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// classifyHTTPError prefers body signatures over Server because Shopware
// (and PHP-FPM) usually sit behind nginx.
func classifyHTTPError(status int, hdr http.Header, body string) string {
	lower := strings.ToLower(body)
	server := ""
	powered := ""
	cfRay := ""
	if hdr != nil {
		server = strings.ToLower(hdr.Get("Server"))
		powered = strings.ToLower(hdr.Get("X-Powered-By"))
		cfRay = hdr.Get("CF-Ray")
	}

	if isShopwareError(lower) {
		return models.HTTPErrorSourceShopware
	}
	if cfRay != "" || strings.Contains(server, "cloudflare") || isCloudflareBody(lower) {
		return models.HTTPErrorSourceCloudflare
	}
	if isPHPFPMError(status, lower, powered) {
		return models.HTTPErrorSourcePHPFPM
	}
	if isNginxDefaultError(lower) || strings.Contains(server, "nginx") {
		return models.HTTPErrorSourceNginx
	}
	return models.HTTPErrorSourceUnknown
}

func isShopwareError(lowerBody string) bool {
	if strings.Contains(lowerBody, "shopware") ||
		strings.Contains(lowerBody, "sw-maintenance") ||
		strings.Contains(lowerBody, "is--maintenance") ||
		strings.Contains(lowerBody, "data-shopware") {
		return true
	}
	if strings.Contains(lowerBody, "sw-") && strings.Contains(lowerBody, "maintenance") {
		return true
	}
	if isNginxDefaultError(lowerBody) {
		return false
	}
	return strings.Contains(lowerBody, "maintenance mode") ||
		strings.Contains(lowerBody, "undergoing maintenance") ||
		strings.Contains(lowerBody, "<title>maintenance</title>")
}

func isCloudflareBody(lowerBody string) bool {
	return strings.Contains(lowerBody, "cf-error") ||
		strings.Contains(lowerBody, "attention required") && strings.Contains(lowerBody, "cloudflare") ||
		strings.Contains(lowerBody, "id=\"cf-wrapper\"")
}

func isPHPFPMError(status int, lowerBody, powered string) bool {
	if status == http.StatusBadGateway || status == http.StatusGatewayTimeout {
		return true
	}
	if strings.Contains(lowerBody, "primary script unknown") {
		return true
	}
	if strings.Contains(lowerBody, "upstream") &&
		(strings.Contains(lowerBody, "connect") ||
			strings.Contains(lowerBody, "timed out") ||
			strings.Contains(lowerBody, "prematurely")) {
		return true
	}
	return strings.Contains(powered, "php") && status >= 500
}

func isNginxDefaultError(lowerBody string) bool {
	return strings.Contains(lowerBody, "<center>nginx</center>") ||
		strings.Contains(lowerBody, "503 service temporarily unavailable") ||
		strings.Contains(lowerBody, "502 bad gateway") ||
		strings.Contains(lowerBody, "504 gateway time-out") ||
		strings.Contains(lowerBody, "504 gateway timeout")
}
