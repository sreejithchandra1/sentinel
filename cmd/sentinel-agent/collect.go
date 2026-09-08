package main

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func collectMetrics(cfg models.HostAgentConfig) (*models.HostIngestPayload, error) {
	p := &models.HostIngestPayload{
		Hostname: hostname(),
		NumCPU:   runtime.NumCPU(),
	}
	if cfg.CollectCPU {
		if v, err := cpuPercent(); err == nil {
			p.CPUPercent = &v
		}
	}
	if cfg.CollectMemory {
		if v, err := memPercent(); err == nil {
			p.MemPercent = &v
		}
	}
	if cfg.CollectLoad {
		l1, l5, l15, err := loadavg()
		if err == nil {
			p.Load1, p.Load5, p.Load15 = &l1, &l5, &l15
		}
	}
	if cfg.CollectDisk {
		p.Disks = disks()
	}
	return p, nil
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	if i := strings.IndexByte(h, '.'); i > 0 {
		return h[:i]
	}
	return h
}

func cpuPercent() (float64, error) {
	idle1, total1, err := cpuTimes()
	if err != nil {
		return 0, err
	}
	time.Sleep(200 * time.Millisecond)
	idle2, total2, err := cpuTimes()
	if err != nil {
		return 0, err
	}
	idle := idle2 - idle1
	total := total2 - total1
	if total <= 0 {
		return 0, nil
	}
	pct := (1 - idle/total) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct, nil
}

func cpuTimes() (idle, total float64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, sc.Err()
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, os.ErrInvalid
	}
	var values []float64
	for _, f := range fields[1:] {
		n, err := strconv.ParseFloat(f, 64)
		if err != nil {
			continue
		}
		values = append(values, n)
		total += n
	}
	if len(values) > 3 {
		idle = values[3]
	}
	return idle, total, nil
}

func memPercent() (float64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var total, available float64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseMemKB(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			available = parseMemKB(line)
		}
	}
	if total <= 0 {
		return 0, os.ErrInvalid
	}
	used := total - available
	if used < 0 {
		used = 0
	}
	return used / total * 100, sc.Err()
}

func parseMemKB(line string) float64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	n, _ := strconv.ParseFloat(fields[1], 64)
	return n
}

func loadavg() (l1, l5, l15 float64, err error) {
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 3 {
		return 0, 0, 0, os.ErrInvalid
	}
	l1, _ = strconv.ParseFloat(fields[0], 64)
	l5, _ = strconv.ParseFloat(fields[1], 64)
	l15, _ = strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15, nil
}

var skipFS = map[string]bool{
	"proc": true, "sysfs": true, "devtmpfs": true, "tmpfs": true, "overlay": true,
	"cgroup": true, "cgroup2": true, "squashfs": true, "autofs": true, "fusectl": true,
	"debugfs": true, "securityfs": true, "pstore": true, "bpf": true, "tracefs": true,
	"devpts": true, "mqueue": true, "hugetlbfs": true, "rpc_pipefs": true, "nsfs": true,
	"ramfs": true, "iso9660": true,
}

func disks() []models.HostDisk {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()
	seen := map[string]bool{}
	var out []models.HostDisk
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		mount, fstype := fields[1], fields[2]
		if skipFS[fstype] || seen[mount] {
			continue
		}
		if !strings.HasPrefix(mount, "/") {
			continue
		}
		var st syscall.Statfs_t
		if err := syscall.Statfs(mount, &st); err != nil || st.Blocks == 0 {
			continue
		}
		used := 1 - float64(st.Bavail)/float64(st.Blocks)
		if used < 0 {
			used = 0
		}
		pct := used * 100
		if pct > 100 {
			pct = 100
		}
		seen[mount] = true
		out = append(out, models.HostDisk{Mount: mount, Percent: pct})
	}
	return out
}
