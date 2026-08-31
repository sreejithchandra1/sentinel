package api

import (
	"testing"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestPasswordResetURL(t *testing.T) {
	got := passwordResetURL("https://monitor.example.com/", "tok")
	want := "https://monitor.example.com/reset-password?token=tok"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDashboardBaseURLUsesServerSettings(t *testing.T) {
	srv, st, _ := newTestMFAServer(t)
	if got := srv.dashboardBaseURL(); got != "http://localhost:8082" {
		t.Fatalf("config fallback=%q", got)
	}
	if err := st.SaveServerSettings(models.ServerSettings{
		DashboardURL:  "https://status.example.com",
		RetentionDays: 30,
		Workers:       10,
	}); err != nil {
		t.Fatal(err)
	}
	if got := srv.dashboardBaseURL(); got != "https://status.example.com" {
		t.Fatalf("settings url=%q", got)
	}
}
