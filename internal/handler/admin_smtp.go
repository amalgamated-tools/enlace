package handler

import (
	"maps"
	"net/http"
	"strconv"
	"strings"

	"github.com/amalgamated-tools/enlace/internal/crypto"
)

// smtpSettingKeys are the known SMTP-related setting keys.
var smtpSettingKeys = []string{
	"smtp_host", "smtp_port", "smtp_user", "smtp_pass", "smtp_from", "smtp_tls_policy",
}

// SMTPConfigHandler handles admin SMTP configuration HTTP requests.
type SMTPConfigHandler struct {
	settingsRepo  SettingsRepositoryInterface
	encryptionKey []byte
}

// NewSMTPConfigHandler creates a new SMTPConfigHandler.
func NewSMTPConfigHandler(settingsRepo SettingsRepositoryInterface, jwtSecret []byte) *SMTPConfigHandler {
	return &SMTPConfigHandler{
		settingsRepo:  settingsRepo,
		encryptionKey: crypto.DeriveKey(jwtSecret, crypto.SMTPEncryptionSalt),
	}
}

// smtpConfigResponse represents the GET response for SMTP configuration.
type smtpConfigResponse struct {
	Host      string `json:"smtp_host"`
	Port      string `json:"smtp_port,omitempty"`
	User      string `json:"smtp_user,omitempty"`
	PassSet   bool   `json:"smtp_pass_set"`
	From      string `json:"smtp_from,omitempty"`
	TLSPolicy string `json:"smtp_tls_policy,omitempty"`
}

// updateSMTPConfigRequest represents the PUT request body for updating SMTP configuration.
type updateSMTPConfigRequest struct {
	Host      *string `json:"smtp_host"`
	Port      *string `json:"smtp_port"`
	User      *string `json:"smtp_user"`
	Pass      *string `json:"smtp_pass"`
	From      *string `json:"smtp_from"`
	TLSPolicy *string `json:"smtp_tls_policy"`
}

// GetSMTPConfig handles GET /api/v1/admin/smtp - returns current DB SMTP settings.
//
//	@Summary		Get SMTP configuration
//	@Description	Returns SMTP configuration overrides stored in the database. Requires admin role. The smtp_pass value is never returned; use smtp_pass_set to check if one is configured.
//	@Tags			admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	APIResponse{data=smtpConfigResponse}
//	@Failure		401	{object}	APIResponse
//	@Failure		403	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/admin/smtp [get]
func (h *SMTPConfigHandler) GetSMTPConfig(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsRepo.GetMultiple(r.Context(), smtpSettingKeys)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve SMTP settings")
		return
	}

	resp := smtpConfigResponse{
		Host:      settings["smtp_host"],
		Port:      settings["smtp_port"],
		User:      settings["smtp_user"],
		PassSet:   len(settings["smtp_pass"]) > 0,
		From:      settings["smtp_from"],
		TLSPolicy: settings["smtp_tls_policy"],
	}

	Success(w, http.StatusOK, resp)
}

