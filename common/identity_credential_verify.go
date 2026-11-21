package common

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	identityIssuerCacheTTL       = 5 * time.Minute
	identityIssuerMaxStale       = 30 * time.Minute
	identityStatusMaxStale       = 15 * time.Minute
	identityIssuerRequestTimeout = 5 * time.Second
	identityIssuerResponseLimit  = 1 << 20
	identityCredentialLimit      = 32
	identityStatusCacheLimit     = 8192
)

type IdentityCredentialVerificationOptions struct {
	NodeURL                 string
	RequiredScopes          []string
	RequiredCredentialTypes []string
	resolver                *identityIssuerResolver
}

type VerifiedIdentityCredential struct {
	Type    string
	Claims  jwt.MapClaims
	Subject map[string]any
}

type identityIssuerMetadata struct {
	Issuer              string `json:"issuer"`
	JWKSURI             string `json:"jwks_uri"`
	CredentialStatusURI string `json:"credential_status_uri"`
}

type cachedIssuerMaterial struct {
	metadata  identityIssuerMetadata
	jwks      map[string]any
	fetchedAt time.Time
}

type cachedCredentialStatus struct {
	status    string
	checkedAt time.Time
	nextCheck time.Time
}

type identityIssuerResolver struct {
	mu          sync.Mutex
	nodeURL     string
	expectedDID string
	client      *http.Client
	material    *cachedIssuerMaterial
	statuses    map[string]cachedCredentialStatus
}

var identityIssuerResolvers = struct {
	sync.Mutex
	items map[string]*identityIssuerResolver
}{items: make(map[string]*identityIssuerResolver)}

func newIdentityIssuerResolver(nodeURL string) (*identityIssuerResolver, error) {
	base, err := parseIdentityNodeURL(nodeURL)
	if err != nil {
		return nil, err
	}
	return &identityIssuerResolver{
		nodeURL:     strings.TrimRight(base.String(), "/"),
		expectedDID: "did:web:" + base.Host,
		client: &http.Client{
			Timeout: identityIssuerRequestTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return errors.New("identity_issuer_redirect_rejected")
			},
		},
		statuses: make(map[string]cachedCredentialStatus),
	}, nil
}

func parseIdentityNodeURL(raw string) (*url.URL, error) {
	base, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("identity_node_url_invalid")
	}
	if base.Scheme != "https" && base.Scheme != "http" {
		return nil, errors.New("identity_node_url_https_required")
	}
	if base.Scheme == "http" {
		host := base.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return nil, errors.New("identity_node_url_https_required")
		}
	}
	base.Path = strings.TrimRight(base.Path, "/")
	return base, nil
}

func getIdentityIssuerResolver(nodeURL string) (*identityIssuerResolver, error) {
	base, err := parseIdentityNodeURL(nodeURL)
	if err != nil {
		return nil, err
	}
	key := strings.TrimRight(base.String(), "/")
	identityIssuerResolvers.Lock()
	defer identityIssuerResolvers.Unlock()
	if resolver := identityIssuerResolvers.items[key]; resolver != nil {
		return resolver, nil
	}
	resolver, err := newIdentityIssuerResolver(key)
	if err != nil {
		return nil, err
	}
	identityIssuerResolvers.items[key] = resolver
	return resolver, nil
}

// VerifyIdentityPresentationCredentials validates Node-issued JWT-VCs using
// issuer metadata, JWKS and credential status fetched from the configured Node
// HTTPS origin. No local trust directory or downloaded manifest is used.
func VerifyIdentityPresentationCredentials(pres *IdentityPresentation, opts IdentityCredentialVerificationOptions) (map[string]VerifiedIdentityCredential, error) {
	if pres == nil || !isWalletIdentityDID(pres.Holder) {
		return nil, errors.New("identity_presentation_invalid")
	}
	for _, scope := range opts.RequiredScopes {
		if !identityContainsString(pres.Scopes, scope) {
			return nil, errors.New("identity_presentation_scope_mismatch")
		}
	}
	if len(pres.Credentials) > identityCredentialLimit {
		return nil, errors.New("identity_credential_count_invalid")
	}
	resolver := opts.resolver
	if resolver == nil {
		var err error
		resolver, err = getIdentityIssuerResolver(opts.NodeURL)
		if err != nil {
			return nil, err
		}
	}
	verified := make(map[string]VerifiedIdentityCredential)
	credentialIDs := make([]string, 0, len(pres.Credentials))
	for _, token := range pres.Credentials {
		credential, credentialID, err := resolver.verifyCredential(token, pres.Holder)
		if err != nil {
			return nil, err
		}
		credentialIDs = append(credentialIDs, credentialID)
		if credential.Type != "" {
			verified[credential.Type] = credential
		}
	}
	statuses, err := resolver.credentialStatuses(credentialIDs)
	if err != nil {
		return nil, err
	}
	for _, credentialID := range credentialIDs {
		if statuses[credentialID] != "active" {
			if statuses[credentialID] == "revoked" || statuses[credentialID] == "replaced" {
				return nil, errors.New("identity_credential_revoked")
			}
			return nil, errors.New("identity_credential_status_unavailable")
		}
	}
	for _, credentialType := range opts.RequiredCredentialTypes {
		if _, ok := verified[credentialType]; !ok {
			return nil, fmt.Errorf("identity_credential_required:%s", credentialType)
		}
	}
	return verified, nil
}

