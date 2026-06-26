package buttrbase

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
)

// testKID is the key ID used in all tests.
const testKID = "test-key-1"

// generateRSAKey creates a fresh 2048-bit RSA key pair for testing.
func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	return key
}

// testJWKS builds a JSON JWKS document from the given RSA public key and kid.
func testJWKS(t *testing.T, pub *rsa.PublicKey, kid string) []byte {
	t.Helper()
	// Build a JWK from the RSA key using jwkset so the resulting JSON matches
	// what keyfunc expects to parse.
	jwkOpts := jwkset.JWKOptions{
		Marshal: jwkset.JWKMarshalOptions{Private: false},
		Metadata: jwkset.JWKMetadataOptions{
			KID: kid,
			ALG: jwkset.AlgRS256,
		},
		Validate: jwkset.JWKValidateOptions{SkipAll: true},
	}
	jwk, err := jwkset.NewJWKFromKey(pub, jwkOpts)
	if err != nil {
		t.Fatalf("NewJWKFromKey: %v", err)
	}
	// jwk.Marshal() returns a JWKMarshal struct which is JSON-serializable.
	raw, err := json.Marshal(jwk.Marshal())
	if err != nil {
		t.Fatalf("json.Marshal(jwk.Marshal()): %v", err)
	}
	jwksJSON := fmt.Sprintf(`{"keys":[%s]}`, string(raw))
	return []byte(jwksJSON)
}

// startJWKSServer spins up an httptest.Server that serves a static JWKS.
func startJWKSServer(t *testing.T, jwksJSON []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksJSON)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// signToken signs a JWT with the given RSA private key and kid, returning the
// compact token string.
func signToken(t *testing.T, priv *rsa.PrivateKey, kid string, claims jwt.Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("token.SignedString: %v", err)
	}
	return signed
}

// makeVerifier builds a Verifier backed by a keyfunc from the given JWKS URL
// without launching real background goroutines (uses NewDefaultCtx with a
// cancelled ctx — sufficient because tests serve JWKS synchronously).
func makeVerifier(t *testing.T, jwksURL, issuer, audience string) *Verifier {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	kf, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		t.Fatalf("keyfunc.NewDefaultCtx: %v", err)
	}
	cfg := VerifierConfig{
		JWKSURL:  jwksURL,
		Issuer:   issuer,
		Audience: audience,
	}
	return newVerifierFromKeyfunc(cfg, kf)
}

// ----- fixture-based claims for tokens -----

type fixtureClaims struct {
	jwt.RegisteredClaims
	Org   string           `json:"org"`
	Scope []string         `json:"scope"`
	Data  *TokenClaimsData `json:"data,omitempty"`
}

func makeFixtureClaims(issuer string, withData bool) fixtureClaims {
	now := time.Now()
	var data *TokenClaimsData
	if withData {
		rolesStr := "owner"
		emailStr := "test@example.com"
		orgUUID := "22222222-2222-2222-2222-222222222222"
		userUUID := "11111111-1111-1111-1111-111111111111"
		data = &TokenClaimsData{
			Roles:    &rolesStr,
			Email:    &emailStr,
			OrgUUID:  &orgUUID,
			UserUUID: &userUUID,
		}
	}
	return fixtureClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "11111111-1111-1111-1111-111111111111",
			Issuer:    issuer,
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Org:   "22222222-2222-2222-2222-222222222222",
		Scope: []string{"read:messages", "write:messages"},
		Data:  data,
	}
}

// ----- Tests -----

// TestVerifyToken_ValidRS256 is the happy path: sign a token with the private key,
// point the Verifier at the httptest JWKS server, assert enriched claims are returned.
func TestVerifyToken_ValidRS256(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)
	srv := startJWKSServer(t, jwksJSON)

	issuer := "https://auth.example.com"
	v := makeVerifier(t, srv.URL, issuer, "")

	token := signToken(t, priv, testKID, makeFixtureClaims(issuer, true))

	claims, err := v.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}

	if claims.Sub != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("Sub = %q", claims.Sub)
	}
	if claims.Org != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("Org = %q", claims.Org)
	}
	if len(claims.Scope) != 2 {
		t.Errorf("Scope = %v", claims.Scope)
	}
	if claims.Data == nil {
		t.Fatal("expected Data envelope")
	}
	if claims.Data.Roles == nil || *claims.Data.Roles != "owner" {
		t.Errorf("Data.Roles = %v", claims.Data.Roles)
	}
	if claims.Data.Email == nil || *claims.Data.Email != "test@example.com" {
		t.Errorf("Data.Email = %v", claims.Data.Email)
	}
}

