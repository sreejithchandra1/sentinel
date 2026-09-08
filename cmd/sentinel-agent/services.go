package main

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func collectServices(names []string) []models.HostServiceStatus {
	names = models.NormalizeWatchedServices(names)
	if len(names) == 0 {
		return nil
	}
	out := make([]models.HostServiceStatus, 0, len(names))
	for _, name := range names {
		out = append(out, serviceStatus(name))
	}
	return out
}

func serviceStatus(name string) models.HostServiceStatus {
	unit := models.ServiceUnitName(name)
	st := models.HostServiceStatus{Name: name, Active: "unknown"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "show", unit, "--property=ActiveState", "--property=SubState", "--no-page")
	raw, err := cmd.Output()
	if err != nil {
		return st
	}
	for _, line := range strings.Split(string(raw), "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "ActiveState":
			st.Active = val
		case "SubState":
			st.Sub = val
		}
	}
	return st
}
