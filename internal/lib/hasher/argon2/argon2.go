package argon2

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
	"golang.org/x/sync/semaphore"
)

type ArgonHasher struct {
	memory         uint32
	iterations     uint32
	parallelism    uint8
	saltLength     uint32
	keyLength      uint32
	sem            *semaphore.Weighted
	byteBufferPool sync.Pool
}

func New(
	memory, iterations uint32,
	parallelism uint8,
	saltLength, keyLength uint32,
	workerLimit uint64,
) *ArgonHasher {
	maxBufferCapacity := keyLength
	if saltLength > maxBufferCapacity {
		maxBufferCapacity = saltLength
	}

	return &ArgonHasher{
		memory:      memory,
		iterations:  iterations,
		parallelism: parallelism,
		saltLength:  saltLength,
		keyLength:   keyLength,
		sem:         semaphore.NewWeighted(int64(workerLimit)),
		byteBufferPool: sync.Pool{
			New: func() any {
				b := make([]byte, maxBufferCapacity)
				return &b // Pool pointers to arrays to avoid interface allocation overhead on Put/Get
			},
		},
	}
}

// HashPassword creates a PHC-formatted string containing the salt and parameters.
func (a *ArgonHasher) HashPassword(ctx context.Context, password string) (string, error) {
	op := "argon2.HashPassword"

	if err := a.sem.Acquire(ctx, 1); err != nil {
		return "", fmt.Errorf("%s: Failed to acquire semaphore: %w", op, err)
	}

	defer a.sem.Release(1)

	saltBufPtr := a.byteBufferPool.Get().(*[]byte)
	defer a.byteBufferPool.Put(saltBufPtr)

	salt := (*saltBufPtr)[:a.saltLength]

	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		a.iterations,
		a.memory,
		a.parallelism,
		a.keyLength,
	)

	// Format: $argon2id$v=19$m=65536,t=3,p=2$salt$hash
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, a.memory, a.iterations, a.parallelism, b64Salt, b64Hash,
	)

	return encodedHash, nil
}

// ComparePassword parses the encoded hash to verify a password against it.
func (a *ArgonHasher) ComparePassword(ctx context.Context, encodedHash, password string) (bool, error) {
	op := "argon2.ComparePassword"

	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("%s: %w", op, errors.New("hash splitting failed"))
	}

	if len(parts[4]) != base64.RawStdEncoding.EncodedLen(int(a.saltLength)) {
		return false, fmt.Errorf("%s: invalid salt length (%d)", op, len(parts[4]))
	}
	if len(parts[5]) != base64.RawStdEncoding.EncodedLen(int(a.keyLength)) {
		return false, fmt.Errorf("%s: invalid hash key length (%d)", op, len(parts[5]))
	}

	var memory, iterations uint32
	var parallelism uint8

	mIdx := strings.Index(parts[3], "m=")
	tIdx := strings.Index(parts[3], "t=")
	pIdx := strings.Index(parts[3], "p=")
	if mIdx == -1 || tIdx == -1 || pIdx == -1 {
		return false, fmt.Errorf("%s: invalid format of parameters for hash", op)
	}

	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	saltBufPtr := a.byteBufferPool.Get().(*[]byte)
	defer a.byteBufferPool.Put(saltBufPtr)
	saltBuf := (*saltBufPtr)[:a.saltLength]

	_, err = base64.RawStdEncoding.Decode(saltBuf[:], []byte(parts[4]))
	if err != nil {
		return false, fmt.Errorf("%s: decode salt failed: %w", op, err)
	}

	keyBufPtr := a.byteBufferPool.Get().(*[]byte)
	defer a.byteBufferPool.Put(keyBufPtr)
	decodedHashBuf := (*keyBufPtr)[:a.keyLength]

	_, err = base64.RawStdEncoding.Decode(decodedHashBuf[:], []byte(parts[5]))
	if err != nil {
		return false, fmt.Errorf("%s: decode hash failed: %w", op, err)
	}

	if err := a.sem.Acquire(ctx, 1); err != nil {
		return false, fmt.Errorf("%s: Failed to acquire semaphore: %w", op, err)
	}

	defer a.sem.Release(1)

	comparisonHash := argon2.IDKey(
		[]byte(password),
		saltBuf[:],
		iterations,
		memory,
		parallelism,
		uint32(len(decodedHashBuf)),
	)

	return subtle.ConstantTimeCompare(decodedHashBuf[:], comparisonHash) == 1, nil
}
