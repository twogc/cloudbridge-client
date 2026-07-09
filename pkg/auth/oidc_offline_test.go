package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Phase D.4 offline: mock OIDC discovery + JWKS + RS256 token validate (no external network).
func TestOIDC_Offline_ValidateRS256Token(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "test-kid-1"
	jwksJSON := rsaPublicToJWKS(t, &priv.PublicKey, kid)

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// issuer filled after server starts — use Host from request
		iss := "http://" + r.Host
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   iss,
			"jwks_uri": iss + "/.well-known/jwks.json",
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksJSON)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	issuer := srv.URL
	audience := "cloudbridge-client-test"
	am, err := NewAuthManager(&AuthConfig{
		Type: "oidc",
		OIDC: &OIDCConfig{
			IssuerURL: issuer,
			Audience:  audience,
			// discovery path: leave JWKSURL empty to exercise setupOIDC discovery
		},
	})
	if err != nil {
		t.Fatalf("NewAuthManager oidc: %v", err)
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":       issuer,
		"aud":       audience,
		"sub":       "user-oidc-1",
		"tenant_id": "tenant-oidc",
		"peer_id":   "peer-oidc-1",
		"iat":       now.Unix(),
		"nbf":       now.Add(-time.Minute).Unix(),
		"exp":       now.Add(time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}

	validated, err := am.ValidateToken(signed)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if !validated.Valid {
		t.Fatal("token not valid")
	}
	sub, err := am.ExtractSubject(validated)
	if err != nil || sub != "user-oidc-1" {
		t.Fatalf("sub=%q err=%v", sub, err)
	}
	tid, err := am.ExtractTenantID(validated)
	if err != nil || tid != "tenant-oidc" {
		t.Fatalf("tenant=%q err=%v", tid, err)
	}
}

func TestOIDC_Offline_RejectWrongAudience(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "kid-aud"
	jwksJSON := rsaPublicToJWKS(t, &priv.PublicKey, kid)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/.well-known/jwks.json" {
			_, _ = w.Write(jwksJSON)
			return
		}
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(srv.Close)

	am, err := NewAuthManager(&AuthConfig{
		Type: "oidc",
		OIDC: &OIDCConfig{
			IssuerURL: srv.URL,
			Audience:  "expected-aud",
			JWKSURL:   srv.URL + "/.well-known/jwks.json",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": srv.URL,
		"aud": "wrong-aud",
		"sub": "u1",
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := am.ValidateToken(signed); err == nil {
		t.Fatal("expected audience rejection")
	}
}

func TestOIDC_Offline_RejectBadSignature(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "kid-sig"
	jwksJSON := rsaPublicToJWKS(t, &priv.PublicKey, kid)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksJSON)
	}))
	t.Cleanup(srv.Close)

	am, err := NewAuthManager(&AuthConfig{
		Type: "oidc",
		OIDC: &OIDCConfig{
			IssuerURL: srv.URL,
			Audience:  "aud",
			JWKSURL:   srv.URL + "/jwks",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": srv.URL, "aud": "aud", "sub": "u",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	// sign with wrong key
	signed, err := tok.SignedString(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := am.ValidateToken(signed); err == nil {
		t.Fatal("expected signature rejection")
	}
}

func rsaPublicToJWKS(t *testing.T, pub *rsa.PublicKey, kid string) []byte {
	t.Helper()
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	body, err := json.Marshal(map[string]interface{}{
		"keys": []map[string]string{{
			"kty": "RSA",
			"kid": kid,
			"use": "sig",
			"alg": "RS256",
			"n":   n,
			"e":   e,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
