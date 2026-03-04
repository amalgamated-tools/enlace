package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// fileRestrictionSettingKeys are the known file-restriction setting keys.
var fileRestrictionSettingKeys = []string{
	"max_file_size", "blocked_extensions",
}

// FileRestrictionsHandler handles admin file restriction configuration HTTP requests.
type FileRestrictionsHandler struct {
	settingsRepo SettingsRepositoryInterface
}

// NewFileRestrictionsHandler creates a new FileRestrictionsHandler.
func NewFileRestrictionsHandler(settingsRepo SettingsRepositoryInterface) *FileRestrictionsHandler {
	return &FileRestrictionsHandler{settingsRepo: settingsRepo}
}

// fileRestrictionsResponse represents the GET response for file restriction configuration.
type fileRestrictionsResponse struct {
	MaxFileSize       *int64   `json:"max_file_size"`
	BlockedExtensions []string `json:"blocked_extensions"`
}

// updateFileRestrictionsRequest represents the PUT request body for updating file restrictions.
type updateFileRestrictionsRequest struct {
	MaxFileSize       *int64  `json:"max_file_size"`
	BlockedExtensions *string `json:"blocked_extensions"`
}

// GetFileRestrictions handles GET /api/v1/admin/files - returns current file restriction settings.
//
//	@Summary		Get file restriction configuration
//	@Description	Returns file upload restriction overrides stored in the database. Requires admin role. A null max_file_size means the default (100 MB) is used.
//	@Tags			admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	APIResponse{data=fileRestrictionsResponse}
//	@Failure		401	{object}	APIResponse
//	@Failure		403	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/admin/files [get]
func (h *FileRestrictionsHandler) GetFileRestrictions(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsRepo.GetMultiple(r.Context(), fileRestrictionSettingKeys)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve file restriction settings")
		return
	}

	resp := buildFileRestrictionsResponse(settings)
	Success(w, http.StatusOK, resp)
}

// UpdateFileRestrictions handles PUT /api/v1/admin/files - updates file restriction settings.
//
//	@Summary		Update file restriction configuration
//	@Description	Updates file upload restriction overrides in the database. Only provided fields are updated. Requires admin role. Changes take effect immediately.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		updateFileRestrictionsRequest	true	"File restriction fields to update"
//	@Success		200		{object}	APIResponse{data=fileRestrictionsResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		403		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/admin/files [put]
func (h *FileRestrictionsHandler) UpdateFileRestrictions(w http.ResponseWriter, r *http.Request) {
	var req updateFileRestrictionsRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate max_file_size if provided
	if req.MaxFileSize != nil && *req.MaxFileSize <= 0 {
		ValidationError(w, map[string]string{
			"max_file_size": "must be a positive integer",
		})
		return
	}

	// Build the map of settings to upsert (only non-nil fields)
	toSet := make(map[string]string)
	if req.MaxFileSize != nil {
		toSet["max_file_size"] = strconv.FormatInt(*req.MaxFileSize, 10)
	}
	if req.BlockedExtensions != nil {
		toSet["blocked_extensions"] = normalizeExtensions(*req.BlockedExtensions)
	}

	if len(toSet) == 0 {
		Error(w, http.StatusBadRequest, "no settings to update")
		return
	}

	if err := h.settingsRepo.SetMultiple(r.Context(), toSet); err != nil {
		Error(w, http.StatusInternalServerError, "failed to update file restriction settings")
		return
	}

	// Re-read from DB to return current state
	settings, err := h.settingsRepo.GetMultiple(r.Context(), fileRestrictionSettingKeys)
	if err != nil {
		Error(w, http.StatusInternalServerError, "settings saved but failed to retrieve updated state")
		return
	}

	resp := buildFileRestrictionsResponse(settings)
	Success(w, http.StatusOK, resp)
}

