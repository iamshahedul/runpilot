package unit

import (
	"strconv"
	"testing"
	"time"

	"runpilot/shared/crypto"
)

func TestPasswordHashing(t *testing.T) {
	pass := "RunPilotSecret2026!"
	hash, err := crypto.HashPassword(pass)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if !crypto.CheckPasswordHash(pass, hash) {
		t.Errorf("Password hash check failed for valid password")
	}

	if crypto.CheckPasswordHash("wrong-pass", hash) {
		t.Errorf("Password hash check succeeded for invalid password")
	}
}

func TestJWTGenerationAndVerification(t *testing.T) {
	secret := []byte("a-very-secure-32-byte-secret-key!")
	claims := crypto.JWTClaims{
		UserID: 42,
		Email:  "developer@runpilot.org",
		Role:   "admin",
	}

	token, err := crypto.GenerateJWT(secret, claims, 2*time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	verified, err := crypto.VerifyJWT(secret, token)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}

	if verified.UserID != 42 || verified.Email != "developer@runpilot.org" || verified.Role != "admin" {
		t.Errorf("Claims mismatch: got %+v", verified)
	}

	// Test invalid secret
	_, err = crypto.VerifyJWT([]byte("wrong-secret-key-1234567890123456"), token)
	if err == nil {
		t.Errorf("Expected signature error with wrong secret")
	}
}

func TestAES256GCMEncryption(t *testing.T) {
	masterKey := []byte("master-encryption-key-for-runpilot")
	originalSecret := "DATABASE_PASSWORD=SuperSecretProductionValue!"

	encrypted, err := crypto.EncryptSecret(masterKey, originalSecret)
	if err != nil {
		t.Fatalf("EncryptSecret failed: %v", err)
	}

	decrypted, err := crypto.DecryptSecret(masterKey, encrypted)
	if err != nil {
		t.Fatalf("DecryptSecret failed: %v", err)
	}

	if decrypted != originalSecret {
		t.Errorf("Decrypted text does not match original: got %q, want %q", decrypted, originalSecret)
	}
}

func TestAgentHMACSignature(t *testing.T) {
	secret := []byte("agent-shared-secret-token")
	payload := []byte(`{"app_id":"app-1","action":"restart"}`)

	ts, sig := crypto.SignAgentRequest(secret, payload)

	err := crypto.VerifyAgentRequest(secret, payload, ts, sig)
	if err != nil {
		t.Errorf("Valid agent signature verification failed: %v", err)
	}

	// Tampered payload
	err = crypto.VerifyAgentRequest(secret, []byte(`{"app_id":"app-2"}`), ts, sig)
	if err == nil {
		t.Errorf("Expected signature verification failure on tampered payload")
	}

	// Replay attack simulation (timestamp older than 60 seconds)
	oldTs := strconv.FormatInt(time.Now().Unix()-120, 10)
	err = crypto.VerifyAgentRequest(secret, payload, oldTs, sig)
	if err == nil {
		t.Errorf("Expected replay detection error on expired timestamp")
	}
}