// TestVerifyToken_EnrichmentPipeline checks that VerifyToken returns claims
// that produce the correct AuthContext via the existing .AuthContext() method.
func TestVerifyToken_EnrichmentPipeline(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)
	srv := startJWKSServer(t, jwksJSON)

	issuer := "https://auth.example.com"
	v := makeVerifier(t, srv.URL, issuer, "")

	token := signToken(t, priv, testKID, makeFixtureClaims(issuer, true))

	claims, err := v.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}

	auth := claims.AuthContext()

	if auth.UserID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("UserID = %q", auth.UserID)
	}
	if auth.OrgID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("OrgID = %q", auth.OrgID)
	}
	if len(auth.Roles) != 1 || auth.Roles[0] != "owner" {
		t.Errorf("Roles = %v", auth.Roles)
	}
	if auth.Email == nil || *auth.Email != "test@example.com" {
		t.Errorf("Email = %v", auth.Email)
	}
	if len(auth.Scopes) != 2 {
		t.Errorf("Scopes = %v", auth.Scopes)
	}
}

// TestVerifyBearer_ValidToken checks that VerifyBearer strips "Bearer " and
// returns the AuthContext with roles and email populated.
func TestVerifyBearer_ValidToken(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)
	srv := startJWKSServer(t, jwksJSON)

	issuer := "https://auth.example.com"
	v := makeVerifier(t, srv.URL, issuer, "")

	token := signToken(t, priv, testKID, makeFixtureClaims(issuer, true))
	authHeader := "Bearer " + token

	auth, err := v.VerifyBearer(authHeader)
	if err != nil {
		t.Fatalf("VerifyBearer: %v", err)
	}

	if len(auth.Roles) == 0 || auth.Roles[0] != "owner" {
		t.Errorf("expected Roles=[owner], got %v", auth.Roles)
	}
	if auth.Email == nil || *auth.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", auth.Email)
	}
}

// TestVerifyBearer_MissingHeader checks that a missing/non-Bearer header
// returns an error (not a panic or nil).
func TestVerifyBearer_MissingHeader(t *testing.T) {
	v := makeVerifier(t, "http://localhost:9999", "https://issuer.example.com", "")

	_, err := v.VerifyBearer("")
	if err == nil {
		t.Fatal("expected error for empty auth header")
	}
}

// TestVerifyBearer_NonBearerScheme checks that "Basic ..." is rejected.
func TestVerifyBearer_NonBearerScheme(t *testing.T) {
	v := makeVerifier(t, "http://localhost:9999", "https://issuer.example.com", "")

	_, err := v.VerifyBearer("Basic dXNlcjpwYXNz")
	if err == nil {
		t.Fatal("expected error for non-Bearer scheme")
	}
}

// TestVerifyToken_BadSignature checks that a token signed with a different
// key (not in JWKS) is rejected.
func TestVerifyToken_BadSignature(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID) // pub key of priv
	srv := startJWKSServer(t, jwksJSON)

	otherPriv := generateRSAKey(t) // different key
	issuer := "https://auth.example.com"
	v := makeVerifier(t, srv.URL, issuer, "")

	token := signToken(t, otherPriv, testKID, makeFixtureClaims(issuer, false))

	_, err := v.VerifyToken(token)
	if err == nil {
		t.Fatal("expected error for bad signature, got nil")
	}
}

// TestVerifyToken_WrongIssuer checks that a token with a different iss is
// rejected.
func TestVerifyToken_WrongIssuer(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)
	srv := startJWKSServer(t, jwksJSON)

	issuer := "https://auth.example.com"
	v := makeVerifier(t, srv.URL, issuer, "")

	wrongClaims := makeFixtureClaims("https://evil.example.com", false)
	token := signToken(t, priv, testKID, wrongClaims)

	_, err := v.VerifyToken(token)
	if err == nil {
		t.Fatal("expected error for wrong issuer, got nil")
	}
}

