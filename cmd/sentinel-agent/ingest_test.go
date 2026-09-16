package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestIngestSetsSentinelAgentUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"config":{"interval_seconds":30}}`)
	}))
	defer srv.Close()

	_, err := ingest(srv.Client(), &agentConfig{ServerURL: srv.URL, Token: "t"}, &models.HostIngestPayload{})
	if err != nil {
		t.Fatal(err)
	}
	want := "Sentinel-Agent/" + models.HostAgentVersion
	if gotUA != want {
		t.Fatalf("User-Agent=%q want %q", gotUA, want)
	}
}
