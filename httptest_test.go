package buttrbase

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---- helpers ----

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New("test-token", WithBaseURL(srv.URL))
	return srv, c
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func assertAuth(t *testing.T, r *http.Request) {
	t.Helper()
	auth := r.Header.Get("Authorization")
	if auth != "Bearer test-token" {
		t.Errorf("expected Authorization 'Bearer test-token', got %q", auth)
	}
}

func assertMethod(t *testing.T, r *http.Request, method string) {
	t.Helper()
	if r.Method != method {
		t.Errorf("expected method %s, got %s", method, r.Method)
	}
}

func assertPath(t *testing.T, r *http.Request, path string) {
	t.Helper()
	if r.URL.Path != path {
		t.Errorf("expected path %s, got %s", path, r.URL.Path)
	}
}

// ---- New / WithHTTPClient / WithBaseURL ----

func TestNew_Defaults(t *testing.T) {
	c := New("my-token")
	if c.AccessToken != "my-token" {
		t.Errorf("expected AccessToken 'my-token', got %q", c.AccessToken)
	}
	if c.BaseURL != defaultBaseURL {
		t.Errorf("expected base URL %q, got %q", defaultBaseURL, c.BaseURL)
	}
	if c.HTTPClient == nil {
		t.Fatal("expected non-nil HTTPClient")
	}
}

func TestWithBaseURL(t *testing.T) {
	c := New("key", WithBaseURL("http://custom.example.com"))
	if c.BaseURL != "http://custom.example.com" {
		t.Errorf("got %q", c.BaseURL)
	}
}

func TestWithHTTPClient(t *testing.T) {
	custom := &http.Client{}
	c := New("key", WithHTTPClient(custom))
	if c.HTTPClient != custom {
		t.Error("expected custom http client to be set")
	}
}

// ---- ButtrbaseError ----

func TestButtrbaseError_WithDetail(t *testing.T) {
	e := &ButtrbaseError{StatusCode: 404, Detail: "not found", Body: []byte(`{"detail":"not found"}`)}
	want := "buttrbase: HTTP 404: not found"
	if e.Error() != want {
		t.Errorf("expected %q, got %q", want, e.Error())
	}
}

func TestButtrbaseError_WithoutDetail(t *testing.T) {
	e := &ButtrbaseError{StatusCode: 500, Body: []byte(`internal error`)}
	want := "buttrbase: HTTP 500"
	if e.Error() != want {
		t.Errorf("expected %q, got %q", want, e.Error())
	}
}

// ---- do — error paths ----

func TestDo_4xxError_WithDetailField(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 422, map[string]any{"detail": "invalid code"})
	})
	_, err := c.ValidateCoupon(context.Background(), "BAD", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	be, ok := err.(*ButtrbaseError)
	if !ok {
		t.Fatalf("expected *ButtrbaseError, got %T", err)
	}
	if be.StatusCode != 422 {
		t.Errorf("expected 422, got %d", be.StatusCode)
	}
	if be.Detail != "invalid code" {
		t.Errorf("expected detail 'invalid code', got %q", be.Detail)
	}
}

func TestDo_4xxError_NoDetailField(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("bad request"))
	})
	_, err := c.ValidateCoupon(context.Background(), "X", nil)
	be, ok := err.(*ButtrbaseError)
	if !ok {
		t.Fatalf("expected *ButtrbaseError, got %T", err)
	}
	if be.StatusCode != 400 {
		t.Errorf("expected 400, got %d", be.StatusCode)
	}
	if be.Detail != "" {
		t.Errorf("expected empty detail, got %q", be.Detail)
	}
}

func TestDo_InvalidJSONResponse(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("not-json{{"))
	})
	_, err := c.ValidateCoupon(context.Background(), "X", nil)
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("expected 'decode response' in error, got %q", err.Error())
	}
}

// ---- Client-credentials token grant ----