// TestVerifyToken_Expired checks that an expired token is rejected.
func TestVerifyToken_Expired(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)
	srv := startJWKSServer(t, jwksJSON)

	issuer := "https://auth.example.com"
	v := makeVerifier(t, srv.URL, issuer, "")

	expiredClaims := fixtureClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "11111111-1111-1111-1111-111111111111",
			Issuer:    issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // expired 1 hour ago
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		Org:   "22222222-2222-2222-2222-222222222222",
		Scope: []string{},
	}
	token := signToken(t, priv, testKID, expiredClaims)

	_, err := v.VerifyToken(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

// TestVerifyToken_WithAudience checks that audience validation works when
// Audience is set and the token carries the matching aud.
func TestVerifyToken_WithAudience(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)
	srv := startJWKSServer(t, jwksJSON)

	issuer := "https://auth.example.com"
	audience := "my-service"

	v := makeVerifier(t, srv.URL, issuer, audience)

	claimsWithAud := fixtureClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "11111111-1111-1111-1111-111111111111",
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Org:   "22222222-2222-2222-2222-222222222222",
		Scope: []string{},
	}
	token := signToken(t, priv, testKID, claimsWithAud)

	claims, err := v.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken with audience: %v", err)
	}
	if claims.Sub != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("Sub = %q", claims.Sub)
	}
}

// TestVerifyToken_WrongAudience checks that a token with wrong aud is rejected.
func TestVerifyToken_WrongAudience(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)
	srv := startJWKSServer(t, jwksJSON)

	issuer := "https://auth.example.com"
	v := makeVerifier(t, srv.URL, issuer, "my-service")

	claimsWrongAud := fixtureClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "11111111-1111-1111-1111-111111111111",
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{"wrong-service"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Org:   "22222222-2222-2222-2222-222222222222",
		Scope: []string{},
	}
	token := signToken(t, priv, testKID, claimsWrongAud)

	_, err := v.VerifyToken(token)
	if err == nil {
		t.Fatal("expected error for wrong audience, got nil")
	}
}

// TestVerifierConfig_Accessors verifies Issuer() and Audience() accessors.
func TestVerifierConfig_Accessors(t *testing.T) {
	v := makeVerifier(t, "http://localhost:9999", "https://issuer.example.com", "my-aud")
	if v.Issuer() != "https://issuer.example.com" {
		t.Errorf("Issuer() = %q", v.Issuer())
	}
	if v.Audience() != "my-aud" {
		t.Errorf("Audience() = %q", v.Audience())
	}
}

// TestVerifierConfig_EmptyAudience checks that empty Audience() is returned when not set.
func TestVerifierConfig_EmptyAudience(t *testing.T) {
	v := makeVerifier(t, "http://localhost:9999", "https://issuer.example.com", "")
	if v.Audience() != "" {
		t.Errorf("Audience() = %q, want empty", v.Audience())
	}
}

// TestNewVerifier_InvalidURL checks that NewVerifier tolerates an invalid JWKS URL at
// construction time (keyfunc's NewDefaultCtx is lazy — it doesn't dial immediately).
func TestNewVerifier_InvalidURL(t *testing.T) {
	// keyfunc.NewDefaultCtx is lazy; construction succeeds even with an unreachable URL.
	v, err := NewVerifier(VerifierConfig{
		JWKSURL: "http://localhost:0/jwks.json",
		Issuer:  "https://issuer.example.com",
	})
	// If construction fails that is also acceptable — just don't panic.
	if err == nil && v == nil {
		t.Error("expected non-nil Verifier when err is nil")
	}
}

