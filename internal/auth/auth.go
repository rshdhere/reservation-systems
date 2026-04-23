// 11
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidToken    = errors.New("invalid token")
	ErrInvalidPassword = errors.New("invalid password")
)

type Config struct {
	Secret   string
	Issuer   string
	TokenTTL time.Duration
}

type Service struct {
	signingKey []byte
	issuer     string
	tokenTTL   time.Duration
}

const (
	defaultTokenTTL = 12 * time.Hour
	defaultIssuer   = "http-api"
	minSecretLength = 32

	argon2SaltLength  = 16
	argon2Memory      = 64 * 1024
	argon2Iterations  = 3
	argon2Parallelism = 2
	argon2KeyLength   = 32
)

func NewService(cfg Config) (*Service, error) {
	secret := strings.TrimSpace(cfg.Secret)
	if len(secret) < minSecretLength {
		return nil, fmt.Errorf("jwt secret must be at least %d characters", minSecretLength)
	}

	ttl := cfg.TokenTTL
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}

	issuer := strings.TrimSpace(cfg.Issuer)
	if issuer == "" {
		issuer = defaultIssuer
	}

	return &Service{
		signingKey: []byte(secret),
		issuer:     issuer,
		tokenTTL:   ttl,
	}, nil
}

func (s *Service) IssueToken(userID uint) (string, error) {
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(uint64(userID), 10),
		Issuer:    s.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenTTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.signingKey)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

func (s *Service) SubjectForToken(ctx context.Context, tokenStr string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(tokenStr)
	if trimmed == "" {
		return "", ErrInvalidToken
	}

	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(
		trimmed,
		claims,
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return s.signingKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithLeeway(5*time.Second),
	)
	if err != nil || !parsed.Valid {
		return "", ErrInvalidToken
	}

	subject := strings.TrimSpace(claims.Subject)
	if subject == "" {
		return "", ErrInvalidToken
	}

	return subject, nil
}

func (s *Service) HashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", ErrInvalidPassword
	}

	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		argon2KeyLength,
	)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func (s *Service) ComparePassword(hash, password string) error {
	params, salt, storedHash, err := parseArgon2Hash(hash)
	if err != nil {
		return fmt.Errorf("parse password hash: %w", err)
	}

	computedHash := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		uint32(len(storedHash)),
	)

	if subtle.ConstantTimeCompare(storedHash, computedHash) != 1 {
		return ErrInvalidPassword
	}

	return nil
}

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func parseArgon2Hash(encoded string) (argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" {
		return argon2Params{}, nil, nil, errors.New("invalid hash format")
	}

	if parts[1] != "argon2id" {
		return argon2Params{}, nil, nil, errors.New("unsupported hash algorithm")
	}

	versionPart := strings.TrimPrefix(parts[2], "v=")
	version, err := strconv.Atoi(versionPart)
	if err != nil || version != argon2.Version {
		return argon2Params{}, nil, nil, errors.New("unsupported argon2 version")
	}

	params, err := parseArgon2Params(parts[3])
	if err != nil {
		return argon2Params{}, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return argon2Params{}, nil, nil, errors.New("invalid salt")
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) == 0 {
		return argon2Params{}, nil, nil, errors.New("invalid hash")
	}

	return params, salt, hash, nil
}

func parseArgon2Params(raw string) (argon2Params, error) {
	parts := strings.Split(raw, ",")
	if len(parts) != 3 {
		return argon2Params{}, errors.New("invalid argon2 params")
	}

	var params argon2Params
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return argon2Params{}, errors.New("invalid argon2 param")
		}

		switch kv[0] {
		case "m":
			value, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return argon2Params{}, errors.New("invalid argon2 memory")
			}
			params.memory = uint32(value)
		case "t":
			value, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return argon2Params{}, errors.New("invalid argon2 iterations")
			}
			params.iterations = uint32(value)
		case "p":
			value, err := strconv.ParseUint(kv[1], 10, 8)
			if err != nil {
				return argon2Params{}, errors.New("invalid argon2 parallelism")
			}
			params.parallelism = uint8(value)
		default:
			return argon2Params{}, errors.New("unknown argon2 param")
		}
	}

	if params.memory == 0 || params.iterations == 0 || params.parallelism == 0 {
		return argon2Params{}, errors.New("argon2 params must be non-zero")
	}

	return params, nil
}