// TestClientCredentials_AutoFetchAndReuse verifies that a client configured
// with client_id/client_secret (and no access token) automatically exchanges
// them for a bearer token before the first authenticated request, then reuses
// that token on subsequent requests without re-hitting the token endpoint.
func TestClientCredentials_AutoFetchAndReuse(t *testing.T) {
	var tokenHits, couponHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/token":
			tokenHits++
			assertMethod(t, r, http.MethodPost)
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("token endpoint should not receive Authorization header, got %q", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode token request: %v", err)
			}
			if body["grant_type"] != "client_credentials" {
				t.Errorf("grant_type = %v, want client_credentials", body["grant_type"])
			}
			if body["client_id"] != "cid" || body["client_secret"] != "csecret" {
				t.Errorf("creds = %v/%v, want cid/csecret", body["client_id"], body["client_secret"])
			}
			writeJSON(w, 200, map[string]any{
				"access_token": "jwt-abc",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/v1/coupons/validate":
			couponHits++
			if got := r.Header.Get("Authorization"); got != "Bearer jwt-abc" {
				t.Errorf("expected 'Bearer jwt-abc', got %q", got)
			}
			writeJSON(w, 200, map[string]any{"valid": true})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	c := New("", WithBaseURL(srv.URL), WithClientCredentials("cid", "csecret"))
	if c.AccessToken != "" {
		t.Fatalf("expected empty initial token, got %q", c.AccessToken)
	}

	for i := 0; i < 3; i++ {
		if _, err := c.ValidateCoupon(context.Background(), "X", nil); err != nil {
			t.Fatalf("ValidateCoupon #%d: %v", i, err)
		}
	}

	if tokenHits != 1 {
		t.Errorf("token endpoint hit %d times, want 1 (token should be cached)", tokenHits)
	}
	if couponHits != 3 {
		t.Errorf("coupon endpoint hit %d times, want 3", couponHits)
	}
	if c.AccessToken != "jwt-abc" {
		t.Errorf("cached token = %q, want jwt-abc", c.AccessToken)
	}
}

// TestClientCredentials_RefreshOnExpiry verifies that once the cached token is
// (near) expired the client fetches a new one on the next authenticated call.
func TestClientCredentials_RefreshOnExpiry(t *testing.T) {
	var tokenHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/token":
			tokenHits++
			writeJSON(w, 200, map[string]any{
				"access_token": "tok-" + strconv.Itoa(tokenHits),
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/v1/coupons/validate":
			writeJSON(w, 200, map[string]any{"valid": true})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	c := New("", WithBaseURL(srv.URL), WithClientCredentials("cid", "csecret"))
	if _, err := c.ValidateCoupon(context.Background(), "X", nil); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if tokenHits != 1 || c.AccessToken != "tok-1" {
		t.Fatalf("after first call: hits=%d token=%q", tokenHits, c.AccessToken)
	}

	// Force the cached token to look expired.
	c.tokenExpiry = time.Now().Add(-time.Minute)
	if _, err := c.ValidateCoupon(context.Background(), "X", nil); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if tokenHits != 2 || c.AccessToken != "tok-2" {
		t.Errorf("expected refresh: hits=%d token=%q, want hits=2 token=tok-2", tokenHits, c.AccessToken)
	}
}

// TestAuthenticate_BadCredentials verifies that bad creds surface the 401 from
// the token endpoint.
func TestAuthenticate_BadCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertPath(t, r, "/api/v1/auth/token")
		writeJSON(w, 401, map[string]any{"error": "invalid client credentials"})
	}))
	t.Cleanup(srv.Close)

	c := New("", WithBaseURL(srv.URL), WithClientCredentials("bad", "creds"))
	err := c.Authenticate(context.Background())
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
	be, ok := err.(*ButtrbaseError)
	if !ok {
		t.Fatalf("expected *ButtrbaseError, got %T", err)
	}
	if be.StatusCode != 401 {
		t.Errorf("expected 401, got %d", be.StatusCode)
	}
	if c.AccessToken != "" {
		t.Errorf("expected no token cached on failure, got %q", c.AccessToken)
	}
}

// TestAuthenticate_NoCredentials verifies Authenticate fails fast without creds.
func TestAuthenticate_NoCredentials(t *testing.T) {
	c := New("")
	if err := c.Authenticate(context.Background()); err == nil {
		t.Fatal("expected error when no client credentials configured")
	}
}

// ---- ValidateCoupon ----

func TestValidateCoupon_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/v1/coupons/validate")
		assertAuth(t, r)
		writeJSON(w, 200, CouponValidation{Valid: true, Code: "SAVE10", DiscountCents: 1000, DiscountType: "fixed"})
	})
	res, err := c.ValidateCoupon(context.Background(), "SAVE10", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Error("expected valid=true")
	}
	if res.Code != "SAVE10" {
		t.Errorf("expected code SAVE10, got %q", res.Code)
	}
}

func TestValidateCoupon_WithOptions(t *testing.T) {
	userID := 42
	var orderTotal int64 = 5000
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["user_id"] == nil {
			t.Error("expected user_id in body")
		}
		if body["order_total_cents"] == nil {
			t.Error("expected order_total_cents in body")
		}
		writeJSON(w, 200, CouponValidation{Valid: false, Reason: "min order not met"})
	})
	opts := &ValidateCouponOptions{UserID: &userID, OrderTotalCents: &orderTotal}
	res, err := c.ValidateCoupon(context.Background(), "CODE", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid {
		t.Error("expected valid=false")
	}
}

func TestValidateCoupon_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "unauthorized"})
	})
	_, err := c.ValidateCoupon(context.Background(), "X", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ValidateGiftCard ----

func TestValidateGiftCard_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/v1/giftcards/validate")
		assertAuth(t, r)
		writeJSON(w, 200, GiftCardValidation{Valid: true, Code: "GC123", BalanceCents: 2000})
	})
	res, err := c.ValidateGiftCard(context.Background(), "GC123")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Error("expected valid=true")
	}
	if res.BalanceCents != 2000 {
		t.Errorf("expected balance 2000, got %d", res.BalanceCents)
	}
}

func TestValidateGiftCard_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "gift card not found"})
	})
	_, err := c.ValidateGiftCard(context.Background(), "NOTEXIST")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- RedeemGiftCard ----

func TestRedeemGiftCard_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/v1/giftcards/redeem")
		assertAuth(t, r)
		writeJSON(w, 200, GiftCardRedemption{Success: true, Code: "GC123", RedeemedCents: 500, RemainingCents: 1500})
	})
	res, err := c.RedeemGiftCard(context.Background(), "GC123", 500, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Error("expected success=true")
	}
}

func TestRedeemGiftCard_WithUserID(t *testing.T) {
	uid := 7
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["user_id"] == nil {
			t.Error("expected user_id in body")
		}
		writeJSON(w, 200, GiftCardRedemption{Success: true})
	})
	_, err := c.RedeemGiftCard(context.Background(), "GC123", 500, &uid)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRedeemGiftCard_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, map[string]any{"detail": "insufficient balance"})
	})
	_, err := c.RedeemGiftCard(context.Background(), "GC123", 99999, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- SendMagicLink ----

func TestSendMagicLink_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/auth/magic-link/send")
		writeJSON(w, 200, MagicLinkSend{Sent: true, ExpiresInSeconds: 900})
	})
	res, err := c.SendMagicLink(context.Background(), "user@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Sent {
		t.Error("expected sent=true")
	}
	if res.ExpiresInSeconds != 900 {
		t.Errorf("expected expires_in_seconds=900, got %d", res.ExpiresInSeconds)
	}
}

