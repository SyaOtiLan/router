package common

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJCSCanonical(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"null", "null", "null"},
		{"bool true", "true", "true"},
		{"bool false", "false", "false"},
		{"string", `"hello"`, `"hello"`},
		{"integer", "42", "42"},
		{"empty object", `{}`, `{}`},
		{"empty array", `[]`, `[]`},
		{"sorted object", `{"b":1,"a":2}`, `{"a":2,"b":1}`},
		{"nested", `{"z":{"b":1,"a":2},"y":[3,2,1]}`, `{"y":[3,2,1],"z":{"a":2,"b":1}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var val any
			if err := json.Unmarshal([]byte(tc.input), &val); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got, err := jcsCanonical(val)
			if err != nil {
				t.Fatalf("canonicalize: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestVerifyIdentityPresentation(t *testing.T) {
	// Generate Ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubB64 := base64.RawURLEncoding.EncodeToString(pub)
	holder := "did:yeying:wid_test1234567890abcdefghijklm"
	controllerID := "controller-1"
	now := time.Now().UTC()
	issuedAt := now.Format(time.RFC3339)
	expiresAt := now.Add(5 * time.Minute).Format(time.RFC3339)

	identityDoc := map[string]any{
		"version": 1,
		"id":      holder,
		"controllers": []any{
			map[string]any{
				"controllerId": controllerID,
				"kind":         "wallet_key",
				"publicKey":    pubB64,
				"algorithm":    "Ed25519",
				"purposes":     []any{"authentication", "assertion", "manage"},
				"status":       "active",
			},
		},
	}
	docCanonical, err := jcsCanonical(identityDoc)
	if err != nil {
		t.Fatalf("canonicalize identity doc: %v", err)
	}
	identityDoc["proof"] = map[string]any{
		"type":               "YeyingIdentityDocumentProofV1",
		"verificationMethod": holder + "#" + controllerID,
		"purpose":            "manage",
		"proofValue":         base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(docCanonical))),
	}

	unsigned := map[string]any{
		"version":          1,
		"holder":           holder,
		"audience":         "https://app.example.com",
		"nonce":            "test-nonce-123",
		"issuedAt":         issuedAt,
		"expiresAt":        expiresAt,
		"scopes":           []any{"identity.basic", "identity.wallet"},
		"identityDocument": identityDoc,
	}

	canonical, err := jcsCanonical(unsigned)
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	sig := ed25519.Sign(priv, []byte(canonical))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	full := map[string]any{
		"version":          unsigned["version"],
		"holder":           unsigned["holder"],
		"audience":         unsigned["audience"],
		"nonce":            unsigned["nonce"],
		"issuedAt":         unsigned["issuedAt"],
		"expiresAt":        unsigned["expiresAt"],
		"scopes":           unsigned["scopes"],
		"identityDocument": unsigned["identityDocument"],
		"proof": map[string]any{
			"type":               "YeyingIdentityPresentationProofV1",
			"verificationMethod": holder + "#" + controllerID,
			"purpose":            "authentication",
			"proofValue":         sigB64,
		},
	}

	presJSON, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Valid presentation
	pres, err := VerifyIdentityPresentation(presJSON, "https://app.example.com", "test-nonce-123")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if pres.Holder != holder {
		t.Fatalf("holder mismatch: %s != %s", pres.Holder, holder)
	}

	// Wrong audience
	_, err = VerifyIdentityPresentation(presJSON, "https://wrong.example.com", "test-nonce-123")
	if err == nil {
		t.Fatal("expected audience mismatch error")
	}

	// Wrong nonce
	_, err = VerifyIdentityPresentation(presJSON, "https://app.example.com", "wrong-nonce")
	if err == nil {
		t.Fatal("expected nonce mismatch error")
	}

	// Tampered presentation
	full["nonce"] = "tampered"
	tamperedJSON, _ := json.Marshal(full)
	_, err = VerifyIdentityPresentation(tamperedJSON, "https://app.example.com", "tampered")
	if err == nil {
		t.Fatal("expected proof invalid error for tampered presentation")
	}
}

func TestVerifyIdentityPresentationCredentials(t *testing.T) {
	issuerPub, issuerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate issuer key: %v", err)
	}
	holder := "did:yeying:wid_test1234567890abcdefghijklm"
	kid := "issuer-key-1"
	issuerKeys := []any{}
	statuses := map[string]string{"cred-1": "active", "revoked-credential": "revoked", "rotated-key-credential": "active"}
	var stateMu sync.Mutex
	var metadataCalls, jwksCalls, statusCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stateMu.Lock()
		defer stateMu.Unlock()
		issuer := "did:web:" + strings.TrimPrefix(server.URL, "http://")
		switch r.URL.Path {
		case "/.well-known/openid-credential-issuer":
			metadataCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": issuer, "jwks_uri": server.URL + "/.well-known/jwks.json",
				"credential_status_uri": server.URL + "/api/v1/public/identity/credentials/status",
			})
		case "/.well-known/jwks.json":
			jwksCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": issuerKeys})
		case "/api/v1/public/identity/credentials/status":
			statusCalls++
			var input struct {
				Issuer      string   `json:"issuer"`
				Credentials []string `json:"credentials"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Issuer != issuer {
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}
			result := make(map[string]string, len(input.Credentials))
			for _, id := range input.Credentials {
				result[id] = statuses[id]
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"issuer": issuer, "checkedAt": time.Now().UTC().Format(time.RFC3339Nano),
					"nextUpdateAt": time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano), "statuses": result,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	issuer := "did:web:" + strings.TrimPrefix(server.URL, "http://")
	issuerKeys = append(issuerKeys, identityTestJWK(issuerPub, kid))
	resolver, err := newIdentityIssuerResolver(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	options := IdentityCredentialVerificationOptions{
		NodeURL:                 server.URL,
		resolver:                resolver,
		RequiredScopes:          []string{"identity.basic", "identity.email"},
		RequiredCredentialTypes: []string{"EmailCredential"},
	}
	makeToken := func(credentialID, tokenIssuer, credentialSubjectDID, tokenKID string, key ed25519.PrivateKey) string {
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
			"iss": tokenIssuer, "sub": holder, "jti": credentialID,
			"nbf": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix(),
			"vc": map[string]any{
				"type":              []string{"VerifiableCredential", "EmailCredential"},
				"credentialSubject": map[string]any{"id": credentialSubjectDID, "email": "user@example.com"},
			},
		})
		token.Header["kid"] = tokenKID
		encoded, signErr := token.SignedString(key)
		if signErr != nil {
			t.Fatal(signErr)
		}
		return encoded
	}
	makePresentation := func(token string) *IdentityPresentation {
		return &IdentityPresentation{Version: 1, Holder: holder, Scopes: []string{"identity.basic", "identity.email"}, Credentials: []string{token}}
	}

	token := makeToken("cred-1", issuer, holder, kid, issuerPriv)
	verified, err := VerifyIdentityPresentationCredentials(makePresentation(token), options)
	if err != nil {
		t.Fatalf("verify online credential failed: %v", err)
	}
	if verified["EmailCredential"].Subject["email"] != "user@example.com" {
		t.Fatalf("email subject mismatch: %#v", verified["EmailCredential"].Subject)
	}
	if _, err := VerifyIdentityPresentationCredentials(makePresentation(token), options); err != nil {
		t.Fatalf("cached credential verify failed: %v", err)
	}
	stateMu.Lock()
	if metadataCalls != 1 || jwksCalls != 1 || statusCalls != 1 {
		t.Fatalf("expected cached metadata/JWKS/status after first verification, got %d/%d/%d", metadataCalls, jwksCalls, statusCalls)
	}
	stateMu.Unlock()
	if _, err := VerifyIdentityPresentationCredentials(makePresentation(makeToken("revoked-credential", issuer, holder, kid, issuerPriv)), options); err == nil || err.Error() != "identity_credential_revoked" {
		t.Fatalf("expected online revoked credential rejection, got %v", err)
	}

	rotatedPublic, rotatedPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	stateMu.Lock()
	issuerKeys = append(issuerKeys, identityTestJWK(rotatedPublic, "issuer-key-2"))
	stateMu.Unlock()
	rotatedToken := makeToken("rotated-key-credential", issuer, holder, "issuer-key-2", rotatedPrivate)
	if _, err := VerifyIdentityPresentationCredentials(makePresentation(rotatedToken), options); err != nil {
		t.Fatalf("unknown kid should refresh JWKS during rotation: %v", err)
	}

	// A verified, fresh in-memory cache permits normal login during a short Node outage.
	server.Close()
	if _, err := VerifyIdentityPresentationCredentials(makePresentation(token), options); err != nil {
		t.Fatalf("valid cache should survive a short Node outage: %v", err)
	}

	if err := resolver.validateSameNodeURL("https://attacker.example/jwks.json"); err == nil {
		t.Fatal("expected external JWKS origin to be rejected")
	}

	otherResolver, err := newIdentityIssuerResolver(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentityPresentationCredentials(makePresentation(makeToken("revoked-credential", issuer, holder, kid, issuerPriv)), IdentityCredentialVerificationOptions{
		NodeURL:                 server.URL,
		resolver:                otherResolver,
		RequiredScopes:          []string{"identity.basic", "identity.email"},
		RequiredCredentialTypes: []string{"EmailCredential"},
	}); err == nil || err.Error() == "" {
		// This resolver has no cache and Node is offline; status cannot be assumed active.
		t.Fatal("credential status lookup must fail closed when Node is unavailable")
	}

}

func identityTestJWK(publicKey ed25519.PublicKey, kid string) map[string]any {
	return map[string]any{
		"kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "use": "sig", "kid": kid,
		"x": base64.RawURLEncoding.EncodeToString(publicKey),
	}
}
