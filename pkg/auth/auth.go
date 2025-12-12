package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents JWT claims with tenant and P2P support
type Claims struct {
	Subject        string         `json:"sub"`
	TenantID       string         `json:"tenant_id,omitempty"`
	OrgID          string         `json:"org_id,omitempty"`
	Permissions    []string       `json:"permissions,omitempty"`
	ConnectionType string         `json:"connection_type,omitempty"`
	QUICConfig     *QUICConfig    `json:"quic_config,omitempty"`
	MeshConfig     *MeshConfig    `json:"mesh_config,omitempty"`
	PeerWhitelist  *PeerWhitelist `json:"peer_whitelist,omitempty"`
	NetworkConfig  *NetworkConfig `json:"network_config,omitempty"`
	Issuer         string         `json:"iss,omitempty"`
	Audience       string         `json:"aud,omitempty"`
	ExpiresAt      int64          `json:"exp,omitempty"`
	IssuedAt       int64          `json:"iat,omitempty"`
	NotBefore      int64          `json:"nbf,omitempty"`
	jwt.RegisteredClaims
}

// QUICConfig represents QUIC configuration from JWT
type QUICConfig struct {
	PublicKey  string   `json:"public_key"`
	AllowedIPs []string `json:"allowed_ips"`
}

// MeshConfig represents mesh network configuration from JWT
type MeshConfig struct {
	AutoDiscovery     bool        `json:"auto_discovery"`
	Persistent        bool        `json:"persistent"`
	Routing           string      `json:"routing"`
	Encryption        string      `json:"encryption"`
	HeartbeatInterval interface{} `json:"heartbeat_interval"`
}

// PeerWhitelist represents peer whitelist configuration from JWT
type PeerWhitelist struct {
	AllowedPeers []string `json:"allowed_peers"`
	AutoApprove  bool     `json:"auto_approve"`
	MaxPeers     int      `json:"max_peers"`
}

// NetworkConfig represents network configuration from JWT
type NetworkConfig struct {
	Subnet string   `json:"subnet"`
	DNS    []string `json:"dns"`
	MTU    int      `json:"mtu"`
}

// AuthManager handles authentication with relay server
type AuthManager struct {
	config     *AuthConfig
	jwtSecret  []byte
	httpClient *http.Client

	// JWKS support for OIDC
	jwksURL   string
	jwksCache map[string]interface{} // kid -> *rsa.PublicKey | *ecdsa.PublicKey
	jwksMu    sync.RWMutex
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Type           string      `json:"type"`                      // "jwt" | "oidc"
	Secret         string      `json:"secret"`                    // для "jwt"
	FallbackSecret string      `json:"fallback_secret,omitempty"` // для "jwt"
	SkipValidation bool        `json:"skip_validation,omitempty"`
	OIDC           *OIDCConfig `json:"oidc,omitempty"`
}

// OIDCConfig for Zitadel/any OIDC IdP
type OIDCConfig struct {
	IssuerURL string `json:"issuer_url"`         // напр. https://<your-zitadel-domain>
	Audience  string `json:"audience"`           // ваш client_id или ожидаемая aud
	JWKSURL   string `json:"jwks_url,omitempty"` // опционально, иначе из discovery
	// опционально можно добавить: AllowedAlgs []string
}

// JWKS represents JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(config *AuthConfig) (*AuthManager, error) {
	am := &AuthManager{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		jwksCache: make(map[string]interface{}),
	}

	switch config.Type {
	case "jwt":
		if config.Secret == "" {
			return nil, fmt.Errorf("jwt secret is required")
		}
		// Support both plain and base64-encoded secrets
		if decoded, err := base64.StdEncoding.DecodeString(config.Secret); err == nil && len(decoded) > 0 {
			am.jwtSecret = decoded
		} else {
			am.jwtSecret = []byte(config.Secret)
		}

	case "oidc":
		if config.OIDC == nil || config.OIDC.IssuerURL == "" {
			return nil, fmt.Errorf("oidc configuration is required")
		}
		if err := am.setupOIDC(); err != nil {
			return nil, fmt.Errorf("failed to setup oidc: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported authentication type: %s", config.Type)
	}

	return am, nil
}