func TestSendMagicLink_WithOptions(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["app_uuid"] == nil {
			t.Error("expected app_uuid in body")
		}
		if body["redirect_to"] == nil {
			t.Error("expected redirect_to in body")
		}
		if body["org_uuid"] == nil {
			t.Error("expected org_uuid in body")
		}
		writeJSON(w, 200, MagicLinkSend{Sent: true})
	})
	opts := &SendMagicLinkOptions{
		AppUUID:    "app-uuid-1",
		RedirectTo: "https://example.com/callback",
		OrgUUID:    "org-uuid-1",
	}
	_, err := c.SendMagicLink(context.Background(), "user@example.com", opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendMagicLink_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 422, map[string]any{"detail": "invalid email"})
	})
	_, err := c.SendMagicLink(context.Background(), "bad-email", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- VerifyMagicLink ----

func TestVerifyMagicLink_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/auth/magic-link/verify")
		writeJSON(w, 200, MagicLinkVerify{
			AccessToken: "rs256.jwt.token",
			TokenType:   "Bearer",
			User:        MagicLinkUser{UserUUID: "user-uuid-99", Email: "user@example.com"},
		})
	})
	res, err := c.VerifyMagicLink(context.Background(), "some-token")
	if err != nil {
		t.Fatal(err)
	}
	if res.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if res.User.Email != "user@example.com" {
		t.Errorf("unexpected user email: %q", res.User.Email)
	}
}

func TestVerifyMagicLink_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "token expired"})
	})
	_, err := c.VerifyMagicLink(context.Background(), "expired-token")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- MfaStatus ----

func TestMfaStatus_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/v1/mfa/status")
		assertAuth(t, r)
		writeJSON(w, 200, MfaStatus{Enrolled: true, Active: true, Label: "myapp"})
	})
	res, err := c.MfaStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Enrolled || !res.Active {
		t.Error("expected enrolled and active")
	}
}

func TestMfaStatus_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "unauthorized"})
	})
	_, err := c.MfaStatus(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- MfaEnroll ----

func TestMfaEnroll_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/v1/mfa/enroll")
		assertAuth(t, r)
		writeJSON(w, 200, MfaEnrollment{Secret: "BASE32SECRET", OtpauthURL: "otpauth://totp/...", Label: "myapp"})
	})
	res, err := c.MfaEnroll(context.Background(), "myapp")
	if err != nil {
		t.Fatal(err)
	}
	if res.Secret == "" {
		t.Error("expected non-empty secret")
	}
}

func TestMfaEnroll_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 409, map[string]any{"detail": "already enrolled"})
	})
	_, err := c.MfaEnroll(context.Background(), "myapp")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- MfaActivate ----

func TestMfaActivate_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/v1/mfa/activate")
		assertAuth(t, r)
		writeJSON(w, 200, MfaStatusResponse{Active: true, Label: "myapp"})
	})
	res, err := c.MfaActivate(context.Background(), "123456")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Active {
		t.Error("expected active=true")
	}
}

func TestMfaActivate_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, map[string]any{"detail": "invalid code"})
	})
	_, err := c.MfaActivate(context.Background(), "000000")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- OrgSign ----

func TestOrgSign_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/v1/orgs/org-uuid-123/sign")
		assertAuth(t, r)
		writeJSON(w, 200, OrgSignResponse{Token: "signed.jwt.token", Kid: "key-1"})
	})
	claims := map[string]any{"sub": "user-1", "role": "admin"}
	res, err := c.OrgSign(context.Background(), "org-uuid-123", claims, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestOrgSign_WithTTL(t *testing.T) {
	ttl := int64(3600)
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["ttl_seconds"] == nil {
			t.Error("expected ttl_seconds in body")
		}
		writeJSON(w, 200, OrgSignResponse{Token: "tok"})
	})
	_, err := c.OrgSign(context.Background(), "org-uuid-123", map[string]any{}, &ttl)
	if err != nil {
		t.Fatal(err)
	}
}

func TestOrgSign_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.OrgSign(context.Background(), "org-uuid-123", map[string]any{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- OrgJWKS ----

func TestOrgJWKS_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/v1/orgs/org-uuid-123/.well-known/jwks.json")
		// No auth needed for JWKS
		writeJSON(w, 200, JWKSResponse{Keys: []map[string]any{{"kty": "RSA", "kid": "key-1"}}})
	})
	res, err := c.OrgJWKS(context.Background(), "org-uuid-123")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Keys) == 0 {
		t.Error("expected at least one key")
	}
}

func TestOrgJWKS_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "org not found"})
	})
	_, err := c.OrgJWKS(context.Background(), "bad-uuid")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- GetSecret ----

func TestGetSecret_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/v1/orgs/org-abc/secrets/my-secret")
		assertAuth(t, r)
		writeJSON(w, 200, SecretGet{Name: "my-secret", Value: "s3cr3t", Description: "a secret"})
	})
	res, err := c.GetSecret(context.Background(), "org-abc", "my-secret")
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != "s3cr3t" {
		t.Errorf("expected value 's3cr3t', got %q", res.Value)
	}
}

