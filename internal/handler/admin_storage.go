package handler

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"strings"

	"github.com/amalgamated-tools/enlace/internal/crypto"
	"github.com/amalgamated-tools/enlace/internal/storage"
)

// storageSettingKeys are the known storage-related setting keys.
var storageSettingKeys = []string{
	"storage_type", "storage_local_path",
	"s3_endpoint", "s3_bucket", "s3_access_key", "s3_secret_key", "s3_region", "s3_path_prefix",
}

// SettingsRepositoryInterface defines the interface for settings repository operations.
type SettingsRepositoryInterface interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetMultiple(ctx context.Context, keys []string) (map[string]string, error)
	SetMultiple(ctx context.Context, settings map[string]string) error
	DeleteMultiple(ctx context.Context, keys []string) error
}

// S3Connector can validate connectivity to an S3-compatible service.
type S3Connector interface {
	ValidateConnection(ctx context.Context) error
}

// S3StorageFactory creates an S3Connector for the given configuration.
// It is called during TestStorageConnection to validate credentials.
type S3StorageFactory func(ctx context.Context, cfg storage.S3Config) (S3Connector, error)

// StorageConfigHandler handles admin storage configuration HTTP requests.
type StorageConfigHandler struct {
	settingsRepo  SettingsRepositoryInterface
	encryptionKey []byte
	newS3Storage  S3StorageFactory
}

// NewStorageConfigHandler creates a new StorageConfigHandler.
func NewStorageConfigHandler(settingsRepo SettingsRepositoryInterface, jwtSecret []byte) *StorageConfigHandler {
	return &StorageConfigHandler{
		settingsRepo:  settingsRepo,
		encryptionKey: crypto.DeriveKey(jwtSecret, crypto.StorageEncryptionSalt),
		newS3Storage: func(ctx context.Context, cfg storage.S3Config) (S3Connector, error) {
			return storage.NewS3Storage(ctx, cfg)
		},
	}
}

// WithS3StorageFactory replaces the default S3 storage factory used by TestStorageConnection.
// This must be called before the handler is registered with a router.
// Intended for testing only.
func (h *StorageConfigHandler) WithS3StorageFactory(factory S3StorageFactory) {
	h.newS3Storage = factory
}

// storageConfigResponse represents the GET response for storage configuration.
type storageConfigResponse struct {
	StorageType      string `json:"storage_type"`
	StorageLocalPath string `json:"storage_local_path,omitempty"`
	S3Endpoint       string `json:"s3_endpoint,omitempty"`
	S3Bucket         string `json:"s3_bucket,omitempty"`
	S3AccessKey      string `json:"s3_access_key,omitempty"`
	S3SecretKeySet   bool   `json:"s3_secret_key_set"`
	S3Region         string `json:"s3_region,omitempty"`
	S3PathPrefix     string `json:"s3_path_prefix,omitempty"`
}

// updateStorageConfigRequest represents the PUT request body for updating storage configuration.
type updateStorageConfigRequest struct {
	StorageType      *string `json:"storage_type"`
	StorageLocalPath *string `json:"storage_local_path"`
	S3Endpoint       *string `json:"s3_endpoint"`
	S3Bucket         *string `json:"s3_bucket"`
	S3AccessKey      *string `json:"s3_access_key"`
	S3SecretKey      *string `json:"s3_secret_key"`
	S3Region         *string `json:"s3_region"`
	S3PathPrefix     *string `json:"s3_path_prefix"`
}

// GetStorageConfig handles GET /api/v1/admin/storage - returns current DB storage settings.
//
//	@Summary		Get storage configuration
//	@Description	Returns storage configuration overrides stored in the database. Requires admin role. The s3_secret_key value is never returned; use s3_secret_key_set to check if one is configured.
//	@Tags			admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	APIResponse{data=storageConfigResponse}
//	@Failure		401	{object}	APIResponse
//	@Failure		403	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/admin/storage [get]
func (h *StorageConfigHandler) GetStorageConfig(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsRepo.GetMultiple(r.Context(), storageSettingKeys)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve storage settings")
		return
	}

	resp := storageConfigResponse{
		StorageType:      settings["storage_type"],
		StorageLocalPath: settings["storage_local_path"],
		S3Endpoint:       settings["s3_endpoint"],
		S3Bucket:         settings["s3_bucket"],
		S3AccessKey:      settings["s3_access_key"],
		S3SecretKeySet:   len(settings["s3_secret_key"]) > 0,
		S3Region:         settings["s3_region"],
		S3PathPrefix:     settings["s3_path_prefix"],
	}

	Success(w, http.StatusOK, resp)
}

