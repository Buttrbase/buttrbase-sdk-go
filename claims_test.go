package buttrbase

import (
	"encoding/json"
	"strings"
	"testing"
)

// fixtureClaimsJSON mirrors tests/fixtures/access_token_claims.json from the
// Rust SDK — the canonical shape for testing data-envelope enrichment.
const fixtureClaimsJSON = `{
  "exp": 1750003600,
  "iat": 1750000000,
  "sub": "11111111-1111-1111-1111-111111111111",
  "org": "22222222-2222-2222-2222-222222222222",
  "scope": ["read:messages", "write:messages"],
  "token_type": "access",
  "jti": "33333333-3333-3333-3333-333333333333",
  "data": {
    "email": "test@example.com",
    "username": "testuser",
    "org_name": "Test Org",
    "uu_id": "11111111-1111-1111-1111-111111111111",
    "superuser_flag": false,
    "user_uuid": "11111111-1111-1111-1111-111111111111",
    "user": {
      "id": 1,
      "email": "test@example.com",
      "username": "testuser",
      "fullname": "Test User",
      "user_uuid": "11111111-1111-1111-1111-111111111111",
      "org_uuid": "22222222-2222-2222-2222-222222222222",
      "superuser_flag": false
    },
    "userdetails": {
      "id": 1,
      "userId": 1,
      "roles": "owner",
      "user_data": null,
      "verify": true,
      "first_name": "Test",
      "last_name": "User"
    },
    "roles": "owner",
    "org_uuid": "22222222-2222-2222-2222-222222222222",
    "org_id": 42,
    "app_uuid": "44444444-4444-4444-4444-444444444444",
    "scopes": ["read:messages", "write:messages"]
  }
}`

// fixtureToken is a fake JWT whose payload is the fixture claims JSON
// base64url-encoded. The header and signature segments are placeholders —
// ParseTokenClaims only decodes the payload; it does not verify the
// signature.
//
// The payload segment was produced by:
//
//	base64.RawURLEncoding.EncodeToString([]byte(fixtureClaimsJSON))
const fixtureToken = "eyJhbGciOiJSUzI1NiIsImtpZCI6InRlc3Qta2V5In0." +
	"eyJleHAiOjE3NTAwMDM2MDAsImlhdCI6MTc1MDAwMDAwMCwic3ViIjoiMTExMTExMTEtMTExMS0xMTExLTExMTEtMTExMTExMTExMTExIiwib3JnIjoiMjIyMjIyMjItMjIyMi0yMjIyLTIyMjItMjIyMjIyMjIyMjIyIiwic2NvcGUiOlsicmVhZDptZXNzYWdlcyIsIndyaXRlOm1lc3NhZ2VzIl0sInRva2VuX3R5cGUiOiJhY2Nlc3MiLCJqdGkiOiIzMzMzMzMzMy0zMzMzLTMzMzMtMzMzMy0zMzMzMzMzMzMzMzMiLCJkYXRhIjp7ImVtYWlsIjoidGVzdEBleGFtcGxlLmNvbSIsInVzZXJuYW1lIjoidGVzdHVzZXIiLCJvcmdfbmFtZSI6IlRlc3QgT3JnIiwidXVfaWQiOiIxMTExMTExMS0xMTExLTExMTEtMTExMS0xMTExMTExMTExMTEiLCJzdXBlcnVzZXJfZmxhZyI6ZmFsc2UsInVzZXJfdXVpZCI6IjExMTExMTExLTExMTEtMTExMS0xMTExLTExMTExMTExMTExMSIsInVzZXIiOnsiaWQiOjEsImVtYWlsIjoidGVzdEBleGFtcGxlLmNvbSIsInVzZXJuYW1lIjoidGVzdHVzZXIiLCJmdWxsbmFtZSI6IlRlc3QgVXNlciIsInVzZXJfdXVpZCI6IjExMTExMTExLTExMTEtMTExMS0xMTExLTExMTExMTExMTExMSIsIm9yZ191dWlkIjoiMjIyMjIyMjItMjIyMi0yMjIyLTIyMjItMjIyMjIyMjIyMjIyIiwic3VwZXJ1c2VyX2ZsYWciOmZhbHNlfSwidXNlcmRldGFpbHMiOnsiaWQiOjEsInVzZXJJZCI6MSwicm9sZXMiOiJvd25lciIsInVzZXJfZGF0YSI6bnVsbCwidmVyaWZ5Ijp0cnVlLCJmaXJzdF9uYW1lIjoiVGVzdCIsImxhc3RfbmFtZSI6IlVzZXIifSwicm9sZXMiOiJvd25lciIsIm9yZ191dWlkIjoiMjIyMjIyMjItMjIyMi0yMjIyLTIyMjItMjIyMjIyMjIyMjIyIiwib3JnX2lkIjo0MiwiYXBwX3V1aWQiOiI0NDQ0NDQ0NC00NDQ0LTQ0NDQtNDQ0NC00NDQ0NDQ0NDQ0NDQiLCJzY29wZXMiOlsicmVhZDptZXNzYWdlcyIsIndyaXRlOm1lc3NhZ2VzIl19fQ." +
	"fakesignature"

