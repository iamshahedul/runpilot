package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrDecryptionFailed = errors.New("failed to decrypt ciphertext")
	ErrReplayDetected   = errors.New("request timestamp out of allowed window (possible replay)")
)

// HashPassword hashes a plaintext password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

// CheckPasswordHash compares a hashed password with a plaintext candidate
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// EncryptSecret encrypts plaintext using AES-256-GCM
func EncryptSecret(masterKey []byte, plaintext string) (string, error) {
	if len(masterKey) != 32 {
		// derive a 32-byte key using sha256 if needed
		hash := sha256.Sum256(masterKey)
		masterKey = hash[:]
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// DecryptSecret decrypts hex-encoded AES-256-GCM ciphertext
func DecryptSecret(masterKey []byte, hexCiphertext string) (string, error) {
	if len(masterKey) != 32 {
		hash := sha256.Sum256(masterKey)
		masterKey = hash[:]
	}

	data, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrDecryptionFailed
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// SignAgentRequest creates an HMAC-SHA256 signature containing a timestamp to prevent replay
func SignAgentRequest(secret []byte, body []byte) (timestamp string, signature string) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(ts))
	h.Write([]byte(":"))
	h.Write(body)
	sig := hex.EncodeToString(h.Sum(nil))
	return ts, sig
}

// VerifyAgentRequest verifies the HMAC signature and checks timestamp within a 60-second window
func VerifyAgentRequest(secret []byte, body []byte, tsStr string, sigStr string) error {
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return errors.New("invalid timestamp header")
	}

	now := time.Now().Unix()
	if now-ts > 60 || ts-now > 60 {
		return ErrReplayDetected
	}

	h := hmac.New(sha256.New, secret)
	h.Write([]byte(tsStr))
	h.Write([]byte(":"))
	h.Write(body)
	expectedSig := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(expectedSig), []byte(sigStr)) {
		return errors.New("invalid agent authorization signature")
	}

	return nil
}
