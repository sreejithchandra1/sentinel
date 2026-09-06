package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

type ctxKey int

const ctxUserKey ctxKey = 1

func currentUser(r *http.Request) *models.User {
	u, _ := r.Context().Value(ctxUserKey).(*models.User)
	return u
}

func isPlatformAdmin(u *models.User) bool {
	return u != nil && u.Role == models.RoleAdmin && strings.TrimSpace(u.TenantID) == ""
}

func isCustomerAdmin(u *models.User) bool {
	return u != nil && u.Role == models.RoleAdmin && strings.TrimSpace(u.TenantID) != ""
}

func canWriteResources(u *models.User) bool {
	return u != nil && u.Role == models.RoleAdmin
}

func canAccessTenant(u *models.User, tenantID string) bool {
	if u == nil {
		return false
	}
	if isPlatformAdmin(u) {
		return true
	}
	tid := strings.TrimSpace(u.TenantID)
	return tid != "" && tid == strings.TrimSpace(tenantID)
}

func (s *Server) sessionUser(r *http.Request) (*models.User, error) {
	cookie, err := r.Cookie("sentinel_session")
	if err != nil {
		return nil, err
	}
	return s.store.GetSessionUser(cookie.Value)
}

func (s *Server) resolveUser(r *http.Request) (*models.User, error) {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if token != "" {
			return s.store.GetUserByAPIToken(token)
		}
	}
	return s.sessionUser(r)
}

func (s *Server) authRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.resolveUser(r)
		if err != nil || user == nil {
			jsonError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxUserKey, user)))
	}
}

// adminRequired allows any admin (platform or customer) — used for resource writes.
func (s *Server) adminRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.resolveUser(r)
		if err != nil || user == nil {
			jsonError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !canWriteResources(user) {
			jsonError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxUserKey, user)))
	}
}

// platformAdminRequired restricts to platform admins (no tenant).
func (s *Server) platformAdminRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.resolveUser(r)
		if err != nil || user == nil {
			jsonError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !isPlatformAdmin(user) {
			jsonError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxUserKey, user)))
	}
}

func roleLabel(role models.UserRole) string {
	if role == models.RoleAdmin {
		return "Admin"
	}
	return "User"
}

func redactMonitor(m *models.Monitor, u *models.User) {
	if m == nil {
		return
	}
	sanitizeMonitorHTTPAuth(m, u)
	if isPlatformAdmin(u) {
		return
	}
	m.HeartbeatToken = ""
	m.RequestHeaders = ""
	m.RequestBody = ""
}

func sanitizeMonitorHTTPAuth(m *models.Monitor, u *models.User) {
	if m == nil {
		return
	}
	m.HTTPAuthSet = strings.TrimSpace(m.HTTPUsername) != "" || m.HTTPPassword != ""
	m.HTTPPassword = ""
	if u != nil && u.Role == models.RoleViewer {
		m.HTTPUsername = ""
	}
}

func applyHTTPAuthUpdate(existing, input *models.Monitor) {
	if existing == nil || input == nil {
		return
	}
	applyHTTPAuthFields(&existing.HTTPUsername, &existing.HTTPPassword, input.HTTPUsername, input.HTTPPassword)
}

func applyPerformanceHTTPAuthUpdate(existing, input *models.PerformanceTarget) {
	if existing == nil || input == nil {
		return
	}
	applyHTTPAuthFields(&existing.HTTPUsername, &existing.HTTPPassword, input.HTTPUsername, input.HTTPPassword)
}

func applyHTTPAuthFields(existingUser, existingPass *string, inputUser, inputPass string) {
	if existingUser == nil || existingPass == nil {
		return
	}
	user := strings.TrimSpace(inputUser)
	if user == "" {
		*existingUser = ""
		*existingPass = ""
		return
	}
	*existingUser = user
	if inputPass == "" || strings.HasPrefix(inputPass, "****") {
		return
	}
	*existingPass = inputPass
}

func redactPerformanceTarget(t *models.PerformanceTarget, u *models.User) {
	if t == nil {
		return
	}
	sanitizePerformanceHTTPAuth(t, u)
	if isPlatformAdmin(u) {
		return
	}
	// keep alert_emails visible to customer admins for their own targets; hide for viewers
	if u != nil && u.Role == models.RoleViewer {
		t.AlertEmails = ""
	}
}

func sanitizePerformanceHTTPAuth(t *models.PerformanceTarget, u *models.User) {
	if t == nil {
		return
	}
	t.HTTPAuthSet = strings.TrimSpace(t.HTTPUsername) != "" || t.HTTPPassword != ""
	t.HTTPPassword = ""
	if u != nil && u.Role == models.RoleViewer {
		t.HTTPUsername = ""
	}
}
