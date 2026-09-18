package store

import "github.com/sentinel-monitoring/sentinel/internal/models"

func (s *Store) migrateV27() error {
	suffixes := []string{
		" (" + models.HTTPErrorSourceLabel(models.HTTPErrorSourceNginx) + ")",
		" (" + models.HTTPErrorSourceLabel(models.HTTPErrorSourceShopware) + ")",
		" (" + models.HTTPErrorSourceLabel(models.HTTPErrorSourcePHPFPM) + ")",
		" (" + models.HTTPErrorSourceLabel(models.HTTPErrorSourceCloudflare) + ")",
		" (" + models.HTTPErrorSourceLabel(models.HTTPErrorSourceUnknown) + ")",
	}
	for _, suffix := range suffixes {
		if _, err := s.db.Exec(
			`UPDATE incidents SET message = REPLACE(message, ?, '') WHERE message LIKE '%' || ?`,
			suffix, suffix,
		); err != nil {
			return err
		}
	}
	return nil
}
