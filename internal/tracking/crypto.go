package tracking

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// PixelPayload is encrypted into the tracking pixel URL
// to be decrypted by the worker.
type PixelPayload struct {
	Recipient   string `json:"r"`
	SubjectHash string `json:"s"`
	SentAt      int64  `json:"t"`
}

var errCiphertextTooShort = errors.New("ciphertext too short")

// Encrypt encrypts a PixelPayload into a URL-safe base64 blob using AES-GCM
func Encrypt(payload *PixelPayload, keyBase64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return "", fmt.Errorf("decode key: %w", err)
	}

	plaintext, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}

	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)

	// URL-safe base64 encode
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a URL-safe base64 blob using AES-GCM
func Decrypt(blob string, keyBase64 string) (*PixelPayload, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(blob)
	if err != nil {
		return nil, fmt.Errorf("decode blob: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	if len(ciphertext) < aead.NonceSize() {
		return nil, errCiphertextTooShort
	}

	nonce := ciphertext[:aead.NonceSize()]
	ciphertext = ciphertext[aead.NonceSize():]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	var payload PixelPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return &payload, nil
}

// GenerateKey generates a new 256-bit AES key as base64
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}

	return base64.StdEncoding.EncodeToString(key), nil
}
