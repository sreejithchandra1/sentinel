package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/store"
)

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

const (
	forgotPasswordLimit  = 5
	forgotPasswordWindow = time.Hour
)

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		jsonError(w, http.StatusBadRequest, "email required")
		return
	}

	ip := clientIP(r)
	ipKey := "forgot-ip:" + ip
	emailKey := "forgot-email:" + email

	const sentMsg = "Password reset link sent to your email."

	actor := auditActor(email)
	if !s.limits.Allow(ipKey, forgotPasswordLimit, forgotPasswordWindow) {
		s.recordSecurityEvent("rate limit exceeded", actor, "rate_limit", "auth",
			"forgot-password ip="+ip+" email="+email,
			"endpoint", "forgot-password", "ip", ip, "email", email)
		jsonError(w, http.StatusTooManyRequests, "too many requests, try again later")
		return
	}
	if !s.limits.Allow(emailKey, forgotPasswordLimit, forgotPasswordWindow) {
		// Same success body to avoid email enumeration; still log + audit.
		s.recordSecurityEvent("rate limit exceeded", actor, "rate_limit", "auth",
			"forgot-password ip="+ip+" email="+email,
			"endpoint", "forgot-password", "ip", ip, "email", email)
		jsonOK(w, map[string]string{"message": sentMsg})
		return
	}

	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		jsonInternal(w, err)
		return
	}

	if user == nil {
		jsonOK(w, map[string]string{"message": sentMsg})
		return
	}

	token, err := randomToken(32)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "token error")
		return
	}
	expires := time.Now().UTC().Add(time.Hour)
	if err := s.store.CreatePasswordResetToken(user.ID, token, expires); err != nil {
		jsonInternal(w, err)
		return
	}

	resetURL := passwordResetURL(s.dashboardBaseURL(), token)
	to, username := user.Email, user.Username
	go func() {
		if err := s.alerter.SendPasswordResetEmail(to, username, resetURL); err != nil {
			log.Printf("password reset email: %v", err)
		}
	}()

	jsonOK(w, map[string]string{"message": sentMsg})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		jsonError(w, http.StatusBadRequest, "token and new password required")
		return
	}
	if len(req.NewPassword) < 8 {
		jsonError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if err := store.ValidatePassword(req.NewPassword); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID, err := s.store.GetPasswordResetUserID(req.Token)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if userID == "" {
		jsonError(w, http.StatusBadRequest, "invalid or expired reset link")
		return
	}

	user, err := s.store.GetUserByID(userID)
	if err != nil || user == nil {
		jsonError(w, http.StatusBadRequest, "invalid or expired reset link")
		return
	}

	updated, err := s.store.UpdateUser(userID, user.Username, user.Email, user.Role, req.NewPassword, user.TenantID, false)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.DeletePasswordResetToken(req.Token)
	to, username := updated.Email, updated.Username
	go func() {
		if err := s.alerter.SendPasswordChangedEmail(to, username); err != nil {
			log.Printf("password changed email: %v", err)
		}
	}()

	jsonOK(w, map[string]bool{"ok": true})
}

func passwordResetURL(dashboardURL, token string) string {
	return strings.TrimRight(strings.TrimSpace(dashboardURL), "/") + "/reset-password?token=" + token
}
