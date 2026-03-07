package service_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/amalgamated-tools/enlace/internal/database"
	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
	"github.com/amalgamated-tools/enlace/internal/service"
	"github.com/amalgamated-tools/enlace/internal/storage"
)

// testStorage implements storage.Storage for testing.
type testStorage struct {
	files       map[string][]byte
	putErr      error
	getErr      error
	deleteErr   error
	deletedKeys []string
}

func newTestStorage() *testStorage {
	return &testStorage{
		files: make(map[string][]byte),
	}
}

func (s *testStorage) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	if s.putErr != nil {
		return s.putErr
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.files[key] = data
	return nil
}

func (s *testStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	data, ok := s.files[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *testStorage) Delete(_ context.Context, key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.files[key]; !ok {
		return storage.ErrNotFound
	}
	delete(s.files, key)
	s.deletedKeys = append(s.deletedKeys, key)
	return nil
}

func (s *testStorage) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.files[key]
	return ok, nil
}

func setupFileService(t *testing.T) (*service.FileService, *testStorage, *repository.ShareRepository, func()) {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	store := newTestStorage()
	fileService := service.NewFileService(fileRepo, shareRepo, store, nil, 0)

	return fileService, store, shareRepo, func() { db.Close() }
}

func setupFileServiceWithUserAndShare(t *testing.T) (*service.FileService, *testStorage, *model.Share, func()) {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	store := newTestStorage()

	// Create a user
	user := &model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "hash",
		DisplayName:  "Test User",
	}
	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a share
	share := &model.Share{
		ID:        "share-123",
		CreatorID: &user.ID,
		Slug:      "test-share",
		Name:      "Test Share",
	}
	if err := shareRepo.Create(context.Background(), share); err != nil {
		t.Fatalf("failed to create test share: %v", err)
	}

	fileService := service.NewFileService(fileRepo, shareRepo, store, nil, 0)

	return fileService, store, share, func() { db.Close() }
}

func TestFileService_Upload(t *testing.T) {
	svc, store, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	input := service.UploadInput{
		ShareID:    share.ID,
		UploaderID: *share.CreatorID,
		Filename:   "test.txt",
		Content:    strings.NewReader("hello world"),
		Size:       11,
	}

	file, err := svc.Upload(ctx, input)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	if file.ID == "" {
		t.Error("expected file ID to be set")
	}
	if file.ShareID != share.ID {
		t.Errorf("expected share ID %s, got %s", share.ID, file.ShareID)
	}
	if file.Name != "test.txt" {
		t.Errorf("expected name 'test.txt', got %s", file.Name)
	}
	if file.Size != 11 {
		t.Errorf("expected size 11, got %d", file.Size)
	}
	if file.MimeType != "text/plain" {
		t.Errorf("expected mime type 'text/plain', got %s", file.MimeType)
	}
	if file.StorageKey == "" {
		t.Error("expected storage key to be set")
	}

	// Verify file was stored
	expectedKey := share.ID + "/" + file.ID + "/test.txt"
	if file.StorageKey != expectedKey {
		t.Errorf("expected storage key %s, got %s", expectedKey, file.StorageKey)
	}
	if _, ok := store.files[expectedKey]; !ok {
		t.Error("file not found in storage")
	}
}

func TestFileService_Upload_DetectsMimeType(t *testing.T) {
	svc, _, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		filename     string
		expectedMime string
	}{
		{"document.pdf", "application/pdf"},
		{"image.png", "image/png"},
		{"image.jpg", "image/jpeg"},
		{"image.jpeg", "image/jpeg"},
		{"image.gif", "image/gif"},
		{"image.svg", "image/svg+xml"},
		{"image.webp", "image/webp"},
		{"script.js", "application/javascript"},
		{"styles.css", "text/css"},
		{"page.html", "text/html"},
		{"data.json", "application/json"},
		{"data.xml", "application/xml"},
		{"archive.zip", "application/zip"},
		{"unknown.xyz", "application/octet-stream"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			input := service.UploadInput{
				ShareID:    share.ID,
				UploaderID: *share.CreatorID,
				Filename:   tc.filename,
				Content:    strings.NewReader("content"),
				Size:       7,
			}

			file, err := svc.Upload(ctx, input)
			if err != nil {
				t.Fatalf("failed to upload file: %v", err)
			}

			if file.MimeType != tc.expectedMime {
				t.Errorf("expected mime type %s, got %s", tc.expectedMime, file.MimeType)
			}
		})
	}
}

