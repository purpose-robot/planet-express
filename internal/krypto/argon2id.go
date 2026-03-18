package krypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	variant = "argon2id"
	// Generated salt length, 16 bytes or more should be used.
	saltLength = 16
)

// ErrInvalidInput indicates that invalid input was provided.
var ErrInvalidInput = errors.New("invalid input")

// defaultParams specifies sane default parameters for argon2.
var defaultParams = &params{
	memory:      12 * 1024,
	iterations:  3,
	parallelism: 1,
}

type params struct {
	// The amount of memory used by the algorithm.
	memory uint32
	// The number of iterations over the memory.
	iterations uint32
	// The number of threads used by the algorithm.
	parallelism uint8
}

type Argon2Hash struct {
	Salt []byte
	Hash []byte
}

// String returns the string representation of the hash.
func (h *Argon2Hash) String() string {
	b64Hash := base64.RawStdEncoding.EncodeToString(h.Hash)
	b64Salt := base64.RawStdEncoding.EncodeToString(h.Salt)

	return fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		variant,
		argon2.Version,
		defaultParams.memory,
		defaultParams.iterations,
		defaultParams.parallelism,
		b64Salt,
		b64Hash,
	)
}

// Scan implements the sql.Scanner interface.
func (h *Argon2Hash) Scan(v any) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type Argon2Hash", v)
	}

	parsed, err := ParseArgon2Hash(s)
	if err != nil {
		return err
	}

	*h = parsed
	return nil
}

// MatchBytes checks if the hash matches the given byte slice.
func (h *Argon2Hash) MatchBytes(b []byte) bool {
	hash := argon2.IDKey(
		b,
		h.Salt,
		defaultParams.iterations,
		defaultParams.memory,
		defaultParams.parallelism,
		uint32(len(h.Hash)),
	)

	return subtle.ConstantTimeCompare(hash, h.Hash) == 1
}

// ParseArgon2Hash parses an argon2 hash from the representation provided by the String method.
func ParseArgon2Hash(message string) (Argon2Hash, error) {
	values := strings.Split(message, "$")
	if len(values) != 6 {
		return Argon2Hash{}, fmt.Errorf("wrong number of components: %w", ErrInvalidInput)
	}

	h := Argon2Hash{}

	for i, v := range values {
		if !parseComponent(i, v, &h) {
			return Argon2Hash{}, fmt.Errorf("failed to parse component %d: %w", i, ErrInvalidInput)
		}
	}

	return h, nil
}

// parseComponent parses a single of a textual argon2 hash representation.
func parseComponent(i int, v string, h *Argon2Hash) bool {
	switch i {
	case 0:
		return v == ""

	case 1:
		return v == variant

	case 2:
		var version int
		_, err := fmt.Sscanf(v, "v=%d", &version)
		return err == nil && version == argon2.Version

	case 3:
		var parallelism uint8
		var memory, iterations uint32
		_, err := fmt.Sscanf(v, "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
		return err == nil

	case 4:
		salt, err := base64.RawStdEncoding.DecodeString(v)
		h.Salt = salt
		return err == nil

	case 5:
		hash, err := base64.RawStdEncoding.DecodeString(v)
		h.Hash = hash
		return err == nil
	}

	return false
}

func generateRandomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)

	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %w", err)
	}

	return b, nil
}

// HashArgon2 hashes a byte slice using the argon2id algorithm.
func HashArgon2[T interface{ string | []byte }](password T) (Argon2Hash, error) {
	salt, err := generateRandomBytes(saltLength)
	if err != nil {
		return Argon2Hash{}, fmt.Errorf("failed to generate random salt: %w", err)
	}

	return hashArgon2WithSalt(password, salt)
}

// HashArgon2WithKey hashes a byte slice using the argon2id algorithm with the provided key.
func HashArgon2WithKey[T interface{ string | []byte }](password T, key Key) (Argon2Hash, error) {
	if len(key.value) <= saltLength {
		return Argon2Hash{}, fmt.Errorf("provided salt too small: %v", ErrInvalidInput)
	}

	return hashArgon2WithSalt(password, key.value[:saltLength])
}

// hashArgon2WithSalt hashes a byte slice using the argon2id algorithm with the provided salt.
func hashArgon2WithSalt[T interface{ string | []byte }](password T, salt []byte) (Argon2Hash, error) {
	if len(password) == 0 {
		return Argon2Hash{}, fmt.Errorf("empty password provided: %v", ErrInvalidInput)
	}

	return Argon2Hash{
		Salt: salt,
		Hash: argon2.IDKey(
			[]byte(password),
			salt,
			defaultParams.iterations,
			defaultParams.memory,
			defaultParams.parallelism,
			keyLength,
		),
	}, nil
}
