package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/config"
	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/safehost"
)

const (
	platformBackupFormat  = "sentinel-platform"
	platformBackupVersion = 1
)

type platformBackupFile struct {
	Format             string                         `json:"format"`
	Version            int                            `json:"version"`
	ExportedAt         time.Time                      `json:"exported_at"`
	ScopeTenantID      string                         `json:"scope_tenant_id,omitempty"`
	Customers          []models.Customer              `json:"customers,omitempty"`
	Users              []models.UserBackup            `json:"users,omitempty"`
	Sites                []models.Site                  `json:"sites,omitempty"`
	Monitors           []models.Monitor               `json:"monitors"`
	PerformanceTargets []models.PerformanceTarget     `json:"performance_targets,omitempty"`
	Hosts              []models.HostBackup            `json:"hosts,omitempty"`
	MaintenanceWindows []models.MaintenanceWindow     `json:"maintenance_windows,omitempty"`
	Settings           *models.PlatformSettingsBackup `json:"settings,omitempty"`
}

type platformBackupImportBody struct {
	platformBackupFile
	Mode string `json:"mode"`
}

type backupSectionPreview struct {
	Total        int                     `json:"total"`
	ToCreate     int                     `json:"to_create"`
	Conflicts    []monitorBackupConflict `json:"conflicts"`
	Invalid      []monitorBackupInvalid  `json:"invalid"`
	WouldUpdate  int                     `json:"would_update"`
	WouldSkip    int                     `json:"would_skip"`
	QuotaBlocked int                     `json:"quota_blocked,omitempty"`
}

type platformBackupPreview struct {
	Format              string               `json:"format"`
	Customers           backupSectionPreview `json:"customers"`
	Users               backupSectionPreview `json:"users"`
	Monitors            backupSectionPreview `json:"monitors"`
	PerformanceTargets  backupSectionPreview `json:"performance_targets"`
	Hosts               backupSectionPreview `json:"hosts"`
	MaintenanceWindows  backupSectionPreview `json:"maintenance_windows"`
	SettingsIncluded    bool                 `json:"settings_included"`
	MissingDependencies []string             `json:"missing_dependencies"`
}

type sectionImportResult struct {
	Created int                    `json:"created"`
	Updated int                    `json:"updated"`
	Skipped int                    `json:"skipped"`
	Failed  []monitorBackupInvalid `json:"failed"`
}

type platformBackupImportResult struct {
	Customers          sectionImportResult `json:"customers"`
	Users              sectionImportResult `json:"users"`
	Monitors           sectionImportResult `json:"monitors"`
	PerformanceTargets sectionImportResult `json:"performance_targets"`
	Hosts              sectionImportResult `json:"hosts"`
	MaintenanceWindows sectionImportResult `json:"maintenance_windows"`
	SettingsApplied    bool                `json:"settings_applied"`
}

