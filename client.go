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

// ===== Auth =====

// Register creates a new account.
// POST /api/auth/register
func (c *Client) Register(ctx context.Context, email, password, orgName string, opts *RegisterOptions) (*LoginResponse, error) {
	body := map[string]any{"email": email, "password": password, "org_name": orgName}
	if opts != nil {
		if opts.FirstName != "" {
			body["first_name"] = opts.FirstName
		}
		if opts.LastName != "" {
			body["last_name"] = opts.LastName
		}
	}
	var out LoginResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/register", body, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.APIKey = out.AccessToken
	}
	return &out, nil
}

// Login authenticates and stores the access token.
// POST /api/auth/login
func (c *Client) Login(ctx context.Context, email, password, orgName string) (*LoginResponse, error) {
	body := map[string]any{"email": email, "password": password, "org_name": orgName}
	var out LoginResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/login", body, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.APIKey = out.AccessToken
	}
	return &out, nil
}

// GetLoginOptions fetches login options for an organization.
// GET /api/auth/organizations/{org_uuid}/login-options
func (c *Client) GetLoginOptions(ctx context.Context, orgUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/auth/organizations/" + url.PathEscape(orgUUID) + "/login-options"
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetStatus returns the current auth status.
// GET /api/auth/status
func (c *Client) GetStatus(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/auth/status", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetProfile returns the authenticated user's profile.
// GET /api/profile
func (c *Client) GetProfile(ctx context.Context) (*Profile, error) {
	var out Profile
	if err := c.do(ctx, http.MethodGet, "/api/profile", nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateProfile updates the authenticated user's profile.
// PUT /api/profile
func (c *Client) UpdateProfile(ctx context.Context, data map[string]any) (*Profile, error) {
	var out Profile
	if err := c.do(ctx, http.MethodPut, "/api/profile", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetOrgByDomain looks up an organization by domain.
// GET /api/auth/orgs-by-domain/{domain}
func (c *Client) GetOrgByDomain(ctx context.Context, domain string) (map[string]any, error) {
	var out map[string]any
	path := "/api/auth/orgs-by-domain/" + url.PathEscape(domain)
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== OTP =====

// OtpSend sends an OTP to a phone number.
// POST /api/auth/otp/send
func (c *Client) OtpSend(ctx context.Context, phone string) (map[string]any, error) {
	body := map[string]any{"phone": phone}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/otp/send", body, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OtpVerify verifies an OTP code.
// POST /api/auth/otp/verify
func (c *Client) OtpVerify(ctx context.Context, phone, code string) (*LoginResponse, error) {
	body := map[string]any{"phone": phone, "code": code}
	var out LoginResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/otp/verify", body, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.APIKey = out.AccessToken
	}
	return &out, nil
}

// ===== MFA (extended) =====

// MfaVerify verifies a TOTP code.
// POST /api/auth/mfa/totp/verify
func (c *Client) MfaVerify(ctx context.Context, code string) (map[string]any, error) {
	body := map[string]any{"code": code}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/mfa/totp/verify", body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MfaChallenge initiates a TOTP challenge.
// POST /api/auth/mfa/totp/challenge
func (c *Client) MfaChallenge(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/mfa/totp/challenge", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MfaDisable disables TOTP MFA.
// DELETE /api/auth/mfa/totp
func (c *Client) MfaDisable(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodDelete, "/api/auth/mfa/totp", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MfaGenerateRecoveryCodes generates new MFA recovery codes.
// POST /api/auth/mfa/recovery-codes
func (c *Client) MfaGenerateRecoveryCodes(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/mfa/recovery-codes", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MfaRedeemRecoveryCode redeems an MFA recovery code.
// POST /api/auth/mfa/recovery-codes/redeem
func (c *Client) MfaRedeemRecoveryCode(ctx context.Context, code string) (map[string]any, error) {
	body := map[string]any{"code": code}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/mfa/recovery-codes/redeem", body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== SSO =====

// OidcAuthorizeURL returns the OIDC authorization URL.
// GET /api/auth/sso/oidc/{connection_uuid}/authorize
func (c *Client) OidcAuthorizeURL(ctx context.Context, connectionUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/auth/sso/oidc/" + url.PathEscape(connectionUUID) + "/authorize"
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SamlAuthorizeURL returns the SAML authorization URL.
// GET /api/auth/sso/saml/{connection_uuid}/authorize
func (c *Client) SamlAuthorizeURL(ctx context.Context, connectionUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/auth/sso/saml/" + url.PathEscape(connectionUUID) + "/authorize"
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OidcCallback handles the OIDC callback.
// POST /api/auth/sso/oidc/{connection_uuid}/callback
func (c *Client) OidcCallback(ctx context.Context, connectionUUID string, params map[string]any) (*LoginResponse, error) {
	var out LoginResponse
	path := "/api/auth/sso/oidc/" + url.PathEscape(connectionUUID) + "/callback"
	if err := c.do(ctx, http.MethodPost, path, params, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.APIKey = out.AccessToken
	}
	return &out, nil
}

// SamlCallback handles the SAML callback.
// POST /api/auth/sso/saml/{connection_uuid}/callback
func (c *Client) SamlCallback(ctx context.Context, connectionUUID string, params map[string]any) (*LoginResponse, error) {
	var out LoginResponse
	path := "/api/auth/sso/saml/" + url.PathEscape(connectionUUID) + "/callback"
	if err := c.do(ctx, http.MethodPost, path, params, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.APIKey = out.AccessToken
	}
	return &out, nil
}

// ===== Users =====

// ListUsers lists users with optional filters.
// GET /api/users
func (c *Client) ListUsers(ctx context.Context, filters map[string]string) ([]map[string]any, error) {
	path := "/api/users"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetUser fetches a single user.
// GET /api/users/{user_uuid}
func (c *Client) GetUser(ctx context.Context, userUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/users/" + url.PathEscape(userUUID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetUserLevel fetches a user's level.
// GET /api/users/{user_uuid}/level
func (c *Client) GetUserLevel(ctx context.Context, userUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/users/" + url.PathEscape(userUUID) + "/level"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetUserLevel sets a user's level.
// POST /api/users/{user_uuid}/level
func (c *Client) SetUserLevel(ctx context.Context, userUUID, userType string) (map[string]any, error) {
	body := map[string]any{"user_type": userType}
	var out map[string]any
	path := "/api/users/" + url.PathEscape(userUUID) + "/level"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateUserStatus updates a user's active status.
// PUT /api/users/{user_uuid}/status
func (c *Client) UpdateUserStatus(ctx context.Context, userUUID string, active bool) (map[string]any, error) {
	body := map[string]any{"active": active}
	var out map[string]any
	path := "/api/users/" + url.PathEscape(userUUID) + "/status"
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateUserRole updates a user's role.
// PUT /api/users/{user_uuid}/role
func (c *Client) UpdateUserRole(ctx context.Context, userUUID, role string) (map[string]any, error) {
	body := map[string]any{"role": role}
	var out map[string]any
	path := "/api/users/" + url.PathEscape(userUUID) + "/role"
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteUser deletes a user.
// DELETE /api/users/{user_uuid}
func (c *Client) DeleteUser(ctx context.Context, userUUID string) error {
	path := "/api/users/" + url.PathEscape(userUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Org Security =====

// GetOrgSecuritySettings fetches org security settings.
// GET /api/orgs/{org_uuid}/security
func (c *Client) GetOrgSecuritySettings(ctx context.Context, orgUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/security"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateOrgSecuritySettings updates org security settings.
// PUT /api/orgs/{org_uuid}/security
func (c *Client) UpdateOrgSecuritySettings(ctx context.Context, orgUUID string, settings map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/security"
	if err := c.do(ctx, http.MethodPut, path, settings, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListSsoConnections lists SSO connections for an org.
// GET /api/orgs/{org_uuid}/sso/connections
func (c *Client) ListSsoConnections(ctx context.Context, orgUUID string) ([]SsoConnection, error) {
	var out []SsoConnection
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/sso/connections"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateSsoConnection creates an SSO connection.
// POST /api/orgs/{org_uuid}/sso/connections
func (c *Client) CreateSsoConnection(ctx context.Context, orgUUID string, data map[string]any) (*SsoConnection, error) {
	var out SsoConnection
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/sso/connections"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSsoConnection fetches a single SSO connection.
// GET /api/orgs/{org_uuid}/sso/connections/{connection_uuid}
func (c *Client) GetSsoConnection(ctx context.Context, orgUUID, connectionUUID string) (*SsoConnection, error) {
	var out SsoConnection
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/sso/connections/" + url.PathEscape(connectionUUID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateSsoConnection updates an SSO connection.
// PUT /api/orgs/{org_uuid}/sso/connections/{connection_uuid}
func (c *Client) UpdateSsoConnection(ctx context.Context, orgUUID, connectionUUID string, data map[string]any) (*SsoConnection, error) {
	var out SsoConnection
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/sso/connections/" + url.PathEscape(connectionUUID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSsoConnection deletes an SSO connection.
// DELETE /api/orgs/{org_uuid}/sso/connections/{connection_uuid}
func (c *Client) DeleteSsoConnection(ctx context.Context, orgUUID, connectionUUID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/sso/connections/" + url.PathEscape(connectionUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ListAuditEvents lists audit events for an org.
// GET /api/orgs/{org_uuid}/audit-events
func (c *Client) ListAuditEvents(ctx context.Context, orgUUID string, filters map[string]string) ([]AuditEventEntry, error) {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/audit-events"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out []AuditEventEntry
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAuditEvent fetches a single audit event.
// GET /api/orgs/{org_uuid}/audit-events/{event_id}
func (c *Client) GetAuditEvent(ctx context.Context, orgUUID string, eventID int64) (*AuditEventEntry, error) {
	var out AuditEventEntry
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/audit-events/" + strconv.FormatInt(eventID, 10)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ===== Branding =====

// GetBranding fetches org branding settings.
// GET /api/orgs/{org_uuid}/branding
func (c *Client) GetBranding(ctx context.Context, orgUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/branding"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateBranding updates org branding settings.
// PUT /api/orgs/{org_uuid}/branding
func (c *Client) UpdateBranding(ctx context.Context, orgUUID string, branding map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/branding"
	if err := c.do(ctx, http.MethodPut, path, branding, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Sessions / Device Accounts =====

// ListDeviceAccounts lists device accounts.
// GET /api/device-accounts
func (c *Client) ListDeviceAccounts(ctx context.Context, deviceUUID string) ([]UserAccount, error) {
	path := "/api/device-accounts"
	if deviceUUID != "" {
		path += "?device_uuid=" + url.QueryEscape(deviceUUID)
	}
	var out []UserAccount
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateDeviceAccount creates a device account.
// POST /api/device-accounts
func (c *Client) CreateDeviceAccount(ctx context.Context, data map[string]any) (*UserAccount, error) {
	var out UserAccount
	if err := c.do(ctx, http.MethodPost, "/api/device-accounts", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDeviceAccount fetches a device account.
// GET /api/device-accounts/{account_uuid}
func (c *Client) GetDeviceAccount(ctx context.Context, accountUUID string) (*UserAccount, error) {
	var out UserAccount
	path := "/api/device-accounts/" + url.PathEscape(accountUUID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteDeviceAccount deletes a device account.
// DELETE /api/device-accounts/{account_uuid}
func (c *Client) DeleteDeviceAccount(ctx context.Context, accountUUID string) error {
	path := "/api/device-accounts/" + url.PathEscape(accountUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ListSessions lists active sessions.
// GET /api/sessions
func (c *Client) ListSessions(ctx context.Context) ([]SessionInfo, error) {
	var out []SessionInfo
	if err := c.do(ctx, http.MethodGet, "/api/sessions", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetSession fetches a single session.
// GET /api/sessions/{session_id}
func (c *Client) GetSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	var out SessionInfo
	path := "/api/sessions/" + url.PathEscape(sessionID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RevokeSessionByID revokes a session by session ID.
// DELETE /api/sessions/{session_id}
func (c *Client) RevokeSessionByID(ctx context.Context, sessionID string) error {
	path := "/api/sessions/" + url.PathEscape(sessionID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// RevokeAllSessions revokes all sessions.
// POST /api/sessions/revoke-all
func (c *Client) RevokeAllSessions(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/sessions/revoke-all", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== API Keys v2 =====

// ListAPIKeysV2 lists API keys (v2).
// GET /api/v2/api-keys
func (c *Client) ListAPIKeysV2(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/v2/api-keys", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateAPIKeyV2 creates an API key (v2).
// POST /api/v2/api-keys
func (c *Client) CreateAPIKeyV2(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/v2/api-keys", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteAPIKeyV2 deletes an API key (v2).
// DELETE /api/v2/api-keys/{key_id}
func (c *Client) DeleteAPIKeyV2(ctx context.Context, keyID string) error {
	path := "/api/v2/api-keys/" + url.PathEscape(keyID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Service Identities =====

// ListServiceIdentities lists service identities.
// GET /api/orgs/{org_uuid}/service-identities
func (c *Client) ListServiceIdentities(ctx context.Context, orgUUID string) ([]map[string]any, error) {
	var out []map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateServiceIdentity creates a service identity.
// POST /api/orgs/{org_uuid}/service-identities
func (c *Client) CreateServiceIdentity(ctx context.Context, orgUUID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetServiceIdentity fetches a single service identity.
// GET /api/orgs/{org_uuid}/service-identities/{identity_id}
func (c *Client) GetServiceIdentity(ctx context.Context, orgUUID, identityID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities/" + url.PathEscape(identityID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateServiceIdentity updates a service identity.
// PUT /api/orgs/{org_uuid}/service-identities/{identity_id}
func (c *Client) UpdateServiceIdentity(ctx context.Context, orgUUID, identityID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities/" + url.PathEscape(identityID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteServiceIdentity deletes a service identity.
// DELETE /api/orgs/{org_uuid}/service-identities/{identity_id}
func (c *Client) DeleteServiceIdentity(ctx context.Context, orgUUID, identityID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities/" + url.PathEscape(identityID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// CreateServiceIdentityToken creates an automation token for a service identity.
// POST /api/orgs/{org_uuid}/service-identities/{identity_id}/token
func (c *Client) CreateServiceIdentityToken(ctx context.Context, orgUUID, identityID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities/" + url.PathEscape(identityID) + "/token"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Entitlements =====

// CheckEntitlement checks a single entitlement.
// POST /api/entitlements/check
func (c *Client) CheckEntitlement(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/entitlements/check", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// BatchCheckEntitlements checks multiple entitlements.
// POST /api/entitlements/batch-check
func (c *Client) BatchCheckEntitlements(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/entitlements/batch-check", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetEffectiveEntitlements returns effective entitlements for a user/org.
// GET /api/entitlements/effective
func (c *Client) GetEffectiveEntitlements(ctx context.Context, filters map[string]string) (map[string]any, error) {
	path := "/api/entitlements/effective"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AdminExplainEntitlement explains an entitlement decision.
// POST /api/admin/entitlements/explain
func (c *Client) AdminExplainEntitlement(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/admin/entitlements/explain", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Pricing =====

// PricingPreview previews pricing for a product.
// POST /api/pricing/preview
func (c *Client) PricingPreview(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/pricing/preview", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PricingQuote gets a pricing quote.
// POST /api/pricing/quote
func (c *Client) PricingQuote(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/pricing/quote", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PricingCheckoutSession creates a pricing checkout session.
// POST /api/pricing/checkout-session
func (c *Client) PricingCheckoutSession(ctx context.Context, data map[string]any) (*PaymentCheckoutSession, error) {
	var out PaymentCheckoutSession
	if err := c.do(ctx, http.MethodPost, "/api/pricing/checkout-session", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AdminExplainPricing explains a pricing decision.
// POST /api/admin/pricing/explain
func (c *Client) AdminExplainPricing(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/admin/pricing/explain", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PricingCatalogPreview previews the pricing catalog.
// GET /api/pricing/catalog
func (c *Client) PricingCatalogPreview(ctx context.Context, filters map[string]string) (map[string]any, error) {
	path := "/api/pricing/catalog"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Coupons Admin =====

// ListCoupons lists coupons for a product.
// GET /api/admin/products/{product_id}/coupons
func (c *Client) ListCoupons(ctx context.Context, productID int) ([]Coupon, error) {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons"
	var out []Coupon
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateCoupon creates a coupon for a product.
// POST /api/admin/products/{product_id}/coupons
func (c *Client) CreateCoupon(ctx context.Context, productID int, data map[string]any) (*Coupon, error) {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons"
	var out Coupon
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCoupon fetches a single coupon.
// GET /api/admin/products/{product_id}/coupons/{coupon_id}
func (c *Client) GetCoupon(ctx context.Context, productID, couponID int) (*Coupon, error) {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID)
	var out Coupon
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateCoupon updates a coupon.
// PUT /api/admin/products/{product_id}/coupons/{coupon_id}
func (c *Client) UpdateCoupon(ctx context.Context, productID, couponID int, data map[string]any) (*Coupon, error) {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID)
	var out Coupon
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteCoupon deletes a coupon.
// DELETE /api/admin/products/{product_id}/coupons/{coupon_id}
func (c *Client) DeleteCoupon(ctx context.Context, productID, couponID int) error {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Labels =====

// SetCouponLabels sets labels on a coupon.
// PUT /api/admin/products/{product_id}/coupons/{coupon_id}/labels
func (c *Client) SetCouponLabels(ctx context.Context, productID, couponID int, labels []string) (map[string]any, error) {
	body := map[string]any{"labels": labels}
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID) + "/labels"
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddCouponLabel adds a label to a coupon.
// POST /api/admin/products/{product_id}/coupons/{coupon_id}/labels
func (c *Client) AddCouponLabel(ctx context.Context, productID, couponID int, label string) (map[string]any, error) {
	body := map[string]any{"label": label}
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID) + "/labels"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveCouponLabel removes a label from a coupon.
// DELETE /api/admin/products/{product_id}/coupons/{coupon_id}/labels/{label}
func (c *Client) RemoveCouponLabel(ctx context.Context, productID, couponID int, label string) error {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID) + "/labels/" + url.PathEscape(label)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// SetProductTags sets tags on a product.
// PUT /api/admin/products/{product_id}/tags
func (c *Client) SetProductTags(ctx context.Context, productID int, tags []string) (map[string]any, error) {
	body := map[string]any{"tags": tags}
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/tags"
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddProductTag adds a tag to a product.
// POST /api/admin/products/{product_id}/tags
func (c *Client) AddProductTag(ctx context.Context, productID int, tag string) (map[string]any, error) {
	body := map[string]any{"tag": tag}
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/tags"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveProductTag removes a tag from a product.
// DELETE /api/admin/products/{product_id}/tags/{tag}
func (c *Client) RemoveProductTag(ctx context.Context, productID int, tag string) error {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/tags/" + url.PathEscape(tag)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Analytics =====

// IngestAnalyticsEvent ingests an analytics event.
// POST /api/analytics/events
func (c *Client) IngestAnalyticsEvent(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/analytics/events", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAppOverview returns the app analytics overview.
// GET /api/analytics/app-overview
func (c *Client) GetAppOverview(ctx context.Context, filters map[string]string) (map[string]any, error) {
	path := "/api/analytics/app-overview"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetOrgOverview returns the org analytics overview.
// GET /api/analytics/org-overview
func (c *Client) GetOrgOverview(ctx context.Context, orgUUID string, filters map[string]string) (map[string]any, error) {
	q := url.Values{}
	q.Set("org_uuid", orgUUID)
	if len(filters) > 0 {
		for k, v := range filters {
			q.Set(k, v)
		}
	}
	path := "/api/analytics/org-overview?" + q.Encode()
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Teams =====

// CreateTeam creates a team.
// POST /api/orgs/{org_uuid}/teams
func (c *Client) CreateTeam(ctx context.Context, orgUUID string, data map[string]any) (*Team, error) {
	var out Team
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListOrgTeams lists teams for an org.
// GET /api/orgs/{org_uuid}/teams
func (c *Client) ListOrgTeams(ctx context.Context, orgUUID string) ([]Team, error) {
	var out []Team
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTeam fetches a single team.
// GET /api/orgs/{org_uuid}/teams/{team_uuid}
func (c *Client) GetTeam(ctx context.Context, orgUUID, teamUUID string) (*Team, error) {
	var out Team
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTeam updates a team.
// PUT /api/orgs/{org_uuid}/teams/{team_uuid}
func (c *Client) UpdateTeam(ctx context.Context, orgUUID, teamUUID string, data map[string]any) (*Team, error) {
	var out Team
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeactivateTeam deactivates a team.
// POST /api/orgs/{org_uuid}/teams/{team_uuid}/deactivate
func (c *Client) DeactivateTeam(ctx context.Context, orgUUID, teamUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID) + "/deactivate"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReactivateTeam reactivates a team.
// POST /api/orgs/{org_uuid}/teams/{team_uuid}/reactivate
func (c *Client) ReactivateTeam(ctx context.Context, orgUUID, teamUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID) + "/reactivate"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ArchiveTeam archives a team.
// POST /api/orgs/{org_uuid}/teams/{team_uuid}/archive
func (c *Client) ArchiveTeam(ctx context.Context, orgUUID, teamUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID) + "/archive"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListInactiveTeams lists inactive teams.
// GET /api/orgs/{org_uuid}/teams/inactive
func (c *Client) ListInactiveTeams(ctx context.Context, orgUUID string) ([]Team, error) {
	var out []Team
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/inactive"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTeamMembers lists members of a team.
// GET /api/orgs/{org_uuid}/teams/{team_uuid}/members
func (c *Client) ListTeamMembers(ctx context.Context, orgUUID, teamUUID string) ([]map[string]any, error) {
	var out []map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID) + "/members"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddTeamMember adds a member to a team.
// POST /api/orgs/{org_uuid}/teams/{team_uuid}/members
func (c *Client) AddTeamMember(ctx context.Context, orgUUID, teamUUID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID) + "/members"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveTeamMember removes a member from a team.
// DELETE /api/orgs/{org_uuid}/teams/{team_uuid}/members/{user_uuid}
func (c *Client) RemoveTeamMember(ctx context.Context, orgUUID, teamUUID, userUUID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID) + "/members/" + url.PathEscape(userUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ListTeamObservers lists observers of a team.
// GET /api/orgs/{org_uuid}/teams/{team_uuid}/observers
func (c *Client) ListTeamObservers(ctx context.Context, orgUUID, teamUUID string) ([]map[string]any, error) {
	var out []map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID) + "/observers"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddTeamObserver adds an observer to a team.
// POST /api/orgs/{org_uuid}/teams/{team_uuid}/observers
func (c *Client) AddTeamObserver(ctx context.Context, orgUUID, teamUUID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID) + "/observers"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveTeamObserver removes an observer from a team.
// DELETE /api/orgs/{org_uuid}/teams/{team_uuid}/observers/{user_uuid}
func (c *Client) RemoveTeamObserver(ctx context.Context, orgUUID, teamUUID, userUUID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/teams/" + url.PathEscape(teamUUID) + "/observers/" + url.PathEscape(userUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ListUserTeams lists teams for a user.
// GET /api/users/{user_uuid}/teams
func (c *Client) ListUserTeams(ctx context.Context, userUUID string) ([]Team, error) {
	var out []Team
	path := "/api/users/" + url.PathEscape(userUUID) + "/teams"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListUserObservedTeams lists teams a user observes.
// GET /api/users/{user_uuid}/observed-teams
func (c *Client) ListUserObservedTeams(ctx context.Context, userUUID string) ([]Team, error) {
	var out []Team
	path := "/api/users/" + url.PathEscape(userUUID) + "/observed-teams"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Org Features =====

// ListOrgFeatures lists org feature flags.
// GET /api/orgs/{org_uuid}/features
func (c *Client) ListOrgFeatures(ctx context.Context, orgUUID string) ([]OrgFeature, error) {
	var out []OrgFeature
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/features"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetOrgFeature sets an org feature flag.
// PUT /api/orgs/{org_uuid}/features/{feature_id}
func (c *Client) SetOrgFeature(ctx context.Context, orgUUID, featureID string, enabled bool) (*OrgFeature, error) {
	body := map[string]any{"enabled": enabled}
	var out OrgFeature
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/features/" + url.PathEscape(featureID)
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveOrgFeature removes an org feature flag.
// DELETE /api/orgs/{org_uuid}/features/{feature_id}
func (c *Client) RemoveOrgFeature(ctx context.Context, orgUUID, featureID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/features/" + url.PathEscape(featureID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Roles =====

// ListRoles lists roles.
// GET /api/roles
func (c *Client) ListRoles(ctx context.Context) ([]Role, error) {
	var out []Role
	if err := c.do(ctx, http.MethodGet, "/api/roles", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListAllPermissions lists all permissions.
// GET /api/permissions
func (c *Client) ListAllPermissions(ctx context.Context) ([]Permission, error) {
	var out []Permission
	if err := c.do(ctx, http.MethodGet, "/api/permissions", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetRolePermissions gets permissions for a role.
// GET /api/roles/{role_id}/permissions
func (c *Client) GetRolePermissions(ctx context.Context, roleID int) ([]Permission, error) {
	var out []Permission
	path := "/api/roles/" + strconv.Itoa(roleID) + "/permissions"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetRolePermissions sets permissions for a role.
// PUT /api/roles/{role_id}/permissions
func (c *Client) SetRolePermissions(ctx context.Context, roleID int, permissionIDs []int) (map[string]any, error) {
	body := map[string]any{"permission_ids": permissionIDs}
	var out map[string]any
	path := "/api/roles/" + strconv.Itoa(roleID) + "/permissions"
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== RBAC =====

// ListProductPermissions lists permissions for a product.
// GET /api/admin/products/{product_id}/permissions
func (c *Client) ListProductPermissions(ctx context.Context, productID int) ([]Permission, error) {
	var out []Permission
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/permissions"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateProductRole creates a role for a product.
// POST /api/admin/products/{product_id}/roles
func (c *Client) CreateProductRole(ctx context.Context, productID int, data map[string]any) (*Role, error) {
	var out Role
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/roles"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAssignableRoles lists assignable roles.
// GET /api/admin/assignable-roles
func (c *Client) ListAssignableRoles(ctx context.Context) ([]Role, error) {
	var out []Role
	if err := c.do(ctx, http.MethodGet, "/api/admin/assignable-roles", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AssignRoleToUser assigns a role to a user.
// POST /api/admin/users/{user_uuid}/roles
func (c *Client) AssignRoleToUser(ctx context.Context, userUUID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/users/" + url.PathEscape(userUUID) + "/roles"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveRoleFromUser removes a role from a user.
// DELETE /api/admin/users/{user_uuid}/roles/{role_id}
func (c *Client) RemoveRoleFromUser(ctx context.Context, userUUID string, roleID int) error {
	path := "/api/admin/users/" + url.PathEscape(userUUID) + "/roles/" + strconv.Itoa(roleID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Billing =====

// BillingCheckout creates a billing checkout session.
// POST /api/billing/checkout
func (c *Client) BillingCheckout(ctx context.Context, data map[string]any) (*PaymentCheckoutSession, error) {
	var out PaymentCheckoutSession
	if err := c.do(ctx, http.MethodPost, "/api/billing/checkout", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBillingHistory returns billing history.
// GET /api/billing/history
func (c *Client) GetBillingHistory(ctx context.Context, filters map[string]string) ([]Invoice, error) {
	path := "/api/billing/history"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out []Invoice
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListInvoices lists invoices.
// GET /api/billing/invoices
func (c *Client) ListInvoices(ctx context.Context, filters map[string]string) ([]Invoice, error) {
	path := "/api/billing/invoices"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out []Invoice
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetInvoice fetches a single invoice.
// GET /api/billing/invoices/{invoice_id}
func (c *Client) GetInvoice(ctx context.Context, invoiceID int) (*Invoice, error) {
	var out Invoice
	path := "/api/billing/invoices/" + strconv.Itoa(invoiceID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendInvoice sends an invoice.
// POST /api/billing/invoices/send
func (c *Client) SendInvoice(ctx context.Context, data map[string]any) (*SendInvoiceResponse, error) {
	var out SendInvoiceResponse
	if err := c.do(ctx, http.MethodPost, "/api/billing/invoices/send", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBillingProviderConfig returns billing provider configuration.
// GET /api/billing/provider-config
func (c *Client) GetBillingProviderConfig(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/billing/provider-config", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateBillingProviderConfig updates billing provider configuration.
// PUT /api/billing/provider-config
func (c *Client) UpdateBillingProviderConfig(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPut, "/api/billing/provider-config", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateBillingAddOn creates a billing add-on.
// POST /api/billing/add-ons
func (c *Client) CreateBillingAddOn(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/billing/add-ons", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListBillingAddOns lists billing add-ons.
// GET /api/billing/add-ons
func (c *Client) ListBillingAddOns(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/billing/add-ons", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveBillingAddOn removes a billing add-on.
// DELETE /api/billing/add-ons/{add_on_id}
func (c *Client) RemoveBillingAddOn(ctx context.Context, addOnID string) error {
	path := "/api/billing/add-ons/" + url.PathEscape(addOnID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// GetWalletBalance returns the wallet balance.
// GET /api/billing/wallet
func (c *Client) GetWalletBalance(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/billing/wallet", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreditWallet credits the wallet.
// POST /api/billing/wallet/credit
func (c *Client) CreditWallet(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/billing/wallet/credit", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DebitWallet debits the wallet.
// POST /api/billing/wallet/debit
func (c *Client) DebitWallet(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/billing/wallet/debit", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Environments =====

// ListEnvironments lists environments.
// GET /api/environments
func (c *Client) ListEnvironments(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/environments", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateEnvironment creates an environment.
// POST /api/environments
func (c *Client) CreateEnvironment(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/environments", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetEnvironment fetches a single environment.
// GET /api/environments/{env_id}
func (c *Client) GetEnvironment(ctx context.Context, envID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/environments/" + url.PathEscape(envID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateEnvironment updates an environment.
// PUT /api/environments/{env_id}
func (c *Client) UpdateEnvironment(ctx context.Context, envID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/environments/" + url.PathEscape(envID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteEnvironment deletes an environment.
// DELETE /api/environments/{env_id}
func (c *Client) DeleteEnvironment(ctx context.Context, envID string) error {
	path := "/api/environments/" + url.PathEscape(envID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Plaid =====

// PlaidCreateLinkToken creates a Plaid link token.
// POST /api/plaid/link-token
func (c *Client) PlaidCreateLinkToken(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/plaid/link-token", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PlaidExchangePublicToken exchanges a Plaid public token.
// POST /api/plaid/exchange-token
func (c *Client) PlaidExchangePublicToken(ctx context.Context, publicToken string) (map[string]any, error) {
	body := map[string]any{"public_token": publicToken}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/plaid/exchange-token", body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PlaidGetAccounts returns Plaid accounts.
// GET /api/plaid/accounts
func (c *Client) PlaidGetAccounts(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/plaid/accounts", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Usage =====

// GetUsageSummary returns the usage summary.
// GET /api/usage/summary
func (c *Client) GetUsageSummary(ctx context.Context, filters map[string]string) (map[string]any, error) {
	path := "/api/usage/summary"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReportUsage reports usage.
// POST /api/usage/report
func (c *Client) ReportUsage(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/usage/report", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetUsageDetails returns detailed usage.
// GET /api/usage/details
func (c *Client) GetUsageDetails(ctx context.Context, filters map[string]string) (map[string]any, error) {
	path := "/api/usage/details"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Help / Search =====

// SearchHelp searches help articles.
// GET /api/help/search
func (c *Client) SearchHelp(ctx context.Context, query string) ([]map[string]any, error) {
	path := "/api/help/search?q=" + url.QueryEscape(query)
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetHelpArticle fetches a help article.
// GET /api/help/articles/{article_id}
func (c *Client) GetHelpArticle(ctx context.Context, articleID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/help/articles/" + url.PathEscape(articleID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GlobalSearch performs a global search.
// GET /api/search
func (c *Client) GlobalSearch(ctx context.Context, query string, filters map[string]string) (map[string]any, error) {
	q := url.Values{}
	q.Set("q", query)
	for k, v := range filters {
		q.Set(k, v)
	}
	path := "/api/search?" + q.Encode()
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== AI Gateway =====

// AIGatewayChat sends a chat completion request through the AI gateway.
// POST /api/ai/chat/completions
// Uses direct http.NewRequest with custom headers.
func (c *Client) AIGatewayChat(ctx context.Context, body map[string]any, headers map[string]string) (map[string]any, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	u := c.BaseURL + "/api/ai/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := ""
		var parsed map[string]any
		if json.Unmarshal(respBody, &parsed) == nil {
			if d, ok := parsed["detail"].(string); ok {
				detail = d
			}
		}
		return nil, &ButtrbaseError{StatusCode: resp.StatusCode, Detail: detail, Body: respBody}
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("buttrbase: decode response: %w", err)
	}
	return out, nil
}

// AIGatewayEmbed sends an embeddings request through the AI gateway.
// POST /api/ai/embeddings
func (c *Client) AIGatewayEmbed(ctx context.Context, body map[string]any, headers map[string]string) (map[string]any, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	u := c.BaseURL + "/api/ai/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := ""
		var parsed map[string]any
		if json.Unmarshal(respBody, &parsed) == nil {
			if d, ok := parsed["detail"].(string); ok {
				detail = d
			}
		}
		return nil, &ButtrbaseError{StatusCode: resp.StatusCode, Detail: detail, Body: respBody}
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("buttrbase: decode response: %w", err)
	}
	return out, nil
}

// AIGatewayListModels lists available AI models.
// GET /api/ai/models
func (c *Client) AIGatewayListModels(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/ai/models", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Signing Keys =====

// ListSigningKeys lists signing keys for an org.
// GET /api/orgs/{org_uuid}/signing-keys
func (c *Client) ListSigningKeys(ctx context.Context, orgUUID string) ([]SigningKey, error) {
	var out []SigningKey
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateSigningKey creates a signing key.
// POST /api/orgs/{org_uuid}/signing-keys
func (c *Client) CreateSigningKey(ctx context.Context, orgUUID string, data map[string]any) (*SigningKey, error) {
	var out SigningKey
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSigningKey fetches a single signing key.
// GET /api/orgs/{org_uuid}/signing-keys/{key_id}
func (c *Client) GetSigningKey(ctx context.Context, orgUUID, keyID string) (*SigningKey, error) {
	var out SigningKey
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys/" + url.PathEscape(keyID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RevokeSigningKey revokes a signing key.
// POST /api/orgs/{org_uuid}/signing-keys/{key_id}/revoke
func (c *Client) RevokeSigningKey(ctx context.Context, orgUUID, keyID string) (*SigningKey, error) {
	var out SigningKey
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys/" + url.PathEscape(keyID) + "/revoke"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSigningKey deletes a signing key.
// DELETE /api/orgs/{org_uuid}/signing-keys/{key_id}
func (c *Client) DeleteSigningKey(ctx context.Context, orgUUID, keyID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys/" + url.PathEscape(keyID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ListSigningAudit lists signing audit entries.
// GET /api/orgs/{org_uuid}/signing-keys/audit
func (c *Client) ListSigningAudit(ctx context.Context, orgUUID string, filters map[string]string) ([]SigningAuditEntryItem, error) {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys/audit"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out []SigningAuditEntryItem
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== mTLS CA =====

// GetMtlsCa fetches the mTLS CA for an org.
// GET /api/orgs/{org_uuid}/mtls-ca
func (c *Client) GetMtlsCa(ctx context.Context, orgUUID string) (*CertificateAuthority, error) {
	var out CertificateAuthority
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls-ca"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateMtlsCa creates an mTLS CA for an org.
// POST /api/orgs/{org_uuid}/mtls-ca
func (c *Client) CreateMtlsCa(ctx context.Context, orgUUID string, data map[string]any) (*CertificateAuthority, error) {
	var out CertificateAuthority
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls-ca"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteMtlsCa deletes the mTLS CA for an org.
// DELETE /api/orgs/{org_uuid}/mtls-ca
func (c *Client) DeleteMtlsCa(ctx context.Context, orgUUID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls-ca"
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// IssueMtlsCert issues an mTLS certificate.
// POST /api/orgs/{org_uuid}/mtls-ca/certificates
func (c *Client) IssueMtlsCert(ctx context.Context, orgUUID string, data map[string]any) (*Certificate, error) {
	var out Certificate
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls-ca/certificates"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListMtlsCerts lists mTLS certificates.
// GET /api/orgs/{org_uuid}/mtls-ca/certificates
func (c *Client) ListMtlsCerts(ctx context.Context, orgUUID string) ([]Certificate, error) {
	var out []Certificate
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls-ca/certificates"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeMtlsCert revokes an mTLS certificate.
// POST /api/orgs/{org_uuid}/mtls-ca/certificates/{serial}/revoke
func (c *Client) RevokeMtlsCert(ctx context.Context, orgUUID, serial string) (*Certificate, error) {
	var out Certificate
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls-ca/certificates/" + url.PathEscape(serial) + "/revoke"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ===== Zero Trust (extended) =====

// ListTrustPolicies lists zero-trust policies.
// GET /api/admin/orgs/{org_uuid}/trust-policies
func (c *Client) ListTrustPolicies(ctx context.Context, orgUUID string) ([]map[string]any, error) {
	var out []map[string]any
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/trust-policies"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateTrustPolicy creates a zero-trust policy.
// POST /api/admin/orgs/{org_uuid}/trust-policies
func (c *Client) CreateTrustPolicy(ctx context.Context, orgUUID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/trust-policies"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTrustPolicy fetches a single trust policy.
// GET /api/admin/orgs/{org_uuid}/trust-policies/{policy_id}
func (c *Client) GetTrustPolicy(ctx context.Context, orgUUID, policyID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/trust-policies/" + url.PathEscape(policyID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateTrustPolicy updates a trust policy.
// PUT /api/admin/orgs/{org_uuid}/trust-policies/{policy_id}
func (c *Client) UpdateTrustPolicy(ctx context.Context, orgUUID, policyID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/trust-policies/" + url.PathEscape(policyID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteTrustPolicy deletes a trust policy.
// DELETE /api/admin/orgs/{org_uuid}/trust-policies/{policy_id}
func (c *Client) DeleteTrustPolicy(ctx context.Context, orgUUID, policyID string) error {
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/trust-policies/" + url.PathEscape(policyID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Secrets (extended) =====

// ListSecrets lists secrets for an org.
// GET /v1/orgs/{org_uuid}/secrets
func (c *Client) ListSecrets(ctx context.Context, orgUUID string) ([]SecretSummary, error) {
	var out []SecretSummary
	path := "/v1/orgs/" + url.PathEscape(orgUUID) + "/secrets"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteSecret deletes a secret.
// DELETE /v1/orgs/{org_uuid}/secrets/{name}
func (c *Client) DeleteSecret(ctx context.Context, orgUUID, name string) error {
	path := "/v1/orgs/" + url.PathEscape(orgUUID) + "/secrets/" + url.PathEscape(name)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Admin Portal =====

// AdminPortalIssueToken issues an admin portal token.
// POST /api/admin/portal/issue
func (c *Client) AdminPortalIssueToken(ctx context.Context, data map[string]any) (*AdminPortalToken, error) {
	var out AdminPortalToken
	if err := c.do(ctx, http.MethodPost, "/api/admin/portal/issue", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AdminPortalRevokeToken revokes an admin portal token.
// POST /api/admin/portal/revoke
func (c *Client) AdminPortalRevokeToken(ctx context.Context, token string) (map[string]any, error) {
	body := map[string]any{"token": token}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/admin/portal/revoke", body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Domains =====

// ListDomains lists domains for an org.
// GET /api/orgs/{org_uuid}/domains
func (c *Client) ListDomains(ctx context.Context, orgUUID string) ([]Domain, error) {
	var out []Domain
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/domains"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddDomain adds a domain to an org.
// POST /api/orgs/{org_uuid}/domains
func (c *Client) AddDomain(ctx context.Context, orgUUID, domain string) (*Domain, error) {
	body := map[string]any{"domain": domain}
	var out Domain
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/domains"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VerifyDomain verifies a domain.
// POST /api/orgs/{org_uuid}/domains/{domain_id}/verify
func (c *Client) VerifyDomain(ctx context.Context, orgUUID string, domainID int) (*Domain, error) {
	var out Domain
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/domains/" + strconv.Itoa(domainID) + "/verify"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteDomain deletes a domain.
// DELETE /api/orgs/{org_uuid}/domains/{domain_id}
func (c *Client) DeleteDomain(ctx context.Context, orgUUID string, domainID int) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/domains/" + strconv.Itoa(domainID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Webhooks Admin =====

// ListWebhookEndpoints lists webhook endpoints.
// GET /api/orgs/{org_uuid}/webhooks/endpoints
func (c *Client) ListWebhookEndpoints(ctx context.Context, orgUUID string) ([]WebhookEndpoint, error) {
	var out []WebhookEndpoint
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/endpoints"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateWebhookEndpoint creates a webhook endpoint.
// POST /api/orgs/{org_uuid}/webhooks/endpoints
func (c *Client) CreateWebhookEndpoint(ctx context.Context, orgUUID string, data map[string]any) (*WebhookEndpoint, error) {
	var out WebhookEndpoint
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/endpoints"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWebhookEndpoint fetches a single webhook endpoint.
// GET /api/orgs/{org_uuid}/webhooks/endpoints/{endpoint_id}
func (c *Client) GetWebhookEndpoint(ctx context.Context, orgUUID string, endpointID int) (*WebhookEndpoint, error) {
	var out WebhookEndpoint
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/endpoints/" + strconv.Itoa(endpointID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateWebhookEndpoint updates a webhook endpoint.
// PUT /api/orgs/{org_uuid}/webhooks/endpoints/{endpoint_id}
func (c *Client) UpdateWebhookEndpoint(ctx context.Context, orgUUID string, endpointID int, data map[string]any) (*WebhookEndpoint, error) {
	var out WebhookEndpoint
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/endpoints/" + strconv.Itoa(endpointID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWebhookEndpoint deletes a webhook endpoint.
// DELETE /api/orgs/{org_uuid}/webhooks/endpoints/{endpoint_id}
func (c *Client) DeleteWebhookEndpoint(ctx context.Context, orgUUID string, endpointID int) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/endpoints/" + strconv.Itoa(endpointID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ListWebhookDeliveries lists webhook deliveries.
// GET /api/orgs/{org_uuid}/webhooks/endpoints/{endpoint_id}/deliveries
func (c *Client) ListWebhookDeliveries(ctx context.Context, orgUUID string, endpointID int) ([]WebhookDelivery, error) {
	var out []WebhookDelivery
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/endpoints/" + strconv.Itoa(endpointID) + "/deliveries"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RetryWebhookDelivery retries a webhook delivery.
// POST /api/orgs/{org_uuid}/webhooks/endpoints/{endpoint_id}/deliveries/{delivery_id}/retry
func (c *Client) RetryWebhookDelivery(ctx context.Context, orgUUID string, endpointID int, deliveryID int64) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/endpoints/" + strconv.Itoa(endpointID) + "/deliveries/" + strconv.FormatInt(deliveryID, 10) + "/retry"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RotateWebhookSecret rotates the webhook signing secret.
// POST /api/orgs/{org_uuid}/webhooks/endpoints/{endpoint_id}/rotate-secret
func (c *Client) RotateWebhookSecret(ctx context.Context, orgUUID string, endpointID int) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/endpoints/" + strconv.Itoa(endpointID) + "/rotate-secret"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== SCIM =====

// ScimListUsers lists SCIM users.
// GET /api/scim/v2/Users
func (c *Client) ScimListUsers(ctx context.Context, filters map[string]string) (map[string]any, error) {
	path := "/api/scim/v2/Users"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScimGetUser fetches a SCIM user.
// GET /api/scim/v2/Users/{user_id}
func (c *Client) ScimGetUser(ctx context.Context, userID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/scim/v2/Users/" + url.PathEscape(userID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScimCreateUser creates a SCIM user.
// POST /api/scim/v2/Users
func (c *Client) ScimCreateUser(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/scim/v2/Users", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScimUpdateUser updates a SCIM user.
// PUT /api/scim/v2/Users/{user_id}
func (c *Client) ScimUpdateUser(ctx context.Context, userID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/scim/v2/Users/" + url.PathEscape(userID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScimPatchUser patches a SCIM user.
// PATCH /api/scim/v2/Users/{user_id}
func (c *Client) ScimPatchUser(ctx context.Context, userID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/scim/v2/Users/" + url.PathEscape(userID)
	if err := c.do(ctx, http.MethodPatch, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScimDeleteUser deletes a SCIM user.
// DELETE /api/scim/v2/Users/{user_id}
func (c *Client) ScimDeleteUser(ctx context.Context, userID string) error {
	path := "/api/scim/v2/Users/" + url.PathEscape(userID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ScimListGroups lists SCIM groups.
// GET /api/scim/v2/Groups
func (c *Client) ScimListGroups(ctx context.Context, filters map[string]string) (map[string]any, error) {
	path := "/api/scim/v2/Groups"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScimGetGroup fetches a SCIM group.
// GET /api/scim/v2/Groups/{group_id}
func (c *Client) ScimGetGroup(ctx context.Context, groupID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/scim/v2/Groups/" + url.PathEscape(groupID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScimCreateGroup creates a SCIM group.
// POST /api/scim/v2/Groups
func (c *Client) ScimCreateGroup(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/scim/v2/Groups", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScimPatchGroup patches a SCIM group.
// PATCH /api/scim/v2/Groups/{group_id}
func (c *Client) ScimPatchGroup(ctx context.Context, groupID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/scim/v2/Groups/" + url.PathEscape(groupID)
	if err := c.do(ctx, http.MethodPatch, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScimDeleteGroup deletes a SCIM group.
// DELETE /api/scim/v2/Groups/{group_id}
func (c *Client) ScimDeleteGroup(ctx context.Context, groupID string) error {
	path := "/api/scim/v2/Groups/" + url.PathEscape(groupID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Payments =====

// CreatePaymentCheckoutSession creates a payment checkout session.
// POST /api/payments/checkout-session
func (c *Client) CreatePaymentCheckoutSession(ctx context.Context, data map[string]any) (*PaymentCheckoutSession, error) {
	var out PaymentCheckoutSession
	if err := c.do(ctx, http.MethodPost, "/api/payments/checkout-session", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPayment fetches a payment.
// GET /api/payments/{payment_id}
func (c *Client) GetPayment(ctx context.Context, paymentID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/payments/" + url.PathEscape(paymentID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RefundPayment refunds a payment.
// POST /api/payments/{payment_id}/refund
func (c *Client) RefundPayment(ctx context.Context, paymentID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/payments/" + url.PathEscape(paymentID) + "/refund"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListPayments lists payments.
// GET /api/payments
func (c *Client) ListPayments(ctx context.Context, filters map[string]string) ([]map[string]any, error) {
	path := "/api/payments"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== SMS =====

// SendSms sends an SMS.
// POST /api/sms/send
func (c *Client) SendSms(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/sms/send", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Email =====

// SendEmail sends an email.
// POST /api/email/send
func (c *Client) SendEmail(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/email/send", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SendTemplatedEmail sends a templated email.
// POST /api/email/send-template
func (c *Client) SendTemplatedEmail(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/email/send-template", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Jobs & Notifications =====

// ListJobs lists background jobs.
// GET /api/admin/jobs
func (c *Client) ListJobs(ctx context.Context, filters map[string]string) ([]map[string]any, error) {
	path := "/api/admin/jobs"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetJob fetches a single job.
// GET /api/admin/jobs/{job_id}
func (c *Client) GetJob(ctx context.Context, jobID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/jobs/" + url.PathEscape(jobID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CancelJob cancels a job.
// POST /api/admin/jobs/{job_id}/cancel
func (c *Client) CancelJob(ctx context.Context, jobID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/jobs/" + url.PathEscape(jobID) + "/cancel"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RetryJob retries a job.
// POST /api/admin/jobs/{job_id}/retry
func (c *Client) RetryJob(ctx context.Context, jobID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/jobs/" + url.PathEscape(jobID) + "/retry"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListNotifications lists notifications.
// GET /api/notifications
func (c *Client) ListNotifications(ctx context.Context, filters map[string]string) ([]map[string]any, error) {
	path := "/api/notifications"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MarkNotificationRead marks a notification as read.
// POST /api/notifications/{notification_id}/read
func (c *Client) MarkNotificationRead(ctx context.Context, notificationID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/notifications/" + url.PathEscape(notificationID) + "/read"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MarkAllNotificationsRead marks all notifications as read.
// POST /api/notifications/read-all
func (c *Client) MarkAllNotificationsRead(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/notifications/read-all", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SendNotification sends a notification.
// POST /api/notifications/send
func (c *Client) SendNotification(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/notifications/send", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Custom Variables =====

// ListCustomVariables lists custom variables.
// GET /api/orgs/{org_uuid}/variables
func (c *Client) ListCustomVariables(ctx context.Context, orgUUID string) ([]map[string]any, error) {
	var out []map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/variables"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetCustomVariable sets a custom variable.
// PUT /api/orgs/{org_uuid}/variables/{key}
func (c *Client) SetCustomVariable(ctx context.Context, orgUUID, key string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/variables/" + url.PathEscape(key)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCustomVariable fetches a custom variable.
// GET /api/orgs/{org_uuid}/variables/{key}
func (c *Client) GetCustomVariable(ctx context.Context, orgUUID, key string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/variables/" + url.PathEscape(key)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteCustomVariable deletes a custom variable.
// DELETE /api/orgs/{org_uuid}/variables/{key}
func (c *Client) DeleteCustomVariable(ctx context.Context, orgUUID, key string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/variables/" + url.PathEscape(key)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Webhooks (legacy) =====

// ListLegacyWebhooks lists legacy webhooks.
// GET /api/webhooks
func (c *Client) ListLegacyWebhooks(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/webhooks", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateLegacyWebhook creates a legacy webhook.
// POST /api/webhooks
func (c *Client) CreateLegacyWebhook(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/webhooks", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetLegacyWebhook fetches a legacy webhook.
// GET /api/webhooks/{webhook_id}
func (c *Client) GetLegacyWebhook(ctx context.Context, webhookID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/webhooks/" + url.PathEscape(webhookID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateLegacyWebhook updates a legacy webhook.
// PUT /api/webhooks/{webhook_id}
func (c *Client) UpdateLegacyWebhook(ctx context.Context, webhookID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/webhooks/" + url.PathEscape(webhookID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteLegacyWebhook deletes a legacy webhook.
// DELETE /api/webhooks/{webhook_id}
func (c *Client) DeleteLegacyWebhook(ctx context.Context, webhookID string) error {
	path := "/api/webhooks/" + url.PathEscape(webhookID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// TestLegacyWebhook sends a test event to a legacy webhook.
// POST /api/webhooks/{webhook_id}/test
func (c *Client) TestLegacyWebhook(ctx context.Context, webhookID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/webhooks/" + url.PathEscape(webhookID) + "/test"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Password Reset =====

// RequestPasswordReset initiates a password reset.
// POST /api/auth/password-reset/request
func (c *Client) RequestPasswordReset(ctx context.Context, email string) (map[string]any, error) {
	body := map[string]any{"email": email}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/password-reset/request", body, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ConfirmPasswordReset confirms a password reset.
// POST /api/auth/password-reset/confirm
func (c *Client) ConfirmPasswordReset(ctx context.Context, token, newPassword string) (map[string]any, error) {
	body := map[string]any{"token": token, "new_password": newPassword}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/password-reset/confirm", body, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ChangePassword changes the authenticated user's password.
// POST /api/auth/change-password
func (c *Client) ChangePassword(ctx context.Context, currentPassword, newPassword string) (map[string]any, error) {
	body := map[string]any{"current_password": currentPassword, "new_password": newPassword}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/change-password", body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Logout =====

// Logout logs out the current session.
// POST /api/auth/logout
func (c *Client) Logout(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/auth/logout", nil, true, nil)
}

// ===== Token Refresh =====

// RefreshToken refreshes an access token.
// POST /api/auth/refresh
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*LoginResponse, error) {
	body := map[string]any{"refresh_token": refreshToken}
	var out LoginResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/refresh", body, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.APIKey = out.AccessToken
	}
	return &out, nil
}

// ===== Org Management =====

// GetOrg fetches an organization.
// GET /api/orgs/{org_uuid}
func (c *Client) GetOrg(ctx context.Context, orgUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateOrg updates an organization.
// PUT /api/orgs/{org_uuid}
func (c *Client) UpdateOrg(ctx context.Context, orgUUID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListOrgs lists organizations.
// GET /api/orgs
func (c *Client) ListOrgs(ctx context.Context, filters map[string]string) ([]map[string]any, error) {
	path := "/api/orgs"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteOrg deletes an organization.
// DELETE /api/orgs/{org_uuid}
func (c *Client) DeleteOrg(ctx context.Context, orgUUID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Org Members =====

// ListOrgMembers lists org members.
// GET /api/orgs/{org_uuid}/members
func (c *Client) ListOrgMembers(ctx context.Context, orgUUID string) ([]map[string]any, error) {
	var out []map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/members"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// InviteOrgMember invites a member to an org.
// POST /api/orgs/{org_uuid}/members/invite
func (c *Client) InviteOrgMember(ctx context.Context, orgUUID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/members/invite"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveOrgMember removes a member from an org.
// DELETE /api/orgs/{org_uuid}/members/{user_uuid}
func (c *Client) RemoveOrgMember(ctx context.Context, orgUUID, userUUID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/members/" + url.PathEscape(userUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// UpdateOrgMemberRole updates a member's role in an org.
// PUT /api/orgs/{org_uuid}/members/{user_uuid}/role
func (c *Client) UpdateOrgMemberRole(ctx context.Context, orgUUID, userUUID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/members/" + url.PathEscape(userUUID) + "/role"
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Invitations =====

// ListInvitations lists pending invitations.
// GET /api/invitations
func (c *Client) ListInvitations(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/invitations", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AcceptInvitation accepts an invitation.
// POST /api/invitations/{invitation_id}/accept
func (c *Client) AcceptInvitation(ctx context.Context, invitationID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/invitations/" + url.PathEscape(invitationID) + "/accept"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeclineInvitation declines an invitation.
// POST /api/invitations/{invitation_id}/decline
func (c *Client) DeclineInvitation(ctx context.Context, invitationID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/invitations/" + url.PathEscape(invitationID) + "/decline"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeInvitation revokes an invitation.
// DELETE /api/invitations/{invitation_id}
func (c *Client) RevokeInvitation(ctx context.Context, invitationID string) error {
	path := "/api/invitations/" + url.PathEscape(invitationID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Products Admin =====

// ListProducts lists products.
// GET /api/admin/products
func (c *Client) ListProducts(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/admin/products", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateProduct creates a product.
// POST /api/admin/products
func (c *Client) CreateProduct(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/admin/products", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetProduct fetches a product.
// GET /api/admin/products/{product_id}
func (c *Client) GetProduct(ctx context.Context, productID int) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateProduct updates a product.
// PUT /api/admin/products/{product_id}
func (c *Client) UpdateProduct(ctx context.Context, productID int, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteProduct deletes a product.
// DELETE /api/admin/products/{product_id}
func (c *Client) DeleteProduct(ctx context.Context, productID int) error {
	path := "/api/admin/products/" + strconv.Itoa(productID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Plans Admin =====

// ListPlans lists plans for a product.
// GET /api/admin/products/{product_id}/plans
func (c *Client) ListPlans(ctx context.Context, productID int) ([]map[string]any, error) {
	var out []map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/plans"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreatePlan creates a plan.
// POST /api/admin/products/{product_id}/plans
func (c *Client) CreatePlan(ctx context.Context, productID int, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/plans"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetPlan fetches a plan.
// GET /api/admin/products/{product_id}/plans/{plan_id}
func (c *Client) GetPlan(ctx context.Context, productID, planID int) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/plans/" + strconv.Itoa(planID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdatePlan updates a plan.
// PUT /api/admin/products/{product_id}/plans/{plan_id}
func (c *Client) UpdatePlan(ctx context.Context, productID, planID int, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/plans/" + strconv.Itoa(planID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeletePlan deletes a plan.
// DELETE /api/admin/products/{product_id}/plans/{plan_id}
func (c *Client) DeletePlan(ctx context.Context, productID, planID int) error {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/plans/" + strconv.Itoa(planID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Subscriptions =====

// ListSubscriptions lists subscriptions.
// GET /api/subscriptions
func (c *Client) ListSubscriptions(ctx context.Context, filters map[string]string) ([]map[string]any, error) {
	path := "/api/subscriptions"
	if len(filters) > 0 {
		q := url.Values{}
		for k, v := range filters {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetSubscription fetches a subscription.
// GET /api/subscriptions/{subscription_id}
func (c *Client) GetSubscription(ctx context.Context, subscriptionID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/subscriptions/" + url.PathEscape(subscriptionID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateSubscription creates a subscription.
// POST /api/subscriptions
func (c *Client) CreateSubscription(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/subscriptions", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CancelSubscription cancels a subscription.
// POST /api/subscriptions/{subscription_id}/cancel
func (c *Client) CancelSubscription(ctx context.Context, subscriptionID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/subscriptions/" + url.PathEscape(subscriptionID) + "/cancel"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ChangeSubscriptionPlan changes a subscription's plan.
// POST /api/subscriptions/{subscription_id}/change-plan
func (c *Client) ChangeSubscriptionPlan(ctx context.Context, subscriptionID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/subscriptions/" + url.PathEscape(subscriptionID) + "/change-plan"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Feature Flags (User-facing) =====

// GetFeatureFlag checks a feature flag.
// GET /api/features/{feature_id}
func (c *Client) GetFeatureFlag(ctx context.Context, featureID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/features/" + url.PathEscape(featureID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListFeatureFlags lists feature flags.
// GET /api/features
func (c *Client) ListFeatureFlags(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/features", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Admin Feature Flags =====

// AdminCreateFeatureFlag creates a feature flag.
// POST /api/admin/features
func (c *Client) AdminCreateFeatureFlag(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/admin/features", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AdminUpdateFeatureFlag updates a feature flag.
// PUT /api/admin/features/{feature_id}
func (c *Client) AdminUpdateFeatureFlag(ctx context.Context, featureID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/features/" + url.PathEscape(featureID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AdminDeleteFeatureFlag deletes a feature flag.
// DELETE /api/admin/features/{feature_id}
func (c *Client) AdminDeleteFeatureFlag(ctx context.Context, featureID string) error {
	path := "/api/admin/features/" + url.PathEscape(featureID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Rate Limits =====

// GetRateLimitConfig returns rate limit configuration.
// GET /api/admin/rate-limits
func (c *Client) GetRateLimitConfig(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/admin/rate-limits", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateRateLimitConfig updates rate limit configuration.
// PUT /api/admin/rate-limits
func (c *Client) UpdateRateLimitConfig(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPut, "/api/admin/rate-limits", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== IP Allow/Block Lists =====

// ListIPAllowlist lists IP allowlist entries.
// GET /api/orgs/{org_uuid}/ip-allowlist
func (c *Client) ListIPAllowlist(ctx context.Context, orgUUID string) ([]map[string]any, error) {
	var out []map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/ip-allowlist"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddIPAllowlistEntry adds an IP allowlist entry.
// POST /api/orgs/{org_uuid}/ip-allowlist
func (c *Client) AddIPAllowlistEntry(ctx context.Context, orgUUID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/ip-allowlist"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveIPAllowlistEntry removes an IP allowlist entry.
// DELETE /api/orgs/{org_uuid}/ip-allowlist/{entry_id}
func (c *Client) RemoveIPAllowlistEntry(ctx context.Context, orgUUID, entryID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/ip-allowlist/" + url.PathEscape(entryID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ===== Email Templates =====

// ListEmailTemplates lists email templates.
// GET /api/admin/email-templates
func (c *Client) ListEmailTemplates(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/admin/email-templates", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetEmailTemplate fetches an email template.
// GET /api/admin/email-templates/{template_id}
func (c *Client) GetEmailTemplate(ctx context.Context, templateID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/email-templates/" + url.PathEscape(templateID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateEmailTemplate updates an email template.
// PUT /api/admin/email-templates/{template_id}
func (c *Client) UpdateEmailTemplate(ctx context.Context, templateID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/email-templates/" + url.PathEscape(templateID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ResetEmailTemplate resets an email template to default.
// POST /api/admin/email-templates/{template_id}/reset
func (c *Client) ResetEmailTemplate(ctx context.Context, templateID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/admin/email-templates/" + url.PathEscape(templateID) + "/reset"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Health =====

// HealthCheck performs a health check.
// GET /api/health
func (c *Client) HealthCheck(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/health", nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ensure strconv stays used (helper for callers building queries).
var _ = strconv.Itoa
