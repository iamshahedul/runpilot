package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid or malformed jwt token")
	ErrTokenExpired = errors.New("jwt token has expired")
	ErrSigMismatch  = errors.New("jwt token signature mismatch")
)

type JWTClaims struct {
	UserID    int64  `json:"uid"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	TeamID    *int64 `json:"team_id,omitempty"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

// GenerateJWT creates a signed HS256 JWT
func GenerateJWT(secret []byte, claims JWTClaims, duration time.Duration) (string, error) {
	now := time.Now()
	claims.IssuedAt = now.Unix()
	claims.ExpiresAt = now.Add(duration).Unix()

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	b64Header := base64.RawURLEncoding.EncodeToString(headerJSON)
	b64Claims := base64.RawURLEncoding.EncodeToString(claimsJSON)

	payload := fmt.Sprintf("%s.%s", b64Header, b64Claims)

	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	signature := h.Sum(nil)
	b64Sig := base64.RawURLEncoding.EncodeToString(signature)

	return fmt.Sprintf("%s.%s", payload, b64Sig), nil
}

// VerifyJWT validates the token signature and expiration
func VerifyJWT(secret []byte, tokenStr string) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	headerB64, claimsB64, sigB64 := parts[0], parts[1], parts[2]
	payload := fmt.Sprintf("%s.%s", headerB64, claimsB64)

	// Verify signature
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	expectedSig := h.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, ErrSigMismatch
	}

	// Decode claims
	claimsData, err := base64.RawURLEncoding.DecodeString(claimsB64)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsData, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	// Check expiration
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}