// setupOIDC: discovery + JWKS
func (am *AuthManager) setupOIDC() error {
	// 1) discovery, если jwks_url не задан
	if am.config.OIDC.JWKSURL == "" {
		wk := struct {
			JWKSURL string `json:"jwks_uri"`
			Issuer  string `json:"issuer"`
		}{}
		wellKnown := strings.TrimRight(am.config.OIDC.IssuerURL, "/") + "/.well-known/openid-configuration"
		resp, err := am.httpClient.Get(wellKnown)
		if err != nil {
			return fmt.Errorf("oidc discovery failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("oidc discovery bad status: %s", resp.Status)
		}
		if err := json.NewDecoder(resp.Body).Decode(&wk); err != nil {
			return fmt.Errorf("oidc discovery decode failed: %w", err)
		}
		if wk.JWKSURL == "" {
			return fmt.Errorf("jwks_uri not found in discovery")
		}
		am.jwksURL = wk.JWKSURL
	} else {
		am.jwksURL = am.config.OIDC.JWKSURL
	}
	// 2) начальная загрузка JWKS (кэш по kid)
	return am.refreshJWKS()
}

// fetchJWKS fetches JSON Web Key Set from OIDC provider
func (am *AuthManager) fetchJWKS() (*JWKS, error) {
	resp, err := am.httpClient.Get(am.jwksURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			_ = cerr // Игнорируем ошибку закрытия response body
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch jwks: %s", resp.Status)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode jwks: %w", err)
	}

	return &jwks, nil
}

// jwkToPublicKey converts JWK to public key (RSA/ECDSA)
func jwkToPublicKey(jwk JWK) (interface{}, error) {
	switch jwk.Kty {
	case "RSA":
		nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			return nil, fmt.Errorf("decode n: %w", err)
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			return nil, fmt.Errorf("decode e: %w", err)
		}
		eInt := 0
		for _, b := range eBytes {
			eInt = (eInt << 8) | int(b)
		}
		if eInt == 0 {
			return nil, fmt.Errorf("invalid exponent")
		}
		pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: eInt}
		if _, err := x509.MarshalPKIXPublicKey(pub); err != nil {
			return nil, fmt.Errorf("marshal rsa: %w", err)
		}
		return pub, nil
	case "EC":
		// Zitadel по умолчанию RS256, но поддержка EC — на будущее.
		return nil, fmt.Errorf("EC JWK not supported in this build")
	default:
		return nil, fmt.Errorf("unsupported kty: %s", jwk.Kty)
	}
}

// refreshJWKS загружает и кеширует JWKS ключи
func (am *AuthManager) refreshJWKS() error {
	jwks, err := am.fetchJWKS()
	if err != nil {
		return err
	}
	cache := make(map[string]interface{})
	for _, k := range jwks.Keys {
		if k.Kid == "" {
			// пропускаем без kid
			continue
		}
		pub, err := jwkToPublicKey(k)
		if err != nil {
			continue
		}
		cache[k.Kid] = pub
	}
	am.jwksMu.Lock()
	am.jwksCache = cache
	am.jwksMu.Unlock()
	return nil
}

// ValidateToken validates a JWT token
func (am *AuthManager) ValidateToken(tokenString string) (*jwt.Token, error) {
	switch am.config.Type {
	case "jwt":
		return am.validateJWTToken(tokenString)
	case "oidc":
		return am.validateOIDCToken(tokenString)
	default:
		return nil, fmt.Errorf("unsupported authentication type")
	}
}

