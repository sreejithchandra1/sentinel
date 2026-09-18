package store

func (s *Store) migrateV26() error {
	return s.addColumnIfMissing("incidents", "details",
		"ALTER TABLE incidents ADD COLUMN details TEXT")
}
