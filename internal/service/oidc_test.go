package service_test

import (
	"testing"

	"github.com/amalgamated-tools/sharer/internal/config"
	"github.com/amalgamated-tools/sharer/internal/service"
)

func TestNewOIDCService_Disabled(t *testing.T) {
	cfg := &config.Config{
		OIDCEnabled: false,
	}

	svc, err := service.NewOIDCService(cfg, nil)
	if err != nil {
		t.Fatalf("expected no error for disabled OIDC, got %v", err)
	}
	if svc != nil {
		t.Error("expected nil service when OIDC is disabled")
	}
}

func TestOIDCService_GenerateState(t *testing.T) {
	// Test that GenerateState returns a non-empty string
	// This can't test the full OIDC flow without a real provider
	t.Skip("requires mock OIDC provider for full testing")
}