func (resolver *identityIssuerResolver) verifyCredential(token string, expectedDID string) (VerifiedIdentityCredential, string, error) {
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(strings.TrimSpace(token), claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
			return nil, errors.New("identity_credential_alg_invalid")
		}
		kid := identityStringValue(token.Header["kid"])
		if kid == "" {
			return nil, errors.New("identity_issuer_jwks_invalid")
		}
		return resolver.publicKey(kid)
	})
	if err != nil || parsed == nil || !parsed.Valid {
		return VerifiedIdentityCredential{}, "", errors.New("identity_credential_invalid")
	}
	material, err := resolver.issuerMaterial(false)
	if err != nil {
		return VerifiedIdentityCredential{}, "", err
	}
	if identityStringValue(claims["iss"]) != material.metadata.Issuer {
		return VerifiedIdentityCredential{}, "", errors.New("identity_issuer_invalid")
	}
	if identityStringValue(claims["sub"]) != expectedDID {
		return VerifiedIdentityCredential{}, "", errors.New("identity_credential_subject_mismatch")
	}
	credentialID := identityStringValue(claims["jti"])
	if credentialID == "" || len(credentialID) > 512 || !identityCredentialTimeValid(claims, time.Now()) {
		return VerifiedIdentityCredential{}, "", errors.New("identity_credential_expired")
	}
	vc, _ := claims["vc"].(map[string]any)
	credentialType := identityCredentialTypeFromVC(vc)
	if credentialType == "" {
		return VerifiedIdentityCredential{}, "", errors.New("identity_credential_type_mismatch")
	}
	subject, _ := vc["credentialSubject"].(map[string]any)
	if identityStringValue(subject["id"]) != expectedDID {
		return VerifiedIdentityCredential{}, "", errors.New("identity_credential_subject_mismatch")
	}
	return VerifiedIdentityCredential{Type: credentialType, Claims: claims, Subject: subject}, credentialID, nil
}

func (resolver *identityIssuerResolver) publicKey(kid string) (ed25519.PublicKey, error) {
	material, err := resolver.issuerMaterial(false)
	if err == nil {
		if key, keyErr := identityPublicKeyFromJWKS(material.jwks, kid, true); keyErr == nil {
			return key, nil
		}
	}
	material, refreshErr := resolver.issuerMaterial(true)
	if refreshErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, refreshErr
	}
	return identityPublicKeyFromJWKS(material.jwks, kid, true)
}

func (resolver *identityIssuerResolver) issuerMaterial(forceRefresh bool) (*cachedIssuerMaterial, error) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	now := time.Now()
	if !forceRefresh && resolver.material != nil && now.Sub(resolver.material.fetchedAt) <= identityIssuerCacheTTL {
		return resolver.material, nil
	}
	material, err := resolver.fetchIssuerMaterial()
	if err == nil {
		resolver.material = material
		return material, nil
	}
	if resolver.material != nil && now.Sub(resolver.material.fetchedAt) <= identityIssuerMaxStale {
		return resolver.material, nil
	}
	return nil, err
}

