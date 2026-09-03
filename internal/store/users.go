package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/config"
	"github.com/sentinel-monitoring/sentinel/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func (s *Store) EnsureDefaultAdmin(fallback config.AuthConfig) error {
	count, err := s.countUsers()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	auth, _ := s.GetAuthConfig(fallback)
	username := strings.TrimSpace(auth.Username)
	password := auth.Password
	if username == "" {
		username = fallback.Username
	}
	if password == "" {
		password = fallback.Password
	}
	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "changeme"
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.db.Exec(`
		INSERT INTO users (id, username, name, email, password_hash, mfa_enabled, role, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
		newID(), username, "", "", hash, 0, models.RoleAdmin, formatTime(now), formatTime(now),
	)
	return err
}

func (s *Store) countUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ValidatePassword enforces a minimum strength policy for new passwords.
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return fmt.Errorf("password must include at least one letter and one number")
	}
	return nil
}

func (s *Store) ListUsers() ([]models.User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, name, email, password_hash, mfa_enabled, role, tenant_id, created_at, updated_at
		FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) ListUsersByTenant(tenantID string) ([]models.User, error) {
	if tenantID == "" {
		return []models.User{}, nil
	}
	rows, err := s.db.Query(`
		SELECT id, username, name, email, password_hash, mfa_enabled, role, tenant_id, created_at, updated_at
		FROM users WHERE tenant_id = ? ORDER BY created_at ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) GetUserByID(id string) (*models.User, error) {
	row := s.db.QueryRow(`
		SELECT id, username, name, email, password_hash, mfa_enabled, role, tenant_id, created_at, updated_at
		FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByUsername(username string) (*models.User, error) {
	row := s.db.QueryRow(`
		SELECT id, username, name, email, password_hash, mfa_enabled, role, tenant_id, created_at, updated_at
		FROM users WHERE username = ? COLLATE NOCASE`, strings.TrimSpace(username))
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByEmail(email string) (*models.User, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil
	}
	row := s.db.QueryRow(`
		SELECT id, username, name, email, password_hash, mfa_enabled, role, tenant_id, created_at, updated_at
		FROM users WHERE email = ? COLLATE NOCASE`, email)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateUser(username, email, password string, role models.UserRole, tenantID string) (*models.User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	tenantID = strings.TrimSpace(tenantID)
	if username == "" {
		return nil, fmt.Errorf("username required")
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	if role != models.RoleAdmin && role != models.RoleViewer {
		return nil, fmt.Errorf("invalid role")
	}
	if tenantID != "" {
		c, err := s.GetCustomer(tenantID)
		if err != nil {
			return nil, err
		}
		if c == nil {
			return nil, fmt.Errorf("customer not found")
		}
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	id := newID()
	_, err = s.db.Exec(`
		INSERT INTO users (id, username, name, email, password_hash, mfa_enabled, role, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, username, "", email, hash, 0, role, nullString(tenantID), formatTime(now), formatTime(now),
	)
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(id)
}

func (s *Store) UpdateUser(id, username, email string, role models.UserRole, password, tenantID string, updateTenant bool) (*models.User, error) {
	existing, err := s.GetUserByID(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("user not found")
	}

	if username != "" {
		existing.Username = strings.TrimSpace(username)
	}
	existing.Email = strings.TrimSpace(email)
	if existing.MFAEnabled && existing.Email == "" {
		return nil, fmt.Errorf("email required while MFA is enabled")
	}
	if role != "" {
		if role != models.RoleAdmin && role != models.RoleViewer {
			return nil, fmt.Errorf("invalid role")
		}
		existing.Role = role
	}
	if password != "" {
		if err := ValidatePassword(password); err != nil {
			return nil, err
		}
		hash, err := HashPassword(password)
		if err != nil {
			return nil, err
		}
		existing.PasswordHash = hash
	}
	if updateTenant {
		tenantID = strings.TrimSpace(tenantID)
		if tenantID != "" {
			c, err := s.GetCustomer(tenantID)
			if err != nil {
				return nil, err
			}
			if c == nil {
				return nil, fmt.Errorf("customer not found")
			}
		}
		existing.TenantID = tenantID
	}

	now := time.Now().UTC()
	_, err = s.db.Exec(`
		UPDATE users SET username = ?, name = ?, email = ?, password_hash = ?, mfa_enabled = ?, role = ?, tenant_id = ?, updated_at = ?
		WHERE id = ?`,
		existing.Username, existing.Name, existing.Email, existing.PasswordHash, boolToInt(existing.MFAEnabled), existing.Role, nullString(existing.TenantID), formatTime(now), id,
	)
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(id)
}

func (s *Store) UpdateOwnProfile(id, username, name, email, password string, mfaEnabled *bool) (*models.User, error) {
	existing, err := s.GetUserByID(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("user not found")
	}

	username = strings.TrimSpace(username)
	if username == "" {
		username = existing.Username
	}
	existing.Username = username
	existing.Name = strings.TrimSpace(name)
	existing.Email = strings.TrimSpace(email)
	if mfaEnabled != nil {
		existing.MFAEnabled = *mfaEnabled
	}
	if existing.MFAEnabled && existing.Email == "" {
		return nil, fmt.Errorf("email required while MFA is enabled")
	}

	if password != "" {
		if err := ValidatePassword(password); err != nil {
			return nil, err
		}
		hash, err := HashPassword(password)
		if err != nil {
			return nil, err
		}
		existing.PasswordHash = hash
	}

	now := time.Now().UTC()
	_, err = s.db.Exec(`
		UPDATE users SET username = ?, name = ?, email = ?, password_hash = ?, mfa_enabled = ?, updated_at = ?
		WHERE id = ?`,
		existing.Username, existing.Name, existing.Email, existing.PasswordHash, boolToInt(existing.MFAEnabled), formatTime(now), id,
	)
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(id)
}

func (s *Store) DeleteUser(id string) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ?`, models.RoleAdmin).Scan(&n)
	return n, err
}

// CountPlatformAdmins counts admins with no tenant (platform-wide).
func (s *Store) CountPlatformAdmins() (int, error) {
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM users
		WHERE role = ? AND (tenant_id IS NULL OR tenant_id = '')`, models.RoleAdmin).Scan(&n)
	return n, err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (models.User, error) {
	var u models.User
	var createdAt, updatedAt string
	var mfaEnabled int
	var tenantID sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &mfaEnabled, &u.Role, &tenantID, &createdAt, &updatedAt); err != nil {
		return u, err
	}
	u.MFAEnabled = intToBool(mfaEnabled)
	u.TenantID = nullableString(tenantID)
	var err error
	u.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return u, err
	}
	u.UpdatedAt, err = parseTime(updatedAt)
	return u, err
}
