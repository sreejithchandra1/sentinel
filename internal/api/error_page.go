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

const errorPageCSP = "default-src 'none'; img-src *; style-src 'unsafe-inline' *; font-src *; base-uri 'self' https: http:"

func (s *Server) exposeErrorPageViewURL(item *models.IncidentListItem) {
	if item == nil || item.ErrorPage == nil {
		return
	}
	token := item.ErrorPage.ViewToken
	if token != "" {
		item.ErrorPage.ViewURL = s.incidentErrorPageURL(item.ID, token)
	}
	item.ErrorPage.ViewToken = ""
}

func (s *Server) incidentErrorPageURL(id, token string) string {
	base := strings.TrimRight(strings.TrimSpace(s.dashboardURL), "/")
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
		body = fallbackErrorPageHTML(page)
	}
	body = injectBaseHref(body, page.PageURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", errorPageCSP)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
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

func fallbackErrorPageHTML(page *models.HTTPErrorPage) string {
	if page == nil {
		return "<!DOCTYPE html><title>Error page</title><p>No response body was captured.</p>"
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>HTTP ")
	b.WriteString(html.EscapeString(fmt.Sprintf("%d", page.StatusCode)))
	b.WriteString("</title></head><body><h1>HTTP ")
	b.WriteString(html.EscapeString(fmt.Sprintf("%d", page.StatusCode)))
	b.WriteString("</h1>")
	if len(page.Headers) > 0 {
		b.WriteString("<pre>")
		for k, v := range page.Headers {
			b.WriteString(html.EscapeString(k))
			b.WriteString(": ")
			b.WriteString(html.EscapeString(v))
			b.WriteString("\n")
		}
		b.WriteString("</pre>")
	}
	if page.Excerpt != "" {
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(page.Excerpt))
		b.WriteString("</p>")
	}
	b.WriteString("</body></html>")
	return b.String()
}