// UpdateStorageConfig handles PUT /api/v1/admin/storage - updates DB storage settings.
//
//	@Summary		Update storage configuration
//	@Description	Updates storage configuration overrides in the database. Only provided fields are updated. Requires admin role. Changes take effect after restart.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		updateStorageConfigRequest	true	"Storage configuration fields to update"
//	@Success		200		{object}	APIResponse{data=storageConfigResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		403		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/admin/storage [put]
func (h *StorageConfigHandler) UpdateStorageConfig(w http.ResponseWriter, r *http.Request) {
	var req updateStorageConfigRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate storage_type if provided
	if req.StorageType != nil {
		st := strings.TrimSpace(*req.StorageType)
		if st != "local" && st != "s3" {
			ValidationError(w, map[string]string{
				"storage_type": "must be 'local' or 's3'",
			})
			return
		}
	}

	// Build the map of settings to upsert (only non-nil fields)
	toSet := make(map[string]string)
	if req.StorageType != nil {
		toSet["storage_type"] = strings.TrimSpace(*req.StorageType)
	}
	if req.StorageLocalPath != nil {
		toSet["storage_local_path"] = strings.TrimSpace(*req.StorageLocalPath)
	}
	if req.S3Endpoint != nil {
		toSet["s3_endpoint"] = strings.TrimSpace(*req.S3Endpoint)
	}
	if req.S3Bucket != nil {
		toSet["s3_bucket"] = strings.TrimSpace(*req.S3Bucket)
	}
	if req.S3AccessKey != nil {
		toSet["s3_access_key"] = strings.TrimSpace(*req.S3AccessKey)
	}
	if req.S3SecretKey != nil {
		secretVal := strings.TrimSpace(*req.S3SecretKey)
		if secretVal != "" {
			encrypted, err := crypto.Encrypt(secretVal, h.encryptionKey)
			if err != nil {
				Error(w, http.StatusInternalServerError, "failed to encrypt secret key")
				return
			}
			toSet["s3_secret_key"] = encrypted
		} else {
			toSet["s3_secret_key"] = ""
		}
	}
	if req.S3Region != nil {
		toSet["s3_region"] = strings.TrimSpace(*req.S3Region)
	}
	if req.S3PathPrefix != nil {
		toSet["s3_path_prefix"] = strings.TrimSpace(*req.S3PathPrefix)
	}

	if len(toSet) == 0 {
		Error(w, http.StatusBadRequest, "no settings to update")
		return
	}

	// Validate the effective configuration by merging existing DB settings with incoming updates.
	// This prevents saving an incomplete S3 config that would cause a startup failure.
	existing, err := h.settingsRepo.GetMultiple(r.Context(), storageSettingKeys)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve current storage settings")
		return
	}
	effective := make(map[string]string)
	maps.Copy(effective, existing)
	maps.Copy(effective, toSet)

	if fieldErrors := validateEffectiveStorageConfig(effective); len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	if err := h.settingsRepo.SetMultiple(r.Context(), toSet); err != nil {
		Error(w, http.StatusInternalServerError, "failed to update storage settings")
		return
	}

	// Re-read from DB to return current state
	settings, err := h.settingsRepo.GetMultiple(r.Context(), storageSettingKeys)
	if err != nil {
		Error(w, http.StatusInternalServerError, "settings saved but failed to retrieve updated state")
		return
	}

	resp := storageConfigResponse{
		StorageType:      settings["storage_type"],
		StorageLocalPath: settings["storage_local_path"],
		S3Endpoint:       settings["s3_endpoint"],
		S3Bucket:         settings["s3_bucket"],
		S3AccessKey:      settings["s3_access_key"],
		S3SecretKeySet:   len(settings["s3_secret_key"]) > 0,
		S3Region:         settings["s3_region"],
		S3PathPrefix:     settings["s3_path_prefix"],
	}

	Success(w, http.StatusOK, resp)
}

// DeleteStorageConfig handles DELETE /api/v1/admin/storage - clears all DB storage overrides.
//
//	@Summary		Delete storage configuration
//	@Description	Removes all storage configuration overrides from the database, reverting to environment variable configuration on next restart. Requires admin role.
//	@Tags			admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	APIResponse
//	@Failure		401	{object}	APIResponse
//	@Failure		403	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/admin/storage [delete]
func (h *StorageConfigHandler) DeleteStorageConfig(w http.ResponseWriter, r *http.Request) {
	if err := h.settingsRepo.DeleteMultiple(r.Context(), storageSettingKeys); err != nil {
		Error(w, http.StatusInternalServerError, "failed to delete storage settings")
		return
	}

	Success(w, http.StatusOK, nil)
}

