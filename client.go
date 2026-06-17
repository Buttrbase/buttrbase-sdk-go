// Package buttrbase is a Go SDK for the Buttrbase API.
package buttrbase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const defaultBaseURL = "https://api.buttrbase.com"

const (
	// defaultMaxRetries is the default number of retry attempts for transient
	// failures (in addition to the initial attempt).
	defaultMaxRetries = 3
	// defaultRetryBaseDelay is the base delay for exponential backoff.
	defaultRetryBaseDelay = 500 * time.Millisecond
	// maxRetryDelay caps the per-attempt backoff delay.
	maxRetryDelay = 4 * time.Second
)

// Client is the Buttrbase API client.
type Client struct {
	BaseURL string

	// AccessToken is the OAuth2 bearer token sent as `Authorization: Bearer`
	// on authenticated requests. For app-server callers this is the access
	// token obtained from the OAuth2 client-credentials grant (exchange your
	// client_id/client_secret for a token, then construct the client with it).
	// For end-user flows it is the access token returned by Login/VerifyOTP/etc.,
	// which those methods refresh in place. Static API keys are no longer supported.
	AccessToken string

	HTTPClient *http.Client

	// MaxRetries is the number of times a request is retried on transient
	// failures (HTTP 429/502/503/504 and transport errors). 0 disables retries.
	// Defaults to defaultMaxRetries.
	MaxRetries int
	// RetryBaseDelay is the base delay for exponential backoff with jitter.
	// Defaults to defaultRetryBaseDelay.
	RetryBaseDelay time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(c *Client) { c.BaseURL = u } }

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.HTTPClient = h } }

// WithMaxRetries overrides the number of retry attempts for transient
// failures. Set to 0 to disable retries.
func WithMaxRetries(n int) Option { return func(c *Client) { c.MaxRetries = n } }

// WithRetryBaseDelay overrides the base delay used for exponential backoff.
func WithRetryBaseDelay(d time.Duration) Option {
	return func(c *Client) { c.RetryBaseDelay = d }
}

// New creates a new Client authenticated with the given OAuth2 bearer
// access token.
//
// App-server callers obtain accessToken from the OAuth2 client-credentials
// grant (exchange your client_id/client_secret for an access token, then
// pass it here). End-user callers may pass an empty string and authenticate
// via Login/VerifyOTP/etc., which populate and refresh the token in place.
// Static API keys (the retired wb_live_/wb_test_ keys) are no longer accepted.
func New(accessToken string, opts ...Option) *Client {
	c := &Client{
		BaseURL:        defaultBaseURL,
		AccessToken:    accessToken,
		HTTPClient:     &http.Client{Timeout: 30 * time.Second},
		MaxRetries:     defaultMaxRetries,
		RetryBaseDelay: defaultRetryBaseDelay,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.RetryBaseDelay <= 0 {
		c.RetryBaseDelay = defaultRetryBaseDelay
	}
	return c
}

// isRetryableStatus reports whether an HTTP status code indicates a transient
// failure that is safe to retry. 502/503/504 are gateway/cold-start errors
// (the app never processed the request, so retrying is safe for any method,
// including POST). 429 is rate limiting.
func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, // 429
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	default:
		return false
	}
}

// retryDelay computes the backoff for the given attempt (0-indexed), honoring a
// Retry-After header if present. resp may be nil (transport error).
func (c *Client) retryDelay(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			return d
		}
	}
	// Exponential backoff: base * 2^attempt, capped, with full jitter.
	backoff := float64(c.RetryBaseDelay) * math.Pow(2, float64(attempt))
	if backoff > float64(maxRetryDelay) {
		backoff = float64(maxRetryDelay)
	}
	// Full jitter in [backoff/2, backoff].
	half := backoff / 2
	return time.Duration(half + rand.Float64()*half)
}

// parseRetryAfter parses a Retry-After header value, which may be either a
// number of seconds or an HTTP-date.
func parseRetryAfter(v string) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

// sleepCtx waits for d or until ctx is done, whichever comes first. It returns
// ctx.Err() if the context was cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, auth bool, out any) error {
	// Buffer the body once so it can be re-read on every retry attempt; a
	// fresh reader is set per attempt below (otherwise retries would send an
	// empty body).
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyBytes = b
	}
	u := c.BaseURL + path

	for attempt := 0; ; attempt++ {
		var rdr io.Reader
		if bodyBytes != nil {
			rdr = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, u, rdr)
		if err != nil {
			return err
		}
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		if auth && c.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.AccessToken)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			// Transport error: retry if attempts remain and ctx not done.
			if attempt < c.MaxRetries && ctx.Err() == nil {
				if serr := sleepCtx(ctx, c.retryDelay(attempt, nil)); serr != nil {
					return err
				}
				continue
			}
			return err
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			detail := ""
			var parsed map[string]any
			if json.Unmarshal(respBody, &parsed) == nil {
				if d, ok := parsed["detail"].(string); ok {
					detail = d
				}
			}
			apiErr := &ButtrbaseError{StatusCode: resp.StatusCode, Detail: detail, Body: respBody}
			// Retry only on transient/cold-start statuses.
			if isRetryableStatus(resp.StatusCode) && attempt < c.MaxRetries {
				if serr := sleepCtx(ctx, c.retryDelay(attempt, resp)); serr != nil {
					return apiErr
				}
				continue
			}
			return apiErr
		}

		if out != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("buttrbase: decode response: %w", err)
			}
		}
		return nil
	}
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