// ----- TokenClaimsData JSON deserialization -----

func TestTokenClaimsData_UnmarshalFromFixture(t *testing.T) {
	var claims TokenClaims
	if err := json.Unmarshal([]byte(fixtureClaimsJSON), &claims); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if claims.Data == nil {
		t.Fatal("expected data envelope to be present")
	}
	if claims.Data.Roles == nil || *claims.Data.Roles != "owner" {
		t.Errorf("expected data.roles = %q, got %v", "owner", claims.Data.Roles)
	}
	if claims.Data.Email == nil || *claims.Data.Email != "test@example.com" {
		t.Errorf("expected data.email = %q, got %v", "test@example.com", claims.Data.Email)
	}
	if claims.Data.OrgUUID == nil || *claims.Data.OrgUUID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("expected data.org_uuid = %q, got %v", "22222222-2222-2222-2222-222222222222", claims.Data.OrgUUID)
	}
	if claims.Data.UserUUID == nil || *claims.Data.UserUUID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("expected data.user_uuid = %q, got %v", "11111111-1111-1111-1111-111111111111", claims.Data.UserUUID)
	}
}

func TestTokenClaims_TopLevelFields(t *testing.T) {
	var claims TokenClaims
	if err := json.Unmarshal([]byte(fixtureClaimsJSON), &claims); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if claims.Sub != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("sub = %q", claims.Sub)
	}
	if claims.Org != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("org = %q", claims.Org)
	}
	if claims.Exp != 1750003600 {
		t.Errorf("exp = %d", claims.Exp)
	}
	if len(claims.Scope) != 2 || claims.Scope[0] != "read:messages" {
		t.Errorf("scope = %v", claims.Scope)
	}
}

// ----- AuthContext derivation (matches Rust SDK claims_expose_roles_and_email_from_data_envelope) -----

func TestAuthContext_RolesAndEmailFromDataEnvelope(t *testing.T) {
	var claims TokenClaims
	if err := json.Unmarshal([]byte(fixtureClaimsJSON), &claims); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	auth := claims.AuthContext()

	// roles split from "owner"
	if len(auth.Roles) != 1 || auth.Roles[0] != "owner" {
		t.Errorf("expected Roles = [\"owner\"], got %v", auth.Roles)
	}

	// email propagated
	if auth.Email == nil || *auth.Email != "test@example.com" {
		t.Errorf("expected Email = \"test@example.com\", got %v", auth.Email)
	}

	// standard fields forwarded
	if auth.UserID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("UserID = %q", auth.UserID)
	}
	if auth.OrgID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("OrgID = %q", auth.OrgID)
	}
	if len(auth.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %v", auth.Scopes)
	}
}

