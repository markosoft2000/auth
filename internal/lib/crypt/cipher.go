package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// EncryptKey takes the raw RSA PEM string and returns an encrypted byte slice
func EncryptKey(plaintextPEM []byte, masterSecret string) ([]byte, error) {
	// 1. Create a 32-byte key from your secret (for AES-256)
	hash := sha256.Sum256([]byte(masterSecret))
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 2. Create GCM (Galois/Counter Mode)
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 3. Create a unique nonce (number used once)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// 4. Encrypt and append the nonce to the front of the result
	// The nonce is required for decryption later and is not secret.
	return gcm.Seal(nonce, nonce, plaintextPEM, nil), nil
}

// DecryptKey takes an encrypted byte slice and returns the raw RSA PEM string
func DecryptKey(encryptedKey []byte, masterSecret string) ([]byte, error) {
	// 1. Hash the masterSecret to get a consistent 32-byte key for AES-256
	hash := sha256.Sum256([]byte(masterSecret))
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 2. Extract nonce and decrypt
	nonceSize := gcm.NonceSize()
	if len(encryptedKey) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := encryptedKey[:nonceSize], encryptedKey[nonceSize:]

	return gcm.Open(nil, nonce, ciphertext, nil)
}
