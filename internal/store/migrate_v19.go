package store

// migrateV19 drops incidents.monitor_id's FK to monitors so SLOW incidents can
// use a performance target id. Deletes are cleaned up in DeleteMonitor / DeletePerformanceTarget.
func (s *Store) migrateV19() error {
	rows, err := s.db.Query("PRAGMA foreign_key_list(incidents)")
	if err != nil {
		return err
	}
	hasFK := rows.Next()
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if !hasFK {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		CREATE TABLE incidents_v19 (
			id TEXT PRIMARY KEY,
			monitor_id TEXT NOT NULL,
			type TEXT NOT NULL,
			message TEXT,
			started_at TEXT NOT NULL,
			resolved_at TEXT,
			acknowledged_at TEXT,
			acknowledged_by TEXT
		)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO incidents_v19 (id, monitor_id, type, message, started_at, resolved_at, acknowledged_at, acknowledged_by)
		SELECT id, monitor_id, type, message, started_at, resolved_at, acknowledged_at, acknowledged_by FROM incidents`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE incidents`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE incidents_v19 RENAME TO incidents`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_incidents_monitor ON incidents(monitor_id, started_at DESC)`); err != nil {
		return err
	}
	return tx.Commit()
}
