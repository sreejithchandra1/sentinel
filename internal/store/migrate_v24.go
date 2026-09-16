package store

func (s *Store) migrateV24() error {
	cols := []struct {
		table, column, ddl string
	}{
		{"hosts", "os_version", "ALTER TABLE hosts ADD COLUMN os_version TEXT NOT NULL DEFAULT ''"},
		{"hosts", "kernel_version", "ALTER TABLE hosts ADD COLUMN kernel_version TEXT NOT NULL DEFAULT ''"},
		{"hosts", "num_cpu", "ALTER TABLE hosts ADD COLUMN num_cpu INTEGER NOT NULL DEFAULT 0"},
		{"hosts", "reboot_required", "ALTER TABLE hosts ADD COLUMN reboot_required INTEGER NOT NULL DEFAULT 0"},
		{"hosts", "collect_swap", "ALTER TABLE hosts ADD COLUMN collect_swap INTEGER NOT NULL DEFAULT 1"},
		{"hosts", "collect_iowait", "ALTER TABLE hosts ADD COLUMN collect_iowait INTEGER NOT NULL DEFAULT 1"},
		{"hosts", "collect_security", "ALTER TABLE hosts ADD COLUMN collect_security INTEGER NOT NULL DEFAULT 1"},
		{"hosts", "collect_services", "ALTER TABLE hosts ADD COLUMN collect_services INTEGER NOT NULL DEFAULT 1"},
		{"hosts", "alert_cpu_warning", "ALTER TABLE hosts ADD COLUMN alert_cpu_warning REAL NOT NULL DEFAULT 80"},
		{"hosts", "alert_memory_warning", "ALTER TABLE hosts ADD COLUMN alert_memory_warning REAL NOT NULL DEFAULT 80"},
		{"hosts", "alert_disk_warning", "ALTER TABLE hosts ADD COLUMN alert_disk_warning REAL NOT NULL DEFAULT 80"},
		{"hosts", "alert_load_warning", "ALTER TABLE hosts ADD COLUMN alert_load_warning REAL NOT NULL DEFAULT 80"},
		{"hosts", "alert_swap_enabled", "ALTER TABLE hosts ADD COLUMN alert_swap_enabled INTEGER NOT NULL DEFAULT 0"},
		{"hosts", "alert_swap_warning", "ALTER TABLE hosts ADD COLUMN alert_swap_warning REAL NOT NULL DEFAULT 80"},
		{"hosts", "alert_swap_threshold", "ALTER TABLE hosts ADD COLUMN alert_swap_threshold REAL NOT NULL DEFAULT 90"},
		{"hosts", "alert_swap_after", "ALTER TABLE hosts ADD COLUMN alert_swap_after INTEGER NOT NULL DEFAULT 10"},
		{"hosts", "alert_iowait_enabled", "ALTER TABLE hosts ADD COLUMN alert_iowait_enabled INTEGER NOT NULL DEFAULT 0"},
		{"hosts", "alert_iowait_warning", "ALTER TABLE hosts ADD COLUMN alert_iowait_warning REAL NOT NULL DEFAULT 80"},
		{"hosts", "alert_iowait_threshold", "ALTER TABLE hosts ADD COLUMN alert_iowait_threshold REAL NOT NULL DEFAULT 90"},
		{"hosts", "alert_iowait_after", "ALTER TABLE hosts ADD COLUMN alert_iowait_after INTEGER NOT NULL DEFAULT 10"},
		{"hosts", "alert_auth_enabled", "ALTER TABLE hosts ADD COLUMN alert_auth_enabled INTEGER NOT NULL DEFAULT 0"},
		{"hosts", "alert_auth_threshold", "ALTER TABLE hosts ADD COLUMN alert_auth_threshold INTEGER NOT NULL DEFAULT 50"},
		{"hosts", "alert_root_login_enabled", "ALTER TABLE hosts ADD COLUMN alert_root_login_enabled INTEGER NOT NULL DEFAULT 0"},
		{"hosts", "alert_reboot_enabled", "ALTER TABLE hosts ADD COLUMN alert_reboot_enabled INTEGER NOT NULL DEFAULT 0"},
		{"hosts", "alert_service_enabled", "ALTER TABLE hosts ADD COLUMN alert_service_enabled INTEGER NOT NULL DEFAULT 1"},
		{"hosts", "services_json", "ALTER TABLE hosts ADD COLUMN services_json TEXT NOT NULL DEFAULT '[]'"},
		{"hosts", "security_json", "ALTER TABLE hosts ADD COLUMN security_json TEXT NOT NULL DEFAULT '{}'"},
		{"hosts", "disks_latest_json", "ALTER TABLE hosts ADD COLUMN disks_latest_json TEXT NOT NULL DEFAULT '[]'"},
		{"hosts", "services_latest_json", "ALTER TABLE hosts ADD COLUMN services_latest_json TEXT NOT NULL DEFAULT '[]'"},
		{"host_samples", "swap_percent", "ALTER TABLE host_samples ADD COLUMN swap_percent REAL"},
		{"host_samples", "iowait_percent", "ALTER TABLE host_samples ADD COLUMN iowait_percent REAL"},
	}
	for _, c := range cols {
		if err := s.addColumnIfMissing(c.table, c.column, c.ddl); err != nil {
			return err
		}
	}
	return nil
}
