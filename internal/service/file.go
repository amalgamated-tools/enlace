package service

import (
	"context"
	"errors"
	"io"
	"mime"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/storage"
)

// ErrFileNotFound is returned when a requested file does not exist.
var ErrFileNotFound = errors.New("file not found")

// ErrInvalidFilename is returned when an uploaded filename is not valid.
var ErrInvalidFilename = errors.New("invalid filename")

var (
	ErrDirectTransferUnsupported = errors.New("direct transfer unsupported")
	ErrUploadNotFound            = errors.New("upload not found")
	ErrUploadExpired             = errors.New("upload expired")
	ErrUploadAlreadyFinalized    = errors.New("upload already finalized")
	ErrIntegrityCheckFailed      = errors.New("integrity check failed")
)

// FileService handles file-related business logic.
type FileService struct {
	fileRepo          *repository.FileRepository
	shareRepo         *repository.ShareRepository
	storage           storage.Storage
	pendingUploadRepo *repository.PendingUploadRepository
	presignExpiry     time.Duration
}

// UploadInput contains the data required to upload a file.
type UploadInput struct {
	ShareID    string
	UploaderID string
	Filename   string
	Content    io.Reader
	Size       int64
}

type DirectUploadInput struct {
	ShareID    string
	UploaderID string
	Filename   string
	Size       int64
}

type DirectUploadResponse struct {
	UploadID   string
	FileID     string
	ShareID    string
	Filename   string
	Size       int64
	MimeType   string
	StorageKey string
	Upload     *storage.PresignedURLResult
}

type DirectDownloadResponse struct {
	FileID string
	URL    *storage.PresignedURLResult
}

type FileServiceOption func(*FileService)

func WithPendingUploads(repo *repository.PendingUploadRepository, expiry time.Duration) FileServiceOption {
	return func(s *FileService) {
		s.pendingUploadRepo = repo
		s.presignExpiry = expiry
	}
}

// NewFileService creates a new FileService instance.
func NewFileService(
	fileRepo *repository.FileRepository,
	shareRepo *repository.ShareRepository,
	store storage.Storage,
	opts ...FileServiceOption,
) *FileService {
	svc := &FileService{
		fileRepo:  fileRepo,
		shareRepo: shareRepo,
		storage:   store,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// Upload stores a file and saves its metadata.
func (s *FileService) Upload(ctx context.Context, input UploadInput) (*model.File, error) {
	// Verify share exists
	_, err := s.shareRepo.GetByID(ctx, input.ShareID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrShareNotFound
		}
		return nil, err
	}

	file, err := s.newFileRecord(input.ShareID, input.UploaderID, input.Filename, input.Size)
	if err != nil {
		return nil, err
	}

	// Store the file
	if err := s.storage.Put(ctx, file.StorageKey, input.Content, input.Size, file.MimeType); err != nil {
		return nil, err
	}

	// Save metadata to database
	if err := s.fileRepo.Create(ctx, file); err != nil {
		// Attempt to clean up stored file on database error
		_ = s.storage.Delete(ctx, file.StorageKey)
		return nil, err
	}

	return file, nil
}

// GetByID retrieves a file by its ID.
func (s *FileService) GetByID(ctx context.Context, id string) (*model.File, error) {
	file, err := s.fileRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	return file, nil
}

// Delete removes a file from storage and database.
func (s *FileService) Delete(ctx context.Context, id string) error {
	// Get file first to get storage key
	file, err := s.fileRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrFileNotFound
		}
		return err
	}

	// Delete from storage (ignore errors to ensure database cleanup)
	_ = s.storage.Delete(ctx, file.StorageKey)

	// Delete from database
	return s.fileRepo.Delete(ctx, id)
}

// ListByShare retrieves all files for a specific share.
func (s *FileService) ListByShare(ctx context.Context, shareID string) ([]*model.File, error) {
	files, err := s.fileRepo.ListByShare(ctx, shareID)
	if err != nil {
		return nil, err
	}
	if files == nil {
		return []*model.File{}, nil
	}
	return files, nil
}