func TestFileService_Upload_SanitizesFilename(t *testing.T) {
	svc, store, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()
	input := service.UploadInput{
		ShareID:    share.ID,
		UploaderID: *share.CreatorID,
		Filename:   "../..//evil.txt",
		Content:    strings.NewReader("content"),
		Size:       7,
	}

	file, err := svc.Upload(ctx, input)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	if file.Name != "evil.txt" {
		t.Fatalf("expected sanitized filename 'evil.txt', got %q", file.Name)
	}
	if strings.Contains(file.StorageKey, "..") {
		t.Fatalf("storage key should not contain traversal components: %s", file.StorageKey)
	}
	if _, ok := store.files[file.StorageKey]; !ok {
		t.Fatalf("expected file to be stored under sanitized key, got %s", file.StorageKey)
	}
}

func TestFileService_Upload_NoUploaderID(t *testing.T) {
	svc, _, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	input := service.UploadInput{
		ShareID:  share.ID,
		Filename: "test.txt",
		Content:  strings.NewReader("hello world"),
		Size:     11,
	}

	file, err := svc.Upload(ctx, input)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	if file.UploaderID != nil {
		t.Errorf("expected uploader ID to be nil, got %v", file.UploaderID)
	}
}

func TestFileService_Upload_ShareNotFound(t *testing.T) {
	svc, _, _, cleanup := setupFileService(t)
	defer cleanup()

	ctx := context.Background()

	input := service.UploadInput{
		ShareID:  "nonexistent-share",
		Filename: "test.txt",
		Content:  strings.NewReader("hello world"),
		Size:     11,
	}

	_, err := svc.Upload(ctx, input)
	if !errors.Is(err, service.ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound, got %v", err)
	}
}

func TestFileService_Upload_StorageError(t *testing.T) {
	svc, store, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	store.putErr = errors.New("storage error")

	input := service.UploadInput{
		ShareID:  share.ID,
		Filename: "test.txt",
		Content:  strings.NewReader("hello world"),
		Size:     11,
	}

	_, err := svc.Upload(ctx, input)
	if err == nil {
		t.Error("expected error on storage failure")
	}
}

func TestFileService_GetByID(t *testing.T) {
	svc, _, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	// Upload a file first
	input := service.UploadInput{
		ShareID:    share.ID,
		UploaderID: *share.CreatorID,
		Filename:   "test.txt",
		Content:    strings.NewReader("hello world"),
		Size:       11,
	}
	uploaded, err := svc.Upload(ctx, input)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	// Get by ID
	file, err := svc.GetByID(ctx, uploaded.ID)
	if err != nil {
		t.Fatalf("failed to get file: %v", err)
	}

	if file.ID != uploaded.ID {
		t.Errorf("expected ID %s, got %s", uploaded.ID, file.ID)
	}
	if file.Name != uploaded.Name {
		t.Errorf("expected name %s, got %s", uploaded.Name, file.Name)
	}
}

