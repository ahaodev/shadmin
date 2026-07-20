package tokenutil

import (
	"testing"

	"shadmin/domain"
)

func TestCreateAccessTokenWithIdentityIncludesProviderClaims(t *testing.T) {
	user := &domain.User{
		ID:       "user-1",
		Username: "alice",
		Email:    "alice@example.com",
		IsAdmin:  true,
		Roles:    []string{"admin"},
	}

	token, err := CreateAccessTokenWithIdentity(user, "secret", 60, "google", "google-subject", "oidc")
	if err != nil {
		t.Fatalf("CreateAccessTokenWithIdentity returned error: %v", err)
	}

	claims, err := ExtractAllClaimsFromToken(token, "secret")
	if err != nil {
		t.Fatalf("ExtractAllClaimsFromToken returned error: %v", err)
	}

	if claims.Provider != "google" {
		t.Fatalf("expected provider google, got %q", claims.Provider)
	}
	if claims.ProviderSubject != "google-subject" {
		t.Fatalf("expected provider subject google-subject, got %q", claims.ProviderSubject)
	}
	if claims.Source != "oidc" {
		t.Fatalf("expected source oidc, got %q", claims.Source)
	}
	if claims.Subject != "google-subject" {
		t.Fatalf("expected sub claim google-subject, got %q", claims.Subject)
	}
	if claims.ID != user.ID {
		t.Fatalf("expected custom id %q, got %q", user.ID, claims.ID)
	}
}
