package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/storage"
)

// Sentinel errors for share operations.
var (
	ErrShareNotFound    = errors.New("share not found")
	ErrShareExpired     = errors.New("share has expired")
	ErrDownloadLimit    = errors.New("download limit reached")
	ErrSlugExists       = errors.New("slug already exists")
	ErrPasswordRequired = errors.New("password required")
)

const (
	slugLength = 8
)

// ShareService handles share-related business logic.
type ShareService struct {
	shareRepo *repository.ShareRepository
	fileRepo  *repository.FileRepository
	storage   storage.Storage
}

// CreateShareInput contains the data required to create a new share.
type CreateShareInput struct {
	CreatorID      string
	Name           string
	Description    string
	Slug           string
	Password       *string
	ExpiresAt      *time.Time
	MaxDownloads   *int
	IsReverseShare bool
}

// UpdateShareInput contains the data for updating an existing share.
type UpdateShareInput struct {
	Name           *string
	Description    *string
	Password       *string
	ClearPassword  bool
	ExpiresAt      *time.Time
	ClearExpiry    bool
	MaxDownloads   *int
	IsReverseShare *bool
}

// NewShareService creates a new ShareService instance.
func NewShareService(
	shareRepo *repository.ShareRepository,
	fileRepo *repository.FileRepository,
	store storage.Storage,
) *ShareService {
	return &ShareService{
		shareRepo: shareRepo,
		fileRepo:  fileRepo,
		storage:   store,
	}
}

// Create creates a new share with the given input.
func (s *ShareService) Create(ctx context.Context, input CreateShareInput) (*model.Share, error) {
	slug := input.Slug
	if slug == "" {
		var err error
		slug, err = s.generateUniqueSlug(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		exists, err := s.shareRepo.SlugExists(ctx, slug)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrSlugExists
		}
	}

	var passwordHash *string
	if input.Password != nil && *input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcryptCost)
		if err != nil {
			return nil, err
		}
		hashStr := string(hash)
		passwordHash = &hashStr
	}

	var creatorID *string
	if input.CreatorID != "" {
		creatorID = &input.CreatorID
	}

	share := &model.Share{
		ID:             uuid.NewString(),
		CreatorID:      creatorID,
		Slug:           slug,
		Name:           input.Name,
		Description:    input.Description,
		PasswordHash:   passwordHash,
		ExpiresAt:      input.ExpiresAt,
		MaxDownloads:   input.MaxDownloads,
		DownloadCount:  0,
		IsReverseShare: input.IsReverseShare,
	}

	if err := s.shareRepo.Create(ctx, share); err != nil {
		return nil, err
	}

	return share, nil
}

// GetByID retrieves a share by its ID.
func (s *ShareService) GetByID(ctx context.Context, id string) (*model.Share, error) {
	share, err := s.shareRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrShareNotFound
		}
		return nil, err
	}
	return share, nil
}

// GetBySlug retrieves a share by its URL slug.
func (s *ShareService) GetBySlug(ctx context.Context, slug string) (*model.Share, error) {
	share, err := s.shareRepo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrShareNotFound
		}
		return nil, err
	}
	return share, nil
}

