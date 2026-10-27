package auth

import (
	"context"
	"encoding/json"
	"errors"
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

// Authenticate verifies email and password, returning user and JWT token
func (s *AuthService) Authenticate(email, password string) (*models.User, string, error) {
	var user models.User
	var isActive int
	query := `SELECT id, email, password_hash, full_name, role, is_active, created_at, updated_at FROM users WHERE email = ?`
	err := s.db.QueryRow(query, strings.ToLower(strings.TrimSpace(email))).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.Role,
		&isActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, "", errors.New("invalid email or password")
	}

	user.IsActive = (isActive == 1)
	if !user.IsActive {
		return nil, "", errors.New("user account is disabled")
	}

	if !crypto.CheckPasswordHash(password, user.PasswordHash) {
		return nil, "", errors.New("invalid email or password")
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
