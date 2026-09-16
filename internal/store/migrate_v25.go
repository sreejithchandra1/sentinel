package store

func (s *Store) migrateV25() error {
	return s.addColumnIfMissing("hosts", "uptime_seconds",
		"ALTER TABLE hosts ADD COLUMN uptime_seconds INTEGER NOT NULL DEFAULT 0")
}