func (s *Server) buildPlatformExport(user *models.User, tenantScope string) (*platformBackupFile, error) {
	includePlatform := isPlatformAdmin(user) && tenantScope == ""
	customers, err := s.store.ListCustomersForExport(tenantScope)
	if err != nil {
		return nil, err
	}
	sites, err := s.store.ListSitesForExport(tenantScope)
	if err != nil {
		return nil, err
	}
	monitors, err := s.store.ListMonitorsForExport(tenantScope)
	if err != nil {
		return nil, err
	}
	exportedMonitors := make([]models.Monitor, len(monitors))
	monitorIDs := make([]string, len(monitors))
	for i, m := range monitors {
		exportedMonitors[i] = monitorForExport(m)
		monitorIDs[i] = m.ID
	}
	users, err := s.store.ListUsersForExport(tenantScope)
	if err != nil {
		return nil, err
	}
	userBackups := make([]models.UserBackup, len(users))
	for i, u := range users {
		userBackups[i] = models.UserBackupFromUser(u)
	}
	targets, err := s.store.ListPerformanceTargetsForExport(tenantScope)
	if err != nil {
		return nil, err
	}
	for i := range targets {
		targets[i].ConsecutiveSlow = 0
		targets[i].LastStatus = models.StatusUnknown
		targets[i].LastCheckedAt = nil
	}
	hosts, err := s.store.ListHostsForExport(tenantScope)
	if err != nil {
		return nil, err
	}
	maint, err := s.store.ListMaintenanceWindowsForExport(tenantScope, monitorIDs)
	if err != nil {
		return nil, err
	}
	settings, err := s.store.ExportPlatformSettings(includePlatform, tenantScope, s.defaultSMTP, config.ServerConfig{
		DashboardURL: s.dashboardURL,
	})
	if err != nil {
		return nil, err
	}
	var settingsPtr *models.PlatformSettingsBackup
	if includePlatform || tenantScope != "" {
		settingsPtr = &settings
	}
	return &platformBackupFile{
		Format:             platformBackupFormat,
		Version:            platformBackupVersion,
		ExportedAt:         time.Now().UTC(),
		ScopeTenantID:      tenantScope,
		Customers:          customers,
		Users:              userBackups,
		Sites:              sites,
		Monitors:           exportedMonitors,
		PerformanceTargets: targets,
		Hosts:              hosts,
		MaintenanceWindows: maint,
		Settings:           settingsPtr,
	}, nil
}

func parsePlatformBackupBody(body []byte) (platformBackupFile, string, error) {
	var req platformBackupImportBody
	if err := json.Unmarshal(body, &req); err != nil {
		return platformBackupFile{}, "", fmt.Errorf("invalid request")
	}
	file := req.platformBackupFile
	mode := strings.TrimSpace(req.Mode)

	if file.Format == monitorBackupFormat || (file.Format == "" && len(file.Monitors) > 0 && len(file.Customers) == 0) {
		if file.Format == "" {
			file.Format = monitorBackupFormat
		}
		return file, mode, nil
	}
	if len(file.Monitors) == 0 && len(file.Customers) == 0 && len(file.Users) == 0 {
		var legacy monitorBackupFile
		if err := json.Unmarshal(body, &legacy); err == nil && len(legacy.Monitors) > 0 {
			return platformBackupFile{
				Format:        monitorBackupFormat,
				Version:       legacy.Version,
				ScopeTenantID: legacy.ScopeTenantID,
				Monitors:      legacy.Monitors,
			}, mode, nil
		}
	}
	if file.Format != "" && file.Format != platformBackupFormat && file.Format != monitorBackupFormat {
		return platformBackupFile{}, "", fmt.Errorf("unsupported backup format")
	}
	if file.Format == "" {
		file.Format = platformBackupFormat
	}
	return file, mode, nil
}

func emptySectionResult() sectionImportResult {
	return sectionImportResult{Failed: []monitorBackupInvalid{}}
}

func normalizeSectionResult(r sectionImportResult) sectionImportResult {
	if r.Failed == nil {
		r.Failed = []monitorBackupInvalid{}
	}
	return r
}

func normalizePlatformImportResult(r platformBackupImportResult) platformBackupImportResult {
	r.Customers = normalizeSectionResult(r.Customers)
	r.Users = normalizeSectionResult(r.Users)
	r.Monitors = normalizeSectionResult(r.Monitors)
	r.PerformanceTargets = normalizeSectionResult(r.PerformanceTargets)
	r.Hosts = normalizeSectionResult(r.Hosts)
	r.MaintenanceWindows = normalizeSectionResult(r.MaintenanceWindows)
	return r
}