// DeleteFileRestrictions handles DELETE /api/v1/admin/files - clears all file restriction overrides.
//
//	@Summary		Delete file restriction configuration
//	@Description	Removes all file restriction overrides from the database, reverting to default behavior. Requires admin role.
//	@Tags			admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	APIResponse
//	@Failure		401	{object}	APIResponse
//	@Failure		403	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/admin/files [delete]
func (h *FileRestrictionsHandler) DeleteFileRestrictions(w http.ResponseWriter, r *http.Request) {
	if err := h.settingsRepo.DeleteMultiple(r.Context(), fileRestrictionSettingKeys); err != nil {
		Error(w, http.StatusInternalServerError, "failed to delete file restriction settings")
		return
	}

	Success(w, http.StatusOK, nil)
}

// buildFileRestrictionsResponse converts raw DB settings to the API response.
func buildFileRestrictionsResponse(settings map[string]string) fileRestrictionsResponse {
	resp := fileRestrictionsResponse{
		BlockedExtensions: []string{},
	}

	if v, ok := settings["max_file_size"]; ok && v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err == nil && parsed > 0 {
			resp.MaxFileSize = &parsed
		}
	}

	if v, ok := settings["blocked_extensions"]; ok && v != "" {
		for _, ext := range strings.Split(v, ",") {
			ext = strings.TrimSpace(strings.ToLower(ext))
			if ext != "" {
				resp.BlockedExtensions = append(resp.BlockedExtensions, ext)
			}
		}
	}

	return resp
}

// normalizeExtensions takes a comma-separated string of extensions, trims whitespace,
// lowercases, ensures a dot prefix, and returns the normalized comma-separated string.
func normalizeExtensions(raw string) string {
	parts := strings.Split(raw, ",")
	normalized := make([]string, 0, len(parts))
	for _, ext := range parts {
		ext = strings.TrimSpace(strings.ToLower(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		normalized = append(normalized, ext)
	}
	return strings.Join(normalized, ",")
}

// LoadFileRestrictions reads current file restriction settings from the database.
// Returns maxFileSize (0 means use default) and blockedExtensions (nil means none).
func LoadFileRestrictions(ctx context.Context, settingsRepo SettingsRepositoryInterface) (maxFileSize int64, blockedExtensions []string, err error) {
	settings, err := settingsRepo.GetMultiple(ctx, fileRestrictionSettingKeys)
	if err != nil {
		return 0, nil, err
	}

	if v, ok := settings["max_file_size"]; ok && v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err == nil && parsed > 0 {
			maxFileSize = parsed
		}
	}

	if v, ok := settings["blocked_extensions"]; ok && v != "" {
		for _, ext := range strings.Split(v, ",") {
			ext = strings.TrimSpace(strings.ToLower(ext))
			if ext != "" {
				blockedExtensions = append(blockedExtensions, ext)
			}
		}
	}

	return maxFileSize, blockedExtensions, nil
}

// IsExtensionBlocked checks whether a filename has a blocked extension.
func IsExtensionBlocked(filename string, blockedExtensions []string) bool {
	if len(blockedExtensions) == 0 {
		return false
	}
	lower := strings.ToLower(filename)
	for _, ext := range blockedExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// loadEffectiveRestrictions reads file restrictions from the settings repo and returns
// the effective max file size and blocked extensions. On error, logs a warning and
// falls back to the provided default max file size with no blocked extensions.
func loadEffectiveRestrictions(ctx context.Context, settingsRepo SettingsRepositoryInterface, defaultMaxSize int64) (int64, []string) {
	if settingsRepo == nil {
		return defaultMaxSize, nil
	}

	adminMax, blocked, err := LoadFileRestrictions(ctx, settingsRepo)
	if err != nil {
		slog.WarnContext(ctx, "failed to load file restrictions from database, using defaults", "error", err)
		return defaultMaxSize, nil
	}

	effectiveMax := defaultMaxSize
	if adminMax > 0 {
		effectiveMax = adminMax
	}

	return effectiveMax, blocked
}