// validateJWTToken validates a JWT token with HMAC
func (am *AuthManager) validateJWTToken(tokenString string) (*jwt.Token, error) {
	// Skip validation if configured (DEV MODE ONLY)
	if am.config.SkipValidation {
		parser := jwt.Parser{}
		tok, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
		if err != nil {
			return nil, errors.NewRelayError(errors.ErrInvalidToken, fmt.Sprintf("JWT parsing failed (skip mode): %v", err))
		}
		tok.Valid = false // подчеркиваем, что подпись не проверена
		return tok, nil
	}

	// Prepare candidate keys based on kid and configured secrets
	var candidates [][]byte

	// Helper to append decoded and raw versions in order
	addSecretCandidates := func(secret string) {
		if secret == "" {
			return
		}
		if decoded, err := base64.StdEncoding.DecodeString(secret); err == nil && len(decoded) > 0 {
			candidates = append(candidates, decoded)
		}
		candidates = append(candidates, []byte(secret))
	}

	// If kid==fallback-key and fallback secret provided, try it first
	{
		// Parse header to inspect kid without verifying signature
		parser := jwt.Parser{}
		if unverifiedToken, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{}); err == nil && unverifiedToken != nil {
			if kid, ok := unverifiedToken.Header["kid"].(string); ok && kid == "fallback-key" && am.config.FallbackSecret != "" {
				addSecretCandidates(am.config.FallbackSecret)
			}
		}
	}

	// If no candidates yet, fall back to primary secret
	if len(candidates) == 0 {
		// Prefer using configured secret directly to preserve exact bytes
		addSecretCandidates(am.config.Secret)
	}

	var lastErr error
	for _, key := range candidates {
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Validate algorithm
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return key, nil
		})
		if err == nil && token != nil && token.Valid {
			return token, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, errors.NewRelayError(errors.ErrInvalidToken, fmt.Sprintf("JWT validation failed: %v", lastErr))
	}
	return nil, errors.NewRelayError(errors.ErrInvalidToken, "invalid JWT token")
}

// validateOIDCToken validates an OIDC token
func (am *AuthManager) validateOIDCToken(tokenString string) (*jwt.Token, error) {
	parser := &jwt.Parser{}
	// 1) прочитаем header без проверки, чтобы взять kid/alg
	unverified, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
	if err == nil && unverified != nil {
		// kid будет использован в keyFunc
	}
	// 2) функция подбора ключа
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		// допустим только RSA/ECDSA (обычно RS256 для Zitadel)
		switch token.Method.Alg() {
		case jwt.SigningMethodRS256.Alg():
			// ok
		case jwt.SigningMethodES256.Alg():
			// (если включите EC)
		default:
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		kidHeader, _ := token.Header["kid"].(string)
		am.jwksMu.RLock()
		pub := am.jwksCache[kidHeader]
		am.jwksMu.RUnlock()
		if pub == nil {
			// попробуем обновить JWKS (ротация ключей)
			if err := am.refreshJWKS(); err != nil {
				return nil, fmt.Errorf("jwks refresh failed: %w", err)
			}
			am.jwksMu.RLock()
			pub = am.jwksCache[kidHeader]
			am.jwksMu.RUnlock()
			if pub == nil {
				return nil, fmt.Errorf("public key not found for kid=%s", kidHeader)
			}
		}
		return pub, nil
	}
	token, err := jwt.Parse(tokenString, keyFunc)
	if err != nil {
		return nil, errors.NewRelayError(errors.ErrInvalidToken, fmt.Sprintf("oidc token validation failed: %v", err))
	}
	if !token.Valid {
		return nil, errors.NewRelayError(errors.ErrInvalidToken, "Invalid OIDC token")
	}
	if err := am.validateOIDCClaims(token.Claims); err != nil {
		return nil, errors.NewRelayError(errors.ErrInvalidToken, fmt.Sprintf("Invalid claims: %v", err))
	}
	return token, nil
}