func (s *Server) importPlatformBackup(user *models.User, file platformBackupFile, mode string) platformBackupImportResult {
	result := platformBackupImportResult{
		Customers:          emptySectionResult(),
		Users:              emptySectionResult(),
		Monitors:           emptySectionResult(),
		PerformanceTargets: emptySectionResult(),
		Hosts:              emptySectionResult(),
		MaintenanceWindows: emptySectionResult(),
	}

	if file.Format == monitorBackupFormat {
		legacy := s.importMonitorsOnly(user, file.Monitors, mode)
		result.Monitors = sectionImportResult{
			Created: legacy.Created,
			Updated: legacy.Updated,
			Skipped: legacy.Skipped,
			Failed:  legacy.Failed,
		}
		return result
	}

	importCustomers(s, user, file.Customers, mode, &result.Customers)
	importUsers(s, user, file.Users, mode, &result.Users)
	importSites(s, user, file.Sites, mode)
	importMonitorsPlatform(s, user, file.Monitors, mode, &result.Monitors)
	importPerformanceTargets(s, user, file.PerformanceTargets, mode, &result.PerformanceTargets)
	importHosts(s, user, file.Hosts, mode, &result.Hosts)
	importMaintenance(s, user, file.MaintenanceWindows, mode, &result.MaintenanceWindows)

	if file.Settings != nil {
		if err := s.applyImportedSettings(user, file.Settings, mode == "overwrite", &result); err != nil {
			result.Customers.Failed = append(result.Customers.Failed, monitorBackupInvalid{Name: "settings", Error: err.Error()})
		}
	}
	return result
}

// applyImportedSettings enforces platform-admin-only writes for global settings (F-02).
// Customer admins may only restore their own Slack config; tenant_id is forced to the actor.
func (s *Server) applyImportedSettings(user *models.User, settings *models.PlatformSettingsBackup, overwrite bool, result *platformBackupImportResult) error {
	if settings == nil {
		return nil
	}
	if isPlatformAdmin(user) {
		if err := s.store.ImportPlatformSettings(settings, overwrite); err != nil {
			return err
		}
		result.SettingsApplied = true
		return nil
	}
	if !isCustomerAdmin(user) {
		return nil
	}
	tenantID := strings.TrimSpace(user.TenantID)
	if tenantID == "" || len(settings.Slack) == 0 {
		// Global org/SMTP/status/server/webhooks are never applied for tenant admins.
		return nil
	}
	for _, sl := range settings.Slack {
		cfg := sl.Config
		cfg.WebhookURL = strings.TrimSpace(cfg.WebhookURL)
		if cfg.WebhookURL != "" {
			if err := safehost.ValidateHTTPURL(cfg.WebhookURL); err != nil {
				return fmt.Errorf("slack webhook: %w", err)
			}
		}
		if err := s.store.SaveSlackConfig(tenantID, cfg); err != nil {
			return err
		}
		result.SettingsApplied = true
	}
	return nil
}

func importCustomers(s *Server, user *models.User, customers []models.Customer, mode string, result *sectionImportResult) {
	ids := make([]string, 0, len(customers))
	for _, c := range customers {
		if c.ID != "" {
			ids = append(ids, c.ID)
		}
	}
	existing, _ := s.store.GetCustomersByIDs(ids)
	for _, raw := range customers {
		c := raw
		if !customerAccessibleByUser(user, c) {
			result.Skipped++
			continue
		}
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Name) == "" {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: c.ID, Name: c.Name, Error: "customer id and name required"})
			continue
		}
		if ex, ok := existing[c.ID]; ok {
			if mode == "create_only" {
				result.Skipped++
				continue
			}
			if err := s.store.ReplaceCustomerImport(&c); err != nil {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: c.ID, Name: c.Name, Error: err.Error()})
				continue
			}
			result.Updated++
			_ = ex
			continue
		}
		if err := s.store.InsertCustomerImport(&c); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: c.ID, Name: c.Name, Error: err.Error()})
			continue
		}
		result.Created++
		existing[c.ID] = c
	}
}

func customerAccessibleByUser(user *models.User, c models.Customer) bool {
	if isPlatformAdmin(user) {
		return true
	}
	return isCustomerAdmin(user) && c.ID == user.TenantID
}

