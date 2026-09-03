package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type customerRequest struct {
	Name         string  `json:"name"`
	MonitorQuota *int    `json:"monitor_quota"`
	AlertEmails  *string `json:"alert_emails"`
}

func (s *Server) handleListCustomers(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListCustomers()
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if items == nil {
		jsonOK(w, []any{})
		return
	}
	jsonOK(w, items)
}

func (s *Server) handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req customerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	quota := 1
	if req.MonitorQuota != nil {
		quota = *req.MonitorQuota
	}
	c, err := s.store.CreateCustomer(req.Name, quota)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.InsertAudit(currentUser(r).Username, "create", "customer", c.Name)
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, c)
}

func (s *Server) handleUpdateCustomer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req customerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	existing, err := s.store.GetCustomer(id)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if existing == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = existing.Name
	}
	quota := existing.MonitorQuota
	if req.MonitorQuota != nil {
		quota = *req.MonitorQuota
	}
	emails := existing.AlertEmails
	if req.AlertEmails != nil {
		emails = strings.TrimSpace(*req.AlertEmails)
	}
	c, err := s.store.UpdateCustomer(id, name, quota, emails)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.InsertAudit(currentUser(r).Username, "update", "customer", c.Name)
	jsonOK(w, c)
}

func (s *Server) handleDeleteCustomer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.store.GetCustomer(id)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if existing == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	if err := s.store.DeleteCustomer(id); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.InsertAudit(currentUser(r).Username, "delete", "customer", existing.Name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetAlertRecipients(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !isCustomerAdmin(user) {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	}
	c, err := s.store.GetCustomer(user.TenantID)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if c == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	jsonOK(w, map[string]string{"alert_emails": c.AlertEmails})
}

func (s *Server) handlePutAlertRecipients(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !isCustomerAdmin(user) {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	}
	var req struct {
		AlertEmails string `json:"alert_emails"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	c, err := s.store.GetCustomer(user.TenantID)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if c == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	updated, err := s.store.UpdateCustomer(c.ID, c.Name, c.MonitorQuota, strings.TrimSpace(req.AlertEmails))
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.InsertAudit(user.Username, "update", "customer", "alert_emails")
	jsonOK(w, map[string]string{"alert_emails": updated.AlertEmails})
}

func (s *Server) handleTestAlertRecipients(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !isCustomerAdmin(user) {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	}
	var req struct {
		AlertEmails string `json:"alert_emails"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	raw := strings.TrimSpace(req.AlertEmails)
	if raw == "" {
		c, err := s.store.GetCustomer(user.TenantID)
		if err != nil {
			jsonInternal(w, err)
			return
		}
		if c == nil {
			jsonError(w, http.StatusNotFound, "not found")
			return
		}
		raw = c.AlertEmails
	}
	s.sendAlertRecipientTest(w, r, raw)
}

func (s *Server) handleTestCustomerEmails(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		AlertEmails string `json:"alert_emails"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	raw := strings.TrimSpace(req.AlertEmails)
	if raw == "" {
		c, err := s.store.GetCustomer(id)
		if err != nil {
			jsonInternal(w, err)
			return
		}
		if c == nil {
			jsonError(w, http.StatusNotFound, "not found")
			return
		}
		raw = c.AlertEmails
	}
	s.sendAlertRecipientTest(w, r, raw)
}

func (s *Server) sendAlertRecipientTest(w http.ResponseWriter, r *http.Request, raw string) {
	user := currentUser(r)
	ip := clientIP(r)
	key := "alert-recipients-test:" + ip
	actor := "unknown"
	if user != nil {
		actor = user.Username
		key = "alert-recipients-test:" + user.ID
	}
	if !s.limits.Allow(key, 3, time.Minute) {
		s.recordSecurityEvent("rate limit exceeded", actor, "rate_limit", "smtp",
			"test ip="+ip,
			"endpoint", "alert-recipients-test", "ip", ip, "user", actor)
		jsonError(w, http.StatusTooManyRequests, "too many requests, try again later")
		return
	}
	if err := s.alerter.SendTestEmails(raw); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}
