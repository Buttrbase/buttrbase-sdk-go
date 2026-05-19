package buttrbase

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

// WebhookEndpoint is the response from webhook endpoints.
type WebhookEndpoint struct {
	ID        int      `json:"id,omitempty"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	CreatedAt string   `json:"created_at,omitempty"`
}

// WebhookDelivery is the response from webhook delivery endpoints.
type WebhookDelivery struct {
	ID          int64  `json:"id"`
	EndpointID  int    `json:"endpoint_id"`
	Event       string `json:"event"`
	Status      string `json:"status"`
	AttemptedAt string `json:"attempted_at"`
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
