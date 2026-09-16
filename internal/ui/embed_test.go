package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSPA_servesIndexForClientRoutes(t *testing.T) {
	h := Handler()
	for _, path := range []string{"/reset-password", "/login", "/monitors/abc", "/performance/xyz", "/hosts", "/hosts/abc"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path+"?token=abc", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			body, _ := io.ReadAll(rec.Body)
			if !strings.Contains(string(body), `<div id="root">`) {
				preview := string(body)
				if len(preview) > 200 {
					preview = preview[:200]
				}
				t.Fatalf("expected SPA index.html, got: %s", preview)
			}
		})
	}
}

func TestSPA_servesStaticAssets(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/assets/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	// asset paths with extensions should not return index.html
	if rec.Code == http.StatusOK {
		body, _ := io.ReadAll(rec.Body)
		if strings.Contains(string(body), `<div id="root">`) && !strings.Contains(req.URL.Path, ".") {
			t.Fatal("directory listing should not return SPA shell")
		}
	}
}