func TestGetSecret_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "secret not found"})
	})
	_, err := c.GetSecret(context.Background(), "org-abc", "missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- PutSecret ----

func TestPutSecret_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPut)
		assertPath(t, r, "/v1/orgs/org-abc/secrets/my-secret")
		assertAuth(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["value"] == nil {
			t.Error("expected value in body")
		}
		writeJSON(w, 200, SecretSummary{Name: "my-secret", Description: "desc"})
	})
	res, err := c.PutSecret(context.Background(), "org-abc", "my-secret", "new-value", "desc")
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "my-secret" {
		t.Errorf("expected name 'my-secret', got %q", res.Name)
	}
}

func TestPutSecret_NoDescription(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["description"] != nil {
			t.Error("expected no description in body when empty")
		}
		writeJSON(w, 200, SecretSummary{Name: "sec"})
	})
	_, err := c.PutSecret(context.Background(), "org-abc", "sec", "val", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestPutSecret_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.PutSecret(context.Background(), "org-abc", "sec", "val", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- AuthStepUp ----

func TestAuthStepUp_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/auth/step-up")
		assertAuth(t, r)
		writeJSON(w, 200, StepUpResponse{AccessToken: "elevated-token", TokenType: "Bearer", ExpiresInSeconds: 300})
	})
	res, err := c.AuthStepUp(context.Background(), "123456", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.AccessToken != "elevated-token" {
		t.Errorf("expected elevated-token, got %q", res.AccessToken)
	}
	// Verify the client's access token was updated
	if c.AccessToken != "elevated-token" {
		t.Errorf("expected client AccessToken to be updated to elevated-token, got %q", c.AccessToken)
	}
}

func TestAuthStepUp_NoToken(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, StepUpResponse{TokenType: "Bearer", ExpiresInSeconds: 300})
	})
	originalKey := c.AccessToken
	_, err := c.AuthStepUp(context.Background(), "123456", false)
	if err != nil {
		t.Fatal(err)
	}
	// Token should not change if no access_token returned
	if c.AccessToken != originalKey {
		t.Errorf("expected access token to remain %q, got %q", originalKey, c.AccessToken)
	}
}

func TestAuthStepUp_Recovery(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if recovery, ok := body["recovery"].(bool); !ok || !recovery {
			t.Error("expected recovery=true in body")
		}
		writeJSON(w, 200, StepUpResponse{AccessToken: "recovery-token"})
	})
	_, err := c.AuthStepUp(context.Background(), "recovery-code", true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAuthStepUp_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "invalid code"})
	})
	_, err := c.AuthStepUp(context.Background(), "bad", false)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ElevationRequest ----

func TestElevationRequest_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/admin/orgs/org-abc/elevation/request")
		assertAuth(t, r)
		writeJSON(w, 200, ElevationGrant{GrantUUID: "grant-1", OrgUUID: "org-abc", Scope: "read", Status: "pending"})
	})
	res, err := c.ElevationRequest(context.Background(), "org-abc", "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.GrantUUID != "grant-1" {
		t.Errorf("expected grant-1, got %q", res.GrantUUID)
	}
}

func TestElevationRequest_WithOptions(t *testing.T) {
	ttl := int64(1800)
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["reason"] == nil {
			t.Error("expected reason in body")
		}
		if body["ttl_seconds"] == nil {
			t.Error("expected ttl_seconds in body")
		}
		writeJSON(w, 200, ElevationGrant{GrantUUID: "grant-1", Status: "pending"})
	})
	opts := &ElevationRequestOptions{Reason: "maintenance", TTLSeconds: &ttl}
	_, err := c.ElevationRequest(context.Background(), "org-abc", "write", opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestElevationRequest_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.ElevationRequest(context.Background(), "org-abc", "write", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ElevationApprove ----

func TestElevationApprove_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/admin/orgs/org-abc/elevation/grant-1/approve")
		assertAuth(t, r)
		writeJSON(w, 200, ElevationGrant{GrantUUID: "grant-1", Status: "approved"})
	})
	res, err := c.ElevationApprove(context.Background(), "org-abc", "grant-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "approved" {
		t.Errorf("expected approved, got %q", res.Status)
	}
}

func TestElevationApprove_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "cannot self-approve"})
	})
	_, err := c.ElevationApprove(context.Background(), "org-abc", "grant-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ElevationList ----

func TestElevationList_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/admin/orgs/org-abc/elevation")
		assertAuth(t, r)
		writeJSON(w, 200, []ElevationGrant{
			{GrantUUID: "grant-1", Status: "pending"},
			{GrantUUID: "grant-2", Status: "approved"},
		})
	})
	res, err := c.ElevationList(context.Background(), "org-abc", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Errorf("expected 2 grants, got %d", len(res))
	}
}

func TestElevationList_WithStatus(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "status=pending" {
			t.Errorf("expected query ?status=pending, got ?%s", r.URL.RawQuery)
		}
		writeJSON(w, 200, []ElevationGrant{{GrantUUID: "grant-1", Status: "pending"}})
	})
	res, err := c.ElevationList(context.Background(), "org-abc", "pending")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 grant, got %d", len(res))
	}
}

func TestElevationList_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.ElevationList(context.Background(), "org-abc", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- SpiffeIssueSvid ----

func TestSpiffeIssueSvid_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/admin/orgs/org-abc/spiffe/svid")
		assertAuth(t, r)
		writeJSON(w, 200, SpiffeSvidResponse{
			SpiffeID:      "spiffe://example.com/workload",
			SvidPEM:       "-----BEGIN CERTIFICATE-----\n...",
			PrivateKeyPEM: "-----BEGIN EC PRIVATE KEY-----\n...",
			IssuedAt:      "2024-01-01T00:00:00Z",
			ExpiresAt:     "2024-01-01T01:00:00Z",
		})
	})
	res, err := c.SpiffeIssueSvid(context.Background(), "org-abc", "/workload", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.SpiffeID == "" {
		t.Error("expected non-empty spiffe_id")
	}
}

