package auth

import (
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofrs/uuid/v5"
	"github.com/purpose-robot/planet-express/internal/errorz"
	"github.com/purpose-robot/planet-express/internal/krypto"
)

type User struct {
	id             uuid.UUID
	createdAt      time.Time
	updatedAt      time.Time
	activated      bool
	email          string
	token          *Token
	hashedPassword krypto.Argon2Hash
}

func (u *User) ID() uuid.UUID {
	return u.id
}

func (u *User) CreatedAt() time.Time {
	return u.createdAt
}

func (u *User) UpdatedAt() time.Time {
	return u.updatedAt
}

func (u *User) Activated() bool {
	return u.activated
}

func (u *User) Email() string {
	return u.email
}

func (u *User) Token() *Token {
	return u.token
}

func (u *User) HashedPassword() krypto.Argon2Hash {
	return u.hashedPassword
}

func (u *User) Activate() error {
	if u.activated {
		return fmt.Errorf("%w", ErrUserActivated)
	}

	u.activated = true
	u.updatedAt = time.Now().UTC()

	return nil
}

func (u *User) MatchPassword(password string) bool {
	return u.hashedPassword.MatchBytes([]byte(password))
}

func (u *User) RequestActivation(ttl time.Duration) error {
	if u.activated {
		return fmt.Errorf("%w", ErrUserActivated)
	}

	token, err := NewToken(ScopeActivation, u.id, ttl)
	if err != nil {
		return err
	}

	u.token = token
	return nil
}

func (u *User) ResetPassword(password string) error {
	parsedPassword, err := ParsePassword(password)
	if err != nil {
		return err
	}

	hashedPassword, err := krypto.HashArgon2(parsedPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	u.updatedAt = time.Now().UTC()
	u.hashedPassword = hashedPassword

	return nil
}

func (u *User) RequestPasswordReset(ttl time.Duration) error {
	if !u.activated {
		return fmt.Errorf("%w", ErrUserNotActivated)
	}

	token, err := NewToken(ScopePasswordReset, u.id, ttl)
	if err != nil {
		return err
	}

	u.token = token
	return nil
}

func (u *User) RequestAuthentication(ttl time.Duration) error {
	if !u.activated {
		return fmt.Errorf("%w", ErrUserNotActivated)
	}

	token, err := NewToken(ScopeAuthentication, u.id, ttl)
	if err != nil {
		return err
	}

	u.token = token
	return nil
}

func NewUser(email, password string) (*User, error) {
	parsedEmail, err := ParseEmail(email)
	if err != nil {
		return nil, err
	}

	parsedPassword, err := ParsePassword(password)
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	now := time.Now().UTC()

	hashedPassword, err := krypto.HashArgon2(parsedPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	return &User{
		id:             id,
		createdAt:      now,
		updatedAt:      now,
		activated:      false,
		email:          parsedEmail,
		hashedPassword: hashedPassword,
	}, nil
}

func ParseEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)

	if trimmed == "" {
		return "", errorz.ValidationFailed{Field: "email", Message: "cannot be empty"}
	}

	normalized := strings.ToLower(trimmed)

	parsedEmail, err := mail.ParseAddress(normalized)
	if err != nil || parsedEmail.Address != normalized {
		return "", errorz.ValidationFailed{Field: "email", Message: "must be a valid email address"}
	}

	return parsedEmail.Address, nil
}

func ParsePassword(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	runeLen := utf8.RuneCountInString(trimmed)

	if trimmed == "" {
		return "", errorz.ValidationFailed{Field: "password", Message: "cannot be empty"}
	}

	if runeLen < 24 {
		return "", errorz.ValidationFailed{Field: "password", Message: "password must be at least 24 characters"}
	}

	if runeLen > 96 {
		return "", errorz.ValidationFailed{Field: "password", Message: "password must be less than 96 characters"}
	}

	return trimmed, nil
}

func UnmarshalUser(id uuid.UUID, createdAt, updatedAt time.Time, activated bool, email string, hashedPassword krypto.Argon2Hash) *User {
	return &User{
		id:             id,
		createdAt:      createdAt,
		updatedAt:      updatedAt,
		activated:      activated,
		email:          email,
		hashedPassword: hashedPassword,
	}
}

type Permission string

type Permissions []Permission

func (p Permissions) Include(code Permission) bool {
	return slices.Contains(p, code)
}

func ParsePermission(raw string) (Permission, error) {
	trimmed := strings.TrimSpace(raw)

	if trimmed == "" {
		return "", errorz.ValidationFailed{Field: "permission", Message: "cannot be empty"}
	}

	return Permission(trimmed), nil
}

type Token struct {
	id     uuid.UUID
	hash   Hash
	plain  string
	scope  Scope
	userID uuid.UUID
	expiry time.Time
}

type Hash []byte

type Scope string

const (
	ScopeActivation     Scope = "activation"
	ScopePasswordReset  Scope = "password-reset"
	ScopeAuthentication Scope = "authentication"
)

func (t *Token) ID() uuid.UUID {
	return t.id
}

func (t *Token) Hash() Hash {
	return t.hash
}

func (t *Token) Plain() string {
	return t.plain
}

func (t *Token) Scope() Scope {
	return t.scope
}

func (t *Token) UserID() uuid.UUID {
	return t.userID
}

func (t *Token) Expiry() time.Time {
	return t.expiry
}

func NewToken(scope Scope, userID uuid.UUID, ttl time.Duration) (*Token, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	plain, err := krypto.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &Token{
		id:     id,
		hash:   plain.Hash(),
		plain:  plain.String(),
		scope:  scope,
		userID: userID,
		expiry: time.Now().UTC().Add(ttl),
	}, nil
}

func ParseToken(raw string) (Hash, error) {
	trimmed := strings.TrimSpace(raw)

	if trimmed == "" {
		return nil, errorz.ValidationFailed{Field: "token", Message: "cannot be empty"}
	}

	parsedToken, err := krypto.ParseToken(trimmed)
	if err != nil {
		return nil, errorz.ValidationFailed{Field: "token", Message: "invalid or expired token"}
	}

	return parsedToken.Hash(), nil
}

func UnmarshalToken(id uuid.UUID, hash []byte, plain string, scope Scope, userID uuid.UUID, expiry time.Time) *Token {
	return &Token{
		id:     id,
		hash:   hash,
		plain:  plain,
		scope:  scope,
		userID: userID,
		expiry: expiry,
	}
}
