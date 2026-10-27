package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"runpilot/control-plane/internal/config"
	"runpilot/control-plane/internal/db"
	"runpilot/control-plane/internal/models"
	"runpilot/shared/crypto"
)

type contextKey string

const (
	UserContextKey contextKey = "runpilot_user"
)

type AuthService struct {
	db  *db.DB
	cfg *config.Config
}

func NewAuthService(database *db.DB, cfg *config.Config) *AuthService {
	return &AuthService{db: database, cfg: cfg}
}

// Authenticate verifies email or username and password, returning user and JWT token
func (s *AuthService) Authenticate(identifier, password string) (*models.User, string, error) {
	var user models.User
	var isActive int
	var username sql.NullString
	ident := strings.ToLower(strings.TrimSpace(identifier))
	query := `
		SELECT id, email, username, password_hash, full_name, role, is_active, created_at, updated_at
		FROM users
		WHERE lower(email) = ? OR lower(COALESCE(username, '')) = ?
		LIMIT 1
	`
	err := s.db.QueryRow(query, ident, ident).Scan(
		&user.ID,
		&user.Email,
		&username,
		&user.PasswordHash,
		&user.FullName,
		&user.Role,
		&isActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, "", errors.New("invalid email/username or password")
	}

	if username.Valid {
		user.Username = username.String
	}
	user.IsActive = (isActive == 1)
	if !user.IsActive {
		return nil, "", errors.New("user account is disabled")
	}

	if !crypto.CheckPasswordHash(password, user.PasswordHash) {
		return nil, "", errors.New("invalid email/username or password")
	}

	claims := crypto.JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}

	token, err := crypto.GenerateJWT([]byte(s.cfg.Server.JWTSecret), claims, 24*time.Hour)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}

// CreateOrUpdateUser provisions or updates a client/user account
func (s *AuthService) CreateOrUpdateUser(username, email, password, fullName, role string) (*models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	username = strings.ToLower(strings.TrimSpace(username))
	if email == "" && username == "" {
		return nil, errors.New("username or email is required")
	}
	if email == "" {
		email = username + "@runpilot.local"
	}
	if username == "" {
		parts := strings.Split(email, "@")
		username = parts[0]
	}
	if fullName == "" {
		fullName = username
	}
	if role == "" {
		role = "user"
	}

	// Check if user already exists by email or username
	var existingID int64
	err := s.db.QueryRow(`SELECT id FROM users WHERE lower(email) = ? OR lower(COALESCE(username, '')) = ? LIMIT 1`, email, username).Scan(&existingID)
	if err == nil && existingID > 0 {
		// Update password if provided
		if password != "" {
			hash, err := crypto.HashPassword(password)
			if err != nil {
				return nil, err
			}
			_, _ = s.db.Exec(`UPDATE users SET password_hash = ?, full_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, hash, fullName, existingID)
		}
		return s.GetUserByID(existingID)
	}

	// Insert new user
	if password == "" {
		password = "RunPilot2026!"
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return nil, err
	}

	res, err := s.db.Exec(`
		INSERT INTO users (username, email, password_hash, full_name, role, is_active)
		VALUES (?, ?, ?, ?, ?, 1)
	`, username, email, hash, fullName, role)
	if err != nil {
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}
	newID, _ := res.LastInsertId()
	return s.GetUserByID(newID)
}

// UpdateProfile updates email, username and full name for a user
func (s *AuthService) UpdateProfile(userID int64, username, email, fullName string) (*models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	username = strings.ToLower(strings.TrimSpace(username))
	if email == "" {
		return nil, errors.New("email cannot be empty")
	}

	if username != "" {
		_, err := s.db.Exec(`UPDATE users SET email = ?, username = ?, full_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, email, username, fullName, userID)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := s.db.Exec(`UPDATE users SET email = ?, full_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, email, fullName, userID)
		if err != nil {
			return nil, err
		}
	}
	return s.GetUserByID(userID)
}

// ListUsers retrieves all users for admin management
func (s *AuthService) ListUsers() ([]models.User, error) {
	rows, err := s.db.Query(`SELECT id, email, username, full_name, role, is_active, created_at, updated_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		var isActive int
		var username sql.NullString
		if err := rows.Scan(&u.ID, &u.Email, &username, &u.FullName, &u.Role, &isActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if username.Valid {
			u.Username = username.String
		}
		u.IsActive = (isActive == 1)
		users = append(users, u)
	}
	return users, nil
}

// ChangePassword allows a user to change their password by validating old password
func (s *AuthService) ChangePassword(userID int64, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("password must be at least 6 characters")
	}

	var hash string
	err := s.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash)
	if err != nil {
		return errors.New("user not found")
	}

	if !crypto.CheckPasswordHash(oldPassword, hash) {
		return errors.New("current password is incorrect")
	}

	newHash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newHash, userID)
	return err
}