func (resolver *identityIssuerResolver) fetchIssuerMaterial() (*cachedIssuerMaterial, error) {
	metadataURL := resolver.nodeURL + "/.well-known/openid-credential-issuer"
	var metadata identityIssuerMetadata
	if err := resolver.getJSON(metadataURL, &metadata); err != nil {
		return nil, fmt.Errorf("identity_issuer_metadata_unavailable: %w", err)
	}
	if metadata.Issuer == "" || metadata.Issuer != resolver.expectedDID {
		return nil, errors.New("identity_issuer_invalid")
	}
	if err := resolver.validateSameNodeURL(metadata.JWKSURI); err != nil {
		return nil, errors.New("identity_issuer_jwks_uri_invalid")
	}
	if err := resolver.validateSameNodeURL(metadata.CredentialStatusURI); err != nil {
		return nil, errors.New("identity_credential_status_uri_invalid")
	}
	var jwks map[string]any
	jwksURL, err := resolver.endpointURL(metadata.JWKSURI)
	if err != nil {
		return nil, errors.New("identity_issuer_jwks_uri_invalid")
	}
	if err := resolver.getJSON(jwksURL, &jwks); err != nil {
		return nil, fmt.Errorf("identity_issuer_jwks_unavailable: %w", err)
	}
	if _, err := identityPublicKeyFromJWKS(jwks, "", false); err != nil {
		return nil, err
	}
	return &cachedIssuerMaterial{metadata: metadata, jwks: jwks, fetchedAt: time.Now()}, nil
}

func (resolver *identityIssuerResolver) validateSameNodeURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Host != "" && !strings.EqualFold(parsed.Host, mustURL(resolver.nodeURL).Host) {
		return errors.New("identity_issuer_endpoint_origin_mismatch")
	}
	if parsed.IsAbs() {
		base := mustURL(resolver.nodeURL)
		if parsed.Scheme != base.Scheme || !strings.EqualFold(parsed.Host, base.Host) {
			return errors.New("identity_issuer_endpoint_origin_mismatch")
		}
	} else if strings.HasPrefix(raw, "//") || !strings.HasPrefix(raw, "/") {
		return errors.New("identity_issuer_endpoint_uri_invalid")
	}
	return nil
}

func (resolver *identityIssuerResolver) endpointURL(raw string) (string, error) {
	base, err := url.Parse(resolver.nodeURL)
	if err != nil {
		return "", err
	}
	endpoint, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(endpoint).String(), nil
}

func mustURL(raw string) *url.URL {
	parsed, _ := url.Parse(raw)
	return parsed
}

func (resolver *identityIssuerResolver) getJSON(endpoint string, target any) error {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	response, err := resolver.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected_http_status:%d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, identityIssuerResponseLimit+1))
	if err != nil || len(body) > identityIssuerResponseLimit {
		return errors.New("identity_issuer_response_invalid")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return errors.New("identity_issuer_response_invalid")
	}
	return nil
}

func (resolver *identityIssuerResolver) credentialStatuses(credentialIDs []string) (map[string]string, error) {
	statuses := make(map[string]string, len(credentialIDs))
	missing := make([]string, 0, len(credentialIDs))
	resolver.mu.Lock()
	now := time.Now()
	for _, credentialID := range credentialIDs {
		cached, ok := resolver.statuses[credentialID]
		if ok && now.Before(cached.nextCheck) {
			statuses[credentialID] = cached.status
			continue
		}
		missing = append(missing, credentialID)
	}
	resolver.mu.Unlock()
	if len(missing) == 0 {
		return statuses, nil
	}
	material, err := resolver.issuerMaterial(false)
	if err != nil {
		return resolver.staleStatuses(statuses, missing, err)
	}
	body, _ := json.Marshal(map[string]any{"issuer": material.metadata.Issuer, "credentials": missing})
	statusURL, err := resolver.endpointURL(material.metadata.CredentialStatusURI)
	if err != nil {
		return nil, errors.New("identity_credential_status_uri_invalid")
	}
	request, err := http.NewRequest(http.MethodPost, statusURL, bytes.NewReader(body))
	if err != nil {
		return resolver.staleStatuses(statuses, missing, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := resolver.client.Do(request)
	if err != nil {
		return resolver.staleStatuses(statuses, missing, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return resolver.staleStatuses(statuses, missing, fmt.Errorf("unexpected_http_status:%d", response.StatusCode))
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Issuer     string            `json:"issuer"`
			CheckedAt  string            `json:"checkedAt"`
			NextUpdate string            `json:"nextUpdateAt"`
			Statuses   map[string]string `json:"statuses"`
		} `json:"data"`
	}
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, identityIssuerResponseLimit+1))
	if readErr != nil || len(responseBody) > identityIssuerResponseLimit || json.Unmarshal(responseBody, &envelope) != nil || envelope.Code != 0 || envelope.Data.Issuer != material.metadata.Issuer {
		return resolver.staleStatuses(statuses, missing, errors.New("identity_credential_status_response_invalid"))
	}
	now = time.Now()
	checkedAt, err := time.Parse(time.RFC3339Nano, envelope.Data.CheckedAt)
	if err != nil || checkedAt.After(now.Add(5*time.Minute)) || now.Sub(checkedAt) > identityStatusMaxStale {
		return resolver.staleStatuses(statuses, missing, errors.New("identity_credential_status_response_stale"))
	}
	nextUpdate := now.Add(identityIssuerCacheTTL)
	if parsed, parseErr := time.Parse(time.RFC3339Nano, envelope.Data.NextUpdate); parseErr == nil && parsed.After(now) {
		nextUpdate = parsed
	}
	if maxNextUpdate := now.Add(identityIssuerCacheTTL); nextUpdate.After(maxNextUpdate) {
		nextUpdate = maxNextUpdate
	}
	resolver.mu.Lock()
	for id, cached := range resolver.statuses {
		if now.After(cached.nextCheck) && (cached.status == "active" || cached.status == "revoked" || cached.status == "replaced") {
			delete(resolver.statuses, id)
		}
	}
	for _, credentialID := range missing {
		status, ok := envelope.Data.Statuses[credentialID]
		if !ok || status == "unknown" || status == "expired" {
			resolver.mu.Unlock()
			return nil, errors.New("identity_credential_status_unavailable")
		}
		if status != "active" && status != "revoked" && status != "replaced" {
			resolver.mu.Unlock()
			return nil, errors.New("identity_credential_status_response_invalid")
		}
		if len(resolver.statuses) < identityStatusCacheLimit {
			resolver.statuses[credentialID] = cachedCredentialStatus{status: status, checkedAt: checkedAt, nextCheck: nextUpdate}
		}
		statuses[credentialID] = status
	}
	resolver.mu.Unlock()
	return statuses, nil
}