func TestAuthContext_MultipleRoles(t *testing.T) {
	payload := `{
		"sub": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"org": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		"exp": 9999999999,
		"iat": 0,
		"data": {"roles": "org_admin,leadership"}
	}`
	var claims TokenClaims
	if err := json.Unmarshal([]byte(payload), &claims); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	auth := claims.AuthContext()
	if len(auth.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %v", auth.Roles)
	}
	if auth.Roles[0] != "org_admin" || auth.Roles[1] != "leadership" {
		t.Errorf("unexpected roles: %v", auth.Roles)
	}
}

func TestAuthContext_SpaceDelimitedRoles(t *testing.T) {
	payload := `{
		"sub": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"org": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		"exp": 9999999999,
		"iat": 0,
		"data": {"roles": "admin member"}
	}`
	var claims TokenClaims
	if err := json.Unmarshal([]byte(payload), &claims); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	auth := claims.AuthContext()
	if len(auth.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %v", auth.Roles)
	}
	if auth.Roles[0] != "admin" || auth.Roles[1] != "member" {
		t.Errorf("unexpected roles: %v", auth.Roles)
	}
}

func TestAuthContext_NoDataEnvelope(t *testing.T) {
	payload := `{
		"sub": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"org": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		"exp": 9999999999,
		"iat": 0
	}`
	var claims TokenClaims
	if err := json.Unmarshal([]byte(payload), &claims); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	auth := claims.AuthContext()
	if len(auth.Roles) != 0 {
		t.Errorf("expected empty Roles, got %v", auth.Roles)
	}
	if auth.Email != nil {
		t.Errorf("expected nil Email, got %v", auth.Email)
	}
}

func TestAuthContext_EmptyRolesString(t *testing.T) {
	payload := `{
		"sub": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"org": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		"exp": 9999999999,
		"iat": 0,
		"data": {"roles": ""}
	}`
	var claims TokenClaims
	if err := json.Unmarshal([]byte(payload), &claims); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	auth := claims.AuthContext()
	if len(auth.Roles) != 0 {
		t.Errorf("expected empty Roles for empty string, got %v", auth.Roles)
	}
}

// ----- ParseTokenClaims -----

func TestParseTokenClaims_FixtureToken(t *testing.T) {
	claims, err := ParseTokenClaims(fixtureToken)
	if err != nil {
		t.Fatalf("ParseTokenClaims: %v", err)
	}

	if claims.Sub != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("sub = %q", claims.Sub)
	}
	if claims.Data == nil {
		t.Fatal("expected data envelope in parsed claims")
	}
	if claims.Data.Roles == nil || *claims.Data.Roles != "owner" {
		t.Errorf("data.roles = %v", claims.Data.Roles)
	}
	if claims.Data.Email == nil || *claims.Data.Email != "test@example.com" {
		t.Errorf("data.email = %v", claims.Data.Email)
	}

	// Verify the full enrichment pipeline: ParseTokenClaims → AuthContext
	auth := claims.AuthContext()
	found := false
	for _, r := range auth.Roles {
		if r == "owner" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Roles to contain \"owner\", got %v", auth.Roles)
	}
	if auth.Email == nil || *auth.Email != "test@example.com" {
		t.Errorf("Email = %v", auth.Email)
	}
}

func TestParseTokenClaims_MalformedToken(t *testing.T) {
	cases := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"only-two-parts", "header.payload"},
		{"four-parts", "a.b.c.d"},
		{"invalid-base64", "header.!!!.sig"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTokenClaims(tc.token)
			if err == nil {
				t.Errorf("expected error for token %q, got nil", tc.token)
			}
		})
	}
}

func TestParseTokenClaims_InvalidPayloadJSON(t *testing.T) {
	// Build a token with valid base64url but invalid JSON payload.
	import64 := "bm90anNvbg" // base64url("notjson")
	token := "header." + import64 + ".sig"
	_, err := ParseTokenClaims(token)
	if err == nil {
		t.Fatal("expected JSON unmarshal error")
	}
	if !strings.Contains(err.Error(), "JSON unmarshal") {
		t.Errorf("expected 'JSON unmarshal' in error, got %q", err.Error())
	}
}
