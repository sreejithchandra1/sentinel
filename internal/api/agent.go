package api

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/agentbin"
	"github.com/sentinel-monitoring/sentinel/internal/models"
)

//go:embed install_agent.sh
var installAgentScript []byte

type agentEnrollRequest struct {
	Token    string `json:"token"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

type agentEnrollResponse struct {
	HostID          string `json:"host_id"`
	IngestToken     string `json:"ingest_token"`
	ServerURL       string `json:"server_url"`
	IntervalSeconds int    `json:"interval_seconds"`
	SHA256          string `json:"sha256"`
}

type agentIngestResponse struct {
	OK     bool                   `json:"ok"`
	Config models.HostAgentConfig `json:"config"`
}

func (s *Server) requireEnrollToken(w http.ResponseWriter, r *http.Request) (*models.Host, string, bool) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		jsonError(w, http.StatusUnauthorized, "token required")
		return nil, "", false
	}
	if !s.limits.Allow("host-enroll-get:"+clientIP(r), 30, time.Minute) {
		jsonError(w, http.StatusTooManyRequests, "too many requests")
		return nil, "", false
	}
	h, err := s.store.LookupHostEnrollToken(token)
	if err != nil {
		jsonInternal(w, err)
		return nil, "", false
	}
	if h == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return nil, "", false
	}
	return h, token, true
}

func (s *Server) handleHostInstallScript(w http.ResponseWriter, r *http.Request) {
	_, token, ok := s.requireEnrollToken(w, r)
	if !ok {
		return
	}
	base := s.publicBaseURL(r)
	script := bytes.ReplaceAll(installAgentScript, []byte("__SENTINEL_URL__"), []byte(base))
	script = bytes.ReplaceAll(script, []byte("__ENROLL_TOKEN__"), []byte(token))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(script)
}

func (s *Server) handleHostAgentDownload(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireEnrollToken(w, r); !ok {
		return
	}
	arch := r.PathValue("arch")
	b, err := agentbin.Binary("linux", arch)
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "agent binary not available")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=sentinel-agent")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}

func (s *Server) handleHostAgentSHA256(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireEnrollToken(w, r); !ok {
		return
	}
	sum, err := agentbin.SHA256("linux", r.PathValue("arch"))
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "agent binary not available")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, sum+"\n")
}

func (s *Server) handleAgentEnroll(w http.ResponseWriter, r *http.Request) {
	if !s.limits.Allow("host-enroll:"+clientIP(r), 10, time.Minute) {
		jsonError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	var req agentEnrollRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	h, err := s.store.ConsumeHostEnrollToken(strings.TrimSpace(req.Token))
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if h == nil {
		jsonError(w, http.StatusUnauthorized, "invalid token")
		return
	}
	ingest, err := randomToken(32)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "token error")
		return
	}
	if err := s.store.SetHostIngestToken(h.ID, ingest); err != nil {
		jsonInternal(w, err)
		return
	}
	h.Hostname = strings.TrimSpace(req.Hostname)
	h.OS = strings.TrimSpace(req.OS)
	h.Arch = strings.TrimSpace(req.Arch)
	if h.OS == "" {
		h.OS = "linux"
	}
	if err := s.store.UpdateHost(h); err != nil {
		jsonInternal(w, err)
		return
	}
	sum, _ := agentbin.SHA256("linux", h.Arch)
	jsonOK(w, agentEnrollResponse{
		HostID:          h.ID,
		IngestToken:     ingest,
		ServerURL:       s.publicBaseURL(r),
		IntervalSeconds: models.ClampHostInterval(h.IntervalSeconds),
		SHA256:          sum,
	})
}

func (s *Server) handleAgentIngest(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !s.limits.Allow("host-ingest:"+hashPrefix(token), 120, time.Minute) {
		jsonError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	h, err := s.store.GetHostByIngestToken(token)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if h == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var payload models.HostIngestPayload
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&payload); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}

	now := time.Now().UTC()
	sample := hostSampleFromPayload(h, &payload, now)
	if err := s.store.InsertHostSample(sample); err != nil {
		jsonInternal(w, err)
		return
	}
	if err := s.store.TouchHostSeen(h.ID, payload.Hostname, "linux", h.Arch, payload.AgentVersion, payload.NumCPU, now); err != nil {
		jsonInternal(w, err)
		return
	}
	if err := s.alerter.HandleHostIngestOnline(h, now); err != nil {
		log.Printf("agent ingest recovery alert: %v", err)
	}
	fresh, err := s.store.GetHost(h.ID)
	if err != nil || fresh == nil {
		jsonInternal(w, err)
		return
	}
	if err := s.alerter.HandleHostSample(fresh, sample); err != nil {
		log.Printf("agent ingest metric alert: %v", err)
	}
	cfg := fresh.AgentConfig()
	jsonOK(w, agentIngestResponse{OK: true, Config: cfg})
}

func hostSampleFromPayload(h *models.Host, p *models.HostIngestPayload, now time.Time) *models.HostSample {
	sample := &models.HostSample{
		HostID:      h.ID,
		NumCPU:      p.NumCPU,
		CollectedAt: now,
	}
	if h.CollectCPU {
		sample.CPUPercent = p.CPUPercent
	}
	if h.CollectMemory {
		sample.MemPercent = p.MemPercent
	}
	if h.CollectLoad {
		sample.Load1, sample.Load5, sample.Load15 = p.Load1, p.Load5, p.Load15
	}
	if h.CollectDisk && len(p.Disks) > 0 {
		worst := -1.0
		var disks []models.HostDisk
		for _, d := range p.Disks {
			mount := strings.TrimSpace(d.Mount)
			if mount == "" {
				continue
			}
			pct := d.Percent
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
			disks = append(disks, models.HostDisk{Mount: mount, Percent: pct})
			if pct > worst {
				worst = pct
			}
		}
		sample.Disks = disks
		if worst >= 0 {
			sample.DiskPercent = &worst
		}
	}
	return sample
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
}

func hashPrefix(token string) string {
	if len(token) > 16 {
		return token[:16]
	}
	return token
}
