package store

func (s *Store) migrateV18() error {
	if err := s.addColumnIfMissing("incidents", "acknowledged_at", "ALTER TABLE incidents ADD COLUMN acknowledged_at TEXT"); err != nil {
		return err
	}
	return s.addColumnIfMissing("incidents", "acknowledged_by", "ALTER TABLE incidents ADD COLUMN acknowledged_by TEXT")
}
