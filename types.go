package buttrbase

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ValidateCouponOptions holds optional parameters for ValidateCoupon.
type ValidateCouponOptions struct {
	UserID          *int   `json:"user_id,omitempty"`
	OrderTotalCents *int64 `json:"order_total_cents,omitempty"`
}

// CouponValidation is the response from ValidateCoupon.
type CouponValidation struct {
	Valid         bool           `json:"valid"`
	Code          string         `json:"code,omitempty"`
	DiscountCents int64          `json:"discount_cents,omitempty"`
	DiscountType  string         `json:"discount_type,omitempty"`
	Reason        string         `json:"reason,omitempty"`
	Raw           map[string]any `json:"-"`
}

// GiftCardValidation is the response from ValidateGiftCard.
type GiftCardValidation struct {
	Valid        bool           `json:"valid"`
	Code         string         `json:"code,omitempty"`
	BalanceCents int64          `json:"balance_cents,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	Raw          map[string]any `json:"-"`
}

// GiftCardRedemption is the response from RedeemGiftCard.
type GiftCardRedemption struct {
	Success        bool           `json:"success"`
	Code           string         `json:"code,omitempty"`
	RedeemedCents  int64          `json:"redeemed_cents,omitempty"`
	RemainingCents int64          `json:"remaining_cents,omitempty"`
	Raw            map[string]any `json:"-"`
}

// SendMagicLinkOptions holds optional parameters for SendMagicLink.
type SendMagicLinkOptions struct {
	RedirectURL string `json:"redirect_url,omitempty"`
	TTLSeconds  *int64 `json:"ttl_seconds,omitempty"`
}

// MagicLinkSend is the response from SendMagicLink.
type MagicLinkSend struct {
	Sent  bool           `json:"sent"`
	Email string         `json:"email,omitempty"`
	Raw   map[string]any `json:"-"`
}

// MagicLinkVerify is the response from VerifyMagicLink.
type MagicLinkVerify struct {
	Valid  bool           `json:"valid"`
	Email  string         `json:"email,omitempty"`
	UserID *int           `json:"user_id,omitempty"`
	Raw    map[string]any `json:"-"`
}

// MfaStatus is the response from MfaStatus.
type MfaStatus struct {
	Enrolled bool           `json:"enrolled"`
	Active   bool           `json:"active"`
	Label    string         `json:"label,omitempty"`
	Raw      map[string]any `json:"-"`
}

// MfaEnrollment is the response from MfaEnroll.
type MfaEnrollment struct {
	Secret     string         `json:"secret,omitempty"`
	OtpauthURL string         `json:"otpauth_url,omitempty"`
	Label      string         `json:"label,omitempty"`
	Raw        map[string]any `json:"-"`
}

// MfaStatusResponse is the response from MfaActivate.
type MfaStatusResponse struct {
	Active bool           `json:"active"`
	Label  string         `json:"label,omitempty"`
	Raw    map[string]any `json:"-"`
}

// OrgSignResponse is the response from OrgSign.
type OrgSignResponse struct {
	Token string         `json:"token"`
	Kid   string         `json:"kid,omitempty"`
	Raw   map[string]any `json:"-"`
}

// JWKSResponse is the response from OrgJWKS.
type JWKSResponse struct {
	Keys []map[string]any `json:"keys"`
	Raw  map[string]any   `json:"-"`
}

// SecretGet is the response from GetSecret.
type SecretGet struct {
	Name        string         `json:"name"`
	Value       string         `json:"value"`
	Description string         `json:"description,omitempty"`
	Raw         map[string]any `json:"-"`
}

// SecretSummary is the response from PutSecret.
type SecretSummary struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Raw         map[string]any `json:"-"`
}

// ----- Zero-trust endpoints -----

// StepUpResponse is the response from AuthStepUp.
type StepUpResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

// ElevationRequestOptions holds optional parameters for ElevationRequest.
type ElevationRequestOptions struct {
	Reason     string
	TTLSeconds *int64
}

// ElevationGrant is the grant view returned by elevation endpoints.
type ElevationGrant struct {
	GrantUUID     string `json:"grant_uuid"`
	OrgUUID       string `json:"org_uuid"`
	RequesterUUID string `json:"requester_uuid"`
	ApproverUUID  string `json:"approver_uuid,omitempty"`
	Scope         string `json:"scope"`
	Reason        string `json:"reason,omitempty"`
	Status        string `json:"status"`
	TTLSeconds    int64  `json:"ttl_seconds,omitempty"`
	CreatedAt     string `json:"created_at"`
	ApprovedAt    string `json:"approved_at,omitempty"`
	ExpiresAt     string `json:"expires_at,omitempty"`
}

// SpiffeSvidResponse is the response from SpiffeIssueSvid.
type SpiffeSvidResponse struct {
	SpiffeID      string `json:"spiffe_id"`
	SvidPEM       string `json:"svid_pem"`
	PrivateKeyPEM string `json:"private_key_pem"`
	IssuedAt      string `json:"issued_at"`
	ExpiresAt     string `json:"expires_at"`
}

// AuthEvent is one entry in the context-aware auth event log.
type AuthEvent struct {
	EventUUID  string  `json:"event_uuid,omitempty"`
	OrgUUID    string  `json:"org_uuid,omitempty"`
	UserUUID   string  `json:"user_uuid,omitempty"`
	Kind       string  `json:"kind"`
	IP         string  `json:"ip,omitempty"`
	UserAgent  string  `json:"user_agent,omitempty"`
	RiskScore  float64 `json:"risk_score,omitempty"`
	OccurredAt string  `json:"occurred_at"`
}

// ListAuthEventsOptions holds optional parameters for ListAuthEvents.
type ListAuthEventsOptions struct {
	UserUUID string
	Limit    int
}

// ReencryptResponse is the response from the reencrypt admin endpoints.
type ReencryptResponse struct {
	Rotated  int64  `json:"rotated"`
	Failed   int64  `json:"failed,omitempty"`
	NewKEKID string `json:"new_kek_id,omitempty"`
}

// RevokeSessionResponse is the response from RevokeSession.
type RevokeSessionResponse struct {
	JTI       string `json:"jti"`
	Revoked   bool   `json:"revoked"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// OrgMetrics is the response from GetOrgMetrics.
type OrgMetrics struct {
	ActiveUsers       int64          `json:"active_users,omitempty"`
	ActiveSessions    int64          `json:"active_sessions,omitempty"`
	PendingElevations int64          `json:"pending_elevations,omitempty"`
	SecretsCount      int64          `json:"secrets_count,omitempty"`
	SigningKeysCount  int64          `json:"signing_keys_count,omitempty"`
	Raw               map[string]any `json:"-"`
}

// ----- Credentials -----

// Credential represents an API credential (client ID / secret pair).
type Credential struct {
	CredentialsID string `json:"credentials_id,omitempty"`
	ClientID      string `json:"client_id,omitempty"`
	ClientSecret  string `json:"client_secret,omitempty"`
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
}

// CredentialList is the response from ListCredentials.
type CredentialList struct {
	Data []Credential `json:"data"`
}

// CreateCredentialRequest is the request body for CreateCredential.
type CreateCredentialRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// RotateSecretResponse is the response from RotateCredentialSecret.
type RotateSecretResponse struct {
	CredentialsID string `json:"credentials_id,omitempty"`
	ClientID      string `json:"client_id,omitempty"`
	ClientSecret  string `json:"client_secret,omitempty"`
}

// ----- Sandbox -----

// SandboxResetRequest is the optional request body for ResetSandbox.
type SandboxResetRequest struct {
	OrgUUID string `json:"org_uuid,omitempty"`
}

// SandboxResetResponse is the response from ResetSandbox.
type SandboxResetResponse struct {
	Reset   bool           `json:"reset"`
	Message string         `json:"message,omitempty"`
	Raw     map[string]any `json:"-"`
}

// ----- Scope context (windowed / JIT scope re-mint) -----

// ScopeContextRequest is the body for ScopeContext
// (POST /api/app/auth/scope-context). RequestedScopes is the explicit scope
// list the caller wants windowed into a fresh access token; the granted set is
// always a subset of the caller's effective scopes and each scope is run
// through the scope-gate (step-up) machinery server-side.
type ScopeContextRequest struct {
	RequestedScopes []string `json:"requested_scopes"`
}

// ScopeContextResponse is the response from ScopeContext. Token is the freshly
// minted, windowed access token; Scopes is the deduplicated, sorted set of
// scopes actually granted (and embedded as the token's data.scopes claim).
//
// Note: the backend re-mints only the access token — the refresh token is
// unchanged — and the response carries no separate expiry field; the access
// token's lifetime is encoded in its exp claim.
type ScopeContextResponse struct {
	Token  string   `json:"token"`
	Scopes []string `json:"scopes"`
}

// ----- Devices (end-user self-service) -----

// Device is a single registered device key, public-safe (no private key
// material beyond the public JWK thumbprint). Mirrors the backend DeviceItem.
type Device struct {
	DeviceUUID string     `json:"device_uuid"`
	JKT        string     `json:"jkt"`
	Label      *string    `json:"label"`
	CreatedAt  time.Time  `json:"created_at"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

// deviceList wraps the {"data": [...]} envelope returned by GET /api/app/devices.
type deviceList struct {
	Data []Device `json:"data"`
}

// RevokeDeviceResponse is the inner data of the response from RevokeDevice
// (POST /api/app/devices/{device_uuid}/revoke).
type RevokeDeviceResponse struct {
	DeviceUUID string `json:"device_uuid"`
	Revoked    bool   `json:"revoked"`
}

// revokeDeviceEnvelope wraps the {"data": {...}} envelope from RevokeDevice.
type revokeDeviceEnvelope struct {
	Data RevokeDeviceResponse `json:"data"`
}

// ----- Tenant home (public discovery) -----

// TenantHome is the public routing info for an active tenant, returned by
// GetTenantHome (GET /api/tenant/home). HomeRegion and HomeBaseURL are nullable.
type TenantHome struct {
	TenancyMode string  `json:"tenancy_mode"`
	HomeRegion  *string `json:"home_region"`
	HomeBaseURL *string `json:"home_base_url"`
}

// tenantHomeEnvelope wraps the {"data": {...}} envelope from GET /api/tenant/home.
type tenantHomeEnvelope struct {
	Data TenantHome `json:"data"`
}

// ----- Auth / Profile -----

// RegisterOptions holds optional parameters for Register.
type RegisterOptions struct {
	FirstName string
	LastName  string
}

// LoginResponse is the response from Login.
type LoginResponse struct {
	AccessToken string         `json:"access_token,omitempty"`
	User        map[string]any `json:"user,omitempty"`
}

// Profile is the response from GetProfile.
type Profile struct {
	ID        int    `json:"id"`
	UserUUID  string `json:"user_uuid"`
	Email     string `json:"email"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	OrgUUID   string `json:"org_uuid,omitempty"`
}

// User represents a user record.
type User struct {
	ID       int            `json:"id"`
	UserUUID string         `json:"user_uuid"`
	Email    string         `json:"email"`
	Status   string         `json:"status,omitempty"`
	Role     string         `json:"role,omitempty"`
	Raw      map[string]any `json:"-"`
}

// ----- Teams -----

// Team is the response from team endpoints.
type Team struct {
	ID       int    `json:"id"`
	TeamUUID string `json:"team_uuid"`
	OrgUUID  string `json:"org_uuid"`
	Name     string `json:"name"`
}

// ----- Billing -----

// Invoice is the response from billing endpoints.
type Invoice struct {
	ID            int    `json:"id"`
	Provider      string `json:"provider"`
	Amount        int    `json:"amount"`
	Status        string `json:"status"`
	InvoicePdfURL string `json:"invoice_pdf_url,omitempty"`
	CreatedAt     string `json:"created_at"`
}

// CheckoutResponse is the response from billing checkout.
type CheckoutResponse struct {
	URL       string         `json:"url,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Raw       map[string]any `json:"-"`
}

// ----- Sessions -----

// SessionInfo represents a session.
type SessionInfo struct {
	SessionID  string `json:"session_id"`
	UserUUID   string `json:"user_uuid"`
	DeviceUUID string `json:"device_uuid,omitempty"`
	IP         string `json:"ip,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ----- Signing Keys -----

// SigningKey is the response from signing key endpoints.
type SigningKey struct {
	KeyID     string `json:"key_id"`
	Algorithm string `json:"algorithm"`
	CreatedAt string `json:"created_at"`
	Status    string `json:"status"`
}

// SigningAuditEntryItem is the response from signing audit endpoints.
type SigningAuditEntryItem struct {
	ID        int64  `json:"id"`
	KeyID     string `json:"key_id"`
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
}

// ----- mTLS CA -----

// CertificateAuthority is the response from CA endpoints.
type CertificateAuthority struct {
	OrgUUID   string `json:"org_uuid"`
	CaPEM     string `json:"ca_pem"`
	CreatedAt string `json:"created_at"`
}

// Certificate is the response from certificate endpoints.
type Certificate struct {
	Serial    string `json:"serial"`
	Subject   string `json:"subject"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
	Status    string `json:"status"`
}

// ----- Domains -----

// Domain is the response from domain endpoints.
type Domain struct {
	ID                int    `json:"id"`
	DomainName        string `json:"domain"`
	Verified          bool   `json:"verified"`
	VerificationToken string `json:"verification_token,omitempty"`
}

// ----- Webhooks Admin -----

// WebhookEndpoint represents a registered webhook endpoint.
type WebhookEndpoint struct {
	ID            int      `json:"id,omitempty"`
	URL           string   `json:"url"`
	Events        []string `json:"events,omitempty"`
	EventTypes    []string `json:"event_types,omitempty"`
	IsActive      bool     `json:"is_active,omitempty"`
	Description   string   `json:"description,omitempty"`
	SecretPresent bool     `json:"secret_present,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
}

// WebhookDelivery represents a single delivery attempt for a webhook endpoint.
type WebhookDelivery struct {
	ID           int64  `json:"id"`
	EndpointID   int    `json:"endpoint_id"`
	Event        string `json:"event,omitempty"`
	EventType    string `json:"event_type,omitempty"`
	Status       string `json:"status"`
	HTTPStatus   *int   `json:"http_status,omitempty"`
	ResponseBody string `json:"response_body,omitempty"`
	AttemptCount int    `json:"attempt_count,omitempty"`
	AttemptedAt  string `json:"attempted_at,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	DeliveredAt  string `json:"delivered_at,omitempty"`
}

// ----- Payments -----

// PaymentCheckoutSession is the response from payment checkout.
type PaymentCheckoutSession struct {
	Provider          string `json:"provider"`
	ProviderPublicKey string `json:"provider_public_key"`
	ClientSecret      string `json:"client_secret"`
	SessionID         string `json:"session_id"`
}

// ----- Admin Portal -----

// AdminPortalToken is the response from admin portal issue.
type AdminPortalToken struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// ----- Audit Events -----

// AuditEventEntry is one entry in the audit event log.
type AuditEventEntry struct {
	ID        int64  `json:"id"`
	OrgUUID   string `json:"org_uuid"`
	Actor     string `json:"actor,omitempty"`
	Action    string `json:"action"`
	Resource  string `json:"resource,omitempty"`
	Timestamp string `json:"timestamp"`
}

// ----- SSO Connections -----

// SsoConnection represents an SSO connection.
type SsoConnection struct {
	ID             int            `json:"id,omitempty"`
	ConnectionUUID string         `json:"connection_uuid"`
	OrgUUID        string         `json:"org_uuid"`
	Provider       string         `json:"provider"`
	Name           string         `json:"name"`
	Config         map[string]any `json:"config,omitempty"`
}

// ----- User Accounts -----

// UserAccount is a device account.
type UserAccount struct {
	ID          int    `json:"id"`
	AccountUUID string `json:"account_uuid"`
	DeviceUUID  string `json:"device_uuid"`
	Email       string `json:"email"`
	OrgName     string `json:"org_name"`
	OrgUUID     string `json:"org_uuid"`
	UserUUID    string `json:"user_uuid,omitempty"`
}

// ----- Coupons Admin -----

// Coupon represents a coupon for admin CRUD.
type Coupon struct {
	ID            int      `json:"id,omitempty"`
	Code          string   `json:"code"`
	ProductID     int      `json:"product_id,omitempty"`
	DiscountType  string   `json:"discount_type"`
	DiscountValue float64  `json:"discount_value"`
	Active        bool     `json:"active,omitempty"`
	Labels        []string `json:"labels,omitempty"`
}

// ----- Org Features -----

// OrgFeature represents an org feature flag.
type OrgFeature struct {
	FeatureID string `json:"feature_id"`
	Enabled   bool   `json:"enabled"`
}

// ----- Permissions -----

// Permission represents a permission.
type Permission struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ----- Roles -----

// Role represents a role.
type Role struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ProductID int    `json:"product_id,omitempty"`
}

// ----- Invoice Send -----

// SendInvoiceResponse is the response from SendInvoice.
type SendInvoiceResponse struct {
	InvoiceUUID string `json:"invoice_uuid"`
	PaymentURL  string `json:"payment_url"`
}

// ----- API Keys v2 -----

// ApiKey represents an API key (v2).
type ApiKey struct {
	KeyUUID   string `json:"key_uuid"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix,omitempty"`
	Key       string `json:"key,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ----- Entitlements -----

// EntitlementCheckResponse is the response from EntitlementsCheck.
type EntitlementCheckResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// ----- Secrets (admin list) -----

// SecretEntry is one entry from listing secrets (no value).
type SecretEntry struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// ----- Help Center -----

// HelpCategory represents a help center category.
type HelpCategory struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Articles []any  `json:"articles,omitempty"`
}

// HelpArticle represents a help center article.
type HelpArticle struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
}

// ----- JIT Grants -----

// JitGrant is a JIT elevation grant.
type JitGrant struct {
	GrantUUID     string `json:"grant_uuid"`
	OrgUUID       string `json:"org_uuid"`
	RequesterUUID string `json:"requester_uuid,omitempty"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

// ----- Recovery Codes -----

// RecoveryCodesResponse is the response from MfaGenerateRecoveryCodes.
type RecoveryCodesResponse struct {
	Codes []string `json:"codes"`
}

// ----- Invite-based registration -----

// InviteAcceptRequest is the request body for InviteAccept.
type InviteAcceptRequest struct {
	Token     string `json:"token"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Phone     string `json:"phone,omitempty"`
}

// InviteAcceptResponse is the response from InviteAccept.
type InviteAcceptResponse struct {
	UserUUID     string `json:"user_uuid"`
	OrgUUID      string `json:"org_uuid"`
	Role         string `json:"role"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Message      string `json:"message"`
}

// OrgCheckResponse is the response from CheckOrgName.
type OrgCheckResponse struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

// SuperuserResponse is the response from GetSuperuserFlag.
type SuperuserResponse struct {
	Email       string `json:"email"`
	IsSuperuser bool   `json:"is_superuser"`
}

// ----- Contact forms -----

// ContactRequest is the request body for PostContact.
type ContactRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Company string `json:"company,omitempty"`
	Message string `json:"message"`
	AppID   string `json:"app_id,omitempty"`
}

// ContactUsRequest is the request body for PostContactUs.
type ContactUsRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

// ContactSubmitResponse is the response from PostContact and PostContactUs.
type ContactSubmitResponse struct {
	Message     string `json:"message"`
	ReferenceID string `json:"reference_id"`
}

// ----- Geo / IP -----

// GeoResponse is the response from GetClientIP.
type GeoResponse struct {
	IP       string `json:"ip"`
	Country  string `json:"country"`
	Timezone string `json:"timezone"`
}

// ----- App-scoped surface (app_uuid era) -----

// OAuthProvider names the supported per-app OAuth identity providers.
// The backend currently fully implements `google` and `microsoft`;
// `github` and `apple` are accepted at the config layer.
type OAuthProvider string

const (
	OAuthProviderGoogle    OAuthProvider = "google"
	OAuthProviderMicrosoft OAuthProvider = "microsoft"
	OAuthProviderGithub    OAuthProvider = "github"
	OAuthProviderApple     OAuthProvider = "apple"
)

// APIKeyType is the lifecycle category of an app-level API key.
type APIKeyType string

const (
	// APIKeyTypeShortLived is the long-lived key that callers exchange for a JWT pair.
	APIKeyTypeShortLived APIKeyType = "short_lived"
	// APIKeyTypePermanent is a non-expiring key used directly as a bearer credential.
	APIKeyTypePermanent APIKeyType = "permanent"
	// APIKeyTypeExpiring is a key with a fixed expiration; rotation preserves the original expiry.
	APIKeyTypeExpiring APIKeyType = "expiring"
)

// APIKeyEnv selects the environment a key is bound to.
type APIKeyEnv string

const (
	APIKeyEnvLive APIKeyEnv = "live"
	APIKeyEnvTest APIKeyEnv = "test"
)

// ExchangeResponse is the response from POST /api/v1/auth/api-key/exchange.
//
// Returned by both the initial exchange (raw API key in) and the refresh
// exchange (opaque refresh token in). The presented refresh token, if any,
// is revoked as a side effect.
type ExchangeResponse struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	TokenType        string    `json:"token_type"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

// APIKeySummary is the metadata view of an app-level API key; the raw key
// material is never returned by list/get endpoints.
type APIKeySummary struct {
	KeyUUID    string     `json:"key_uuid"`
	AppUUID    string     `json:"app_uuid"`
	KeyPrefix  string     `json:"key_prefix"`
	Name       string     `json:"name"`
	KeyType    APIKeyType `json:"key_type"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreatedKeyResponse is returned by CreateAppAPIKey and RotateAppAPIKey.
// RawKey is shown exactly once — the caller must persist it immediately,
// because the backend stores only sha256(raw_key).
type CreatedKeyResponse struct {
	KeyUUID   string     `json:"key_uuid"`
	RawKey    string     `json:"raw_key"`
	KeyPrefix string     `json:"key_prefix"`
	KeyType   APIKeyType `json:"key_type"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// CreateAPIKeyInput is the request body for CreateAppAPIKey.
//
// When KeyType is APIKeyTypeExpiring, Expiry is required; otherwise it
// must be nil.
type CreateAPIKeyInput struct {
	Name    string       `json:"name"`
	Env     APIKeyEnv    `json:"env"`
	KeyType APIKeyType   `json:"key_type"`
	Expiry  *ExpiryInput `json:"expiry,omitempty"`
}

// ExpiryInput is a tagged union — exactly one of Absolute or InDays must
// be non-nil for an expiring key.
type ExpiryInput struct {
	Absolute *time.Time `json:"absolute,omitempty"`
	InDays   *int       `json:"in_days,omitempty"`
}

// OAuthConfigSummary is the per-app per-provider OAuth configuration.
// Secrets are never returned — only the metadata required to mint
// authorize URLs and validate callbacks.
type OAuthConfigSummary struct {
	Provider     OAuthProvider `json:"provider"`
	ClientID     string        `json:"client_id"`
	RedirectURIs []string      `json:"redirect_uris"`
	Scopes       []string      `json:"scopes"`
	Enabled      bool          `json:"enabled"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// CreateOAuthConfigInput is the request body for CreateOAuthConfig.
//
// ProviderExtras carries provider-specific extras as raw JSON. Required for
// Apple sign-in (shape: {"team_id": ..., "key_id": ..., "private_key": <PEM>});
// the backend strips the private_key field and re-stores it as
// private_key_encrypted under the app's DEK. Nil / empty map for providers
// that don't need extras (Google, Microsoft, GitHub).
type CreateOAuthConfigInput struct {
	Provider       OAuthProvider  `json:"provider"`
	ClientID       string         `json:"client_id"`
	ClientSecret   string         `json:"client_secret"`
	RedirectURIs   []string       `json:"redirect_uris"`
	Scopes         []string       `json:"scopes"`
	Enabled        bool           `json:"enabled"`
	ProviderExtras map[string]any `json:"provider_extras,omitempty"`
}

// UpdateOAuthConfigInput is the PATCH body for UpdateOAuthConfig. Only
// non-nil fields are sent — nil fields preserve the existing value.
// ProviderExtras replaces the stored JSON blob entirely when non-nil;
// for Apple a fresh private_key triggers re-encryption under the app's
// DEK and rotates the stored ciphertext.
type UpdateOAuthConfigInput struct {
	ClientID       *string        `json:"client_id,omitempty"`
	ClientSecret   *string        `json:"client_secret,omitempty"`
	RedirectURIs   *[]string      `json:"redirect_uris,omitempty"`
	Scopes         *[]string      `json:"scopes,omitempty"`
	Enabled        *bool          `json:"enabled,omitempty"`
	ProviderExtras map[string]any `json:"provider_extras,omitempty"`
}

// AppRpConfig is the per-app WebAuthn relying-party configuration.
// RPID is a pointer because nil signals that the app inherits the
// deployment-wide BUTTRBASE_WEBAUTHN_RP_ID env var rather than pinning
// its own value. RPOrigins lists every full origin (scheme + host +
// optional port) permitted to drive passkey ceremonies for this RP.
type AppRpConfig struct {
	AppUUID   string   `json:"app_uuid"`
	RPID      *string  `json:"rp_id"`
	RPOrigins []string `json:"rp_origins"`
}

// UpdateAppRpConfigInput is the PATCH body for UpdateAppRPConfig. Only
// non-nil fields are sent — nil fields preserve the existing value.
// Known limitation: there is no way to clear rp_id back to nil through
// this struct (omitempty drops nil pointers); callers that need to
// reset to the env-var fallback must issue the PATCH with a raw JSON
// body of `{"rp_id": null}`.
type UpdateAppRpConfigInput struct {
	RPID      *string   `json:"rp_id,omitempty"`
	RPOrigins *[]string `json:"rp_origins,omitempty"`
}

// AuditLogQuery holds optional filters for ReadAuditLog. The backend
// caps Limit at 1000 (defaults to 200 when zero). ActionPrefix matches
// `action LIKE 'prefix%'` — e.g. "api_key." or "oauth_config.".
type AuditLogQuery struct {
	Limit        int    `json:"-"`
	ActionPrefix string `json:"-"`
}

// AuditRow is one entry in the per-app security audit log.
type AuditRow struct {
	ID            int64                  `json:"id"`
	AppUUID       string                 `json:"app_uuid"`
	ActorUserUUID *string                `json:"actor_user_uuid"`
	Action        string                 `json:"action"`
	TargetID      *string                `json:"target_id"`
	Details       map[string]interface{} `json:"details"`
	IP            *string                `json:"ip"`
	UserAgent     *string                `json:"user_agent"`
	CreatedAt     time.Time              `json:"created_at"`
}

// ----- Passkeys (WebAuthn) -----
//
// The backend exposes the WebAuthn ceremonies as two-phase begin/complete
// endpoints. The challenge / credential blobs are pass-through
// json.RawMessage — we don't pull in a webauthn helper library; the
// browser's navigator.credentials.create / .get APIs consume and produce
// these JSON shapes directly.

// PasskeyRegistrationChallenge is the response from
// POST /api/passkeys/register/begin. Challenge is a WebAuthn
// CreationChallengeResponse; pass it to navigator.credentials.create in
// the browser. RegistrationState is an opaque server-signed blob the
// client must echo back unchanged on the matching complete call.
type PasskeyRegistrationChallenge struct {
	Challenge         json.RawMessage `json:"challenge"`
	RegistrationState string          `json:"registration_state"`
}

// PasskeyRegistrationComplete is the body for
// POST /api/passkeys/register/complete. Credential is the WebAuthn
// RegisterPublicKeyCredential returned by the browser.
type PasskeyRegistrationComplete struct {
	RegistrationState string          `json:"registration_state"`
	Credential        json.RawMessage `json:"credential"`
}

// PasskeyRegistrationResult is the response from
// POST /api/passkeys/register/complete.
type PasskeyRegistrationResult struct {
	CredentialID string `json:"credential_id"`
	Message      string `json:"message"`
}

// PasskeyAuthChallenge is the response from
// POST /api/passkeys/authenticate/begin. Challenge is a WebAuthn
// RequestChallengeResponse.
type PasskeyAuthChallenge struct {
	Challenge json.RawMessage `json:"challenge"`
	AuthState string          `json:"auth_state"`
}

// PasskeyAuthComplete is the body for
// POST /api/passkeys/authenticate/complete. Credential is the WebAuthn
// PublicKeyCredential assertion returned by the browser.
type PasskeyAuthComplete struct {
	AuthState  string          `json:"auth_state"`
	Credential json.RawMessage `json:"credential"`
}

// PasskeyListItem is a single row returned by GET /api/v1/me/passkeys.
//
// CredentialIDPrefix is the first 12 characters of the WebAuthn credential
// ID — enough to disambiguate in a dashboard without exposing the full
// identifier. Timestamps use the standard RFC 3339 form via time.Time.
type PasskeyListItem struct {
	CredentialUUID     string     `json:"credential_uuid"`
	CredentialIDPrefix string     `json:"credential_id_prefix"`
	AppUUID            *string    `json:"app_uuid"`
	Nickname           *string    `json:"nickname"`
	LastUsedAt         *time.Time `json:"last_used_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

// ----- Password reset -----

// MessageResponse is a generic message response used by password-reset endpoints.
type MessageResponse struct {
	Message string `json:"message"`
}

// ----- Webhooks (v1 API) -----

// WebhookListResponse is the response from ListWebhooks.
type WebhookListResponse struct {
	Data []WebhookEndpoint `json:"data"`
}

// WebhookEndpointResponse is the response from CreateWebhook.
type WebhookEndpointResponse struct {
	Data WebhookEndpoint `json:"data"`
}

// CreateWebhookRequest is the request body for CreateWebhook.
type CreateWebhookRequest struct {
	URL           string   `json:"url"`
	EventTypes    []string `json:"event_types,omitempty"`
	SigningSecret string   `json:"signing_secret,omitempty"`
	Description   string   `json:"description,omitempty"`
}

// RetryDeliveryResponse is the response from RetryWebhookDelivery.
type RetryDeliveryResponse struct {
	Status string `json:"status"`
}

// ----- OAuth refresh -----

// OAuthRefreshResponse is the response from RefreshOAuthConnection.
type OAuthRefreshResponse struct {
	Provider  string `json:"provider"`
	Refreshed bool   `json:"refreshed"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// ----- Email -----

// SendEmailRequest is the request body for SendEmail.
type SendEmailRequest struct {
	To          string `json:"to"`
	Subject     string `json:"subject"`
	HTMLBody    string `json:"html_body,omitempty"`
	TextBody    string `json:"text_body,omitempty"`
	FromAddress string `json:"from_address,omitempty"`
	ReplyTo     string `json:"reply_to,omitempty"`
}

// SendEmailResponse is the response from SendEmail.
type SendEmailResponse struct {
	Status    string `json:"status"`
	Provider  string `json:"provider"`
	Message   string `json:"message,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

// ----- 0.3.0 registration flow -----

// TokenPair is the response from VerifyOTP.
type TokenPair struct {
	Token        string  `json:"token"`
	RefreshToken *string `json:"refresh_token,omitempty"`
	UserUUID     *string `json:"user_uuid,omitempty"`
}

// RegistrationResult is returned by FinalizeRegistration and Register.
// Full signup flow: SendOTP → VerifyOTP (get signup_token) → FinalizeRegistration
type RegistrationResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    *int64 `json:"expires_in,omitempty"`
	UserUUID     string `json:"user_uuid"`
	// UUID of the org that was created or joined.
	OrgUUID string `json:"org_uuid"`
	// Role the user holds in that org ("admin" for new orgs, or whatever the invitation granted).
	Role    string  `json:"role"`
	Message *string `json:"message,omitempty"`
}

// OrgChoiceType distinguishes between creating a new org and accepting an invite.
type OrgChoiceType string

const (
	OrgChoiceCreate       OrgChoiceType = "create"
	OrgChoiceAcceptInvite OrgChoiceType = "accept_invite"
)

// OrgChoice selects the org action during registration.
type OrgChoice struct {
	Type            OrgChoiceType `json:"type"`
	Name            string        `json:"name,omitempty"`             // for OrgChoiceCreate
	InvitationToken string        `json:"invitation_token,omitempty"` // for OrgChoiceAcceptInvite
}

// FinalizeRegistrationRequest is the body for FinalizeRegistration.
type FinalizeRegistrationRequest struct {
	Email       string    `json:"email"`
	Password    string    `json:"password"`
	AppUUID     uuid.UUID `json:"app_uuid"`
	SignupToken string    `json:"signup_token"`
	OrgChoice   OrgChoice `json:"org_choice"`
	FirstName   string    `json:"first_name,omitempty"`
	LastName    string    `json:"last_name,omitempty"`
}

// CheckOrgNameResponse is the response from CheckOrgNameV2.
type CheckOrgNameResponse struct {
	Available  bool   `json:"available"`
	Reason     string `json:"reason,omitempty"`
	Normalized string `json:"normalized"`
}

// CreateInvitationRequest is the body for CreateInvitation.
type CreateInvitationRequest struct {
	Email          string `json:"email,omitempty"`
	Role           string `json:"role,omitempty"`
	ExpiresInHours *int   `json:"expires_in_hours,omitempty"`
}

// InvitationResponse is returned by CreateInvitation.
type InvitationResponse struct {
	ID        int       `json:"id"`
	OrgUUID   uuid.UUID `json:"org_uuid"`
	Email     *string   `json:"email,omitempty"`
	Role      string    `json:"role"`
	ExpiresAt string    `json:"expires_at"`
	Token     string    `json:"token"`
	SignupURL string    `json:"signup_url"`
}

// InvitationPreview is returned by PreviewInvitation.
type InvitationPreview struct {
	OrgUUID       uuid.UUID `json:"org_uuid"`
	OrgName       string    `json:"org_name"`
	Email         *string   `json:"email,omitempty"`
	Role          string    `json:"role"`
	ExpiresAt     string    `json:"expires_at"`
	Valid         bool      `json:"valid"`
	InvalidReason *string   `json:"invalid_reason,omitempty"`
}

// AcceptInvitationResponse is returned by AcceptInvitation.
type AcceptInvitationResponse struct {
	OrgUUID uuid.UUID `json:"org_uuid"`
	OrgName string    `json:"org_name"`
	Role    string    `json:"role"`
}

// InvitationListItem is one entry in the list from ListInvitations.
type InvitationListItem struct {
	ID         int     `json:"id"`
	Email      *string `json:"email,omitempty"`
	Role       string  `json:"role"`
	ExpiresAt  string  `json:"expires_at"`
	AcceptedAt *string `json:"accepted_at,omitempty"`
	RevokedAt  *string `json:"revoked_at,omitempty"`
}