func importUsers(s *Server, user *models.User, users []models.UserBackup, mode string, result *sectionImportResult) {
	ids := make([]string, 0, len(users))
	for _, u := range users {
		if u.ID != "" {
			ids = append(ids, u.ID)
		}
	}
	existing, _ := s.store.GetUsersByIDs(ids)
	for _, raw := range users {
		b := raw
		u := b.ToUser()
		if err := normalizeBackupUser(user, &u); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: u.ID, Name: u.Username, Error: err.Error()})
			continue
		}
		if strings.TrimSpace(u.ID) == "" {
			result.Failed = append(result.Failed, monitorBackupInvalid{Name: u.Username, Error: "user id required"})
			continue
		}
		if u.TenantID != "" {
			ok, err := s.store.CustomerExists(u.TenantID)
			if err != nil || !ok {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: u.ID, Name: u.Username, Error: "customer not found for tenant_id"})
				continue
			}
		}
		if ex, ok := existing[u.ID]; ok {
			if !userAccessibleByUser(user, ex) {
				result.Skipped++
				continue
			}
			if mode == "create_only" {
				result.Skipped++
				continue
			}
			if err := s.store.ReplaceUserImport(&u); err != nil {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: u.ID, Name: u.Username, Error: err.Error()})
				continue
			}
			result.Updated++
			continue
		}
		if err := s.store.InsertUserImport(&u); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: u.ID, Name: u.Username, Error: err.Error()})
			continue
		}
		result.Created++
		existing[u.ID] = u
	}
}

func normalizeBackupUser(actor *models.User, u *models.User) error {
	if isCustomerAdmin(actor) {
		u.TenantID = actor.TenantID
		return nil
	}
	if !isPlatformAdmin(actor) {
		return fmt.Errorf("forbidden")
	}
	u.TenantID = strings.TrimSpace(u.TenantID)
	return nil
}

func userAccessibleByUser(actor *models.User, u models.User) bool {
	if isPlatformAdmin(actor) {
		return true
	}
	return isCustomerAdmin(actor) && u.TenantID == actor.TenantID
}

func importMonitorsPlatform(s *Server, user *models.User, monitors []models.Monitor, mode string, result *sectionImportResult) {
	legacy := s.importMonitorsOnly(user, monitors, mode)
	*result = sectionImportResult{Created: legacy.Created, Updated: legacy.Updated, Skipped: legacy.Skipped, Failed: legacy.Failed}
}

func (s *Server) importMonitorsOnly(user *models.User, monitors []models.Monitor, mode string) monitorBackupImportResult {
	result := monitorBackupImportResult{Failed: []monitorBackupInvalid{}}
	ids := make([]string, 0, len(monitors))
	for _, m := range monitors {
		if strings.TrimSpace(m.ID) != "" {
			ids = append(ids, m.ID)
		}
	}
	existing, err := s.store.GetMonitorsByIDs(ids)
	if err != nil {
		result.Failed = append(result.Failed, monitorBackupInvalid{Error: err.Error()})
		return result
	}
	for _, raw := range monitors {
		m := raw
		if err := s.normalizeBackupMonitor(user, &m); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: m.ID, Name: m.Name, Error: err.Error()})
			continue
		}
		if err := validateBackupMonitor(&m); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: m.ID, Name: m.Name, Error: err.Error()})
			continue
		}
		if m.TenantID != "" {
			ok, err := s.store.CustomerExists(m.TenantID)
			if err != nil || !ok {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: m.ID, Name: m.Name, Error: "customer not found for tenant_id"})
				continue
			}
		}
		ex, hasConflict := existing[m.ID]
		if hasConflict {
			if !monitorAccessibleByUser(user, ex) {
				result.Skipped++
				continue
			}
			if mode == "create_only" {
				result.Skipped++
				continue
			}
			if m.Type == models.MonitorHeartbeat && strings.TrimSpace(m.HeartbeatToken) == "" {
				m.HeartbeatToken = ex.HeartbeatToken
			}
			if err := s.store.ReplaceMonitorImport(&m); err != nil {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: m.ID, Name: m.Name, Error: err.Error()})
				continue
			}
			result.Updated++
			continue
		}
		if m.TenantID != "" {
			if err := s.store.AssertMonitorQuota(m.TenantID); err != nil {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: m.ID, Name: m.Name, Error: err.Error()})
				continue
			}
		}
		if m.Type == models.MonitorHeartbeat && strings.TrimSpace(m.HeartbeatToken) == "" {
			token, err := randomToken(24)
			if err != nil {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: m.ID, Name: m.Name, Error: "token error"})
				continue
			}
			m.HeartbeatToken = token
		} else if m.Type != models.MonitorHeartbeat {
			m.HeartbeatToken = ""
		}
		if err := s.store.InsertMonitorImport(&m); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: m.ID, Name: m.Name, Error: err.Error()})
			continue
		}
		result.Created++
		existing[m.ID] = m
	}
	return result
}

