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
		Hostname:       hostname(),
		NumCPU:         runtime.NumCPU(),
		OSVersion:      osPrettyName(),
		KernelVersion:  kernelRelease(),
		RebootRequired: rebootRequired(),
		UptimeSeconds:  uptimeSeconds(),
	}
	if cfg.CollectCPU || cfg.CollectIOWait {
		cpu, iowait, err := cpuAndIOWait()
		if err == nil {
			if cfg.CollectCPU {
				p.CPUPercent = &cpu
			}
			if cfg.CollectIOWait {
				p.IOWaitPercent = &iowait
			}
		}
	}
	if cfg.CollectMemory {
		if v, err := memPercent(); err == nil {
			p.MemPercent = &v
		}
	}
	if cfg.CollectSwap {
		if v, err := swapPercent(); err == nil {
			p.SwapPercent = &v
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
	if cfg.CollectSecurity {
		p.Security = collectSecurity(time.Now(), models.HostAuthFailWindow)
	}
	if cfg.CollectServices {
		p.Services = collectServices(cfg.Services)
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

func osPrettyName() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		}
	}
	return ""
}

func kernelRelease() string {
	raw, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func rebootRequired() bool {
	_, err := os.Stat("/run/reboot-required")
	if err == nil {
		return true
	}
	_, err = os.Stat("/var/run/reboot-required")
	return err == nil
}

func uptimeSeconds() int {
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0
	}
	n, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || n < 0 {
		return 0
	}
	return int(n)
}

func cpuAndIOWait() (cpuPct, ioWaitPct float64, err error) {
	idle1, wait1, total1, err := cpuTimes()
	if err != nil {
		return 0, 0, err
	}
	time.Sleep(200 * time.Millisecond)
	idle2, wait2, total2, err := cpuTimes()
	if err != nil {
		return 0, 0, err
	}
	total := total2 - total1
	if total <= 0 {
		return 0, 0, nil
	}
	cpuPct = (1 - (idle2-idle1)/total) * 100
	ioWaitPct = (wait2 - wait1) / total * 100
	return clampPct(cpuPct), clampPct(ioWaitPct), nil
}

func clampPct(pct float64) float64 {
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

func cpuTimes() (idle, iowait, total float64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, 0, sc.Err()
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, 0, os.ErrInvalid
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
	if len(values) > 4 {
		iowait = values[4]
	}
	return idle, iowait, total, nil
}

func memPercent() (float64, error) {
	vals, err := meminfo()
	if err != nil {
		return 0, err
	}
	total := vals["MemTotal"]
	available := vals["MemAvailable"]
	if total <= 0 {
		return 0, os.ErrInvalid
	}
	used := total - available
	if used < 0 {
		used = 0
	}
	return used / total * 100, nil
}

func swapPercent() (float64, error) {
	vals, err := meminfo()
	if err != nil {
		return 0, err
	}
	total := vals["SwapTotal"]
	if total <= 0 {
		return 0, nil
	}
	free := vals["SwapFree"]
	used := total - free
	if used < 0 {
		used = 0
	}
	return clampPct(used / total * 100), nil
}

func meminfo() (map[string]float64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]float64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		key, _, _ := strings.Cut(line, ":")
		out[key] = parseMemKB(line)
	}
	return out, sc.Err()
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
	"ramfs": true, "iso9660": true, "fuse.gvfsd-fuse": true, "fuse.portal": true,
}

var skipMountPrefix = []string{"/proc", "/sys", "/dev", "/run", "/snap", "/var/lib/docker", "/var/lib/containers"}

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
		if skipMount(mount) {
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
		pct := clampPct(used * 100)
		seen[mount] = true
		out = append(out, models.HostDisk{Mount: mount, Percent: pct})
		if len(out) >= models.MaxHostDiskMounts {
			break
		}
	}
	return out
}

func skipMount(mount string) bool {
	for _, p := range skipMountPrefix {
		if mount == p || strings.HasPrefix(mount, p+"/") {
			return true
		}
	}
	return false
}
