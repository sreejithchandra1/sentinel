package alerter

import (
	"regexp"
	"strings"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

type HostServiceRow struct {
	Name   string
	Status string
}

var hostServicePair = regexp.MustCompile(`([A-Za-z0-9:_.\\@-]+)\s*\(([^)]+)\)`)

func ParseHostServiceMessage(msg string) []HostServiceRow {
	msg = strings.TrimSpace(msg)
	if msg == "" || !strings.Contains(strings.ToLower(msg), "watched services") {
		return nil
	}
	matches := hostServicePair.FindAllStringSubmatch(msg, -1)
	if len(matches) == 0 {
		return nil
	}
	var out []HostServiceRow
	seen := map[string]bool{}
	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		status := strings.TrimSpace(m[2])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, HostServiceRow{Name: name, Status: status})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func FormatHostServiceMessage(rows []HostServiceRow, down bool) string {
	if len(rows) == 0 {
		return ""
	}
	prefix := "Watched services"
	if down {
		prefix = "Watched services not active"
	}
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		parts = append(parts, r.Name+" ("+r.Status+")")
	}
	return prefix + ": " + strings.Join(parts, ", ")
}

func hostServiceRows(h *models.Host) []HostServiceRow {
	byName := map[string]models.HostServiceStatus{}
	for _, st := range h.ServiceStatus {
		byName[st.Name] = st
	}
	rows := make([]HostServiceRow, 0, len(h.Services))
	for _, name := range h.Services {
		st, ok := byName[name]
		status := "missing"
		if ok && strings.TrimSpace(st.Active) != "" {
			status = strings.TrimSpace(st.Active)
		}
		rows = append(rows, HostServiceRow{Name: name, Status: status})
	}
	return rows
}

func hostServiceCollectionUnusable(rows []HostServiceRow) bool {
	if len(rows) == 0 {
		return true
	}
	usable := 0
	for _, r := range rows {
		if r.Status != "missing" && r.Status != "unknown" {
			usable++
		}
	}
	return usable == 0
}

func hostServicesDown(rows []HostServiceRow) bool {
	for _, r := range rows {
		if r.Status == "unknown" {
			continue
		}
		if !models.ServiceIsHealthy(r.Status) {
			return true
		}
	}
	return false
}
