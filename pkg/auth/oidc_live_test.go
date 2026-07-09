package auth

import (
	"os"
	"testing"
)

// TestOIDC_LiveOptional validates a real access token when env is set.
// Skip by default so CI stays offline. Phase D.4 live residual path.
//
//	OIDC_LIVE=1 OIDC_LIVE_ISSUER=... OIDC_LIVE_AUDIENCE=... OIDC_LIVE_TOKEN=... \
//	  go test ./pkg/auth/ -run TestOIDC_LiveOptional -count=1
func TestOIDC_LiveOptional(t *testing.T) {
	if os.Getenv("OIDC_LIVE") != "1" {
		t.Skip("set OIDC_LIVE=1 with issuer/audience/token for live OIDC smoke")
	}
	issuer := os.Getenv("OIDC_LIVE_ISSUER")
	aud := os.Getenv("OIDC_LIVE_AUDIENCE")
	tok := os.Getenv("OIDC_LIVE_TOKEN")
	if issuer == "" || aud == "" || tok == "" {
		t.Fatal("OIDC_LIVE_ISSUER, OIDC_LIVE_AUDIENCE, OIDC_LIVE_TOKEN required")
	}
	cfg := &AuthConfig{
		Type: "oidc",
		OIDC: &OIDCConfig{
			IssuerURL: issuer,
			Audience:  aud,
			JWKSURL:   os.Getenv("OIDC_LIVE_JWKS_URL"),
		},
	}
	am, err := NewAuthManager(cfg)
	if err != nil {
		t.Fatalf("NewAuthManager: %v", err)
	}
	validated, err := am.ValidateToken(tok)
	if err != nil {
		t.Fatalf("ValidateToken live: %v", err)
	}
	if validated == nil || !validated.Valid {
		t.Fatal("token not valid")
	}
	sub, _ := am.ExtractSubject(validated)
	t.Logf("OIDC_LIVE_OK sub=%s", sub)
}
