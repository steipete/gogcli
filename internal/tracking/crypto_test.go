package tracking

import (
	"testing"
	"time"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	payload := &PixelPayload{
		Recipient:   "test@example.com",
		SubjectHash: "abc123",
		SentAt:      time.Now().Unix(),
	}

	encrypted, err := Encrypt(payload, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted.Recipient != payload.Recipient {
		t.Errorf("Recipient mismatch: got %q, want %q", decrypted.Recipient, payload.Recipient)
	}

	if decrypted.SubjectHash != payload.SubjectHash {
		t.Errorf("SubjectHash mismatch: got %q, want %q", decrypted.SubjectHash, payload.SubjectHash)
	}

	if decrypted.SentAt != payload.SentAt {
		t.Errorf("SentAt mismatch: got %d, want %d", decrypted.SentAt, payload.SentAt)
	}
}

func TestEncryptProducesURLSafeOutput(t *testing.T) {
	key, _ := GenerateKey()
	payload := &PixelPayload{
		Recipient:   "test@example.com",
		SubjectHash: "abc123",
		SentAt:      time.Now().Unix(),
	}

	encrypted, err := Encrypt(payload, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// URL-safe base64 should not contain +, /, or =
	for _, c := range encrypted {
		if c == '+' || c == '/' || c == '=' {
			t.Errorf("Output contains non-URL-safe character: %c", c)
		}
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()

	payload := &PixelPayload{
		Recipient:   "test@example.com",
		SubjectHash: "abc123",
		SentAt:      time.Now().Unix(),
	}

	encrypted, _ := Encrypt(payload, key1)

	_, err := Decrypt(encrypted, key2)
	if err == nil {
		t.Error("Expected error when decrypting with wrong key")
	}
}