func TestSpiffeIssueSvid_WithTTL(t *testing.T) {
	ttl := int64(3600)
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["ttl_seconds"] == nil {
			t.Error("expected ttl_seconds in body")
		}
		writeJSON(w, 200, SpiffeSvidResponse{SpiffeID: "spiffe://x"})
	})
	_, err := c.SpiffeIssueSvid(context.Background(), "org-abc", "/workload", &ttl)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSpiffeIssueSvid_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.SpiffeIssueSvid(context.Background(), "org-abc", "/workload", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ListAuthEvents ----

func TestListAuthEvents_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/admin/orgs/org-abc/auth-events")
		assertAuth(t, r)
		if r.URL.Query().Get("limit") != "50" {
			t.Errorf("expected limit=50, got %q", r.URL.Query().Get("limit"))
		}
		writeJSON(w, 200, []AuthEvent{{EventUUID: "ev-1", Kind: "login"}})
	})
	res, err := c.ListAuthEvents(context.Background(), "org-abc", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 event, got %d", len(res))
	}
}

func TestListAuthEvents_WithOptions(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "10" {
			t.Errorf("expected limit=10, got %q", q.Get("limit"))
		}
		if q.Get("user_uuid") != "user-123" {
			t.Errorf("expected user_uuid=user-123, got %q", q.Get("user_uuid"))
		}
		writeJSON(w, 200, []AuthEvent{})
	})
	opts := &ListAuthEventsOptions{Limit: 10, UserUUID: "user-123"}
	_, err := c.ListAuthEvents(context.Background(), "org-abc", opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListAuthEvents_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.ListAuthEvents(context.Background(), "org-abc", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ReencryptSecrets ----

func TestReencryptSecrets_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/admin/orgs/org-abc/reencrypt/secrets")
		assertAuth(t, r)
		writeJSON(w, 200, ReencryptResponse{Rotated: 5, NewKEKID: "kek-2"})
	})
	res, err := c.ReencryptSecrets(context.Background(), "org-abc")
	if err != nil {
		t.Fatal(err)
	}
	if res.Rotated != 5 {
		t.Errorf("expected 5 rotated, got %d", res.Rotated)
	}
}

func TestReencryptSecrets_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 500, map[string]any{"detail": "internal error"})
	})
	_, err := c.ReencryptSecrets(context.Background(), "org-abc")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ReencryptSigningKeys ----

func TestReencryptSigningKeys_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/admin/orgs/org-abc/reencrypt/signing-keys")
		assertAuth(t, r)
		writeJSON(w, 200, ReencryptResponse{Rotated: 3})
	})
	res, err := c.ReencryptSigningKeys(context.Background(), "org-abc")
	if err != nil {
		t.Fatal(err)
	}
	if res.Rotated != 3 {
		t.Errorf("expected 3 rotated, got %d", res.Rotated)
	}
}

func TestReencryptSigningKeys_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 500, map[string]any{"detail": "internal error"})
	})
	_, err := c.ReencryptSigningKeys(context.Background(), "org-abc")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ReencryptMtlsCa ----

func TestReencryptMtlsCa_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/admin/orgs/org-abc/reencrypt/mtls-ca")
		assertAuth(t, r)
		writeJSON(w, 200, ReencryptResponse{Rotated: 1})
	})
	res, err := c.ReencryptMtlsCa(context.Background(), "org-abc")
	if err != nil {
		t.Fatal(err)
	}
	if res.Rotated != 1 {
		t.Errorf("expected 1 rotated, got %d", res.Rotated)
	}
}

func TestReencryptMtlsCa_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 500, map[string]any{"detail": "error"})
	})
	_, err := c.ReencryptMtlsCa(context.Background(), "org-abc")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- RevokeSession ----

func TestRevokeSession_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/admin/sessions/revoke")
		assertAuth(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["jti"] == nil {
			t.Error("expected jti in body")
		}
		writeJSON(w, 200, RevokeSessionResponse{JTI: "jti-123", Revoked: true})
	})
	res, err := c.RevokeSession(context.Background(), "jti-123", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Revoked {
		t.Error("expected revoked=true")
	}
}

func TestRevokeSession_WithTTL(t *testing.T) {
	ttl := int64(86400)
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["ttl_seconds"] == nil {
			t.Error("expected ttl_seconds in body")
		}
		writeJSON(w, 200, RevokeSessionResponse{JTI: "jti-1", Revoked: true})
	})
	_, err := c.RevokeSession(context.Background(), "jti-1", &ttl)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRevokeSession_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "session not found"})
	})
	_, err := c.RevokeSession(context.Background(), "bad-jti", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- GetOrgMetrics ----

func TestGetOrgMetrics_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/admin/orgs/org-abc/metrics")
		assertAuth(t, r)
		writeJSON(w, 200, OrgMetrics{ActiveUsers: 10, ActiveSessions: 5, SecretsCount: 3})
	})
	res, err := c.GetOrgMetrics(context.Background(), "org-abc")
	if err != nil {
		t.Fatal(err)
	}
	if res.ActiveUsers != 10 {
		t.Errorf("expected 10 active users, got %d", res.ActiveUsers)
	}
}

