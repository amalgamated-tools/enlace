package service

import "github.com/amalgamated-tools/enlace/internal/repository"

// NewOIDCServiceForTest creates an OIDCService without provider discovery, for testing
// service methods that only require userRepo and issuerURL.
func NewOIDCServiceForTest(userRepo *repository.UserRepository, issuerURL string, totpDisabler TOTPDisabler) *OIDCService {
	return &OIDCService{
		userRepo:     userRepo,
		issuerURL:    issuerURL,
		totpDisabler: totpDisabler,
	}
}