// Update modifies an existing share with the given input.
func (s *ShareService) Update(ctx context.Context, id string, input UpdateShareInput) (*model.Share, error) {
	share, err := s.shareRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrShareNotFound
		}
		return nil, err
	}

	// Create updated share (immutability pattern)
	updated := &model.Share{
		ID:             share.ID,
		CreatorID:      share.CreatorID,
		Slug:           share.Slug,
		Name:           share.Name,
		Description:    share.Description,
		PasswordHash:   share.PasswordHash,
		ExpiresAt:      share.ExpiresAt,
		MaxDownloads:   share.MaxDownloads,
		DownloadCount:  share.DownloadCount,
		IsReverseShare: share.IsReverseShare,
		CreatedAt:      share.CreatedAt,
		UpdatedAt:      share.UpdatedAt,
	}

	if input.Name != nil {
		updated.Name = *input.Name
	}
	if input.Description != nil {
		updated.Description = *input.Description
	}
	if input.ClearPassword {
		updated.PasswordHash = nil
	} else if input.Password != nil && *input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcryptCost)
		if err != nil {
			return nil, err
		}
		hashStr := string(hash)
		updated.PasswordHash = &hashStr
	}
	if input.ClearExpiry {
		updated.ExpiresAt = nil
	} else if input.ExpiresAt != nil {
		updated.ExpiresAt = input.ExpiresAt
	}
	if input.MaxDownloads != nil {
		updated.MaxDownloads = input.MaxDownloads
	}
	if input.IsReverseShare != nil {
		updated.IsReverseShare = *input.IsReverseShare
	}

	if err := s.shareRepo.Update(ctx, updated); err != nil {
		return nil, err
	}

	return updated, nil
}

// Delete removes a share and all associated files from storage.
func (s *ShareService) Delete(ctx context.Context, id string) error {
	// Check share exists first
	_, err := s.shareRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrShareNotFound
		}
		return err
	}

	// Get storage keys for all files in this share
	storageKeys, err := s.fileRepo.GetStorageKeysByShare(ctx, id)
	if err != nil {
		return err
	}

	// Delete files from storage (continue on error to clean up as much as possible)
	for _, key := range storageKeys {
		// Ignore errors on individual file deletion to ensure we clean up the database
		_ = s.storage.Delete(ctx, key)
	}

	// Delete from database (CASCADE will handle file records)
	return s.shareRepo.Delete(ctx, id)
}

// ListByCreator retrieves all shares created by a specific user.
func (s *ShareService) ListByCreator(ctx context.Context, creatorID string) ([]*model.Share, error) {
	shares, err := s.shareRepo.ListByCreator(ctx, creatorID)
	if err != nil {
		return nil, err
	}
	if shares == nil {
		return []*model.Share{}, nil
	}
	return shares, nil
}

// VerifyPassword checks if the provided password matches the share's password.
// Returns true if the share has no password or if the password matches.
func (s *ShareService) VerifyPassword(ctx context.Context, id string, password string) bool {
	share, err := s.shareRepo.GetByID(ctx, id)
	if err != nil {
		return false
	}

	if !share.HasPassword() {
		return true
	}

	err = bcrypt.CompareHashAndPassword([]byte(*share.PasswordHash), []byte(password))
	return err == nil
}

// ValidateAccess checks if a share is accessible (not expired, within limits).
func (s *ShareService) ValidateAccess(_ context.Context, share *model.Share) error {
	if share.IsExpired() {
		return ErrShareExpired
	}
	if share.IsDownloadLimitReached() {
		return ErrDownloadLimit
	}
	return nil
}

// IncrementDownloadCount atomically increments the download counter for a share.
func (s *ShareService) IncrementDownloadCount(ctx context.Context, id string) error {
	err := s.shareRepo.IncrementDownloadCount(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrShareNotFound
		}
		return err
	}
	return nil
}

// generateUniqueSlug generates a random 8-character slug that doesn't exist in the database.
func (s *ShareService) generateUniqueSlug(ctx context.Context) (string, error) {
	for i := 0; i < 10; i++ {
		slug, err := generateRandomSlug()
		if err != nil {
			return "", err
		}

		exists, err := s.shareRepo.SlugExists(ctx, slug)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
	}
	return "", errors.New("failed to generate unique slug after 10 attempts")
}

// generateRandomSlug generates a random 8-character alphanumeric slug.
func generateRandomSlug() (string, error) {
	bytes := make([]byte, slugLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Use base32 encoding for URL-safe characters, then truncate and lowercase
	encoded := base32.StdEncoding.EncodeToString(bytes)
	return strings.ToLower(encoded[:slugLength]), nil
}