// UpdateSMTPConfig handles PUT /api/v1/admin/smtp - updates DB SMTP settings.
//
//	@Summary		Update SMTP configuration
//	@Description	Updates SMTP configuration overrides in the database. Only provided fields are updated. Requires admin role. Changes take effect after restart.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		updateSMTPConfigRequest	true	"SMTP configuration fields to update"
//	@Success		200		{object}	APIResponse{data=smtpConfigResponse}
//	@Failure		400		{object}	ValidationErrorResponse
//	@Failure		401		{object}	APIResponse
//	@Failure		403		{object}	APIResponse
//	@Failure		500		{object}	APIResponse
//	@Router			/api/v1/admin/smtp [put]
func (h *SMTPConfigHandler) UpdateSMTPConfig(w http.ResponseWriter, r *http.Request) {
	var req updateSMTPConfigRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate TLS policy if provided
	if req.TLSPolicy != nil {
		policy := strings.TrimSpace(strings.ToLower(*req.TLSPolicy))
		if policy != "opportunistic" && policy != "mandatory" && policy != "none" {
			ValidationError(w, map[string]string{
				"smtp_tls_policy": "must be 'opportunistic', 'mandatory', or 'none'",
			})
			return
		}
	}

	// Validate port if provided
	if req.Port != nil {
		portStr := strings.TrimSpace(*req.Port)
		if portStr != "" {
			port, err := strconv.Atoi(portStr)
			if err != nil || port < 1 || port > 65535 {
				ValidationError(w, map[string]string{
					"smtp_port": "must be a valid port number (1-65535)",
				})
				return
			}
		}
	}

	// Build the map of settings to upsert (only non-nil fields)
	toSet := make(map[string]string)
	if req.Host != nil {
		toSet["smtp_host"] = strings.TrimSpace(*req.Host)
	}
	if req.Port != nil {
		toSet["smtp_port"] = strings.TrimSpace(*req.Port)
	}
	if req.User != nil {
		toSet["smtp_user"] = strings.TrimSpace(*req.User)
	}
	if req.Pass != nil {
		passVal := strings.TrimSpace(*req.Pass)
		if passVal != "" {
			encrypted, err := crypto.Encrypt(passVal, h.encryptionKey)
			if err != nil {
				Error(w, http.StatusInternalServerError, "failed to encrypt password")
				return
			}
			toSet["smtp_pass"] = encrypted
		} else {
			toSet["smtp_pass"] = ""
		}
	}
	if req.From != nil {
		toSet["smtp_from"] = strings.TrimSpace(*req.From)
	}
	if req.TLSPolicy != nil {
		toSet["smtp_tls_policy"] = strings.TrimSpace(strings.ToLower(*req.TLSPolicy))
	}

	if len(toSet) == 0 {
		Error(w, http.StatusBadRequest, "no settings to update")
		return
	}

	// Validate the effective configuration by merging existing DB settings with incoming updates.
	existing, err := h.settingsRepo.GetMultiple(r.Context(), smtpSettingKeys)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to retrieve current SMTP settings")
		return
	}
	effective := make(map[string]string)
	maps.Copy(effective, existing)
	maps.Copy(effective, toSet)

	if fieldErrors := validateEffectiveSMTPConfig(effective); len(fieldErrors) > 0 {
		ValidationError(w, fieldErrors)
		return
	}

	if err := h.settingsRepo.SetMultiple(r.Context(), toSet); err != nil {
		Error(w, http.StatusInternalServerError, "failed to update SMTP settings")
		return
	}

	// Re-read from DB to return current state
	settings, err := h.settingsRepo.GetMultiple(r.Context(), smtpSettingKeys)
	if err != nil {
		Error(w, http.StatusInternalServerError, "settings saved but failed to retrieve updated state")
		return
	}

	resp := smtpConfigResponse{
		Host:      settings["smtp_host"],
		Port:      settings["smtp_port"],
		User:      settings["smtp_user"],
		PassSet:   len(settings["smtp_pass"]) > 0,
		From:      settings["smtp_from"],
		TLSPolicy: settings["smtp_tls_policy"],
	}

	Success(w, http.StatusOK, resp)
}

// DeleteSMTPConfig handles DELETE /api/v1/admin/smtp - clears all DB SMTP overrides.
//
//	@Summary		Delete SMTP configuration
//	@Description	Removes all SMTP configuration overrides from the database, reverting to environment variable configuration on next restart. Requires admin role.
//	@Tags			admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	APIResponse
//	@Failure		401	{object}	APIResponse
//	@Failure		403	{object}	APIResponse
//	@Failure		500	{object}	APIResponse
//	@Router			/api/v1/admin/smtp [delete]
func (h *SMTPConfigHandler) DeleteSMTPConfig(w http.ResponseWriter, r *http.Request) {
	if err := h.settingsRepo.DeleteMultiple(r.Context(), smtpSettingKeys); err != nil {
		Error(w, http.StatusInternalServerError, "failed to delete SMTP settings")
		return
	}

	Success(w, http.StatusOK, nil)
}

// validateEffectiveSMTPConfig checks that the merged configuration has all
// required fields when SMTP host is set. This prevents saving an incomplete
// config that would cause issues at startup.
func validateEffectiveSMTPConfig(effective map[string]string) map[string]string {
	errs := make(map[string]string)

	host := effective["smtp_host"]
	if host == "" {
		// No host configured; env vars will be used at startup.
		return errs
	}

	// When host is set, from address is required
	if effective["smtp_from"] == "" {
		errs["smtp_from"] = "required when SMTP host is set"
	}

	// Validate port if present
	if portStr := effective["smtp_port"]; portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			errs["smtp_port"] = "must be a valid port number (1-65535)"
		}
	}

	// Validate TLS policy if present
	if policy := effective["smtp_tls_policy"]; policy != "" {
		switch strings.ToLower(policy) {
		case "opportunistic", "mandatory", "none":
			// valid
		default:
			errs["smtp_tls_policy"] = "must be 'opportunistic', 'mandatory', or 'none'"
		}
	}

	return errs
}