func importPerformanceTargets(s *Server, user *models.User, targets []models.PerformanceTarget, mode string, result *sectionImportResult) {
	ids := make([]string, 0, len(targets))
	for _, t := range targets {
		if t.ID != "" {
			ids = append(ids, t.ID)
		}
	}
	existing, _ := s.store.GetPerformanceTargetsByIDs(ids)
	for _, raw := range targets {
		t := raw
		if err := normalizeBackupTenant(user, &t.TenantID); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: t.ID, Name: t.Name, Error: err.Error()})
			continue
		}
		if strings.TrimSpace(t.ID) == "" || strings.TrimSpace(t.Name) == "" {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: t.ID, Name: t.Name, Error: "id and name required"})
			continue
		}
		if t.TenantID != "" {
			ok, err := s.store.CustomerExists(t.TenantID)
			if err != nil || !ok {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: t.ID, Name: t.Name, Error: "customer not found for tenant_id"})
				continue
			}
		}
		if err := safehost.ValidateHTTPURL(t.URL); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: t.ID, Name: t.Name, Error: err.Error()})
			continue
		}
		if ex, ok := existing[t.ID]; ok {
			if !tenantResourceAccessible(user, ex.TenantID) {
				result.Skipped++
				continue
			}
			if mode == "create_only" {
				result.Skipped++
				continue
			}
			if err := s.store.ReplacePerformanceTargetImport(&t); err != nil {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: t.ID, Name: t.Name, Error: err.Error()})
				continue
			}
			result.Updated++
			continue
		}
		if err := s.store.InsertPerformanceTargetImport(&t); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: t.ID, Name: t.Name, Error: err.Error()})
			continue
		}
		result.Created++
		existing[t.ID] = t
	}
}

func importHosts(s *Server, user *models.User, hosts []models.HostBackup, mode string, result *sectionImportResult) {
	ids := make([]string, 0, len(hosts))
	for _, h := range hosts {
		if h.ID != "" {
			ids = append(ids, h.ID)
		}
	}
	existing, _ := s.store.GetHostsByIDs(ids)
	for _, raw := range hosts {
		b := raw
		if err := normalizeBackupTenant(user, &b.TenantID); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: b.ID, Name: b.Name, Error: err.Error()})
			continue
		}
		if strings.TrimSpace(b.ID) == "" || strings.TrimSpace(b.Name) == "" {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: b.ID, Name: b.Name, Error: "id and name required"})
			continue
		}
		if b.TenantID != "" {
			ok, err := s.store.CustomerExists(b.TenantID)
			if err != nil || !ok {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: b.ID, Name: b.Name, Error: "customer not found for tenant_id"})
				continue
			}
		}
		if ex, ok := existing[b.ID]; ok {
			if !tenantResourceAccessible(user, ex.TenantID) {
				result.Skipped++
				continue
			}
			if mode == "create_only" {
				result.Skipped++
				continue
			}
			if err := s.store.ReplaceHostImport(b); err != nil {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: b.ID, Name: b.Name, Error: err.Error()})
				continue
			}
			result.Updated++
			continue
		}
		if err := s.store.InsertHostImport(b); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: b.ID, Name: b.Name, Error: err.Error()})
			continue
		}
		result.Created++
	}
}

