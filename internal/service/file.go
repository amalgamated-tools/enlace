package service

import (
	"context"
	"errors"
	"io"
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

// ErrDirectTransferUnsupported is returned when the storage backend does not support direct transfer.
var ErrDirectTransferUnsupported = errors.New("direct transfer not supported by current storage backend")

// ErrUploadNotFound is returned when a pending upload does not exist.
var ErrUploadNotFound = errors.New("pending upload not found")

// ErrUploadExpired is returned when a pending upload has passed its expiry.
var ErrUploadExpired = errors.New("pending upload has expired")

// ErrUploadAlreadyFinalized is returned when a pending upload has already been finalized.
var ErrUploadAlreadyFinalized = errors.New("upload already finalized")

// ErrIntegrityCheckFailed is returned when the uploaded object does not match expected metadata.
var ErrIntegrityCheckFailed = errors.New("uploaded file integrity check failed")

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

// DirectUploadInput contains the data required to initiate a direct upload.
type DirectUploadInput struct {
	ShareID     string
	UploaderID  string
	Filename    string
	Size        int64
	ContentType string
}

// DirectUploadResponse is returned when a direct upload is initiated.
type DirectUploadResponse struct {
	UploadID  string    `json:"upload_id"`
	UploadURL string    `json:"upload_url"`
	FileID    string    `json:"file_id"`
	Method    string    `json:"method"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DirectDownloadResponse is returned when a presigned download URL is generated.
type DirectDownloadResponse struct {
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// NewFileService creates a new FileService instance.
func NewFileService(
	fileRepo *repository.FileRepository,
	shareRepo *repository.ShareRepository,
	store storage.Storage,
	pendingUploadRepo *repository.PendingUploadRepository,
	presignExpiry time.Duration,
) *FileService {
	return &FileService{
		fileRepo:          fileRepo,
		shareRepo:         shareRepo,
		storage:           store,
		pendingUploadRepo: pendingUploadRepo,
		presignExpiry:     presignExpiry,
	}
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

	// Generate file ID
	fileID := uuid.NewString()

	filename, err := sanitizeFilename(input.Filename)
	if err != nil {
		return nil, err
	}

	// Detect MIME type from file extension
	mimeType := detectMimeType(filename)

	// Create storage key: {shareID}/{fileID}/{filename}
	// Storage keys are logical paths that use forward slashes across backends;
	// validation is enforced by the storage implementation.
	storageKey := input.ShareID + "/" + fileID + "/" + filename

	// Store the file
	if err := s.storage.Put(ctx, storageKey, input.Content, input.Size, mimeType); err != nil {
		return nil, err
	}

	// Create file metadata
	var uploaderID *string
	if input.UploaderID != "" {
		uploaderID = &input.UploaderID
	}

	file := &model.File{
		ID:         fileID,
		ShareID:    input.ShareID,
		UploaderID: uploaderID,
		Name:       filename,
		Size:       input.Size,
		MimeType:   mimeType,
		StorageKey: storageKey,
	}

	// Save metadata to database
	if err := s.fileRepo.Create(ctx, file); err != nil {
		// Attempt to clean up stored file on database error
		_ = s.storage.Delete(ctx, storageKey)
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

// maxPresignExpiry caps the configurable expiry to prevent excessively long-lived URLs.
const maxPresignExpiry = 60 * time.Minute

// clampExpiry ensures the expiry does not exceed the maximum.
func clampExpiry(d time.Duration) time.Duration {
	if d > maxPresignExpiry {
		return maxPresignExpiry
	}
	return d
}

// InitiateDirectUpload creates a pending upload and returns a presigned PUT URL.
func (s *FileService) InitiateDirectUpload(ctx context.Context, input DirectUploadInput) (*DirectUploadResponse, error) {
	dt, ok := s.storage.(storage.DirectTransfer)
	if !ok {
		return nil, ErrDirectTransferUnsupported
	}

	// Verify share exists
	_, err := s.shareRepo.GetByID(ctx, input.ShareID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrShareNotFound
		}
		return nil, err
	}

	filename, err := sanitizeFilename(input.Filename)
	if err != nil {
		return nil, err
	}

	fileID := uuid.NewString()
	uploadID := uuid.NewString()

	mimeType := input.ContentType
	if mimeType == "" {
		mimeType = detectMimeType(filename)
	}

	storageKey := input.ShareID + "/" + fileID + "/" + filename

	expiry := clampExpiry(s.presignExpiry)

	presigned, err := dt.PresignUpload(ctx, storageKey, input.Size, mimeType, expiry)
	if err != nil {
		return nil, err
	}

	var uploaderID *string
	if input.UploaderID != "" {
		uploaderID = &input.UploaderID
	}

	pu := &model.PendingUpload{
		ID:         uploadID,
		FileID:     fileID,
		ShareID:    input.ShareID,
		UploaderID: uploaderID,
		Filename:   filename,
		Size:       input.Size,
		MimeType:   mimeType,
		StorageKey: storageKey,
		Status:     "pending",
		ExpiresAt:  presigned.ExpiresAt,
	}

	if err := s.pendingUploadRepo.Create(ctx, pu); err != nil {
		return nil, err
	}

	return &DirectUploadResponse{
		UploadID:  uploadID,
		UploadURL: presigned.URL,
		FileID:    fileID,
		Method:    presigned.Method,
		ExpiresAt: presigned.ExpiresAt,
	}, nil
}

// FinalizeDirectUpload validates a completed direct upload and creates the file record.
func (s *FileService) FinalizeDirectUpload(ctx context.Context, uploadID string) (*model.File, error) {
	dt, ok := s.storage.(storage.DirectTransfer)
	if !ok {
		return nil, ErrDirectTransferUnsupported
	}

	pu, err := s.pendingUploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUploadNotFound
		}
		return nil, err
	}

	if pu.Status == "finalized" {
		return nil, ErrUploadAlreadyFinalized
	}
	if pu.Status != "pending" {
		return nil, ErrUploadExpired
	}
	if time.Now().After(pu.ExpiresAt) {
		return nil, ErrUploadExpired
	}

	// Verify the object was actually uploaded and matches expected metadata
	info, err := dt.StatObject(ctx, pu.StorageKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrIntegrityCheckFailed
		}
		return nil, err
	}

	if info.Size != pu.Size {
		// Orphaned object — attempt cleanup
		_ = s.storage.Delete(ctx, pu.StorageKey)
		return nil, ErrIntegrityCheckFailed
	}

	// Atomically mark as finalized (prevents replay)
	if err := s.pendingUploadRepo.Finalize(ctx, uploadID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUploadAlreadyFinalized
		}
		return nil, err
	}

	file := &model.File{
		ID:         pu.FileID,
		ShareID:    pu.ShareID,
		UploaderID: pu.UploaderID,
		Name:       pu.Filename,
		Size:       pu.Size,
		MimeType:   pu.MimeType,
		StorageKey: pu.StorageKey,
	}

	if err := s.fileRepo.Create(ctx, file); err != nil {
		return nil, err
	}

	return file, nil
}

// GetPresignedDownloadURL returns a presigned download URL for a file.
func (s *FileService) GetPresignedDownloadURL(ctx context.Context, fileID string, disposition string) (*DirectDownloadResponse, error) {
	dt, ok := s.storage.(storage.DirectTransfer)
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

	expiry := clampExpiry(s.presignExpiry)

	presigned, err := dt.PresignDownload(ctx, file.StorageKey, disposition, expiry)
	if err != nil {
		return nil, err
	}

	return &DirectDownloadResponse{
		DownloadURL: presigned.URL,
		ExpiresAt:   presigned.ExpiresAt,
	}, nil
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
