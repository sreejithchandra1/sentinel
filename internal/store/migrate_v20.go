package store

func (s *Store) migrateV20() error {
	return s.addColumnIfMissing("customers", "alert_emails",
		"ALTER TABLE customers ADD COLUMN alert_emails TEXT NOT NULL DEFAULT ''")
}