// GetContent retrieves the file content for download.
func (s *FileService) GetContent(ctx context.Context, id string) (io.ReadCloser, *model.File, error) {
	// Get file metadata
	file, err := s.fileRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, ErrFileNotFound
		}
		return nil, nil, err
	}

	// Get content from storage
	content, err := s.storage.Get(ctx, file.StorageKey)
	if err != nil {
		return nil, nil, err
	}

	return content, file, nil
}

// IsPreviewable checks if a file type supports preview.
// Returns true for: images (jpeg, png, gif, svg, webp), PDF, and text files.
func (s *FileService) IsPreviewable(file *model.File) bool {
	return isPreviewableMimeType(file.MimeType)
}

// InitiateDirectUpload creates a pending direct upload and returns a short-lived signed PUT request.
func (s *FileService) InitiateDirectUpload(ctx context.Context, input DirectUploadInput) (*DirectUploadResponse, error) {
	presignedStorage, err := s.requireDirectUploadSupport()
	if err != nil {
		return nil, err
	}

	if _, err := s.shareRepo.GetByID(ctx, input.ShareID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrShareNotFound
		}
		return nil, err
	}

	if _, err := s.pendingUploadRepo.ExpireStale(ctx, time.Now()); err != nil {
		return nil, err
	}

	file, err := s.newFileRecord(input.ShareID, input.UploaderID, input.Filename, input.Size)
	if err != nil {
		return nil, err
	}

	uploadID := uuid.NewString()
	upload, err := presignedStorage.PresignPut(ctx, file.StorageKey, input.Size, file.MimeType, s.presignExpiry)
	if err != nil {
		return nil, err
	}

	pendingUpload := &model.PendingUpload{
		ID:         uploadID,
		FileID:     file.ID,
		ShareID:    input.ShareID,
		UploaderID: file.UploaderID,
		Filename:   file.Name,
		Size:       input.Size,
		MimeType:   file.MimeType,
		StorageKey: file.StorageKey,
		Status:     repository.PendingUploadStatusPending,
		ExpiresAt:  upload.ExpiresAt,
	}
	if err := s.pendingUploadRepo.Create(ctx, pendingUpload); err != nil {
		return nil, err
	}

	return &DirectUploadResponse{
		UploadID:   uploadID,
		FileID:     file.ID,
		ShareID:    input.ShareID,
		Filename:   file.Name,
		Size:       input.Size,
		MimeType:   file.MimeType,
		StorageKey: file.StorageKey,
		Upload:     upload,
	}, nil
}

// FinalizeDirectUpload validates an uploaded object and promotes the pending upload into a persisted file row.
func (s *FileService) FinalizeDirectUpload(ctx context.Context, uploadID string) (*model.File, error) {
	presignedStorage, err := s.requireDirectUploadSupport()
	if err != nil {
		return nil, err
	}

	now := time.Now()

	if _, err := s.pendingUploadRepo.ExpireStale(ctx, now); err != nil {
		return nil, err
	}

	upload, err := s.pendingUploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUploadNotFound
		}
		return nil, err
	}

	switch upload.Status {
	case repository.PendingUploadStatusFinalized:
		return nil, ErrUploadAlreadyFinalized
	case repository.PendingUploadStatusExpired:
		return nil, ErrUploadExpired
	}

	if now.After(upload.ExpiresAt) {
		return nil, ErrUploadExpired
	}

	size, contentType, err := presignedStorage.HeadObject(ctx, upload.StorageKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrIntegrityCheckFailed
		}
		return nil, err
	}
	if size != upload.Size {
		_ = s.storage.Delete(ctx, upload.StorageKey)
		return nil, ErrIntegrityCheckFailed
	}
	if !contentTypesMatch(upload.MimeType, contentType) {
		_ = s.storage.Delete(ctx, upload.StorageKey)
		return nil, ErrIntegrityCheckFailed
	}

	file := &model.File{
		ID:         upload.FileID,
		ShareID:    upload.ShareID,
		UploaderID: upload.UploaderID,
		Name:       upload.Filename,
		Size:       upload.Size,
		MimeType:   upload.MimeType,
		StorageKey: upload.StorageKey,
	}
	if err := s.pendingUploadRepo.Finalize(ctx, uploadID, file); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUploadNotFound
		}
		if errors.Is(err, repository.ErrPendingUploadConflict) {
			return nil, ErrUploadAlreadyFinalized
		}
		return nil, err
	}

	return file, nil
}

