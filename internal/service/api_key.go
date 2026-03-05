package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

const (
	// APIKeyTokenPrefix is the token prefix used in generated API keys.
	APIKeyTokenPrefix = "enl"
)

var (
	// ErrInvalidAPIKey is returned when an API key is invalid or revoked.
	ErrInvalidAPIKey = errors.New("invalid api key")
	// ErrInvalidScopeSet is returned when requested scopes contain invalid values.
	ErrInvalidScopeSet = errors.New("invalid scopes")
)

// APIKeyIdentity represents an authenticated API key principal.
type APIKeyIdentity struct {
	KeyID    string
	UserID   string
	Scopes   []string
	AuthType string
}

// APIKeyService handles API key lifecycle and authentication.
type APIKeyService struct {
	repo *repository.APIKeyRepository
}

// NewAPIKeyService creates a new APIKeyService.
func NewAPIKeyService(repo *repository.APIKeyRepository) *APIKeyService {
	return &APIKeyService{repo: repo}
}

// AllowedScopes is the supported scope set for API keys.
func AllowedScopes() []string {
	return []string{
		"shares:read",
		"shares:write",
		"files:read",
		"files:write",
	}
}

// Create creates a scoped API key and returns plaintext token once.
func (s *APIKeyService) Create(ctx context.Context, creatorID, name string, scopes []string) (*model.APIKey, string, error) {
	if creatorID == "" {
		return nil, "", ErrInvalidAPIKey
	}
	normalized, err := normalizeScopes(scopes)
	if err != nil {
		return nil, "", err
	}

	secret, err := generateAPISecret()
	if err != nil {
		return nil, "", err
	}

	key := &model.APIKey{
		ID:        uuid.NewString(),
		CreatorID: creatorID,
		Name:      strings.TrimSpace(name),
		Scopes:    normalized,
	}
	if key.Name == "" {
		key.Name = "automation"
	}

	token := APIKeyTokenPrefix + "_" + key.ID + "_" + secret
	key.KeyPrefix = tokenPrefix(token)
	key.KeyHash = hashToken(token)

	if err := s.repo.Create(ctx, key); err != nil {
		return nil, "", err
	}
	return key, token, nil
}

// ListByCreator lists API keys created by a user.
func (s *APIKeyService) ListByCreator(ctx context.Context, creatorID string) ([]*model.APIKey, error) {
	return s.repo.ListByCreator(ctx, creatorID)
}

// Revoke revokes an API key by ID.
func (s *APIKeyService) Revoke(ctx context.Context, id string) error {
	return s.repo.Revoke(ctx, id)
}

// Authenticate validates a bearer token and returns API key identity.
func (s *APIKeyService) Authenticate(ctx context.Context, token string) (*APIKeyIdentity, error) {
	if !strings.HasPrefix(token, APIKeyTokenPrefix+"_") {
		return nil, ErrInvalidAPIKey
	}

	hash := hashToken(token)
	key, err := s.repo.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidAPIKey
		}
		return nil, err
	}
	if key.IsRevoked() {
		return nil, ErrInvalidAPIKey
	}

	if err := s.repo.TouchLastUsed(ctx, key.ID, time.Now()); err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	identity := &APIKeyIdentity{
		KeyID:    key.ID,
		UserID:   key.CreatorID,
		Scopes:   append([]string(nil), key.Scopes...),
		AuthType: "api_key",
	}
	return identity, nil
}

func normalizeScopes(scopes []string) ([]string, error) {
	allowed := make(map[string]struct{}, len(AllowedScopes()))
	for _, scope := range AllowedScopes() {
		allowed[scope] = struct{}{}
	}

	out := make([]string, 0, len(scopes))
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		normalized := strings.TrimSpace(scope)
		if normalized == "" {
			continue
		}
		if _, ok := allowed[normalized]; !ok {
			return nil, ErrInvalidScopeSet
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil, ErrInvalidScopeSet
	}
	sort.Strings(out)
	return out, nil
}

func generateAPISecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func tokenPrefix(token string) string {
	if len(token) <= 14 {
		return token
	}
	return token[:14]
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