// validateEffectiveStorageConfig checks that the merged configuration has all
// required fields for the effective storage type. This prevents saving an
// incomplete config that would cause a startup failure.
func validateEffectiveStorageConfig(effective map[string]string) map[string]string {
	errs := make(map[string]string)

	storageType := effective["storage_type"]
	if storageType == "" {
		// No storage type configured; env vars will be used at startup.
		return errs
	}

	switch storageType {
	case "s3":
		if effective["s3_bucket"] == "" {
			errs["s3_bucket"] = "required when storage_type is 's3'"
		}
		if effective["s3_access_key"] == "" {
			errs["s3_access_key"] = "required when storage_type is 's3'"
		}
		if effective["s3_secret_key"] == "" {
			errs["s3_secret_key"] = "required when storage_type is 's3'"
		}
	case "local":
		if effective["storage_local_path"] == "" {
			errs["storage_local_path"] = "required when storage_type is 'local'"
		}
	}

	return errs
}

// TestStorageConnection handles POST /api/v1/admin/storage/test - validates S3 connectivity.
//
//	@Summary		Test S3 connection
//	@Description	Tests the S3 connection by performing a HeadBucket operation using the provided credentials merged with existing DB settings. Requires admin role.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		updateStorageConfigRequest	true	"S3 configuration fields to test (merged with existing DB settings)"
//	@Success		200		{object}	APIResponse
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		403		{object}	APIResponse
//	@Failure		422		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/admin/storage/test [post]
func (h *StorageConfigHandler) TestStorageConnection(w http.ResponseWriter, r *http.Request) {
	var req updateStorageConfigRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Build the effective configuration by merging existing DB settings with the request.
	existing, err := h.settingsRepo.GetMultiple(r.Context(), storageSettingKeys)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve current storage settings")
		return
	}

	effective := make(map[string]string)
	for k, v := range existing {
		effective[k] = v
	}
	if req.S3Endpoint != nil {
		effective["s3_endpoint"] = strings.TrimSpace(*req.S3Endpoint)
	}
	if req.S3Bucket != nil {
		effective["s3_bucket"] = strings.TrimSpace(*req.S3Bucket)
	}
	if req.S3AccessKey != nil {
		effective["s3_access_key"] = strings.TrimSpace(*req.S3AccessKey)
	}
	if req.S3SecretKey != nil {
		effective["s3_secret_key"] = strings.TrimSpace(*req.S3SecretKey)
	}
	if req.S3Region != nil {
		effective["s3_region"] = strings.TrimSpace(*req.S3Region)
	}
	if req.S3PathPrefix != nil {
		effective["s3_path_prefix"] = strings.TrimSpace(*req.S3PathPrefix)
	}

	// Validate required S3 fields are present.
	errs := make(map[string]string)
	if effective["s3_bucket"] == "" {
		errs["s3_bucket"] = "required for connection test"
	}
	if effective["s3_access_key"] == "" {
		errs["s3_access_key"] = "required for connection test"
	}
	if effective["s3_secret_key"] == "" {
		errs["s3_secret_key"] = "required for connection test"
	}
	if len(errs) > 0 {
		ValidationError(w, errs)
		return
	}

	// Handle the secret key, decrypting only when appropriate. If decryption
	// fails (e.g., for a user-provided plaintext that happens to start with
	// "enc:"), fall back to using the original value as plaintext so that the
	// connection test can still proceed.
	secretKey := effective["s3_secret_key"]
	decrypted := secretKey
	if strings.HasPrefix(secretKey, "enc:") {
		if v, err := crypto.Decrypt(secretKey, h.encryptionKey); err == nil {
			decrypted = v
		}
	}

	s3Store, err := h.newS3Storage(r.Context(), storage.S3Config{
		Endpoint:   effective["s3_endpoint"],
		Bucket:     effective["s3_bucket"],
		AccessKey:  effective["s3_access_key"],
		SecretKey:  decrypted,
		Region:     effective["s3_region"],
		PathPrefix: effective["s3_path_prefix"],
	})
	if err != nil {
		slog.WarnContext(r.Context(), "failed to initialize S3 client", "error", err)
		Error(w, http.StatusUnprocessableEntity, "failed to initialize S3 client")
		return
	}

	if err := s3Store.ValidateConnection(r.Context()); err != nil {
		slog.WarnContext(r.Context(), "S3 connection test failed", "error", err)
		Error(w, http.StatusUnprocessableEntity, s3ConnectionErrorMessage(err))
		return
	}

	Success(w, http.StatusOK, nil)
}

// s3ConnectionErrorMessage maps an S3 error to a user-facing message that does not
// expose internal endpoint or SDK details.
func s3ConnectionErrorMessage(err error) string {
	var apiErr interface {
		ErrorCode() string
	}
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchBucket":
			return "S3 connection failed: bucket not found"
		case "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch":
			return "S3 connection failed: authentication failed"
		}
	}
	return "S3 connection failed"
}