func TestGetOrgMetrics_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.GetOrgMetrics(context.Background(), "org-abc")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ListCredentials ----

func TestListCredentials_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/credentials")
		assertAuth(t, r)
		writeJSON(w, 200, CredentialList{Data: []Credential{
			{CredentialsID: "cred-1", ClientID: "client-1", Name: "My Cred"},
		}})
	})
	res, err := c.ListCredentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Data) != 1 {
		t.Errorf("expected 1 credential, got %d", len(res.Data))
	}
}

func TestListCredentials_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"detail": "unauthorized"})
	})
	_, err := c.ListCredentials(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- CreateCredential ----

func TestCreateCredential_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/credentials")
		assertAuth(t, r)
		var body CreateCredentialRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "My New Cred" {
			t.Errorf("expected name 'My New Cred', got %q", body.Name)
		}
		writeJSON(w, 201, Credential{
			CredentialsID: "cred-new",
			ClientID:      "client-new",
			ClientSecret:  "secret-value",
			Name:          body.Name,
		})
	})
	req := CreateCredentialRequest{Name: "My New Cred", Description: "Test"}
	res, err := c.CreateCredential(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.ClientSecret == "" {
		t.Error("expected non-empty client_secret on create")
	}
}

func TestCreateCredential_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 422, map[string]any{"detail": "name required"})
	})
	_, err := c.CreateCredential(context.Background(), CreateCredentialRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- GetCredential ----

func TestGetCredential_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/credentials/cred-1")
		assertAuth(t, r)
		writeJSON(w, 200, Credential{CredentialsID: "cred-1", ClientID: "client-1", Name: "Test"})
	})
	res, err := c.GetCredential(context.Background(), "cred-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.CredentialsID != "cred-1" {
		t.Errorf("expected cred-1, got %q", res.CredentialsID)
	}
}

func TestGetCredential_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "not found"})
	})
	_, err := c.GetCredential(context.Background(), "bad-id")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- DeleteCredential ----

func TestDeleteCredential_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodDelete)
		assertPath(t, r, "/credentials/cred-1")
		assertAuth(t, r)
		w.WriteHeader(204)
	})
	err := c.DeleteCredential(context.Background(), "cred-1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteCredential_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "not found"})
	})
	err := c.DeleteCredential(context.Background(), "bad-id")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- RotateCredentialSecret ----

func TestRotateCredentialSecret_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/credentials/cred-1/rotate-secret")
		assertAuth(t, r)
		writeJSON(w, 200, RotateSecretResponse{
			CredentialsID: "cred-1",
			ClientID:      "client-1",
			ClientSecret:  "new-secret-value",
		})
	})
	res, err := c.RotateCredentialSecret(context.Background(), "cred-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.ClientSecret == "" {
		t.Error("expected non-empty client_secret after rotation")
	}
}

func TestRotateCredentialSecret_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "credential not found"})
	})
	_, err := c.RotateCredentialSecret(context.Background(), "bad-id")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ResetSandbox ----

func TestResetSandbox_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/sandbox/reset")
		assertAuth(t, r)
		writeJSON(w, 200, SandboxResetResponse{Reset: true, Message: "sandbox reset"})
	})
	res, err := c.ResetSandbox(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Reset {
		t.Error("expected reset=true")
	}
}

func TestResetSandbox_WithRequest(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body SandboxResetRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.OrgUUID != "org-abc" {
			t.Errorf("expected org-abc, got %q", body.OrgUUID)
		}
		writeJSON(w, 200, SandboxResetResponse{Reset: true})
	})
	req := &SandboxResetRequest{OrgUUID: "org-abc"}
	res, err := c.ResetSandbox(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Reset {
		t.Error("expected reset=true")
	}
}

func TestResetSandbox_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.ResetSandbox(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- InviteAccept ----

func TestInviteAccept_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/auth/invite/accept")
		// No auth for this endpoint
		writeJSON(w, 200, InviteAcceptResponse{
			UserUUID:    "user-new",
			OrgUUID:     "org-abc",
			Role:        "member",
			AccessToken: "access-tok",
			TokenType:   "Bearer",
		})
	})
	req := InviteAcceptRequest{
		Token:     "invite-token",
		FirstName: "John",
		LastName:  "Doe",
		Username:  "johndoe",
		Password:  "s3cr3t!",
	}
	res, err := c.InviteAccept(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.UserUUID == "" {
		t.Error("expected non-empty user_uuid")
	}
}

func TestInviteAccept_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 410, map[string]any{"detail": "invite expired"})
	})
	_, err := c.InviteAccept(context.Background(), InviteAcceptRequest{Token: "expired"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- CheckOrgName ----

func TestCheckOrgName_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/auth/check-org-name")
		if r.URL.Query().Get("name") != "acme" {
			t.Errorf("expected name=acme, got %q", r.URL.Query().Get("name"))
		}
		writeJSON(w, 200, OrgCheckResponse{Name: "acme", Available: true})
	})
	res, err := c.CheckOrgName(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Available {
		t.Error("expected available=true")
	}
}

func TestCheckOrgName_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, map[string]any{"detail": "invalid name"})
	})
	_, err := c.CheckOrgName(context.Background(), "bad name!")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- GetSuperuserFlag ----

func TestGetSuperuserFlag_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/admin/superuser")
		assertAuth(t, r)
		if r.URL.Query().Get("email") != "admin@example.com" {
			t.Errorf("expected email=admin@example.com, got %q", r.URL.Query().Get("email"))
		}
		writeJSON(w, 200, SuperuserResponse{Email: "admin@example.com", IsSuperuser: true})
	})
	res, err := c.GetSuperuserFlag(context.Background(), "admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsSuperuser {
		t.Error("expected is_superuser=true")
	}
}