// GetPresignedDownloadURL returns a short-lived signed download URL for an existing file.
func (s *FileService) GetPresignedDownloadURL(ctx context.Context, fileID string, expiry time.Duration) (*DirectDownloadResponse, error) {
	presignedStorage, ok := s.storage.(storage.PresignedStorage)
	if !ok {
		return nil, ErrDirectTransferUnsupported
	}

	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": file.Name})
	url, err := presignedStorage.PresignGet(ctx, file.StorageKey, expiry, disposition)
	if err != nil {
		return nil, err
	}

	return &DirectDownloadResponse{
		FileID: file.ID,
		URL:    url,
	}, nil
}

func (s *FileService) newFileRecord(shareID, uploaderID, filename string, size int64) (*model.File, error) {
	fileID := uuid.NewString()

	sanitizedFilename, err := sanitizeFilename(filename)
	if err != nil {
		return nil, err
	}

	mimeType := detectMimeType(sanitizedFilename)

	var normalizedUploaderID *string
	if uploaderID != "" {
		normalizedUploaderID = &uploaderID
	}

	return &model.File{
		ID:         fileID,
		ShareID:    shareID,
		UploaderID: normalizedUploaderID,
		Name:       sanitizedFilename,
		Size:       size,
		MimeType:   mimeType,
		StorageKey: shareID + "/" + fileID + "/" + sanitizedFilename,
	}, nil
}

func (s *FileService) requireDirectUploadSupport() (storage.PresignedStorage, error) {
	presignedStorage, ok := s.storage.(storage.PresignedStorage)
	if !ok || s.pendingUploadRepo == nil || s.presignExpiry <= 0 {
		return nil, ErrDirectTransferUnsupported
	}
	return presignedStorage, nil
}

func contentTypesMatch(expected, actual string) bool {
	if expected == "" && actual == "" {
		return true
	}
	if expected == "" || actual == "" {
		return false
	}

	expectedType, _, err := mime.ParseMediaType(expected)
	if err != nil {
		expectedType = expected
	}
	actualType, _, err := mime.ParseMediaType(actual)
	if err != nil {
		actualType = actual
	}

	return strings.EqualFold(expectedType, actualType)
}

func sanitizeFilename(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrInvalidFilename
	}

	if strings.ContainsRune(trimmed, '\x00') {
		return "", ErrInvalidFilename
	}

	normalized := strings.ReplaceAll(trimmed, "\\", "/")
	base := path.Base(normalized)
	if base == "." || base == "/" || base == ".." {
		return "", ErrInvalidFilename
	}

	return base, nil
}

// detectMimeType determines the MIME type based on file extension.
func detectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	mimeTypes := map[string]string{
		// Images
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".webp": "image/webp",
		".ico":  "image/x-icon",
		".bmp":  "image/bmp",
		".tiff": "image/tiff",
		".tif":  "image/tiff",

		// Documents
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",

		// Text and code
		".txt":  "text/plain",
		".html": "text/html",
		".htm":  "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".csv":  "text/csv",
		".md":   "text/markdown",

		// Archives
		".zip": "application/zip",
		".tar": "application/x-tar",
		".gz":  "application/gzip",
		".rar": "application/vnd.rar",
		".7z":  "application/x-7z-compressed",

		// Audio
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".flac": "audio/flac",

		// Video
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".mkv":  "video/x-matroska",
	}

	if mimeType, ok := mimeTypes[ext]; ok {
		return mimeType
	}
	return "application/octet-stream"
}

// isPreviewableMimeType checks if a MIME type supports preview.
func isPreviewableMimeType(mimeType string) bool {
	previewable := map[string]bool{
		// Images
		"image/jpeg":    true,
		"image/png":     true,
		"image/gif":     true,
		"image/svg+xml": true,
		"image/webp":    true,

		// PDF
		"application/pdf": true,

		// Text types
		"text/plain":      true,
		"text/html":       true,
		"text/css":        true,
		"text/javascript": true,
		"text/markdown":   true,
		"text/csv":        true,
	}

	return previewable[mimeType]
}
