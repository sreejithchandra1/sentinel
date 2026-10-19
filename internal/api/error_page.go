package api

import (
	"crypto/subtle"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

// Captured third-party HTML must never execute as text/html on the Sentinel
// origin (stored XSS). Serve as plain text only; the dashboard renders a
// sandboxed srcdoc preview from the authenticated incident JSON.
const errorPageCSP = "default-src 'none'; sandbox; base-uri 'none'; form-action 'none'"

func (s *Server) exposeErrorPageViewURL(item *models.IncidentListItem) {
	if item == nil || item.ErrorPage == nil {
		return
	}
	// Point operators at the incident detail page (sandboxed preview), not a
	// top-level HTML capture URL.
	if item.ID != "" {
		item.ErrorPage.ViewURL = s.dashboardBaseURL() + "/incidents/" + url.PathEscape(item.ID)
	}
	item.ErrorPage.ViewToken = ""
}

func (s *Server) incidentErrorPageURL(id, token string) string {
	// Legacy tokenized capture URL — still served as text/plain for tooling.
	base := s.dashboardBaseURL()
	return base + "/api/incidents/" + id + "/error-page?token=" + url.QueryEscape(token)
}

func (s *Server) handleIncidentErrorPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := r.URL.Query().Get("token")
	item, _, err := s.store.GetIncident(id)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if item == nil || item.ErrorPage == nil || item.ErrorPage.ViewToken == "" {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	stored := []byte(item.ErrorPage.ViewToken)
	got := []byte(token)
	if len(stored) != len(got) || subtle.ConstantTimeCompare(stored, got) != 1 {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}

	page := item.ErrorPage
	body := strings.TrimSpace(page.BodyHTML)
	if body == "" {
		body = fallbackErrorPagePlain(page)
	}

	// text/plain + noscript CSP: captured HTML must not execute on this origin.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Security-Policy", errorPageCSP)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func injectBaseHref(htmlBody, pageURL string) string {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return htmlBody
	}
	if parsed, err := url.Parse(pageURL); err == nil && parsed.Path != "" && !strings.HasSuffix(parsed.Path, "/") {
		if i := strings.LastIndex(parsed.Path, "/"); i >= 0 {
			parsed.Path = parsed.Path[:i+1]
			parsed.RawQuery = ""
			parsed.Fragment = ""
			pageURL = parsed.String()
		}
	}
	base := `<base href="` + html.EscapeString(pageURL) + `">`
	lower := strings.ToLower(htmlBody)
	if i := strings.Index(lower, "<head"); i >= 0 {
		if j := strings.Index(htmlBody[i:], ">"); j >= 0 {
			at := i + j + 1
			return htmlBody[:at] + base + htmlBody[at:]
		}
	}
	if i := strings.Index(lower, "<html"); i >= 0 {
		if j := strings.Index(htmlBody[i:], ">"); j >= 0 {
			at := i + j + 1
			return htmlBody[:at] + "<head>" + base + "</head>" + htmlBody[at:]
		}
	}
	return "<head>" + base + "</head>" + htmlBody
}

func fallbackErrorPagePlain(page *models.HTTPErrorPage) string {
	if page == nil {
		return "No response body was captured."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("HTTP %d\n", page.StatusCode))
	if len(page.Headers) > 0 {
		b.WriteString("\n")
		for k, v := range page.Headers {
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(v)
			b.WriteString("\n")
		}
	}
	if page.Excerpt != "" {
		b.WriteString("\n")
		b.WriteString(page.Excerpt)
	}
	return b.String()
}
