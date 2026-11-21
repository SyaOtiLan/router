package auth

import "testing"

func TestValidateLegacyWalletIdentityBinding(t *testing.T) {
	requested := "did:yeying:wid_requestedidentity12345"
	other := "did:yeying:wid_otheridentity1234567890"

	if err := validateLegacyWalletIdentityBinding(nil, requested); err != nil {
		t.Fatalf("unbound legacy wallet should be attachable: %v", err)
	}
	if err := validateLegacyWalletIdentityBinding(&requested, requested); err != nil {
		t.Fatalf("same DID should be accepted: %v", err)
	}
	if err := validateLegacyWalletIdentityBinding(&other, requested); err == nil {
		t.Fatal("wallet address already owned by another DID must be rejected")
	}
}
