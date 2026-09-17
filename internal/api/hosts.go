package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func (s *Server) handleListHosts(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	customerFilter := strings.TrimSpace(r.URL.Query().Get("customer"))

	var hosts []models.Host
	var err error
	if isPlatformAdmin(user) {
		if customerFilter != "" {
			hosts, err = s.store.ListHostsByTenant(customerFilter)
		} else {
			hosts, err = s.store.ListHosts()
		}
	} else if user.TenantID != "" {
		hosts, err = s.store.ListHostsByTenant(user.TenantID)
	} else {
		hosts = []models.Host{}
	}
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if hosts == nil {
		hosts = []models.Host{}
	}
	jsonOK(w, hosts)
}

func (s *Server) loadVisibleHost(w http.ResponseWriter, r *http.Request) (*models.Host, bool) {
	user := currentUser(r)
	h, err := s.store.GetHost(r.PathValue("id"))
	if err != nil {
		jsonInternal(w, err)
		return nil, false
	}
	if h == nil || !canAccessTenant(user, h.TenantID) || (!isPlatformAdmin(user) && h.TenantID == "") {
		jsonError(w, http.StatusNotFound, "not found")
		return nil, false
	}
	return h, true
}

func (s *Server) handleGetHost(w http.ResponseWriter, r *http.Request) {
	h, ok := s.loadVisibleHost(w, r)
	if !ok {
		return
	}
	jsonOK(w, h)
}

func (s *Server) handleCreateHost(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	var h models.Host
	if err := json.Unmarshal(body, &h); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	applyHostNotifyDefaults(&h, body, true)
	h.Enabled = true
	h.Status = models.HostPending
	h.CollectCPU, h.CollectMemory, h.CollectDisk, h.CollectLoad = true, true, true, true
	h.CollectSwap, h.CollectIOWait, h.CollectSecurity, h.CollectServices = true, true, true, true
	h.AlertCPUEnabled, h.AlertMemoryEnabled, h.AlertDiskEnabled, h.AlertLoadEnabled = false, false, false, false
	h.AlertSwapEnabled, h.AlertIOWaitEnabled, h.AlertAuthEnabled, h.AlertRootLoginEnabled, h.AlertRebootEnabled = false, false, false, false, false
	h.AlertServiceEnabled = true
	if isCustomerAdmin(user) {
		h.TenantID = user.TenantID
	} else if !isPlatformAdmin(user) {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	} else {
		h.TenantID = strings.TrimSpace(h.TenantID)
	}
	if strings.TrimSpace(h.Name) == "" {
		h.Name = "New host"
	}
	models.ApplyHostDefaults(&h)
	if err := s.store.CreateHost(&h); err != nil {
		jsonInternal(w, err)
		return
	}
	if err := s.issueHostEnroll(r, &h); err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(user.Username, "create", "host", h.DisplayName())
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, h)
}

