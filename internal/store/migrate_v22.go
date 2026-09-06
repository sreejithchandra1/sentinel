package store

func (s *Store) migrateV22() error {
	if err := s.addColumnIfMissing("performance_targets", "http_username",
		"ALTER TABLE performance_targets ADD COLUMN http_username TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("performance_targets", "http_password",
		"ALTER TABLE performance_targets ADD COLUMN http_password TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		UPDATE performance_targets
		SET http_username = (
			SELECT m.http_username FROM monitors m
			WHERE m.type = 'http'
			  AND m.url = performance_targets.url
			  AND COALESCE(m.tenant_id, '') = COALESCE(performance_targets.tenant_id, '')
			  AND m.http_username != ''
			ORDER BY m.updated_at DESC
			LIMIT 1
		),
		http_password = (
			SELECT m.http_password FROM monitors m
			WHERE m.type = 'http'
			  AND m.url = performance_targets.url
			  AND COALESCE(m.tenant_id, '') = COALESCE(performance_targets.tenant_id, '')
			  AND m.http_username != ''
			ORDER BY m.updated_at DESC
			LIMIT 1
		)
		WHERE (http_username = '' OR http_password = '')
		  AND EXISTS (
			SELECT 1 FROM monitors m
			WHERE m.type = 'http'
			  AND m.url = performance_targets.url
			  AND COALESCE(m.tenant_id, '') = COALESCE(performance_targets.tenant_id, '')
			  AND m.http_username != ''
		  )`)
	return err
}