func TestGetSuperuserFlag_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.GetSuperuserFlag(context.Background(), "user@example.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- PostContact ----

func TestPostContact_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/contact")
		var body ContactRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Email == "" {
			t.Error("expected non-empty email")
		}
		writeJSON(w, 200, ContactSubmitResponse{Message: "submitted", ReferenceID: "ref-1"})
	})
	req := ContactRequest{
		Name:    "Jane Doe",
		Email:   "jane@example.com",
		Message: "Hello there",
	}
	res, err := c.PostContact(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.ReferenceID == "" {
		t.Error("expected non-empty reference_id")
	}
}

func TestPostContact_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 422, map[string]any{"detail": "invalid email"})
	})
	_, err := c.PostContact(context.Background(), ContactRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- PostContactUs ----

func TestPostContactUs_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/contact-us")
		var body ContactUsRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Subject == "" {
			t.Error("expected non-empty subject")
		}
		writeJSON(w, 200, ContactSubmitResponse{Message: "received", ReferenceID: "ref-2"})
	})
	req := ContactUsRequest{
		Name:    "Bob",
		Email:   "bob@example.com",
		Subject: "Help",
		Message: "I need help",
	}
	res, err := c.PostContactUs(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestPostContactUs_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 422, map[string]any{"detail": "missing fields"})
	})
	_, err := c.PostContactUs(context.Background(), ContactUsRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- GetClientIP ----

func TestGetClientIP_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/geo/ip")
		writeJSON(w, 200, GeoResponse{IP: "1.2.3.4", Country: "US", Timezone: "America/New_York"})
	})
	res, err := c.GetClientIP(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.IP == "" {
		t.Error("expected non-empty IP")
	}
	if res.Country != "US" {
		t.Errorf("expected country US, got %q", res.Country)
	}
}

func TestGetClientIP_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 500, map[string]any{"detail": "geo service unavailable"})
	})
	_, err := c.GetClientIP(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- CreateInvitation ----

func TestCreateInvitation_HappyPath(t *testing.T) {
	email := "member@example.com"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/organizations/org-abc/invitations")
		assertAuth(t, r)
		var body OrgInvitationRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Email != email {
			t.Errorf("expected email %q, got %q", email, body.Email)
		}
		writeJSON(w, 201, map[string]any{
			"data": OrgInvitation{
				ID:        1,
				OrgUUID:   "org-abc",
				Email:     &email,
				Role:      "member",
				ExpiresAt: "2026-07-01T00:00:00Z",
				Token:     "inv-tok-abc",
				SignupURL: "https://example.com/signup?token=inv-tok-abc",
			},
		})
	})
	hrs := int64(48)
	req := OrgInvitationRequest{Email: email, Role: "member", ExpiresInHours: &hrs}
	res, err := c.CreateInvitation(context.Background(), "org-abc", req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Token != "inv-tok-abc" {
		t.Errorf("expected token inv-tok-abc, got %q", res.Token)
	}
	if res.SignupURL == "" {
		t.Error("expected non-empty signup_url")
	}
}

func TestCreateInvitation_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 422, map[string]any{"detail": "invalid email"})
	})
	_, err := c.CreateInvitation(context.Background(), "org-abc", OrgInvitationRequest{Email: "bad"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- ListInvitations ----

func TestListInvitations_HappyPath(t *testing.T) {
	email := "a@example.com"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/organizations/org-abc/invitations")
		assertAuth(t, r)
		writeJSON(w, 200, map[string]any{
			"data": []OrgInvitation{
				{ID: 1, OrgUUID: "org-abc", Email: &email, Role: "member", ExpiresAt: "2026-07-01T00:00:00Z", Token: "t1", SignupURL: "https://x"},
				{ID: 2, OrgUUID: "org-abc", Role: "admin", ExpiresAt: "2026-07-02T00:00:00Z", Token: "t2", SignupURL: "https://y"},
			},
		})
	})
	res, err := c.ListInvitations(context.Background(), "org-abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Errorf("expected 2 invitations, got %d", len(res))
	}
	if res[0].Token != "t1" {
		t.Errorf("expected t1, got %q", res[0].Token)
	}
}

func TestListInvitations_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"detail": "forbidden"})
	})
	_, err := c.ListInvitations(context.Background(), "org-abc")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- RevokeInvitation ----

func TestRevokeInvitation_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodDelete)
		assertPath(t, r, "/api/organizations/org-abc/invitations/42")
		assertAuth(t, r)
		w.WriteHeader(204)
	})
	err := c.RevokeInvitation(context.Background(), "org-abc", "42")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRevokeInvitation_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "invitation not found"})
	})
	err := c.RevokeInvitation(context.Background(), "org-abc", "999")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- PreviewInvitation ----

func TestPreviewInvitation_HappyPath(t *testing.T) {
	email := "member@example.com"
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodGet)
		assertPath(t, r, "/api/auth/invitations/my-token")
		// No auth — this is public
		if r.Header.Get("Authorization") != "" {
			t.Error("expected no Authorization header on public endpoint")
		}
		writeJSON(w, 200, OrgInvitationPreview{
			OrgUUID:   "org-abc",
			OrgName:   "Acme Corp",
			Email:     &email,
			Role:      "member",
			ExpiresAt: "2026-07-01T00:00:00Z",
			Valid:      true,
		})
	})
	res, err := c.PreviewInvitation(context.Background(), "my-token")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Error("expected valid=true")
	}
	if res.OrgName != "Acme Corp" {
		t.Errorf("expected org name 'Acme Corp', got %q", res.OrgName)
	}
}

