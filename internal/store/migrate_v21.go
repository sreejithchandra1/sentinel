package store

func (s *Store) migrateV21() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS email_log (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			kind TEXT NOT NULL,
			to_addr TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			monitor_id TEXT,
			monitor_name TEXT NOT NULL DEFAULT '',
			tenant_id TEXT,
			created_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_email_log_created ON email_log(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_email_log_status ON email_log(status, created_at DESC);
	`)
	return err
}