func importSites(s *Server, user *models.User, sites []models.Site, mode string) {
	for _, raw := range sites {
		site := raw
		if !tenantResourceAccessible(user, site.TenantID) {
			continue
		}
		if strings.TrimSpace(site.ID) == "" || strings.TrimSpace(site.Name) == "" {
			continue
		}
		existing, err := s.store.GetSite(site.ID)
		if err != nil {
			continue
		}
		if existing != nil {
			if mode == "create_only" {
				continue
			}
			_ = s.store.ReplaceSiteImport(&site)
			continue
		}
		_ = s.store.InsertSiteImport(&site)
	}
}

func importMaintenance(s *Server, user *models.User, windows []models.MaintenanceWindow, mode string, result *sectionImportResult) {
	if !isPlatformAdmin(user) {
		for range windows {
			result.Skipped++
		}
		return
	}
	ids := make([]string, 0, len(windows))
	for _, w := range windows {
		if w.ID != "" {
			ids = append(ids, w.ID)
		}
	}
	existing, _ := s.store.GetMaintenanceWindowsByIDs(ids)
	for _, raw := range windows {
		w := raw
		if strings.TrimSpace(w.ID) == "" || strings.TrimSpace(w.Name) == "" {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: w.ID, Name: w.Name, Error: "id and name required"})
			continue
		}
		if w.MonitorID != "" {
			ok, err := s.store.MonitorExists(w.MonitorID)
			if err != nil || !ok {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: w.ID, Name: w.Name, Error: "monitor not found for maintenance window"})
				continue
			}
		}
		if _, ok := existing[w.ID]; ok {
			if mode == "create_only" {
				result.Skipped++
				continue
			}
			if err := s.store.ReplaceMaintenanceImport(&w); err != nil {
				result.Failed = append(result.Failed, monitorBackupInvalid{ID: w.ID, Name: w.Name, Error: err.Error()})
				continue
			}
			result.Updated++
			continue
		}
		if err := s.store.InsertMaintenanceImport(&w); err != nil {
			result.Failed = append(result.Failed, monitorBackupInvalid{ID: w.ID, Name: w.Name, Error: err.Error()})
			continue
		}
		result.Created++
	}
}

func normalizeBackupTenant(user *models.User, tenantID *string) error {
	if tenantID == nil {
		return nil
	}
	if isCustomerAdmin(user) {
		*tenantID = user.TenantID
		return nil
	}
	if !isPlatformAdmin(user) {
		return fmt.Errorf("forbidden")
	}
	*tenantID = strings.TrimSpace(*tenantID)
	return nil
}

func tenantResourceAccessible(user *models.User, tenantID string) bool {
	if isPlatformAdmin(user) {
		return true
	}
	return isCustomerAdmin(user) && tenantID == user.TenantID
}

func emptyBackupSectionPreview() backupSectionPreview {
	return backupSectionPreview{
		Conflicts: []monitorBackupConflict{},
		Invalid:   []monitorBackupInvalid{},
	}
}

func emptyPlatformBackupPreview(format string) *platformBackupPreview {
	return &platformBackupPreview{
		Format:              format,
		MissingDependencies: []string{},
		Customers:           emptyBackupSectionPreview(),
		Users:               emptyBackupSectionPreview(),
		Monitors:            emptyBackupSectionPreview(),
		PerformanceTargets:  emptyBackupSectionPreview(),
		Hosts:               emptyBackupSectionPreview(),
		MaintenanceWindows:  emptyBackupSectionPreview(),
	}
}