// validateOIDCClaims validates OIDC-specific claims
func (am *AuthManager) validateOIDCClaims(c jwt.Claims) error {
	claims, ok := c.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("invalid claims type")
	}
	// iss
	if iss, ok := claims["iss"].(string); ok {
		if !strings.HasPrefix(iss, am.config.OIDC.IssuerURL) {
			return fmt.Errorf("invalid issuer: %s", iss)
		}
	}
	// aud (Zitadel может отдавать строку или массив)
	if audRaw, ok := claims["aud"]; ok {
		want := am.config.OIDC.Audience
		switch v := audRaw.(type) {
		case string:
			if v != want {
				return fmt.Errorf("invalid audience: %s", v)
			}
		case []interface{}:
			found := false
			for _, a := range v {
				if s, ok := a.(string); ok && s == want {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("audience not found")
			}
		}
	}
	// exp/nbf проверяет сама библиотека при Parse, но можно дополнительно:
	return nil
}

// ExtractSubject extracts subject from token
func (am *AuthManager) ExtractSubject(token *jwt.Token) (string, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}

	subject, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("subject claim not found or invalid")
	}

	return subject, nil
}

// ExtractTenantID extracts tenant_id from token
func (am *AuthManager) ExtractTenantID(token *jwt.Token) (string, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}

	tenantID, ok := claims["tenant_id"].(string)
	if !ok {
		// Return empty string if tenant_id is not present (backward compatibility)
		return "", nil
	}

	return tenantID, nil
}

// ExtractClaims extracts both subject and tenant_id from token
func (am *AuthManager) ExtractClaims(token *jwt.Token) (string, string, error) {
	subject, err := am.ExtractSubject(token)
	if err != nil {
		return "", "", err
	}

	tenantID, err := am.ExtractTenantID(token)
	if err != nil {
		return "", "", err
	}

	return subject, tenantID, nil
}

// CreateAuthMessage creates an authentication message for relay server
func (am *AuthManager) CreateAuthMessage(tokenString string) (map[string]interface{}, error) {
	// Validate token first
	token, err := am.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Extract subject for rate limiting
	subject, err := am.ExtractSubject(token)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"type":  "auth",
		"token": tokenString,
		"sub":   subject,
	}, nil
}

// ExtractConnectionType extracts connection type from token
func (am *AuthManager) ExtractConnectionType(token *jwt.Token) (string, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}

	connectionType, ok := claims["connection_type"].(string)
	if !ok {
		// Fallback: determine from roles/permissions
		return am.determineConnectionTypeFromClaims(claims)
	}

	// Map JWT connection types to P2P connection types
	switch connectionType {
	case "wireguard":
		return "p2p-mesh", nil
	case "p2p-mesh":
		return "p2p-mesh", nil
	case "client-server":
		return "client-server", nil
	case "server-server":
		return "server-server", nil
	default:
		// Default to p2p-mesh for wireguard connections
		return "p2p-mesh", nil
	}
}

// ExtractQUICConfig extracts QUIC configuration from token
func (am *AuthManager) ExtractQUICConfig(token *jwt.Token) (*QUICConfig, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	quicConfig, ok := claims["quic_config"].(map[string]interface{})
	if !ok {
		// Return default QUIC config if not specified
		return &QUICConfig{
			PublicKey:  fmt.Sprintf("generated-quic-key-%d", time.Now().UnixNano()),
			AllowedIPs: []string{"10.0.0.0/24"},
		}, nil
	}

	config := &QUICConfig{}
	if publicKey, ok := quicConfig["public_key"].(string); ok {
		config.PublicKey = publicKey
	}
	if allowedIPs, ok := quicConfig["allowed_ips"].([]interface{}); ok {
		for _, ip := range allowedIPs {
			if ipStr, ok := ip.(string); ok {
				config.AllowedIPs = append(config.AllowedIPs, ipStr)
			}
		}
	}

	// Set defaults if not specified
	if config.PublicKey == "" {
		config.PublicKey = fmt.Sprintf("generated-quic-key-%d", time.Now().UnixNano())
	}
	if len(config.AllowedIPs) == 0 {
		config.AllowedIPs = []string{"10.0.0.0/24"}
	}

	return config, nil
}