func TestPreviewInvitation_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"detail": "invitation not found"})
	})
	_, err := c.PreviewInvitation(context.Background(), "bad-token")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- AcceptInvitation ----

func TestAcceptInvitation_HappyPath(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/auth/invitations/my-token/accept")
		assertAuth(t, r)
		writeJSON(w, 200, OrgInvitationAccept{
			OrgUUID: "org-abc",
			OrgName: "Acme Corp",
			Role:    "member",
		})
	})
	res, err := c.AcceptInvitation(context.Background(), "my-token")
	if err != nil {
		t.Fatal(err)
	}
	if res.OrgUUID != "org-abc" {
		t.Errorf("expected org-abc, got %q", res.OrgUUID)
	}
	if res.Role != "member" {
		t.Errorf("expected member, got %q", res.Role)
	}
}

func TestAcceptInvitation_Error(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 410, map[string]any{"detail": "invitation expired"})
	})
	_, err := c.AcceptInvitation(context.Background(), "expired-token")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- do — additional edge case branches ----

// Test the http.NewRequestWithContext failure branch by using an invalid URL.
func TestDo_InvalidURL(t *testing.T) {
	// Use a base URL with a control character which makes http.NewRequest fail.
	c := New("key", WithBaseURL("http://\x7f"))
	_, err := c.GetClientIP(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// Test HTTPClient.Do failure by using a client that always fails network calls.
func TestDo_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close() // close immediately so connections will be refused
	c := New("key", WithBaseURL(addr))
	_, err := c.GetClientIP(context.Background())
	if err == nil {
		t.Fatal("expected network error")
	}
}

// Test io.ReadAll failure by returning a body that errors mid-read.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestDo_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write a valid status but then close connection abruptly via hijack
		w.Header().Set("Content-Length", "1000") // claim large body
		w.WriteHeader(200)
		// Write partial body then the server closes — client ReadAll will fail
		w.(http.Flusher).Flush()
		// Close the underlying connection
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()
	c := New("key", WithBaseURL(srv.URL))
	_, err := c.GetClientIP(context.Background())
	if err == nil {
		t.Fatal("expected read error due to abrupt connection close")
	}
}

// Test json.Marshal failure by passing an un-marshalable body directly via do.
// We can't easily pass a channel via the public API, so we call do indirectly.
// Instead, we test json.Marshal with the auth=false path that has no AccessToken check.
// We craft a scenario using a custom type that embeds a channel (un-marshallable).
// Since do is unexported but in the same package (package buttrbase), we can call it directly.
type badMarshal struct {
	Ch chan int
}

func TestDo_MarshalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c := New("key", WithBaseURL(srv.URL))
	// Call do directly since we're in the same package
	err := c.do(context.Background(), http.MethodPost, "/test", badMarshal{Ch: make(chan int)}, false, nil)
	if err == nil {
		t.Fatal("expected marshal error for channel type")
	}
}

// ---- Webhook signature edge cases ----

func TestVerifyWebhookSignature_EmptyInputs(t *testing.T) {
	// All empty — should return false
	if VerifyWebhookSignature(nil, "", "", "", 0) {
		t.Error("expected false for empty inputs")
	}
	// Empty signature
	if VerifyWebhookSignature([]byte("body"), "", "1234567890", "secret", 0) {
		t.Error("expected false for empty signature")
	}
	// Empty timestamp
	if VerifyWebhookSignature([]byte("body"), "abc", "", "secret", 0) {
		t.Error("expected false for empty timestamp")
	}
	// Empty secret
	if VerifyWebhookSignature([]byte("body"), "abc", "1234567890", "", 0) {
		t.Error("expected false for empty secret")
	}
}

func TestVerifyWebhookSignature_InvalidTimestamp(t *testing.T) {
	if VerifyWebhookSignature([]byte("body"), "sig", "not-a-number", "secret", 0) {
		t.Error("expected false for non-numeric timestamp")
	}
}

func TestVerifyWebhookSignature_InvalidHexSignature(t *testing.T) {
	if VerifyWebhookSignature([]byte("body"), "not-valid-hex!!", "1234567890", "secret", 0) {
		t.Error("expected false for invalid hex signature")
	}
	// Also with sha256= prefix
	if VerifyWebhookSignature([]byte("body"), "sha256=not-valid-hex!!", "1234567890", "secret", 0) {
		t.Error("expected false for invalid hex signature with prefix")
	}
}

func TestVerifyWebhookSignature_FutureTimestamp(t *testing.T) {
	// A timestamp far in the future should fail tolerance check
	// (diff = future - now > tolerance)
	futureTS := "9999999999"
	// With any signature (even wrong), it should fail on timestamp check
	if VerifyWebhookSignature([]byte("body"), "aabbcc", futureTS, "secret", 300) {
		t.Error("expected false for far-future timestamp beyond tolerance")
	}
}

func TestVerifyWebhookSignature_ZeroTolerance(t *testing.T) {
	// With tolerance=0 — skip timing check regardless of age
	// We just need a correctly computed sig (the existing round-trip test covers this).
	// Here just confirm tolerance=0 doesn't panic.
	old := "1000000000"
	if VerifyWebhookSignature([]byte("body"), "aabbcc", old, "secret", 0) {
		t.Log("expected false due to wrong signature, not tolerance")
	}
}
