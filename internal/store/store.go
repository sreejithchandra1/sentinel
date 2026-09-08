package store

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations.sql
var migrationsFS embed.FS

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	data, err := migrationsFS.ReadFile("migrations.sql")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	if _, err := s.db.Exec(string(data)); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	if err := s.migrateV2(); err != nil {
		return fmt.Errorf("run v2 migrations: %w", err)
	}
	if err := s.migrateV3(); err != nil {
		return fmt.Errorf("run v3 migrations: %w", err)
	}
	if err := s.migrateV4(); err != nil {
		return fmt.Errorf("run v4 migrations: %w", err)
	}
	if err := s.migrateV5(); err != nil {
		return fmt.Errorf("run v5 migrations: %w", err)
	}
	if err := s.migrateV6(); err != nil {
		return fmt.Errorf("run v6 migrations: %w", err)
	}
	if err := s.migrateV7(); err != nil {
		return fmt.Errorf("run v7 migrations: %w", err)
	}
	if err := s.migrateV8(); err != nil {
		return fmt.Errorf("run v8 migrations: %w", err)
	}
	if err := s.migrateV9(); err != nil {
		return fmt.Errorf("run v9 migrations: %w", err)
	}
	if err := s.migrateV10(); err != nil {
		return fmt.Errorf("run v10 migrations: %w", err)
	}
	if err := s.migrateV11(); err != nil {
		return fmt.Errorf("run v11 migrations: %w", err)
	}
	if err := s.migrateV12(); err != nil {
		return fmt.Errorf("run v12 migrations: %w", err)
	}
	if err := s.migrateV13(); err != nil {
		return fmt.Errorf("run v13 migrations: %w", err)
	}
	if err := s.migrateV14(); err != nil {
		return fmt.Errorf("run v14 migrations: %w", err)
	}
	if err := s.migrateV15(); err != nil {
		return fmt.Errorf("run v15 migrations: %w", err)
	}
	if err := s.migrateV16(); err != nil {
		return fmt.Errorf("run v16 migrations: %w", err)
	}
	if err := s.migrateV17(); err != nil {
		return fmt.Errorf("run v17 migrations: %w", err)
	}
	if err := s.migrateV18(); err != nil {
		return fmt.Errorf("run v18 migrations: %w", err)
	}
	if err := s.migrateV19(); err != nil {
		return fmt.Errorf("run v19 migrations: %w", err)
	}
	if err := s.migrateV20(); err != nil {
		return fmt.Errorf("run v20 migrations: %w", err)
	}
	if err := s.migrateV21(); err != nil {
		return fmt.Errorf("run v21 migrations: %w", err)
	}
	if err := s.migrateV22(); err != nil {
		return fmt.Errorf("run v22 migrations: %w", err)
	}
	if err := s.migrateV23(); err != nil {
		return fmt.Errorf("run v23 migrations: %w", err)
	}
	return nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func sqlPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

func nullableTime(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := parseTime(s.String)
	if err != nil {
		return nil
	}
	return &t
}

func nullableInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func nullableString(s sql.NullString) string {
	if !s.Valid {
		return ""
	}
	return s.String
}

func newID() string {
	return uuid.New().String()
}