// ResetPasswordDirectly allows admin or CLI to reset a user's password directly
func (s *AuthService) ResetPasswordDirectly(identifier, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("password must be at least 6 characters")
	}

	ident := strings.ToLower(strings.TrimSpace(identifier))
	newHash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}

	res, err := s.db.Exec(`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE lower(email) = ? OR lower(COALESCE(username, '')) = ?`, newHash, ident, ident)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no user found matching '%s'", identifier)
	}
	return nil
}

// GetUserByID loads user by ID
func (s *AuthService) GetUserByID(userID int64) (*models.User, error) {
	var user models.User
	var isActive int
	var username sql.NullString
	query := `SELECT id, email, username, full_name, role, is_active, created_at, updated_at FROM users WHERE id = ?`
	err := s.db.QueryRow(query, userID).Scan(
		&user.ID,
		&user.Email,
		&username,
		&user.FullName,
		&user.Role,
		&isActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if username.Valid {
		user.Username = username.String
	}
	user.IsActive = (isActive == 1)
	return &user, nil
}

// GetUserByIdentifier loads user by username or email
func (s *AuthService) GetUserByIdentifier(identifier string) (*models.User, error) {
	var user models.User
	var isActive int
	var username sql.NullString
	ident := strings.ToLower(strings.TrimSpace(identifier))
	query := `SELECT id, email, username, full_name, role, is_active, created_at, updated_at FROM users WHERE lower(email) = ? OR lower(COALESCE(username, '')) = ? LIMIT 1`
	err := s.db.QueryRow(query, ident, ident).Scan(
		&user.ID,
		&user.Email,
		&username,
		&user.FullName,
		&user.Role,
		&isActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if username.Valid {
		user.Username = username.String
	}
	user.IsActive = (isActive == 1)
	return &user, nil
}

// GenerateTokenForUser creates a signed JWT token for any user ID and role
func (s *AuthService) GenerateTokenForUser(userID int64, email, role string) (string, error) {
	claims := crypto.JWTClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
	}
	return crypto.GenerateJWT([]byte(s.cfg.Server.JWTSecret), claims, 24*time.Hour)
}


// Middleware verifies Authorization Bearer token or cookie and injects User into context
func (s *AuthService) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		var tokenStr string

		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			// Check cookie
			if cookie, err := r.Cookie("runpilot_token"); err == nil {
				tokenStr = cookie.Value
			}
		}

		if tokenStr == "" {
			http.Error(w, `{"error":"missing authorization token"}`, http.StatusUnauthorized)
			return
		}

		claims, err := crypto.VerifyJWT([]byte(s.cfg.Server.JWTSecret), tokenStr)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromContext extracts JWT claims from context
func GetUserFromContext(ctx context.Context) (*crypto.JWTClaims, bool) {
	claims, ok := ctx.Value(UserContextKey).(*crypto.JWTClaims)
	return claims, ok
}

// RequireAdmin middleware ensures caller has admin role
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetUserFromContext(r.Context())
		if !ok || claims.Role != "admin" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden: administrator privileges required"})
			return
		}
		next(w, r)
	}
}
