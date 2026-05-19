// Package buttrbase is a Go SDK for the Buttrbase API.
package buttrbase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.buttrbase.com"

// Client is the Buttrbase API client.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(c *Client) { c.BaseURL = u } }

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.HTTPClient = h } }

// New creates a new Client.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		BaseURL:    defaultBaseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) do(ctx context.Context, method, path string, body any, auth bool, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	u := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if auth && c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := ""
		var parsed map[string]any
		if json.Unmarshal(respBody, &parsed) == nil {
			if d, ok := parsed["detail"].(string); ok {
				detail = d
			}
		}
		return &ButtrbaseError{StatusCode: resp.StatusCode, Detail: detail, Body: respBody}
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("buttrbase: decode response: %w", err)
		}
	}
	return nil
}

// ----- Coupons -----

func (c *Client) ValidateCoupon(ctx context.Context, code string, opts *ValidateCouponOptions) (*CouponValidation, error) {
	body := map[string]any{"code": code}
	if opts != nil {
		if opts.UserID != nil {
			body["user_id"] = *opts.UserID
		}
		if opts.OrderTotalCents != nil {
			body["order_total_cents"] = *opts.OrderTotalCents
		}
	}
	var out CouponValidation
	if err := c.do(ctx, http.MethodPost, "/v1/coupons/validate", body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Gift cards -----

func (c *Client) ValidateGiftCard(ctx context.Context, code string) (*GiftCardValidation, error) {
	body := map[string]any{"code": code}
	var out GiftCardValidation
	if err := c.do(ctx, http.MethodPost, "/v1/giftcards/validate", body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RedeemGiftCard(ctx context.Context, code string, amountCents int64, userID *int) (*GiftCardRedemption, error) {
	body := map[string]any{"code": code, "amount_cents": amountCents}
	if userID != nil {
		body["user_id"] = *userID
	}
	var out GiftCardRedemption
	if err := c.do(ctx, http.MethodPost, "/v1/giftcards/redeem", body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Magic link -----

func (c *Client) SendMagicLink(ctx context.Context, email string, opts *SendMagicLinkOptions) (*MagicLinkSend, error) {
	body := map[string]any{"email": email}
	if opts != nil {
		if opts.RedirectURL != "" {
			body["redirect_url"] = opts.RedirectURL
		}
		if opts.TTLSeconds != nil {
			body["ttl_seconds"] = *opts.TTLSeconds
		}
	}
	var out MagicLinkSend
	if err := c.do(ctx, http.MethodPost, "/v1/magic-link/send", body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) VerifyMagicLink(ctx context.Context, token string) (*MagicLinkVerify, error) {
	body := map[string]any{"token": token}
	var out MagicLinkVerify
	if err := c.do(ctx, http.MethodPost, "/v1/magic-link/verify", body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- MFA -----

func (c *Client) MfaStatus(ctx context.Context) (*MfaStatus, error) {
	var out MfaStatus
	if err := c.do(ctx, http.MethodGet, "/v1/mfa/status", nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) MfaEnroll(ctx context.Context, label string) (*MfaEnrollment, error) {
	body := map[string]any{"label": label}
	var out MfaEnrollment
	if err := c.do(ctx, http.MethodPost, "/v1/mfa/enroll", body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) MfaActivate(ctx context.Context, code string) (*MfaStatusResponse, error) {
	body := map[string]any{"code": code}
	var out MfaStatusResponse
	if err := c.do(ctx, http.MethodPost, "/v1/mfa/activate", body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Org signing -----

func (c *Client) OrgSign(ctx context.Context, orgUUID string, claims map[string]any, ttlSeconds *int64) (*OrgSignResponse, error) {
	body := map[string]any{"claims": claims}
	if ttlSeconds != nil {
		body["ttl_seconds"] = *ttlSeconds
	}
	var out OrgSignResponse
	path := "/v1/orgs/" + url.PathEscape(orgUUID) + "/sign"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) OrgJWKS(ctx context.Context, orgUUID string) (*JWKSResponse, error) {
	var out JWKSResponse
	path := "/v1/orgs/" + url.PathEscape(orgUUID) + "/.well-known/jwks.json"
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Secrets -----

func (c *Client) GetSecret(ctx context.Context, orgUUID, name string) (*SecretGet, error) {
	var out SecretGet
	path := "/v1/orgs/" + url.PathEscape(orgUUID) + "/secrets/" + url.PathEscape(name)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PutSecret(ctx context.Context, orgUUID, name, value, description string) (*SecretSummary, error) {
	body := map[string]any{"value": value}
	if description != "" {
		body["description"] = description
	}
	var out SecretSummary
	path := "/v1/orgs/" + url.PathEscape(orgUUID) + "/secrets/" + url.PathEscape(name)
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ===== Zero-trust endpoints =====

// AuthStepUp exchanges an MFA TOTP (or recovery) code for a short-lived
// elevated access token (~5 min). POST /api/auth/step-up.
//
// On success the client's APIKey is REPLACED with the returned access token
// so subsequent admin / JIT calls carry the elevated session.
func (c *Client) AuthStepUp(ctx context.Context, code string, recovery bool) (*StepUpResponse, error) {
	body := map[string]any{"code": code, "recovery": recovery}
	var out StepUpResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/step-up", body, true, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.APIKey = out.AccessToken
	}
	return &out, nil
}

// ----- JIT elevation (admin) — all require an active step-up session -----

// ElevationRequest opens a JIT elevation grant.
// POST /api/admin/orgs/{org}/elevation/request.
func (c *Client) ElevationRequest(ctx context.Context, orgUUID, scope string, opts *ElevationRequestOptions) (*ElevationGrant, error) {
	body := map[string]any{"scope": scope}
	if opts != nil {
		if opts.Reason != "" {
			body["reason"] = opts.Reason
		}
		if opts.TTLSeconds != nil {
			body["ttl_seconds"] = *opts.TTLSeconds
		}
	}
	var out ElevationGrant
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/elevation/request"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ElevationApprove approves a pending JIT grant.
// POST /api/admin/orgs/{org}/elevation/{grant}/approve.
// The server returns 403 if the approver is the same admin as the requester.
func (c *Client) ElevationApprove(ctx context.Context, orgUUID, grantUUID string) (*ElevationGrant, error) {
	var out ElevationGrant
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/elevation/" + url.PathEscape(grantUUID) + "/approve"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ElevationList lists JIT grants for an org. Pass status="" to list all.
// GET /api/admin/orgs/{org}/elevation.
func (c *Client) ElevationList(ctx context.Context, orgUUID, status string) ([]ElevationGrant, error) {
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/elevation"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	var out []ElevationGrant
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SpiffeIssueSvid issues an X.509 SVID for a workload.
// POST /api/admin/orgs/{org}/spiffe/svid.
func (c *Client) SpiffeIssueSvid(ctx context.Context, orgUUID, workloadPath string, ttlSeconds *int64) (*SpiffeSvidResponse, error) {
	body := map[string]any{"workload_path": workloadPath}
	if ttlSeconds != nil {
		body["ttl_seconds"] = *ttlSeconds
	}
	var out SpiffeSvidResponse
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/spiffe/svid"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAuthEvents fetches the context-aware auth event log.
// GET /api/admin/orgs/{org}/auth-events.
func (c *Client) ListAuthEvents(ctx context.Context, orgUUID string, opts *ListAuthEventsOptions) ([]AuthEvent, error) {
	limit := 50
	userUUID := ""
	if opts != nil {
		if opts.Limit > 0 {
			limit = opts.Limit
		}
		userUUID = opts.UserUUID
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	if userUUID != "" {
		q.Set("user_uuid", userUUID)
	}
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/auth-events?" + q.Encode()
	var out []AuthEvent
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReencryptSecrets rotates the KEK used for org secrets.
// POST /api/admin/orgs/{org}/reencrypt/secrets.
func (c *Client) ReencryptSecrets(ctx context.Context, orgUUID string) (*ReencryptResponse, error) {
	return c.reencrypt(ctx, orgUUID, "secrets")
}

// ReencryptSigningKeys rotates the KEK used for org signing keys.
// POST /api/admin/orgs/{org}/reencrypt/signing-keys.
func (c *Client) ReencryptSigningKeys(ctx context.Context, orgUUID string) (*ReencryptResponse, error) {
	return c.reencrypt(ctx, orgUUID, "signing-keys")
}

// ReencryptMtlsCa rotates the KEK used for the org mTLS CA.
// POST /api/admin/orgs/{org}/reencrypt/mtls-ca.
func (c *Client) ReencryptMtlsCa(ctx context.Context, orgUUID string) (*ReencryptResponse, error) {
	return c.reencrypt(ctx, orgUUID, "mtls-ca")
}

func (c *Client) reencrypt(ctx context.Context, orgUUID, kind string) (*ReencryptResponse, error) {
	var out ReencryptResponse
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/reencrypt/" + kind
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RevokeSession adds a session JTI to the revocation list.
// POST /api/admin/sessions/revoke.
func (c *Client) RevokeSession(ctx context.Context, jti string, ttlSeconds *int64) (*RevokeSessionResponse, error) {
	body := map[string]any{"jti": jti}
	if ttlSeconds != nil {
		body["ttl_seconds"] = *ttlSeconds
	}
	var out RevokeSessionResponse
	if err := c.do(ctx, http.MethodPost, "/api/admin/sessions/revoke", body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetOrgMetrics fetches per-org metrics.
// GET /api/admin/orgs/{org}/metrics.
func (c *Client) GetOrgMetrics(ctx context.Context, orgUUID string) (*OrgMetrics, error) {
	var out OrgMetrics
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/metrics"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Credentials -----

// ListCredentials returns all credentials for the authenticated client.
// GET /credentials
func (c *Client) ListCredentials(ctx context.Context) (*CredentialList, error) {
	var out CredentialList
	if err := c.do(ctx, http.MethodGet, "/credentials", nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateCredential creates a new API credential.
// POST /credentials — returns 201 with client_secret included.
func (c *Client) CreateCredential(ctx context.Context, req CreateCredentialRequest) (*Credential, error) {
	var out Credential
	if err := c.do(ctx, http.MethodPost, "/credentials", req, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCredential fetches a single credential by ID (no client_secret returned).
// GET /credentials/:id
func (c *Client) GetCredential(ctx context.Context, credentialsID string) (*Credential, error) {
	var out Credential
	path := "/credentials/" + url.PathEscape(credentialsID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteCredential deletes a credential by ID.
// DELETE /credentials/:id — returns 204 with no body.
func (c *Client) DeleteCredential(ctx context.Context, credentialsID string) error {
	path := "/credentials/" + url.PathEscape(credentialsID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// RotateCredentialSecret rotates the client_secret for a credential.
// POST /credentials/:id/rotate-secret
func (c *Client) RotateCredentialSecret(ctx context.Context, credentialsID string) (*RotateSecretResponse, error) {
	var out RotateSecretResponse
	path := "/credentials/" + url.PathEscape(credentialsID) + "/rotate-secret"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Sandbox -----

// ResetSandbox resets the sandbox environment.
// POST /api/sandbox/reset — org_uuid is optional.
func (c *Client) ResetSandbox(ctx context.Context, req *SandboxResetRequest) (*SandboxResetResponse, error) {
	var body any
	if req != nil {
		body = req
	}
	var out SandboxResetResponse
	if err := c.do(ctx, http.MethodPost, "/api/sandbox/reset", body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Invite-based registration -----

// InviteAccept accepts an invitation and creates a user account.
// No authentication required — the token in the request acts as the credential.
// POST /api/auth/invite/accept.
func (c *Client) InviteAccept(ctx context.Context, req InviteAcceptRequest) (*InviteAcceptResponse, error) {
	var out InviteAcceptResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/invite/accept", req, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckOrgName reports whether an organisation name is available.
// GET /api/auth/orgs/check.
func (c *Client) CheckOrgName(ctx context.Context, name string) (*OrgCheckResponse, error) {
	var out OrgCheckResponse
	path := "/api/auth/orgs/check?name=" + url.QueryEscape(name)
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSuperuserFlag looks up the superuser flag for a given email address.
// Requires platform-admin authentication.
// GET /api/auth/superuser.
func (c *Client) GetSuperuserFlag(ctx context.Context, email string) (*SuperuserResponse, error) {
	var out SuperuserResponse
	path := "/api/auth/superuser?email=" + url.QueryEscape(email)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Contact forms -----

// PostContact submits an account / sales enquiry form.
// POST /api/contact.
func (c *Client) PostContact(ctx context.Context, req ContactRequest) (*ContactSubmitResponse, error) {
	var out ContactSubmitResponse
	if err := c.do(ctx, http.MethodPost, "/api/contact", req, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PostContactUs submits a general contact-us form.
// POST /api/contact-us.
func (c *Client) PostContactUs(ctx context.Context, req ContactUsRequest) (*ContactSubmitResponse, error) {
	var out ContactSubmitResponse
	if err := c.do(ctx, http.MethodPost, "/api/contact-us", req, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Geo / IP -----

// GetClientIP returns the caller's IP address and basic geo context.
// Useful during registration for timezone / country pre-fill.
// GET /api/geo/ip.
func (c *Client) GetClientIP(ctx context.Context) (*GeoResponse, error) {
	var out GeoResponse
	if err := c.do(ctx, http.MethodGet, "/api/geo/ip", nil, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ensure strconv stays used (helper for callers building queries).
var _ = strconv.Itoa