// ExtractMeshConfig extracts mesh configuration from token
func (am *AuthManager) ExtractMeshConfig(token *jwt.Token) (*MeshConfig, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	meshConfig, ok := claims["mesh_config"].(map[string]interface{})
	if !ok {
		return nil, nil // Mesh not configured
	}

	config := &MeshConfig{}
	if autoDiscovery, ok := meshConfig["auto_discovery"].(bool); ok {
		config.AutoDiscovery = autoDiscovery
	}
	if persistent, ok := meshConfig["persistent"].(bool); ok {
		config.Persistent = persistent
	}
	if routing, ok := meshConfig["routing"].(string); ok {
		config.Routing = routing
	}
	if encryption, ok := meshConfig["encryption"].(string); ok {
		config.Encryption = encryption
	}
	if heartbeatInterval, ok := meshConfig["heartbeat_interval"]; ok {
		config.HeartbeatInterval = heartbeatInterval
	}

	return config, nil
}

// ExtractPeerWhitelist extracts peer whitelist from token
func (am *AuthManager) ExtractPeerWhitelist(token *jwt.Token) (*PeerWhitelist, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	whitelist, ok := claims["peer_whitelist"].(map[string]interface{})
	if !ok {
		return nil, nil // No whitelist configured
	}

	config := &PeerWhitelist{}
	if allowedPeers, ok := whitelist["allowed_peers"].([]interface{}); ok {
		for _, peer := range allowedPeers {
			if peerStr, ok := peer.(string); ok {
				config.AllowedPeers = append(config.AllowedPeers, peerStr)
			}
		}
	}
	if autoApprove, ok := whitelist["auto_approve"].(bool); ok {
		config.AutoApprove = autoApprove
	}
	if maxPeers, ok := whitelist["max_peers"].(float64); ok {
		config.MaxPeers = int(maxPeers)
	}

	return config, nil
}

// ExtractNetworkConfig extracts network configuration from token
func (am *AuthManager) ExtractNetworkConfig(token *jwt.Token) (*NetworkConfig, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	networkConfig, ok := claims["network_config"].(map[string]interface{})
	if !ok {
		return nil, nil // No network config
	}

	config := &NetworkConfig{}
	if subnet, ok := networkConfig["subnet"].(string); ok {
		config.Subnet = subnet
	}
	if dns, ok := networkConfig["dns"].([]interface{}); ok {
		for _, dnsServer := range dns {
			if dnsStr, ok := dnsServer.(string); ok {
				config.DNS = append(config.DNS, dnsStr)
			}
		}
	}
	if mtu, ok := networkConfig["mtu"].(float64); ok {
		config.MTU = int(mtu)
	}

	return config, nil
}

// ExtractPermissions extracts permissions from token
func (am *AuthManager) ExtractPermissions(token *jwt.Token) ([]string, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	permissions, ok := claims["permissions"].([]interface{})
	if !ok {
		return []string{}, nil // No permissions
	}

	var result []string
	for _, perm := range permissions {
		if permStr, ok := perm.(string); ok {
			result = append(result, permStr)
		}
	}

	return result, nil
}

// determineConnectionTypeFromClaims determines connection type from claims when not explicitly set
func (am *AuthManager) determineConnectionTypeFromClaims(claims jwt.MapClaims) (string, error) {
	// Check permissions for P2P capabilities
	if permissions, ok := claims["permissions"].([]interface{}); ok {
		for _, perm := range permissions {
			if permStr, ok := perm.(string); ok {
				if permStr == "mesh:connect" || permStr == "mesh:discover" {
					return "p2p-mesh", nil
				}
			}
		}
	}

	// Default to client-server
	return "client-server", nil
}

// GetTokenFromHeader extracts token from Authorization header
func (am *AuthManager) GetTokenFromHeader(header string) (string, error) {
	if header == "" {
		return "", fmt.Errorf("authorization header is empty")
	}

	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return parts[1], nil
}