func (s *Server) handleUpdateHost(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	existing, ok := s.loadWritableHost(w, r)
	if !ok {
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	var patch models.Host
	if err := json.Unmarshal(body, &patch); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	existing.Name = strings.TrimSpace(patch.Name)
	if len(existing.Name) > models.MaxHostNameLen {
		existing.Name = strings.TrimSpace(existing.Name[:models.MaxHostNameLen])
	}
	if existing.Name == "" {
		existing.Name = existing.DisplayName()
	}
	existing.AlertEmails = patch.AlertEmails
	existing.IntervalSeconds = models.ClampHostInterval(patch.IntervalSeconds)
	existing.AlertAfterFailures = patch.AlertAfterFailures
	existing.CollectCPU = patch.CollectCPU
	existing.CollectMemory = patch.CollectMemory
	existing.CollectDisk = patch.CollectDisk
	existing.CollectLoad = patch.CollectLoad
	existing.CollectSwap = patch.CollectSwap
	existing.CollectIOWait = patch.CollectIOWait
	existing.CollectSecurity = patch.CollectSecurity
	existing.CollectServices = patch.CollectServices
	existing.AlertCPUEnabled = patch.AlertCPUEnabled
	existing.AlertCPUWarning = patch.AlertCPUWarning
	existing.AlertCPUThreshold = patch.AlertCPUThreshold
	existing.AlertCPUAfter = patch.AlertCPUAfter
	existing.AlertMemoryEnabled = patch.AlertMemoryEnabled
	existing.AlertMemoryWarning = patch.AlertMemoryWarning
	existing.AlertMemoryThreshold = patch.AlertMemoryThreshold
	existing.AlertMemoryAfter = patch.AlertMemoryAfter
	existing.AlertDiskEnabled = patch.AlertDiskEnabled
	existing.AlertDiskWarning = patch.AlertDiskWarning
	existing.AlertDiskThreshold = patch.AlertDiskThreshold
	existing.AlertDiskAfter = patch.AlertDiskAfter
	existing.AlertLoadEnabled = patch.AlertLoadEnabled
	existing.AlertLoadWarning = patch.AlertLoadWarning
	existing.AlertLoadThreshold = patch.AlertLoadThreshold
	existing.AlertLoadAfter = patch.AlertLoadAfter
	existing.AlertSwapEnabled = patch.AlertSwapEnabled
	existing.AlertSwapWarning = patch.AlertSwapWarning
	existing.AlertSwapThreshold = patch.AlertSwapThreshold
	existing.AlertSwapAfter = patch.AlertSwapAfter
	existing.AlertIOWaitEnabled = patch.AlertIOWaitEnabled
	existing.AlertIOWaitWarning = patch.AlertIOWaitWarning
	existing.AlertIOWaitThreshold = patch.AlertIOWaitThreshold
	existing.AlertIOWaitAfter = patch.AlertIOWaitAfter
	existing.AlertAuthEnabled = patch.AlertAuthEnabled
	existing.AlertAuthThreshold = patch.AlertAuthThreshold
	existing.AlertRootLoginEnabled = patch.AlertRootLoginEnabled
	existing.AlertRebootEnabled = patch.AlertRebootEnabled
	existing.AlertServiceEnabled = patch.AlertServiceEnabled
	existing.Services = models.NormalizeWatchedServices(patch.Services)
	if isPlatformAdmin(user) {
		existing.TenantID = strings.TrimSpace(patch.TenantID)
	}
	applyHostNotifyUpdate(existing, body)
	models.ApplyHostDefaults(existing)
	if err := s.store.UpdateHost(existing); err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(user.Username, "update", "host", existing.DisplayName())
	jsonOK(w, existing)
}

func (s *Server) handleSetHostEnabled(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	existing, ok := s.loadWritableHost(w, r)
	if !ok {
		return
	}
	enabled, ok := decodeEnabled(w, r)
	if !ok {
		return
	}
	if err := s.store.SetHostEnabled(existing.ID, enabled); err != nil {
		jsonInternal(w, err)
		return
	}
	updated, err := s.store.GetHost(existing.ID)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	action := "pause"
	if enabled {
		action = "resume"
	}
	_ = s.store.InsertAudit(user.Username, action, "host", updated.DisplayName())
	jsonOK(w, updated)
}

func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	existing, ok := s.loadWritableHost(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteHost(existing.ID); err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(user.Username, "delete", "host", existing.DisplayName())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHostEnrollCommand(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	h, ok := s.loadWritableHost(w, r)
	if !ok {
		return
	}
	if err := s.issueHostEnroll(r, h); err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(user.Username, "enroll", "host", h.DisplayName())
	jsonOK(w, h)
}

func (s *Server) handleGetHostStats(w http.ResponseWriter, r *http.Request) {
	h, ok := s.loadVisibleHost(w, r)
	if !ok {
		return
	}
	win := parseStatsRange(r.URL.Query().Get("period"), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	stats, err := s.store.GetHostStats(h.ID, win.From, win.To)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	jsonOK(w, stats)
}

func (s *Server) loadWritableHost(w http.ResponseWriter, r *http.Request) (*models.Host, bool) {
	user := currentUser(r)
	existing, err := s.store.GetHost(r.PathValue("id"))
	if err != nil {
		jsonInternal(w, err)
		return nil, false
	}
	if existing == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return nil, false
	}
	if !canWriteMonitor(user, existing.TenantID) {
		if isCustomerAdmin(user) {
			jsonError(w, http.StatusNotFound, "not found")
			return nil, false
		}
		jsonError(w, http.StatusForbidden, "forbidden")
		return nil, false
	}
	return existing, true
}

func (s *Server) issueHostEnroll(r *http.Request, h *models.Host) error {
	token, err := randomToken(24)
	if err != nil {
		return err
	}
	if err := s.store.InvalidateUnusedHostEnrollTokens(h.ID); err != nil {
		return err
	}
	expires := time.Now().UTC().Add(models.HostEnrollTTL)
	if err := s.store.CreateHostEnrollToken(h.ID, token, expires); err != nil {
		return err
	}
	base := s.publicBaseURL(r)
	h.EnrollToken = token
	h.EnrollExpires = &expires
	h.InstallCommand = "curl -fsSL '" + base + "/api/hosts/install.sh?token=" + token + "' | sudo sh"
	return nil
}

func (s *Server) publicBaseURL(r *http.Request) string {
	if u := s.dashboardBaseURL(); u != "" {
		return u
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func applyHostNotifyDefaults(h *models.Host, body []byte, create bool) {
	var flags notifyChannelFlags
	_ = json.Unmarshal(body, &flags)
	if flags.NotifyEmail != nil {
		h.NotifyEmail = *flags.NotifyEmail
	} else if create {
		h.NotifyEmail = true
	}
	if flags.NotifySlack != nil {
		h.NotifySlack = *flags.NotifySlack
	} else if create {
		h.NotifySlack = true
	}
	if flags.NotifyWebhooks != nil {
		h.NotifyWebhooks = *flags.NotifyWebhooks
	} else if create {
		h.NotifyWebhooks = true
	}
}

func applyHostNotifyUpdate(existing *models.Host, body []byte) {
	var flags notifyChannelFlags
	_ = json.Unmarshal(body, &flags)
	if flags.NotifyEmail != nil {
		existing.NotifyEmail = *flags.NotifyEmail
	}
	if flags.NotifySlack != nil {
		existing.NotifySlack = *flags.NotifySlack
	}
	if flags.NotifyWebhooks != nil {
		existing.NotifyWebhooks = *flags.NotifyWebhooks
	}
}
