package buttrbase

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"
)

func smokeClient(t *testing.T) *Client {
	t.Helper()
	key := os.Getenv("BUTTRBASE_SMOKE_API")
	if key == "" {
		t.Skip("BUTTRBASE_SMOKE_API not set; skipping smoke test")
	}
	opts := []Option{}
	if base := os.Getenv("BUTTRBASE_BASE_URL"); base != "" {
		opts = append(opts, WithBaseURL(base))
	}
	return New(key, opts...)
}

func TestValidateCoupon_Nonexistent(t *testing.T) {
	c := smokeClient(t)
	res, err := c.ValidateCoupon(context.Background(), "NONEXISTENT", nil)
	if err != nil {
		t.Fatalf("ValidateCoupon error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected valid=false for NONEXISTENT, got true")
	}
}

func TestValidateGiftCard_Nonexistent(t *testing.T) {
	c := smokeClient(t)
	res, err := c.ValidateGiftCard(context.Background(), "NONEXISTENT")
	if err != nil {
		t.Fatalf("ValidateGiftCard error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected valid=false for NONEXISTENT, got true")
	}
}

func TestScopeContextRequest_JSON(t *testing.T) {
	b, err := json.Marshal(ScopeContextRequest{RequestedScopes: []string{"orders:read", "orders:write"}})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if got, want := string(b), `{"requested_scopes":["orders:read","orders:write"]}`; got != want {
		t.Fatalf("scope-context request body = %s, want %s", got, want)
	}
}

func TestScopeContextResponse_JSON(t *testing.T) {
	var out ScopeContextResponse
	if err := json.Unmarshal([]byte(`{"token":"jwt.token.here","scopes":["orders:read"]}`), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.Token != "jwt.token.here" {
		t.Fatalf("token = %q, want jwt.token.here", out.Token)
	}
	if len(out.Scopes) != 1 || out.Scopes[0] != "orders:read" {
		t.Fatalf("scopes = %v, want [orders:read]", out.Scopes)
	}
}

func TestDeviceList_JSON(t *testing.T) {
	var out deviceList
	body := `{"data":[{"device_uuid":"dev-1","jkt":"thumb","label":"laptop","created_at":"2026-06-14T00:00:00Z","last_seen_at":null}]}`
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(out.Data) != 1 {
		t.Fatalf("expected 1 device, got %d", len(out.Data))
	}
	d := out.Data[0]
	if d.DeviceUUID != "dev-1" || d.JKT != "thumb" {
		t.Fatalf("unexpected device: %+v", d)
	}
	if d.Label == nil || *d.Label != "laptop" {
		t.Fatalf("label = %v, want laptop", d.Label)
	}
	if d.LastSeenAt != nil {
		t.Fatalf("last_seen_at = %v, want nil", d.LastSeenAt)
	}
}

func TestTenantHome_JSON(t *testing.T) {
	var out tenantHomeEnvelope
	body := `{"data":{"tenancy_mode":"dedicated","home_region":"us-east-1","home_base_url":null}}`
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.Data.TenancyMode != "dedicated" {
		t.Fatalf("tenancy_mode = %q, want dedicated", out.Data.TenancyMode)
	}
	if out.Data.HomeRegion == nil || *out.Data.HomeRegion != "us-east-1" {
		t.Fatalf("home_region = %v, want us-east-1", out.Data.HomeRegion)
	}
	if out.Data.HomeBaseURL != nil {
		t.Fatalf("home_base_url = %v, want nil", out.Data.HomeBaseURL)
	}
}

func TestVerifyWebhookSignature_RoundTrip(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"event":"test","data":{"id":1}}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if !VerifyWebhookSignature(body, sig, ts, secret, 300) {
		t.Fatalf("expected signature to verify")
	}
	if !VerifyWebhookSignature(body, "sha256="+sig, ts, secret, 300) {
		t.Fatalf("expected prefixed signature to verify")
	}
	if VerifyWebhookSignature(body, sig, ts, "wrong-secret", 300) {
		t.Fatalf("expected wrong secret to fail")
	}
	if VerifyWebhookSignature([]byte("tampered"), sig, ts, secret, 300) {
		t.Fatalf("expected tampered body to fail")
	}
	oldTs := strconv.FormatInt(time.Now().Unix()-10000, 10)
	mac2 := hmac.New(sha256.New, []byte(secret))
	mac2.Write([]byte(oldTs))
	mac2.Write([]byte("."))
	mac2.Write(body)
	oldSig := hex.EncodeToString(mac2.Sum(nil))
	if VerifyWebhookSignature(body, oldSig, oldTs, secret, 300) {
		t.Fatalf("expected stale timestamp to fail")
	}
}