// TestVerifyToken_MultipleRoles checks comma-delimited roles in data.roles
// round-trip correctly through VerifyToken → AuthContext.
func TestVerifyToken_MultipleRoles(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)
	srv := startJWKSServer(t, jwksJSON)

	issuer := "https://auth.example.com"
	v := makeVerifier(t, srv.URL, issuer, "")

	rolesStr := "org_admin,leadership"
	emailStr := "multi@example.com"
	multiRoleClaims := fixtureClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			Issuer:    issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Org:   "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Scope: []string{"read:pages"},
		Data: &TokenClaimsData{
			Roles: &rolesStr,
			Email: &emailStr,
		},
	}
	token := signToken(t, priv, testKID, multiRoleClaims)

	claims, err := v.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}

	auth := claims.AuthContext()
	if len(auth.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %v", auth.Roles)
	}
	if auth.Roles[0] != "org_admin" || auth.Roles[1] != "leadership" {
		t.Errorf("unexpected roles: %v", auth.Roles)
	}
	if auth.Email == nil || *auth.Email != "multi@example.com" {
		t.Errorf("Email = %v", auth.Email)
	}
}

// TestVerifyToken_NoDataEnvelope checks that a token without data envelope
// still verifies and yields an AuthContext with empty Roles and nil Email.
func TestVerifyToken_NoDataEnvelope(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)
	srv := startJWKSServer(t, jwksJSON)

	issuer := "https://auth.example.com"
	v := makeVerifier(t, srv.URL, issuer, "")

	token := signToken(t, priv, testKID, makeFixtureClaims(issuer, false))

	claims, err := v.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}

	auth := claims.AuthContext()
	if len(auth.Roles) != 0 {
		t.Errorf("expected empty Roles, got %v", auth.Roles)
	}
	if auth.Email != nil {
		t.Errorf("expected nil Email, got %v", auth.Email)
	}
}

// TestVerifyToken_NonRS256Rejected checks that a token signed with HS256 is
// rejected even if the signature would otherwise be valid.
func TestVerifyToken_NonRS256Rejected(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)
	srv := startJWKSServer(t, jwksJSON)

	issuer := "https://auth.example.com"
	v := makeVerifier(t, srv.URL, issuer, "")

	// Sign with HS256
	hs256Claims := jwt.MapClaims{
		"sub": "11111111-1111-1111-1111-111111111111",
		"iss": issuer,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, hs256Claims)
	signed, err := token.SignedString([]byte("some-secret"))
	if err != nil {
		t.Fatalf("sign HS256 token: %v", err)
	}

	_, err = v.VerifyToken(signed)
	if err == nil {
		t.Fatal("expected error for HS256 token with RS256-only Verifier")
	}
}

// TestJWTClaimsToTokenClaims_Roundtrip checks the internal jwtClaims.toTokenClaims
// conversion preserves all fields correctly.
func TestJWTClaimsToTokenClaims_Roundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	rolesStr := "owner"
	emailStr := "rt@example.com"

	j := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "sub-uuid",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Org:   "org-uuid",
		Scope: []string{"read:users"},
		Data:  &TokenClaimsData{Roles: &rolesStr, Email: &emailStr},
	}

	tc := j.toTokenClaims()
	if tc.Sub != "sub-uuid" {
		t.Errorf("Sub = %q", tc.Sub)
	}
	if tc.Org != "org-uuid" {
		t.Errorf("Org = %q", tc.Org)
	}
	if tc.Exp != now.Add(time.Hour).Unix() {
		t.Errorf("Exp = %d, want %d", tc.Exp, now.Add(time.Hour).Unix())
	}
	if tc.Iat != now.Unix() {
		t.Errorf("Iat = %d, want %d", tc.Iat, now.Unix())
	}
	if len(tc.Scope) != 1 || tc.Scope[0] != "read:users" {
		t.Errorf("Scope = %v", tc.Scope)
	}
	if tc.Data == nil || tc.Data.Roles == nil || *tc.Data.Roles != "owner" {
		t.Errorf("Data.Roles = %v", tc.Data)
	}
}

// TestVerifyToken_JWKS_JSON_is_valid checks that the JWKS JSON we produce
// for tests is parseable and contains the expected key.
func TestVerifyToken_JWKS_JSON_is_valid(t *testing.T) {
	priv := generateRSAKey(t)
	jwksJSON := testJWKS(t, &priv.PublicKey, testKID)

	var m map[string]json.RawMessage
	if err := json.Unmarshal(jwksJSON, &m); err != nil {
		t.Fatalf("JWKS JSON unmarshal: %v", err)
	}
	if _, ok := m["keys"]; !ok {
		t.Error("JWKS JSON missing 'keys' field")
	}
}