func (resolver *identityIssuerResolver) staleStatuses(statuses map[string]string, credentialIDs []string, cause error) (map[string]string, error) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	now := time.Now()
	for _, credentialID := range credentialIDs {
		cached, ok := resolver.statuses[credentialID]
		if !ok || cached.status != "active" || now.Sub(cached.checkedAt) > identityStatusMaxStale {
			return nil, fmt.Errorf("identity_credential_status_unavailable: %w", cause)
		}
		statuses[credentialID] = cached.status
	}
	return statuses, nil
}

func identityPublicKeyFromJWKS(jwks map[string]any, kid string, requireKID bool) (ed25519.PublicKey, error) {
	keys, _ := jwks["keys"].([]any)
	for _, item := range keys {
		key, ok := item.(map[string]any)
		if !ok || identityStringValue(key["kty"]) != "OKP" || identityStringValue(key["crv"]) != "Ed25519" || identityStringValue(key["alg"]) != "EdDSA" || identityStringValue(key["use"]) != "sig" {
			continue
		}
		if kid != "" && identityStringValue(key["kid"]) != kid {
			continue
		}
		if requireKID && kid == "" {
			return nil, errors.New("identity_issuer_jwks_invalid")
		}
		x, err := identityBase64URLDecode(identityStringValue(key["x"]))
		if err != nil || len(x) != ed25519.PublicKeySize {
			return nil, errors.New("identity_issuer_jwks_invalid")
		}
		return ed25519.PublicKey(x), nil
	}
	return nil, errors.New("identity_issuer_jwks_invalid")
}

func identityBase64URLDecode(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(strings.TrimSpace(value))
	}
	return decoded, err
}

func identityCredentialTypeFromVC(vc map[string]any) string {
	if vc == nil {
		return ""
	}
	types := identityStringSlice(vc["type"])
	if !identityContainsString(types, "VerifiableCredential") {
		return ""
	}
	for _, value := range types {
		if value != "VerifiableCredential" {
			return value
		}
	}
	return ""
}

func identityCredentialTimeValid(claims jwt.MapClaims, now time.Time) bool {
	expiresAt, err := claims.GetExpirationTime()
	if err != nil || expiresAt == nil || !expiresAt.Time.After(now) {
		return false
	}
	notBefore, err := claims.GetNotBefore()
	if err != nil {
		return false
	}
	if notBefore != nil && notBefore.Time.After(now) {
		return false
	}
	return true
}

func identityStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := identityStringValue(item); value != "" {
				result = append(result, value)
			}
		}
		return result
	case string:
		return strings.Fields(typed)
	default:
		return nil
	}
}

func identityContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func identityStringValue(value any) string {
	result, _ := value.(string)
	return strings.TrimSpace(result)
}

func isWalletIdentityDID(did string) bool {
	did = strings.TrimSpace(did)
	return strings.HasPrefix(did, "did:yeying:wid_") && len(did) >= len("did:yeying:wid_")+22
}