func TestFileService_GetByID_NotFound(t *testing.T) {
	svc, _, _, cleanup := setupFileService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.GetByID(ctx, "nonexistent-id")
	if !errors.Is(err, service.ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestFileService_Delete(t *testing.T) {
	svc, store, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	// Upload a file first
	input := service.UploadInput{
		ShareID:    share.ID,
		UploaderID: *share.CreatorID,
		Filename:   "test.txt",
		Content:    strings.NewReader("hello world"),
		Size:       11,
	}
	uploaded, err := svc.Upload(ctx, input)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	// Verify file exists in storage
	if _, ok := store.files[uploaded.StorageKey]; !ok {
		t.Fatal("file should exist in storage before deletion")
	}

	// Delete the file
	err = svc.Delete(ctx, uploaded.ID)
	if err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	// Verify file is removed from storage
	if _, ok := store.files[uploaded.StorageKey]; ok {
		t.Error("file should be removed from storage after deletion")
	}

	// Verify file is removed from database
	_, err = svc.GetByID(ctx, uploaded.ID)
	if !errors.Is(err, service.ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound after deletion, got %v", err)
	}
}

func TestFileService_Delete_NotFound(t *testing.T) {
	svc, _, _, cleanup := setupFileService(t)
	defer cleanup()

	ctx := context.Background()

	err := svc.Delete(ctx, "nonexistent-id")
	if !errors.Is(err, service.ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestFileService_Delete_StorageError(t *testing.T) {
	svc, store, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	// Upload a file first
	input := service.UploadInput{
		ShareID:    share.ID,
		UploaderID: *share.CreatorID,
		Filename:   "test.txt",
		Content:    strings.NewReader("hello world"),
		Size:       11,
	}
	uploaded, err := svc.Upload(ctx, input)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	// Force storage error
	store.deleteErr = errors.New("storage error")

	// Delete should still remove from database
	err = svc.Delete(ctx, uploaded.ID)
	if err != nil {
		t.Fatalf("expected delete to succeed despite storage error, got: %v", err)
	}

	// Verify file is removed from database
	_, err = svc.GetByID(ctx, uploaded.ID)
	if !errors.Is(err, service.ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound after deletion, got %v", err)
	}
}

func TestFileService_ListByShare(t *testing.T) {
	svc, _, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	// Upload multiple files
	filenames := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, name := range filenames {
		input := service.UploadInput{
			ShareID:  share.ID,
			Filename: name,
			Content:  strings.NewReader("content"),
			Size:     7,
		}
		_, err := svc.Upload(ctx, input)
		if err != nil {
			t.Fatalf("failed to upload file: %v", err)
		}
	}

	// List files
	files, err := svc.ListByShare(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
}

func TestFileService_ListByShare_Empty(t *testing.T) {
	svc, _, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	files, err := svc.ListByShare(ctx, share.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestFileService_ListByShare_NonexistentShare(t *testing.T) {
	svc, _, _, cleanup := setupFileService(t)
	defer cleanup()

	ctx := context.Background()

	files, err := svc.ListByShare(ctx, "nonexistent-share")
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 files for nonexistent share, got %d", len(files))
	}
}

func TestFileService_GetContent(t *testing.T) {
	svc, _, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	content := "hello world"
	input := service.UploadInput{
		ShareID:  share.ID,
		Filename: "test.txt",
		Content:  strings.NewReader(content),
		Size:     int64(len(content)),
	}
	uploaded, err := svc.Upload(ctx, input)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	// Get content
	reader, file, err := svc.GetContent(ctx, uploaded.ID)
	if err != nil {
		t.Fatalf("failed to get content: %v", err)
	}
	defer reader.Close()

	// Verify file metadata
	if file.ID != uploaded.ID {
		t.Errorf("expected file ID %s, got %s", uploaded.ID, file.ID)
	}

	// Verify content
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read content: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected content %q, got %q", content, string(data))
	}
}

func TestFileService_GetContent_NotFound(t *testing.T) {
	svc, _, _, cleanup := setupFileService(t)
	defer cleanup()

	ctx := context.Background()

	_, _, err := svc.GetContent(ctx, "nonexistent-id")
	if !errors.Is(err, service.ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestFileService_GetContent_StorageError(t *testing.T) {
	svc, store, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	input := service.UploadInput{
		ShareID:  share.ID,
		Filename: "test.txt",
		Content:  strings.NewReader("hello world"),
		Size:     11,
	}
	uploaded, err := svc.Upload(ctx, input)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	// Force storage error
	store.getErr = errors.New("storage error")

	_, _, err = svc.GetContent(ctx, uploaded.ID)
	if err == nil {
		t.Error("expected error on storage failure")
	}
}

func TestFileService_IsPreviewable(t *testing.T) {
	svc, _, _, cleanup := setupFileService(t)
	defer cleanup()

	tests := []struct {
		mimeType    string
		previewable bool
	}{
		// Images
		{"image/jpeg", true},
		{"image/png", true},
		{"image/gif", true},
		{"image/svg+xml", true},
		{"image/webp", true},
		// PDF
		{"application/pdf", true},
		// Text
		{"text/plain", true},
		{"text/html", true},
		{"text/css", true},
		{"text/javascript", true},
		// Non-previewable
		{"application/zip", false},
		{"application/octet-stream", false},
		{"video/mp4", false},
		{"audio/mpeg", false},
		{"application/msword", false},
	}

	for _, tc := range tests {
		t.Run(tc.mimeType, func(t *testing.T) {
			file := &model.File{MimeType: tc.mimeType}
			result := svc.IsPreviewable(file)
			if result != tc.previewable {
				t.Errorf("expected IsPreviewable(%s) = %v, got %v", tc.mimeType, tc.previewable, result)
			}
		})
	}
}

func TestFileService_StorageKeyFormat(t *testing.T) {
	svc, _, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	input := service.UploadInput{
		ShareID:  share.ID,
		Filename: "my-file.txt",
		Content:  strings.NewReader("content"),
		Size:     7,
	}

	file, err := svc.Upload(ctx, input)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	// Storage key format: {shareID}/{fileID}/{filename}
	expectedFormat := share.ID + "/" + file.ID + "/my-file.txt"
	if file.StorageKey != expectedFormat {
		t.Errorf("expected storage key format %s, got %s", expectedFormat, file.StorageKey)
	}
}

func TestFileService_Upload_SpecialCharactersInFilename(t *testing.T) {
	svc, _, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()

	// Test with spaces and special characters
	filenames := []string{
		"my file.txt",
		"document (1).pdf",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
	}

	for _, filename := range filenames {
		t.Run(filename, func(t *testing.T) {
			input := service.UploadInput{
				ShareID:  share.ID,
				Filename: filename,
				Content:  strings.NewReader("content"),
				Size:     7,
			}

			file, err := svc.Upload(ctx, input)
			if err != nil {
				t.Fatalf("failed to upload file with name %q: %v", filename, err)
			}

			if file.Name != filename {
				t.Errorf("expected filename %q, got %q", filename, file.Name)
			}
		})
	}
}

// testDirectTransferStorage implements both storage.Storage and storage.DirectTransfer for testing.
type testDirectTransferStorage struct {
	testStorage
	presignUploadFn   func(ctx context.Context, key string, size int64, contentType string, expiry time.Duration) (*storage.PresignedURLResult, error)
	presignDownloadFn func(ctx context.Context, key string, disposition string, expiry time.Duration) (*storage.PresignedURLResult, error)
	statObjectFn      func(ctx context.Context, key string) (*storage.ObjectInfo, error)
}

func (s *testDirectTransferStorage) PresignUpload(ctx context.Context, key string, size int64, contentType string, expiry time.Duration) (*storage.PresignedURLResult, error) {
	if s.presignUploadFn != nil {
		return s.presignUploadFn(ctx, key, size, contentType, expiry)
	}
	return &storage.PresignedURLResult{
		URL:       "https://s3.example.com/presigned-put?key=" + key,
		Method:    "PUT",
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

func (s *testDirectTransferStorage) PresignDownload(ctx context.Context, key string, disposition string, expiry time.Duration) (*storage.PresignedURLResult, error) {
	if s.presignDownloadFn != nil {
		return s.presignDownloadFn(ctx, key, disposition, expiry)
	}
	return &storage.PresignedURLResult{
		URL:       "https://s3.example.com/presigned-get?key=" + key,
		Method:    "GET",
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

func (s *testDirectTransferStorage) StatObject(ctx context.Context, key string) (*storage.ObjectInfo, error) {
	if s.statObjectFn != nil {
		return s.statObjectFn(ctx, key)
	}
	data, ok := s.files[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &storage.ObjectInfo{
		Size:        int64(len(data)),
		ContentType: "application/octet-stream",
	}, nil
}

func setupDirectTransferService(t *testing.T) (*service.FileService, *testDirectTransferStorage, *model.Share, func()) {
	t.Helper()
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	userRepo := repository.NewUserRepository(db.DB())
	shareRepo := repository.NewShareRepository(db.DB())
	fileRepo := repository.NewFileRepository(db.DB())
	pendingUploadRepo := repository.NewPendingUploadRepository(db.DB())
	store := &testDirectTransferStorage{testStorage: testStorage{files: make(map[string][]byte)}}

	user := &model.User{
		ID:           "user-dt",
		Email:        "dt@example.com",
		PasswordHash: "hash",
		DisplayName:  "DT User",
	}
	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	share := &model.Share{
		ID:        "share-dt",
		CreatorID: &user.ID,
		Slug:      "dt-share",
		Name:      "DT Share",
	}
	if err := shareRepo.Create(context.Background(), share); err != nil {
		t.Fatalf("failed to create test share: %v", err)
	}

	fileService := service.NewFileService(fileRepo, shareRepo, store, pendingUploadRepo, 10*time.Minute)

	return fileService, store, share, func() { db.Close() }
}

func TestFileService_InitiateDirectUpload(t *testing.T) {
	svc, _, share, cleanup := setupDirectTransferService(t)
	defer cleanup()

	ctx := context.Background()
	input := service.DirectUploadInput{
		ShareID:     share.ID,
		UploaderID:  *share.CreatorID,
		Filename:    "direct.txt",
		Size:        2048,
		ContentType: "text/plain",
	}

	resp, err := svc.InitiateDirectUpload(ctx, input)
	if err != nil {
		t.Fatalf("InitiateDirectUpload failed: %v", err)
	}

	if resp.UploadID == "" {
		t.Error("expected upload_id to be set")
	}
	if resp.UploadURL == "" {
		t.Error("expected upload_url to be set")
	}
	if resp.FileID == "" {
		t.Error("expected file_id to be set")
	}
	if resp.Method != "PUT" {
		t.Errorf("expected method PUT, got %s", resp.Method)
	}
}

func TestFileService_InitiateDirectUpload_UnsupportedStorage(t *testing.T) {
	svc, _, _, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()
	input := service.DirectUploadInput{
		ShareID:  "share-123",
		Filename: "test.txt",
		Size:     1024,
	}

	_, err := svc.InitiateDirectUpload(ctx, input)
	if !errors.Is(err, service.ErrDirectTransferUnsupported) {
		t.Errorf("expected ErrDirectTransferUnsupported, got %v", err)
	}
}

func TestFileService_FinalizeDirectUpload(t *testing.T) {
	svc, store, share, cleanup := setupDirectTransferService(t)
	defer cleanup()

	ctx := context.Background()
	input := service.DirectUploadInput{
		ShareID:     share.ID,
		UploaderID:  *share.CreatorID,
		Filename:    "finalize.txt",
		Size:        5,
		ContentType: "text/plain",
	}

	resp, err := svc.InitiateDirectUpload(ctx, input)
	if err != nil {
		t.Fatalf("InitiateDirectUpload failed: %v", err)
	}

	// Simulate the client uploading to S3 by putting data in our mock store
	store.files[share.ID+"/"+resp.FileID+"/finalize.txt"] = []byte("hello")

	file, err := svc.FinalizeDirectUpload(ctx, resp.UploadID)
	if err != nil {
		t.Fatalf("FinalizeDirectUpload failed: %v", err)
	}

	if file.ID != resp.FileID {
		t.Errorf("expected file ID %s, got %s", resp.FileID, file.ID)
	}
	if file.Name != "finalize.txt" {
		t.Errorf("expected name finalize.txt, got %s", file.Name)
	}
	if file.Size != 5 {
		t.Errorf("expected size 5, got %d", file.Size)
	}
}

func TestFileService_FinalizeDirectUpload_AlreadyFinalized(t *testing.T) {
	svc, store, share, cleanup := setupDirectTransferService(t)
	defer cleanup()

	ctx := context.Background()
	input := service.DirectUploadInput{
		ShareID:    share.ID,
		UploaderID: *share.CreatorID,
		Filename:   "double.txt",
		Size:       3,
	}

	resp, _ := svc.InitiateDirectUpload(ctx, input)
	store.files[share.ID+"/"+resp.FileID+"/double.txt"] = []byte("abc")

	_, err := svc.FinalizeDirectUpload(ctx, resp.UploadID)
	if err != nil {
		t.Fatalf("first FinalizeDirectUpload failed: %v", err)
	}

	_, err = svc.FinalizeDirectUpload(ctx, resp.UploadID)
	if !errors.Is(err, service.ErrUploadAlreadyFinalized) {
		t.Errorf("expected ErrUploadAlreadyFinalized, got %v", err)
	}
}

func TestFileService_FinalizeDirectUpload_IntegrityCheckFailed(t *testing.T) {
	svc, store, share, cleanup := setupDirectTransferService(t)
	defer cleanup()

	ctx := context.Background()
	input := service.DirectUploadInput{
		ShareID:    share.ID,
		UploaderID: *share.CreatorID,
		Filename:   "mismatch.txt",
		Size:       100,
	}

	resp, _ := svc.InitiateDirectUpload(ctx, input)
	// Put data with wrong size
	store.files[share.ID+"/"+resp.FileID+"/mismatch.txt"] = []byte("short")

	_, err := svc.FinalizeDirectUpload(ctx, resp.UploadID)
	if !errors.Is(err, service.ErrIntegrityCheckFailed) {
		t.Errorf("expected ErrIntegrityCheckFailed, got %v", err)
	}
}

func TestFileService_FinalizeDirectUpload_ObjectNotUploaded(t *testing.T) {
	svc, _, share, cleanup := setupDirectTransferService(t)
	defer cleanup()

	ctx := context.Background()
	input := service.DirectUploadInput{
		ShareID:    share.ID,
		UploaderID: *share.CreatorID,
		Filename:   "missing.txt",
		Size:       100,
	}

	resp, _ := svc.InitiateDirectUpload(ctx, input)
	// Don't put any data in store

	_, err := svc.FinalizeDirectUpload(ctx, resp.UploadID)
	if !errors.Is(err, service.ErrIntegrityCheckFailed) {
		t.Errorf("expected ErrIntegrityCheckFailed, got %v", err)
	}
}

func TestFileService_GetPresignedDownloadURL(t *testing.T) {
	svc, _, share, cleanup := setupDirectTransferService(t)
	defer cleanup()

	ctx := context.Background()

	// Upload a file via the standard path first
	input := service.UploadInput{
		ShareID:    share.ID,
		UploaderID: *share.CreatorID,
		Filename:   "download-me.txt",
		Content:    strings.NewReader("content"),
		Size:       7,
	}
	uploaded, err := svc.Upload(ctx, input)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	resp, err := svc.GetPresignedDownloadURL(ctx, uploaded.ID, "attachment; filename=\"download-me.txt\"")
	if err != nil {
		t.Fatalf("GetPresignedDownloadURL failed: %v", err)
	}

	if resp.DownloadURL == "" {
		t.Error("expected download URL to be set")
	}
}

func TestFileService_GetPresignedDownloadURL_UnsupportedStorage(t *testing.T) {
	svc, _, share, cleanup := setupFileServiceWithUserAndShare(t)
	defer cleanup()

	ctx := context.Background()
	input := service.UploadInput{
		ShareID:    share.ID,
		UploaderID: *share.CreatorID,
		Filename:   "no-direct.txt",
		Content:    strings.NewReader("content"),
		Size:       7,
	}
	uploaded, _ := svc.Upload(ctx, input)

	_, err := svc.GetPresignedDownloadURL(ctx, uploaded.ID, "attachment")
	if !errors.Is(err, service.ErrDirectTransferUnsupported) {
		t.Errorf("expected ErrDirectTransferUnsupported, got %v", err)
	}
}
