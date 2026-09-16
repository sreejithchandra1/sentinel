package store

func (s *Store) migrateV23() error {
	var hosts int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='hosts'`).Scan(&hosts); err != nil {
		return err
	}
	if hosts > 0 {
		var hasStatus int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('hosts') WHERE name='status'`).Scan(&hasStatus); err != nil {
			return err
		}
		if hasStatus == 0 {
			if _, err := s.db.Exec(`
				DROP TABLE IF EXISTS host_incidents;
				DROP TABLE IF EXISTS host_samples;
				DROP TABLE IF EXISTS host_enroll_tokens;
				DROP TABLE IF EXISTS host_alert_state;
				DROP TABLE IF EXISTS hosts;
			`); err != nil {
				return err
			}
		}
	}

	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS hosts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			hostname TEXT NOT NULL DEFAULT '',
			tenant_id TEXT NOT NULL DEFAULT '',
			os TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT '',
			agent_version TEXT NOT NULL DEFAULT '',
			token_hash TEXT NOT NULL DEFAULT '',
			last_seen_at TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			enabled INTEGER NOT NULL DEFAULT 1,
			interval_seconds INTEGER NOT NULL DEFAULT 30,
			alert_after_failures INTEGER NOT NULL DEFAULT 2,
			consecutive_misses INTEGER NOT NULL DEFAULT 0,
			collect_cpu INTEGER NOT NULL DEFAULT 1,
			collect_memory INTEGER NOT NULL DEFAULT 1,
			collect_disk INTEGER NOT NULL DEFAULT 1,
			collect_load INTEGER NOT NULL DEFAULT 1,
			alert_cpu_enabled INTEGER NOT NULL DEFAULT 0,
			alert_cpu_threshold REAL NOT NULL DEFAULT 90,
			alert_cpu_after INTEGER NOT NULL DEFAULT 10,
			alert_memory_enabled INTEGER NOT NULL DEFAULT 0,
			alert_memory_threshold REAL NOT NULL DEFAULT 90,
			alert_memory_after INTEGER NOT NULL DEFAULT 10,
			alert_disk_enabled INTEGER NOT NULL DEFAULT 0,
			alert_disk_threshold REAL NOT NULL DEFAULT 90,
			alert_disk_after INTEGER NOT NULL DEFAULT 20,
			alert_load_enabled INTEGER NOT NULL DEFAULT 0,
			alert_load_threshold REAL NOT NULL DEFAULT 0,
			alert_load_after INTEGER NOT NULL DEFAULT 10,
			alert_emails TEXT NOT NULL DEFAULT '',
			notify_email INTEGER NOT NULL DEFAULT 1,
			notify_slack INTEGER NOT NULL DEFAULT 1,
			notify_webhooks INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_hosts_tenant ON hosts(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_hosts_status ON hosts(status);
		CREATE INDEX IF NOT EXISTS idx_hosts_token_hash ON hosts(token_hash);

		CREATE TABLE IF NOT EXISTS host_enroll_tokens (
			id TEXT PRIMARY KEY,
			host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			used_at TEXT,
			created_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_host_enroll_hash ON host_enroll_tokens(token_hash);
		CREATE INDEX IF NOT EXISTS idx_host_enroll_host ON host_enroll_tokens(host_id);

		CREATE TABLE IF NOT EXISTS host_samples (
			id TEXT PRIMARY KEY,
			host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
			cpu_percent REAL,
			mem_percent REAL,
			load1 REAL,
			load5 REAL,
			load15 REAL,
			disk_percent REAL,
			disks_json TEXT NOT NULL DEFAULT '[]',
			num_cpu INTEGER NOT NULL DEFAULT 0,
			collected_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_host_samples_host_time ON host_samples(host_id, collected_at DESC);

		CREATE TABLE IF NOT EXISTS host_alert_state (
			host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
			metric TEXT NOT NULL,
			consecutive_high INTEGER NOT NULL DEFAULT 0,
			consecutive_ok INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (host_id, metric)
		);
	`)
	return err
}