func (s *Server) previewPlatformBackup(user *models.User, file platformBackupFile, mode string) (*platformBackupPreview, error) {
	if file.Format == monitorBackupFormat {
		mp, err := s.buildMonitorBackupPreview(user, file.Monitors, mode)
		if err != nil {
			return nil, err
		}
		out := emptyPlatformBackupPreview(monitorBackupFormat)
		out.Monitors = sectionFromMonitorPreview(mp, mode)
		return out, nil
	}
	out := emptyPlatformBackupPreview(platformBackupFormat)
	out.SettingsIncluded = file.Settings != nil && isPlatformAdmin(user)
	if file.Settings != nil && isCustomerAdmin(user) && len(file.Settings.Slack) > 0 {
		// Tenant admins may restore only their own Slack block.
		out.SettingsIncluded = true
	}
	out.Customers = previewCustomers(s, user, file.Customers, mode)
	out.Users = previewUsers(s, user, file.Users, mode)
	mp, err := s.buildMonitorBackupPreview(user, file.Monitors, mode)
	if err != nil {
		return nil, err
	}
	out.Monitors = sectionFromMonitorPreview(mp, mode)
	out.PerformanceTargets = previewPerformanceTargets(s, user, file.PerformanceTargets, mode)
	out.Hosts = previewHosts(s, user, file.Hosts, mode)
	out.MaintenanceWindows = previewMaintenance(user, file.MaintenanceWindows, mode)
	out.MissingDependencies = findMissingDependencies(s, file)
	return out, nil
}

func sectionFromMonitorPreview(mp *monitorBackupPreview, mode string) backupSectionPreview {
	conflicts := mp.Conflicts
	if conflicts == nil {
		conflicts = []monitorBackupConflict{}
	}
	invalid := mp.Invalid
	if invalid == nil {
		invalid = []monitorBackupInvalid{}
	}
	sec := backupSectionPreview{
		Total:        mp.Total,
		ToCreate:     mp.ToCreate,
		Conflicts:    conflicts,
		Invalid:      invalid,
		QuotaBlocked: mp.QuotaBlocked,
		WouldSkip:    mp.WouldSkip,
	}
	if mode == "overwrite" {
		sec.WouldUpdate = mp.WouldUpdate
	}
	return sec
}

func previewCustomers(s *Server, user *models.User, customers []models.Customer, mode string) backupSectionPreview {
	sec := backupSectionPreview{Conflicts: []monitorBackupConflict{}, Invalid: []monitorBackupInvalid{}}
	sec.Total = len(customers)
	ids := make([]string, 0, len(customers))
	for _, c := range customers {
		if c.ID != "" {
			ids = append(ids, c.ID)
		}
	}
	existing, _ := s.store.GetCustomersByIDs(ids)
	for _, c := range customers {
		if !customerAccessibleByUser(user, c) {
			sec.WouldSkip++
			continue
		}
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Name) == "" {
			sec.Invalid = append(sec.Invalid, monitorBackupInvalid{ID: c.ID, Name: c.Name, Error: "customer id and name required"})
			continue
		}
		if ex, ok := existing[c.ID]; ok {
			sec.Conflicts = append(sec.Conflicts, monitorBackupConflict{ID: c.ID, ImportName: c.Name, ExistingName: ex.Name})
			if mode == "overwrite" {
				sec.WouldUpdate++
			} else {
				sec.WouldSkip++
			}
			continue
		}
		sec.ToCreate++
	}
	return sec
}

func previewUsers(s *Server, user *models.User, users []models.UserBackup, mode string) backupSectionPreview {
	sec := backupSectionPreview{Conflicts: []monitorBackupConflict{}, Invalid: []monitorBackupInvalid{}}
	sec.Total = len(users)
	ids := make([]string, 0, len(users))
	for _, u := range users {
		if u.ID != "" {
			ids = append(ids, u.ID)
		}
	}
	existing, _ := s.store.GetUsersByIDs(ids)
	for _, b := range users {
		u := b.ToUser()
		if err := normalizeBackupUser(user, &u); err != nil {
			sec.Invalid = append(sec.Invalid, monitorBackupInvalid{ID: u.ID, Name: u.Username, Error: err.Error()})
			continue
		}
		if ex, ok := existing[u.ID]; ok {
			if !userAccessibleByUser(user, ex) {
				sec.WouldSkip++
				continue
			}
			sec.Conflicts = append(sec.Conflicts, monitorBackupConflict{ID: u.ID, ImportName: u.Username, ExistingName: ex.Username})
			if mode == "overwrite" {
				sec.WouldUpdate++
			} else {
				sec.WouldSkip++
			}
			continue
		}
		sec.ToCreate++
	}
	return sec
}

