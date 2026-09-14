package api

import (
	"log"
	"net/http"
	"time"
)

func sessionCookie(value string, expires time.Time) *http.Cookie {
	c := &http.Cookie{
		Name:     "sentinel_session",
		Value:    value,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	}
	c.HttpOnly = true
	return c
}

func clearSessionCookie() *http.Cookie {
	c := sessionCookie("", time.Unix(0, 0).UTC())
	c.MaxAge = -1
	return c
}

func maskPassword(p string) string {
	if p == "" {
		return ""
	}
	return "********"
}

func jsonInternal(w http.ResponseWriter, err error) {
	if err != nil {
		log.Printf("api: %v", err)
	}
	jsonError(w, http.StatusInternalServerError, "internal error")
}