// SendMagicLink sends a magic-link email for the given app.
// POST /api/auth/magic-link/send
//
// appUUID must be the target app's UUID. The legacy `app` / `appName`
// slug fields are no longer accepted by the backend.
func (c *Client) SendMagicLink(ctx context.Context, appUUID, email string, opts *SendMagicLinkOptions) (*MagicLinkSend, error) {
	body := map[string]any{"app_uuid": appUUID, "email": email}
	if opts != nil {
		if opts.RedirectURL != "" {
			body["redirect_to"] = opts.RedirectURL
		}
		if opts.TTLSeconds != nil {
			body["ttl_seconds"] = *opts.TTLSeconds
		}
	}
	var out MagicLinkSend
	if err := c.do(ctx, http.MethodPost, "/api/auth/magic-link/send", body, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VerifyMagicLink consumes a magic-link token and returns the session.
// POST /api/auth/magic-link/verify
func (c *Client) VerifyMagicLink(ctx context.Context, token string) (*MagicLinkVerify, error) {
	body := map[string]any{"token": token}
	var out MagicLinkVerify
	if err := c.do(ctx, http.MethodPost, "/api/auth/magic-link/verify", body, false, &out); err != nil {
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

func (c *Client) AuthStepUp(ctx context.Context, code string, recovery bool) (*StepUpResponse, error) {
	body := map[string]any{"code": code, "recovery": recovery}
	var out StepUpResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/step-up", body, true, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.AccessToken = out.AccessToken
	}
	return &out, nil
}

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

func (c *Client) ElevationApprove(ctx context.Context, orgUUID, grantUUID string) (*ElevationGrant, error) {
	var out ElevationGrant
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/elevation/" + url.PathEscape(grantUUID) + "/approve"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

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

func (c *Client) ReencryptSecrets(ctx context.Context, orgUUID string) (*ReencryptResponse, error) {
	return c.reencrypt(ctx, orgUUID, "secrets")
}

func (c *Client) ReencryptSigningKeys(ctx context.Context, orgUUID string) (*ReencryptResponse, error) {
	return c.reencrypt(ctx, orgUUID, "signing-keys")
}

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

func (c *Client) GetOrgMetrics(ctx context.Context, orgUUID string) (*OrgMetrics, error) {
	var out OrgMetrics
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/metrics"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListCredentials(ctx context.Context) (*CredentialList, error) {
	var out CredentialList
	if err := c.do(ctx, http.MethodGet, "/credentials", nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateCredential(ctx context.Context, req CreateCredentialRequest) (*Credential, error) {
	var out Credential
	if err := c.do(ctx, http.MethodPost, "/credentials", req, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetCredential(ctx context.Context, credentialsID string) (*Credential, error) {
	var out Credential
	path := "/credentials/" + url.PathEscape(credentialsID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteCredential(ctx context.Context, credentialsID string) error {
	path := "/credentials/" + url.PathEscape(credentialsID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) RotateCredentialSecret(ctx context.Context, credentialsID string) (*RotateSecretResponse, error) {
	var out RotateSecretResponse
	path := "/credentials/" + url.PathEscape(credentialsID) + "/rotate-secret"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

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

// Register creates a new account scoped to the given app.
// POST /api/auth/register
//
// appUUID identifies the target app — this replaces the legacy `app`
// slug parameter, which the backend no longer accepts.
//
// Deprecated: Use the 0.3.0 flow instead: SendOTP → VerifyOTP → FinalizeRegistration.
func (c *Client) Register(ctx context.Context, appUUID, email, password, orgName string, opts *RegisterOptions) (*LoginResponse, error) {
	body := map[string]any{"app_uuid": appUUID, "email": email, "password": password, "org_name": orgName}
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
		c.AccessToken = out.AccessToken
	}
	return &out, nil
}

// Login authenticates against the given app and stores the access token.
// POST /api/auth/login
//
// appUUID identifies the target app — this replaces the legacy `app`
// slug parameter, which the backend no longer accepts.
func (c *Client) Login(ctx context.Context, appUUID, email, password, orgName string) (*LoginResponse, error) {
	body := map[string]any{"app_uuid": appUUID, "email": email, "password": password}
	if orgName != "" {
		body["org_name"] = orgName
	}
	var out LoginResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/login", body, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.AccessToken = out.AccessToken
	}
	return &out, nil
}

func (c *Client) GetLoginOptions(ctx context.Context, orgUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/auth/organizations/" + url.PathEscape(orgUUID) + "/login-options"
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetStatus(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/auth/status", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetProfile(ctx context.Context) (*Profile, error) {
	var out Profile
	if err := c.do(ctx, http.MethodGet, "/api/profile", nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateProfile(ctx context.Context, data map[string]any) (*Profile, error) {
	var out Profile
	if err := c.do(ctx, http.MethodPut, "/api/profile", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetOrgByDomain(ctx context.Context, domain string) (map[string]any, error) {
	var out map[string]any
	path := "/api/auth/orgs-by-domain/" + url.PathEscape(domain)
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== OTP =====

// OtpSend sends an OTP to a phone number or email for the given app.
// POST /api/auth/otp
//
// appUUID identifies the target app — required by the backend in place
// of the legacy `app` slug. Pass either a phone or email destination
// (set the other to "").
func (c *Client) OtpSend(ctx context.Context, appUUID, phone string) (map[string]any, error) {
	body := map[string]any{"app_uuid": appUUID, "phone": phone}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/otp", body, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OtpVerify verifies an OTP code against the given app.
// POST /api/auth/otp/verify
//
// appUUID identifies the target app — required by the backend in place
// of the legacy `app` slug.
func (c *Client) OtpVerify(ctx context.Context, appUUID, phone, code string) (*LoginResponse, error) {
	body := map[string]any{"app_uuid": appUUID, "phone": phone, "otp": code}
	var out LoginResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/otp/verify", body, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.AccessToken = out.AccessToken
	}
	return &out, nil
}

func (c *Client) MfaVerify(ctx context.Context, code string) (map[string]any, error) {
	body := map[string]any{"code": code}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/mfa/totp/verify", body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) MfaChallenge(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/mfa/totp/challenge", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) MfaDisable(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodDelete, "/api/auth/mfa/totp", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) MfaGenerateRecoveryCodes(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/mfa/recovery-codes", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) MfaRedeemRecoveryCode(ctx context.Context, code string) (map[string]any, error) {
	body := map[string]any{"code": code}
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/auth/mfa/recovery-codes/redeem", body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) OidcAuthorizeURL(ctx context.Context, connectionUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/auth/sso/oidc/" + url.PathEscape(connectionUUID) + "/authorize"
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) SamlAuthorizeURL(ctx context.Context, connectionUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/auth/sso/saml/" + url.PathEscape(connectionUUID) + "/authorize"
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) OidcCallback(ctx context.Context, connectionUUID string, params map[string]any) (*LoginResponse, error) {
	var out LoginResponse
	path := "/api/auth/sso/oidc/" + url.PathEscape(connectionUUID) + "/callback"
	if err := c.do(ctx, http.MethodPost, path, params, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.AccessToken = out.AccessToken
	}
	return &out, nil
}

func (c *Client) SamlCallback(ctx context.Context, connectionUUID string, params map[string]any) (*LoginResponse, error) {
	var out LoginResponse
	path := "/api/auth/sso/saml/" + url.PathEscape(connectionUUID) + "/callback"
	if err := c.do(ctx, http.MethodPost, path, params, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.AccessToken = out.AccessToken
	}
	return &out, nil
}

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

func (c *Client) GetUser(ctx context.Context, userUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/users/" + url.PathEscape(userUUID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetUserLevel(ctx context.Context, userUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/users/" + url.PathEscape(userUUID) + "/level"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) SetUserLevel(ctx context.Context, userUUID, userType string) (map[string]any, error) {
	body := map[string]any{"user_type": userType}
	var out map[string]any
	path := "/api/users/" + url.PathEscape(userUUID) + "/level"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UpdateUserStatus(ctx context.Context, userUUID string, active bool) (map[string]any, error) {
	body := map[string]any{"active": active}
	var out map[string]any
	path := "/api/users/" + url.PathEscape(userUUID) + "/status"
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UpdateUserRole(ctx context.Context, userUUID, role string) (map[string]any, error) {
	body := map[string]any{"role": role}
	var out map[string]any
	path := "/api/users/" + url.PathEscape(userUUID) + "/role"
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeleteUser(ctx context.Context, userUUID string) error {
	path := "/api/users/" + url.PathEscape(userUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) GetOrgSecuritySettings(ctx context.Context, orgUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/security"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UpdateOrgSecuritySettings(ctx context.Context, orgUUID string, settings map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/security"
	if err := c.do(ctx, http.MethodPut, path, settings, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListSsoConnections(ctx context.Context, orgUUID string) ([]SsoConnection, error) {
	var out []SsoConnection
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/sso/connections"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateSsoConnection(ctx context.Context, orgUUID string, data map[string]any) (*SsoConnection, error) {
	var out SsoConnection
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/sso/connections"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetSsoConnection(ctx context.Context, orgUUID, connectionUUID string) (*SsoConnection, error) {
	var out SsoConnection
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/sso/connections/" + url.PathEscape(connectionUUID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateSsoConnection(ctx context.Context, orgUUID, connectionUUID string, data map[string]any) (*SsoConnection, error) {
	var out SsoConnection
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/sso/connections/" + url.PathEscape(connectionUUID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteSsoConnection(ctx context.Context, orgUUID, connectionUUID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/sso/connections/" + url.PathEscape(connectionUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

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

func (c *Client) GetAuditEvent(ctx context.Context, orgUUID string, eventID int64) (*AuditEventEntry, error) {
	var out AuditEventEntry
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/audit-events/" + strconv.FormatInt(eventID, 10)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetBranding(ctx context.Context, orgUUID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/branding"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UpdateBranding(ctx context.Context, orgUUID string, branding map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/branding"
	if err := c.do(ctx, http.MethodPut, path, branding, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

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

func (c *Client) CreateDeviceAccount(ctx context.Context, data map[string]any) (*UserAccount, error) {
	var out UserAccount
	if err := c.do(ctx, http.MethodPost, "/api/device-accounts", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDeviceAccount(ctx context.Context, accountUUID string) (*UserAccount, error) {
	var out UserAccount
	path := "/api/device-accounts/" + url.PathEscape(accountUUID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteDeviceAccount(ctx context.Context, accountUUID string) error {
	path := "/api/device-accounts/" + url.PathEscape(accountUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) ListSessions(ctx context.Context) ([]SessionInfo, error) {
	var out []SessionInfo
	if err := c.do(ctx, http.MethodGet, "/api/sessions", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	var out SessionInfo
	path := "/api/sessions/" + url.PathEscape(sessionID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RevokeSessionByID(ctx context.Context, sessionID string) error {
	path := "/api/sessions/" + url.PathEscape(sessionID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) RevokeAllSessions(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/sessions/revoke-all", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListServiceIdentities(ctx context.Context, orgUUID string) ([]map[string]any, error) {
	var out []map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateServiceIdentity(ctx context.Context, orgUUID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetServiceIdentity(ctx context.Context, orgUUID, identityID string) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities/" + url.PathEscape(identityID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UpdateServiceIdentity(ctx context.Context, orgUUID, identityID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities/" + url.PathEscape(identityID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeleteServiceIdentity(ctx context.Context, orgUUID, identityID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities/" + url.PathEscape(identityID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) CreateServiceIdentityToken(ctx context.Context, orgUUID, identityID string, data map[string]any) (map[string]any, error) {
	var out map[string]any
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/service-identities/" + url.PathEscape(identityID) + "/token"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CheckEntitlement(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/entitlements/check", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) BatchCheckEntitlements(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/entitlements/batch-check", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

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

func (c *Client) AdminExplainEntitlement(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/admin/entitlements/explain", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) PricingPreview(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/pricing/preview", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) PricingQuote(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/pricing/quote", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) PricingCheckoutSession(ctx context.Context, data map[string]any) (*PaymentCheckoutSession, error) {
	var out PaymentCheckoutSession
	if err := c.do(ctx, http.MethodPost, "/api/pricing/checkout-session", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) AdminExplainPricing(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/admin/pricing/explain", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

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

func (c *Client) ListCoupons(ctx context.Context, productID int) ([]Coupon, error) {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons"
	var out []Coupon
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateCoupon(ctx context.Context, productID int, data map[string]any) (*Coupon, error) {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons"
	var out Coupon
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetCoupon(ctx context.Context, productID, couponID int) (*Coupon, error) {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID)
	var out Coupon
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateCoupon(ctx context.Context, productID, couponID int, data map[string]any) (*Coupon, error) {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID)
	var out Coupon
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteCoupon(ctx context.Context, productID, couponID int) error {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) SetCouponLabels(ctx context.Context, productID, couponID int, labels []string) (map[string]any, error) {
	body := map[string]any{"labels": labels}
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID) + "/labels"
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) AddCouponLabel(ctx context.Context, productID, couponID int, label string) (map[string]any, error) {
	body := map[string]any{"label": label}
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID) + "/labels"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) RemoveCouponLabel(ctx context.Context, productID, couponID int, label string) error {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/coupons/" + strconv.Itoa(couponID) + "/labels/" + url.PathEscape(label)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) SetProductTags(ctx context.Context, productID int, tags []string) (map[string]any, error) {
	body := map[string]any{"tags": tags}
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/tags"
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) AddProductTag(ctx context.Context, productID int, tag string) (map[string]any, error) {
	body := map[string]any{"tag": tag}
	var out map[string]any
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/tags"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) RemoveProductTag(ctx context.Context, productID int, tag string) error {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/tags/" + url.PathEscape(tag)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) IngestAnalyticsEvent(ctx context.Context, data map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/analytics/events", data, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

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

func (c *Client) GetOrgOverview(ctx context.Context, orgUUID string, filters map[string]string) (map[string]any, error) {
	q := url.Values{}
	q.Set("org_uuid", orgUUID)
	for k, v := range filters {
		q.Set(k, v)
	}
	path := "/api/analytics/org-overview?" + q.Encode()
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetUserOverview(ctx context.Context, userUUID string, filters map[string]string) (map[string]any, error) {
	q := url.Values{}
	q.Set("user_uuid", userUUID)
	for k, v := range filters {
		q.Set(k, v)
	}
	path := "/api/analytics/user-overview?" + q.Encode()
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetFunnelAnalytics(ctx context.Context, filters map[string]string) (map[string]any, error) {
	path := "/api/analytics/funnel"
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

func (c *Client) GetRetentionAnalytics(ctx context.Context, filters map[string]string) (map[string]any, error) {
	path := "/api/analytics/retention"
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

func (c *Client) ListBillingInvoices(ctx context.Context) ([]Invoice, error) {
	var out []Invoice
	if err := c.do(ctx, http.MethodGet, "/api/billing/invoices", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetBillingInvoice(ctx context.Context, invoiceID int) (*Invoice, error) {
	var out Invoice
	path := "/api/billing/invoices/" + strconv.Itoa(invoiceID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) BillingCheckout(ctx context.Context, data map[string]any) (*CheckoutResponse, error) {
	var out CheckoutResponse
	if err := c.do(ctx, http.MethodPost, "/api/billing/checkout", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetBillingPortal(ctx context.Context) (*CheckoutResponse, error) {
	var out CheckoutResponse
	if err := c.do(ctx, http.MethodGet, "/api/billing/portal", nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SendInvoice(ctx context.Context, data map[string]any) (*SendInvoiceResponse, error) {
	var out SendInvoiceResponse
	if err := c.do(ctx, http.MethodPost, "/api/billing/send-invoice", data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListSigningKeys(ctx context.Context, orgUUID string) ([]SigningKey, error) {
	var out []SigningKey
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateSigningKey(ctx context.Context, orgUUID string, data map[string]any) (*SigningKey, error) {
	var out SigningKey
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetSigningKey(ctx context.Context, orgUUID, keyID string) (*SigningKey, error) {
	var out SigningKey
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys/" + url.PathEscape(keyID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RevokeSigningKey(ctx context.Context, orgUUID, keyID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys/" + url.PathEscape(keyID) + "/revoke"
	return c.do(ctx, http.MethodPost, path, nil, true, nil)
}

func (c *Client) ListSigningAudit(ctx context.Context, orgUUID string) ([]SigningAuditEntryItem, error) {
	var out []SigningAuditEntryItem
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/signing-keys/audit"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetMtlsCa(ctx context.Context, orgUUID string) (*CertificateAuthority, error) {
	var out CertificateAuthority
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls/ca"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateMtlsCa(ctx context.Context, orgUUID string) (*CertificateAuthority, error) {
	var out CertificateAuthority
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls/ca"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) IssueMtlsCert(ctx context.Context, orgUUID string, data map[string]any) (*Certificate, error) {
	var out Certificate
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls/issue"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListMtlsCerts(ctx context.Context, orgUUID string) ([]Certificate, error) {
	var out []Certificate
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls/certificates"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) RevokeMtlsCert(ctx context.Context, orgUUID, serial string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/mtls/certificates/" + url.PathEscape(serial) + "/revoke"
	return c.do(ctx, http.MethodPost, path, nil, true, nil)
}

func (c *Client) ListDomains(ctx context.Context, orgUUID string) ([]Domain, error) {
	var out []Domain
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/domains"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) AddDomain(ctx context.Context, orgUUID, domain string) (*Domain, error) {
	body := map[string]any{"domain": domain}
	var out Domain
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/domains"
	if err := c.do(ctx, http.MethodPost, path, body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) VerifyDomain(ctx context.Context, orgUUID string, domainID int) (*Domain, error) {
	var out Domain
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/domains/" + strconv.Itoa(domainID) + "/verify"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RemoveDomain(ctx context.Context, orgUUID string, domainID int) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/domains/" + strconv.Itoa(domainID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) ListWebhookEndpoints(ctx context.Context, orgUUID string) ([]WebhookEndpoint, error) {
	var out []WebhookEndpoint
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateWebhookEndpoint(ctx context.Context, orgUUID string, data map[string]any) (*WebhookEndpoint, error) {
	var out WebhookEndpoint
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetWebhookEndpoint(ctx context.Context, orgUUID string, endpointID int) (*WebhookEndpoint, error) {
	var out WebhookEndpoint
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/" + strconv.Itoa(endpointID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateWebhookEndpoint(ctx context.Context, orgUUID string, endpointID int, data map[string]any) (*WebhookEndpoint, error) {
	var out WebhookEndpoint
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/" + strconv.Itoa(endpointID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteWebhookEndpoint(ctx context.Context, orgUUID string, endpointID int) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/" + strconv.Itoa(endpointID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) ListWebhookEndpointDeliveries(ctx context.Context, orgUUID string, endpointID int) ([]WebhookDelivery, error) {
	var out []WebhookDelivery
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/webhooks/" + strconv.Itoa(endpointID) + "/deliveries"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetAdminPortalToken(ctx context.Context, orgUUID string) (*AdminPortalToken, error) {
	var out AdminPortalToken
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/admin-portal/issue"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListOrgFeatures(ctx context.Context, orgUUID string) ([]OrgFeature, error) {
	var out []OrgFeature
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/features"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) SetOrgFeature(ctx context.Context, orgUUID, featureID string, enabled bool) (*OrgFeature, error) {
	body := map[string]any{"enabled": enabled}
	var out OrgFeature
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/features/" + url.PathEscape(featureID)
	if err := c.do(ctx, http.MethodPut, path, body, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListPermissions(ctx context.Context, productID int) ([]Permission, error) {
	var out []Permission
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/permissions"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreatePermission(ctx context.Context, productID int, data map[string]any) (*Permission, error) {
	var out Permission
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/permissions"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPermission(ctx context.Context, productID, permissionID int) (*Permission, error) {
	var out Permission
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/permissions/" + strconv.Itoa(permissionID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdatePermission(ctx context.Context, productID, permissionID int, data map[string]any) (*Permission, error) {
	var out Permission
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/permissions/" + strconv.Itoa(permissionID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeletePermission(ctx context.Context, productID, permissionID int) error {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/permissions/" + strconv.Itoa(permissionID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) ListRoles(ctx context.Context, productID int) ([]Role, error) {
	var out []Role
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/roles"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateRole(ctx context.Context, productID int, data map[string]any) (*Role, error) {
	var out Role
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/roles"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRole(ctx context.Context, productID, roleID int) (*Role, error) {
	var out Role
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/roles/" + strconv.Itoa(roleID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateRole(ctx context.Context, productID, roleID int, data map[string]any) (*Role, error) {
	var out Role
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/roles/" + strconv.Itoa(roleID)
	if err := c.do(ctx, http.MethodPut, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteRole(ctx context.Context, productID, roleID int) error {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/roles/" + strconv.Itoa(roleID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) AssignRolePermission(ctx context.Context, productID, roleID, permissionID int) error {
	body := map[string]any{"permission_id": permissionID}
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/roles/" + strconv.Itoa(roleID) + "/permissions"
	return c.do(ctx, http.MethodPost, path, body, true, nil)
}

func (c *Client) RemoveRolePermission(ctx context.Context, productID, roleID, permissionID int) error {
	path := "/api/admin/products/" + strconv.Itoa(productID) + "/roles/" + strconv.Itoa(roleID) + "/permissions/" + strconv.Itoa(permissionID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) ListSecrets(ctx context.Context, orgUUID string) ([]SecretEntry, error) {
	var out []SecretEntry
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/secrets"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeleteSecret(ctx context.Context, orgUUID, name string) error {
	path := "/api/admin/orgs/" + url.PathEscape(orgUUID) + "/secrets/" + url.PathEscape(name)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) GetHelpCategories(ctx context.Context) ([]HelpCategory, error) {
	var out []HelpCategory
	if err := c.do(ctx, http.MethodGet, "/api/help/categories", nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetHelpCategory(ctx context.Context, slug string) (*HelpCategory, error) {
	var out HelpCategory
	path := "/api/help/categories/" + url.PathEscape(slug)
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetHelpArticle(ctx context.Context, categorySlug, articleSlug string) (*HelpArticle, error) {
	var out HelpArticle
	path := "/api/help/categories/" + url.PathEscape(categorySlug) + "/articles/" + url.PathEscape(articleSlug)
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SearchHelp(ctx context.Context, query string) ([]HelpArticle, error) {
	var out []HelpArticle
	path := "/api/help/search?q=" + url.QueryEscape(query)
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListJitGrants(ctx context.Context, orgUUID string) ([]JitGrant, error) {
	var out []JitGrant
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/jit/grants"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) RequestJitGrant(ctx context.Context, orgUUID string, data map[string]any) (*JitGrant, error) {
	var out JitGrant
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/jit/grants"
	if err := c.do(ctx, http.MethodPost, path, data, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ApproveJitGrant(ctx context.Context, orgUUID, grantUUID string) (*JitGrant, error) {
	var out JitGrant
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/jit/grants/" + url.PathEscape(grantUUID) + "/approve"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RevokeJitGrant(ctx context.Context, orgUUID, grantUUID string) error {
	path := "/api/orgs/" + url.PathEscape(orgUUID) + "/jit/grants/" + url.PathEscape(grantUUID) + "/revoke"
	return c.do(ctx, http.MethodPost, path, nil, true, nil)
}

func (c *Client) InviteAccept(ctx context.Context, req InviteAcceptRequest) (*InviteAcceptResponse, error) {
	var out InviteAcceptResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/invite/accept", req, false, &out); err != nil {
		return nil, err
	}
	if out.AccessToken != "" {
		c.AccessToken = out.AccessToken
	}
	return &out, nil
}

func (c *Client) CheckOrgName(ctx context.Context, name string) (*OrgCheckResponse, error) {
	var out OrgCheckResponse
	path := "/api/auth/check-org-name?name=" + url.QueryEscape(name)
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ===== 0.3.0 registration flow =====

// SendOTP sends a one-time password to the given email for the app.
// POST /api/v1/auth/otp/send
// The flow is: SendOTP → VerifyOTP → FinalizeRegistration.
func (c *Client) SendOTP(ctx context.Context, email string, appUUID uuid.UUID) error {
	body := map[string]any{"email": email, "app_uuid": appUUID}
	return c.do(ctx, http.MethodPost, "/api/v1/auth/otp/send", body, false, nil)
}

// VerifyOTP verifies an email OTP and returns a token pair.
// POST /api/v1/auth/otp/verify
// The Token field of the response is the signup_token for FinalizeRegistration.
func (c *Client) VerifyOTP(ctx context.Context, email, otp string, appUUID uuid.UUID) (*TokenPair, error) {
	body := map[string]any{"email": email, "otp": otp, "app_uuid": appUUID}
	var out TokenPair
	if err := c.do(ctx, http.MethodPost, "/api/v1/auth/otp/verify", body, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckOrgNameV2 checks whether an org name is available.
// POST /api/v1/auth/check-org-name
// Returns available, normalized form, and reason if unavailable.
func (c *Client) CheckOrgNameV2(ctx context.Context, name string) (*CheckOrgNameResponse, error) {
	var out CheckOrgNameResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/auth/check-org-name", map[string]any{"name": name}, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FinalizeRegistration completes user registration after OTP verification.
// POST /api/v1/auth/finalize-registration
// req.SignupToken must be the Token from VerifyOTP.
func (c *Client) FinalizeRegistration(ctx context.Context, req FinalizeRegistrationRequest) (*RegistrationResult, error) {
	var out RegistrationResult
	if err := c.do(ctx, http.MethodPost, "/api/v1/auth/finalize-registration", req, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateInvitation creates an org invitation (app-level auth).
// POST /api/v1/organizations/{orgUUID}/invitations
// The plaintext Token in the response is shown once.
func (c *Client) CreateInvitation(ctx context.Context, orgUUID uuid.UUID, req CreateInvitationRequest) (*InvitationResponse, error) {
	var out InvitationResponse
	path := fmt.Sprintf("/api/v1/organizations/%s/invitations", orgUUID)
	if err := c.do(ctx, http.MethodPost, path, req, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PreviewInvitation fetches a public invitation preview by token (no auth).
// GET /api/v1/invitations/{token}/preview
func (c *Client) PreviewInvitation(ctx context.Context, token string) (*InvitationPreview, error) {
	var out InvitationPreview
	path := fmt.Sprintf("/api/v1/invitations/%s/preview", url.PathEscape(token))
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AcceptInvitation accepts an org invitation for an already-authenticated user.
// POST /api/v1/invitations/{token}/accept
// Brand-new users should use FinalizeRegistration with OrgChoice.AcceptInvite instead.
func (c *Client) AcceptInvitation(ctx context.Context, token string) (*AcceptInvitationResponse, error) {
	var out AcceptInvitationResponse
	path := fmt.Sprintf("/api/v1/invitations/%s/accept", url.PathEscape(token))
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListInvitations lists all invitations for an org.
// GET /api/v1/organizations/{orgUUID}/invitations
func (c *Client) ListInvitations(ctx context.Context, orgUUID uuid.UUID) ([]InvitationListItem, error) {
	var out []InvitationListItem
	path := fmt.Sprintf("/api/v1/organizations/%s/invitations", orgUUID)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeInvitation revokes a pending invitation by its integer ID.
// DELETE /api/v1/organizations/{orgUUID}/invitations/{invitationID}
func (c *Client) RevokeInvitation(ctx context.Context, orgUUID uuid.UUID, invitationID int) error {
	path := fmt.Sprintf("/api/v1/organizations/%s/invitations/%d", orgUUID, invitationID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

func (c *Client) GetSuperuserFlag(ctx context.Context, email string) (*SuperuserResponse, error) {
	var out SuperuserResponse
	path := "/api/admin/superuser?email=" + url.QueryEscape(email)
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PostContact(ctx context.Context, req ContactRequest) (*ContactSubmitResponse, error) {
	var out ContactSubmitResponse
	if err := c.do(ctx, http.MethodPost, "/api/contact", req, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PostContactUs(ctx context.Context, req ContactUsRequest) (*ContactSubmitResponse, error) {
	var out ContactSubmitResponse
	if err := c.do(ctx, http.MethodPost, "/api/contact-us", req, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetClientIP(ctx context.Context) (*GeoResponse, error) {
	var out GeoResponse
	if err := c.do(ctx, http.MethodGet, "/api/geo/ip", nil, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Password reset -----

// RequestPasswordReset sends a password-reset email for the given address.
// POST /api/auth/request-password-reset. No auth required.
func (c *Client) RequestPasswordReset(ctx context.Context, email string) (*MessageResponse, error) {
	body := map[string]any{"email": email}
	var out MessageResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/request-password-reset", body, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResetPassword sets a new password using the JWT token from the reset email.
// POST /api/auth/reset-password. No auth required.
func (c *Client) ResetPassword(ctx context.Context, token, password string) (*MessageResponse, error) {
	body := map[string]any{"token": token, "password": password}
	var out MessageResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth/reset-password", body, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ===== App-scoped: OAuth start URL =====

// OAuthStartURL builds the absolute URL of the OAuth start endpoint for
// the given provider, app, and post-callback return target. Issuing a
// GET against this URL yields a 302 to the provider's authorize URL
// with a signed `state` carrying app_uuid + return_to.
//
// The returned URL is safe to send as a redirect target — no secrets
// are included. returnTo must exactly match one of the redirect URIs
// configured on the app's OAuth config for the provider.
func (c *Client) OAuthStartURL(provider OAuthProvider, appUUID string, returnTo string) string {
	q := url.Values{}
	q.Set("app_uuid", appUUID)
	if returnTo != "" {
		q.Set("return_to", returnTo)
	}
	return c.BaseURL + "/api/v1/auth/oauth/" + url.PathEscape(string(provider)) + "/start?" + q.Encode()
}

// ----- Webhooks (v1 API) -----

// ListWebhooks returns all webhook endpoints for the authenticated org.
// GET /api/v1/webhooks
func (c *Client) ListWebhooks(ctx context.Context) (*WebhookListResponse, error) {
	var out WebhookListResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/webhooks", nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateWebhook registers a new webhook endpoint.
// POST /api/v1/webhooks
func (c *Client) CreateWebhook(ctx context.Context, req CreateWebhookRequest) (*WebhookEndpointResponse, error) {
	var out WebhookEndpointResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/webhooks", req, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWebhook removes a webhook endpoint by ID. Returns 204 with no body.
// DELETE /api/v1/webhooks/{id}
func (c *Client) DeleteWebhook(ctx context.Context, id int) error {
	path := fmt.Sprintf("/api/v1/webhooks/%d", id)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ListWebhookDeliveries returns delivery history for a webhook endpoint.
// GET /api/v1/webhooks/{id}/deliveries
func (c *Client) ListWebhookDeliveries(ctx context.Context, webhookID int) ([]WebhookDelivery, error) {
	path := fmt.Sprintf("/api/v1/webhooks/%d/deliveries", webhookID)
	var out []WebhookDelivery
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RetryWebhookDelivery re-queues a permanently-failed delivery.
// POST /api/v1/webhooks/{webhookID}/deliveries/{deliveryID}/retry
func (c *Client) RetryWebhookDelivery(ctx context.Context, webhookID, deliveryID int) (*RetryDeliveryResponse, error) {
	path := fmt.Sprintf("/api/v1/webhooks/%d/deliveries/%d/retry", webhookID, deliveryID)
	var out RetryDeliveryResponse
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ===== App-scoped: OAuth config admin =====

// ListOAuthConfigs returns the per-provider OAuth configs registered
// for the app. Client secrets are never returned.
//
// GET /api/v1/apps/{app_uuid}/oauth-configs
func (c *Client) ListOAuthConfigs(ctx context.Context, appUUID string) ([]OAuthConfigSummary, error) {
	var out []OAuthConfigSummary
	path := "/api/v1/apps/" + url.PathEscape(appUUID) + "/oauth-configs"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateOAuthConfig registers an OAuth provider config for the app.
//
// POST /api/v1/apps/{app_uuid}/oauth-configs
func (c *Client) CreateOAuthConfig(ctx context.Context, appUUID string, input CreateOAuthConfigInput) (*OAuthConfigSummary, error) {
	var out OAuthConfigSummary
	path := "/api/v1/apps/" + url.PathEscape(appUUID) + "/oauth-configs"
	if err := c.do(ctx, http.MethodPost, path, input, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateOAuthConfig partially updates an existing OAuth provider
// config. Only the non-nil fields in patch are sent on the wire.
//
// PATCH /api/v1/apps/{app_uuid}/oauth-configs/{provider}
func (c *Client) UpdateOAuthConfig(ctx context.Context, appUUID, provider string, patch UpdateOAuthConfigInput) (*OAuthConfigSummary, error) {
	var out OAuthConfigSummary
	path := "/api/v1/apps/" + url.PathEscape(appUUID) + "/oauth-configs/" + url.PathEscape(provider)
	if err := c.do(ctx, http.MethodPatch, path, patch, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteOAuthConfig removes the OAuth provider config for the app.
//
// DELETE /api/v1/apps/{app_uuid}/oauth-configs/{provider}
func (c *Client) DeleteOAuthConfig(ctx context.Context, appUUID, provider string) error {
	path := "/api/v1/apps/" + url.PathEscape(appUUID) + "/oauth-configs/" + url.PathEscape(provider)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ----- OAuth refresh -----

// RefreshOAuthConnection refreshes the stored access token using the saved refresh token.
// POST /v1/oauth/connections/{provider}/refresh
func (c *Client) RefreshOAuthConnection(ctx context.Context, provider string) (*OAuthRefreshResponse, error) {
	path := "/v1/oauth/connections/" + url.PathEscape(provider) + "/refresh"
	var out OAuthRefreshResponse
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Email -----

// SendEmail sends a transactional email via the org's configured email provider.
// POST /api/email/send
func (c *Client) SendEmail(ctx context.Context, req SendEmailRequest) (*SendEmailResponse, error) {
	var out SendEmailResponse
	if err := c.do(ctx, http.MethodPost, "/api/email/send", req, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ===== App-scoped: WebAuthn relying-party config admin =====

// GetAppRPConfig fetches the per-app WebAuthn relying-party config.
//
// GET /api/v1/apps/{app_uuid}/rp-config
//
// A nil RPID on the returned struct means the app has no override and
// falls back to the deployment-wide BUTTRBASE_WEBAUTHN_RP_ID env var.
func (c *Client) GetAppRPConfig(ctx context.Context, appUUID string) (*AppRpConfig, error) {
	var out AppRpConfig
	path := "/api/v1/apps/" + url.PathEscape(appUUID) + "/rp-config"
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateAppRPConfig partially updates the app's WebAuthn relying-party
// config. Only the non-nil fields in patch are sent on the wire.
//
// PATCH /api/v1/apps/{app_uuid}/rp-config
//
// Note: a nil RPID in the response means the app falls back to the
// env-var RP id. Clearing rp_id back to nil cannot be expressed through
// UpdateAppRpConfigInput (omitempty drops nil pointers); callers needing
// that must send a raw `{"rp_id": null}` body — known limitation.
func (c *Client) UpdateAppRPConfig(ctx context.Context, appUUID string, patch UpdateAppRpConfigInput) (*AppRpConfig, error) {
	var out AppRpConfig
	path := "/api/v1/apps/" + url.PathEscape(appUUID) + "/rp-config"
	if err := c.do(ctx, http.MethodPatch, path, patch, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ===== App-scoped: audit log =====

// ReadAuditLog returns rows from the per-app security audit log,
// newest first. The backend defaults limit to 200 and caps it at 1000.
// ActionPrefix narrows by event family (e.g. "oauth_config.", "credential.").
//
// GET /api/v1/apps/{app_uuid}/audit-log
func (c *Client) ReadAuditLog(ctx context.Context, appUUID string, opts AuditLogQuery) ([]AuditRow, error) {
	q := url.Values{}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.ActionPrefix != "" {
		q.Set("action_prefix", opts.ActionPrefix)
	}
	path := "/api/v1/apps/" + url.PathEscape(appUUID) + "/audit-log"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out []AuditRow
	if err := c.do(ctx, http.MethodGet, path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ===== Passkeys (WebAuthn) =====
//
// Thin HTTP wrappers around the four passkey ceremony endpoints. The
// WebAuthn challenge / credential blobs are pass-through json.RawMessage
// — the browser's navigator.credentials.create / .get APIs do the heavy
// lifting. Begin endpoints unwrap the backend's {"data": ...} envelope
// for ergonomics.

// passkeyDataEnvelope is the {"data": ...} shape the backend returns for
// passkey endpoints. Kept private; we unwrap it for callers.
type passkeyDataEnvelope[T any] struct {
	Data T `json:"data"`
}

// PasskeyRegisterBegin starts passkey registration. Requires an
// authenticated caller (passkey is added to the user's existing account).
// Pass the returned Challenge to navigator.credentials.create in the
// browser.
//
// POST /api/passkeys/register/begin
func (c *Client) PasskeyRegisterBegin(ctx context.Context) (*PasskeyRegistrationChallenge, error) {
	var env passkeyDataEnvelope[PasskeyRegistrationChallenge]
	if err := c.do(ctx, http.MethodPost, "/api/passkeys/register/begin", nil, true, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// PasskeyRegisterComplete finishes passkey registration. body.Credential
// is the WebAuthn RegisterPublicKeyCredential returned by the browser.
//
// POST /api/passkeys/register/complete
func (c *Client) PasskeyRegisterComplete(ctx context.Context, body PasskeyRegistrationComplete) (*PasskeyRegistrationResult, error) {
	var env passkeyDataEnvelope[PasskeyRegistrationResult]
	if err := c.do(ctx, http.MethodPost, "/api/passkeys/register/complete", body, true, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// PasskeyAuthenticateBegin starts passkey authentication. Anonymous; no
// Authorization header is sent. Pass the returned Challenge to
// navigator.credentials.get in the browser.
//
// POST /api/passkeys/authenticate/begin
func (c *Client) PasskeyAuthenticateBegin(ctx context.Context) (*PasskeyAuthChallenge, error) {
	var env passkeyDataEnvelope[PasskeyAuthChallenge]
	if err := c.do(ctx, http.MethodPost, "/api/passkeys/authenticate/begin", nil, false, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// PasskeyAuthenticateComplete finishes passkey authentication. The
// session payload shape is currently unstable on the backend, so we
// return raw JSON — callers should narrow at the call site.
//
// POST /api/passkeys/authenticate/complete
func (c *Client) PasskeyAuthenticateComplete(ctx context.Context, body PasskeyAuthComplete) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/api/passkeys/authenticate/complete", body, false, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListMyPasskeys returns the signed-in user's enrolled passkeys, in
// descending CreatedAt order. Each row carries a CredentialUUID (for
// revocation) and a 12-char CredentialIDPrefix for display.
//
// Requires a bearer token.
//
// GET /api/v1/me/passkeys
func (c *Client) ListMyPasskeys(ctx context.Context) ([]PasskeyListItem, error) {
	var out []PasskeyListItem
	if err := c.do(ctx, http.MethodGet, "/api/v1/me/passkeys", nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteMyPasskey revokes one of the signed-in user's enrolled passkeys
// by its credential UUID. The owner check is enforced on the backend;
// UUIDs owned by another user return 404.
//
// DELETE /api/v1/me/passkeys/{credential_uuid}
func (c *Client) DeleteMyPasskey(ctx context.Context, credentialUUID string) error {
	path := "/api/v1/me/passkeys/" + url.PathEscape(credentialUUID)
	return c.do(ctx, http.MethodDelete, path, nil, true, nil)
}

// ----- Scope context (windowed / JIT scope re-mint) -----

// ScopeContext re-mints the caller's access token windowed to an explicit,
// gate-checked scope subset (least-privilege "windowed" strategy). The caller
// must already hold a valid access token; the granted set is always a subset of
// the caller's effective scopes. Requires a bearer token.
//
// POST /api/app/auth/scope-context
func (c *Client) ScopeContext(ctx context.Context, req ScopeContextRequest) (*ScopeContextResponse, error) {
	var out ScopeContextResponse
	if err := c.do(ctx, http.MethodPost, "/api/app/auth/scope-context", req, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ----- Devices (end-user self-service) -----

// ListDevices returns the caller's active (non-revoked) device keys, descending
// by CreatedAt. Only public-safe fields are returned. Requires a bearer token.
//
// GET /api/app/devices
func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	var out deviceList
	if err := c.do(ctx, http.MethodGet, "/api/app/devices", nil, true, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// RevokeDevice soft-revokes a device the caller owns, by its device UUID. The
// ownership check is enforced server-side; a device owned by another user (or
// that does not exist / is already revoked) returns 404. Requires a bearer
// token.
//
// POST /api/app/devices/{device_uuid}/revoke
func (c *Client) RevokeDevice(ctx context.Context, deviceUUID string) (*RevokeDeviceResponse, error) {
	var out revokeDeviceEnvelope
	path := "/api/app/devices/" + url.PathEscape(deviceUUID) + "/revoke"
	if err := c.do(ctx, http.MethodPost, path, nil, true, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// ----- Tenant home (public discovery) -----

// GetTenantHome resolves an active tenant's public home (routing info) for the
// given org and app, so a client can target it directly. appID is optional;
// pass nil to omit the app_id query parameter. Returns a 404 ButtrbaseError if
// no active tenant home exists for the org/app. Public — no bearer token.
//
// GET /api/tenant/home?org_uuid=&app_id=
func (c *Client) GetTenantHome(ctx context.Context, orgUUID string, appID *int) (*TenantHome, error) {
	q := url.Values{}
	q.Set("org_uuid", orgUUID)
	if appID != nil {
		q.Set("app_id", strconv.Itoa(*appID))
	}
	path := "/api/tenant/home?" + q.Encode()
	var out tenantHomeEnvelope
	if err := c.do(ctx, http.MethodGet, path, nil, false, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// ensure strconv stays used (helper for callers building queries).
var _ = strconv.Itoa