func previewPerformanceTargets(s *Server, user *models.User, targets []models.PerformanceTarget, mode string) backupSectionPreview {
	sec := backupSectionPreview{Conflicts: []monitorBackupConflict{}, Invalid: []monitorBackupInvalid{}}
	sec.Total = len(targets)
	ids := make([]string, 0, len(targets))
	for _, t := range targets {
		if t.ID != "" {
			ids = append(ids, t.ID)
		}
	}
	existing, _ := s.store.GetPerformanceTargetsByIDs(ids)
	for _, t := range targets {
		if err := normalizeBackupTenant(user, &t.TenantID); err != nil {
			sec.Invalid = append(sec.Invalid, monitorBackupInvalid{ID: t.ID, Name: t.Name, Error: err.Error()})
			continue
		}
		if ex, ok := existing[t.ID]; ok {
			if !tenantResourceAccessible(user, ex.TenantID) {
				sec.WouldSkip++
				continue
			}
			sec.Conflicts = append(sec.Conflicts, monitorBackupConflict{ID: t.ID, ImportName: t.Name, ExistingName: ex.Name})
			if mode == "overwrite" {
				sec.WouldUpdate++
			} else {
				sec.WouldSkip++
			}
			continue
		}
		sec.ToCreate++
	}
	return sec
}

func previewHosts(s *Server, user *models.User, hosts []models.HostBackup, mode string) backupSectionPreview {
	sec := backupSectionPreview{Conflicts: []monitorBackupConflict{}, Invalid: []monitorBackupInvalid{}}
	sec.Total = len(hosts)
	ids := make([]string, 0, len(hosts))
	for _, h := range hosts {
		if h.ID != "" {
			ids = append(ids, h.ID)
		}
	}
	existing, _ := s.store.GetHostsByIDs(ids)
	for _, h := range hosts {
		if err := normalizeBackupTenant(user, &h.TenantID); err != nil {
			sec.Invalid = append(sec.Invalid, monitorBackupInvalid{ID: h.ID, Name: h.Name, Error: err.Error()})
			continue
		}
		if ex, ok := existing[h.ID]; ok {
			if !tenantResourceAccessible(user, ex.TenantID) {
				sec.WouldSkip++
				continue
			}
			sec.Conflicts = append(sec.Conflicts, monitorBackupConflict{ID: h.ID, ImportName: h.Name, ExistingName: ex.Name})
			if mode == "overwrite" {
				sec.WouldUpdate++
			} else {
				sec.WouldSkip++
			}
			continue
		}
		sec.ToCreate++
	}
	return sec
}

func previewMaintenance(user *models.User, windows []models.MaintenanceWindow, mode string) backupSectionPreview {
	sec := backupSectionPreview{Conflicts: []monitorBackupConflict{}, Invalid: []monitorBackupInvalid{}}
	sec.Total = len(windows)
	if !isPlatformAdmin(user) {
		sec.WouldSkip = len(windows)
		return sec
	}
	for _, w := range windows {
		if strings.TrimSpace(w.ID) == "" {
			sec.Invalid = append(sec.Invalid, monitorBackupInvalid{ID: w.ID, Name: w.Name, Error: "id required"})
			continue
		}
		sec.ToCreate++
	}
	return sec
}

func findMissingDependencies(s *Server, file platformBackupFile) []string {
	inFile := map[string]bool{}
	for _, c := range file.Customers {
		inFile[c.ID] = true
	}
	var missing []string
	for _, m := range file.Monitors {
		if m.TenantID == "" {
			continue
		}
		if inFile[m.TenantID] {
			continue
		}
		ok, err := s.store.CustomerExists(m.TenantID)
		if err != nil || ok {
			continue
		}
		missing = append(missing, "customer "+m.TenantID+" for monitor "+m.Name)
	}
	return missing
}
