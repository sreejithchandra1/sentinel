package api

import (
	"net/http"
	"testing"
	"time"
)

func TestSessionCookieSetsHttpOnlyAndSecure(t *testing.T) {
	c := sessionCookie("sid", time.Now().UTC().Add(time.Hour))
	if !c.HttpOnly {
		t.Fatal("session cookie must set HttpOnly")
	}
	if !c.Secure {
		t.Fatal("session cookie must set Secure")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite=%v want Lax", c.SameSite)
	}
	if c.Path != "/" || c.Name != "sentinel_session" {
		t.Fatalf("cookie identity Path=%q Name=%q", c.Path, c.Name)
	}
}

func TestClearSessionCookieSetsHttpOnly(t *testing.T) {
	c := clearSessionCookie()
	if !c.HttpOnly {
		t.Fatal("cleared session cookie must set HttpOnly")
	}
	if !c.Secure {
		t.Fatal("cleared session cookie must set Secure")
	}
	if c.MaxAge != -1 {
		t.Fatalf("MaxAge=%d want -1", c.MaxAge)
	}
}
