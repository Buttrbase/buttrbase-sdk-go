package buttrbase

// ValidateCouponOptions holds optional parameters for ValidateCoupon.
type ValidateCouponOptions struct {
	UserID         *int    `json:"user_id,omitempty"`
	OrderTotalCents *int64 `json:"order_total_cents,omitempty"`
}

// CouponValidation is the response from ValidateCoupon.
type CouponValidation struct {
	Valid          bool    `json:"valid"`
	Code           string  `json:"code,omitempty"`
	DiscountCents  int64   `json:"discount_cents,omitempty"`
	DiscountType   string  `json:"discount_type,omitempty"`
	Reason         string  `json:"reason,omitempty"`
	Raw            map[string]any `json:"-"`
}

// GiftCardValidation is the response from ValidateGiftCard.
type GiftCardValidation struct {
	Valid        bool   `json:"valid"`
	Code         string `json:"code,omitempty"`
	BalanceCents int64  `json:"balance_cents,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Raw          map[string]any `json:"-"`
}

// GiftCardRedemption is the response from RedeemGiftCard.
type GiftCardRedemption struct {
	Success            bool   `json:"success"`
	Code               string `json:"code,omitempty"`
	RedeemedCents      int64  `json:"redeemed_cents,omitempty"`
	RemainingCents     int64  `json:"remaining_cents,omitempty"`
	Raw                map[string]any `json:"-"`
}

// SendMagicLinkOptions holds optional parameters for SendMagicLink.
type SendMagicLinkOptions struct {
	RedirectURL string `json:"redirect_url,omitempty"`
	TTLSeconds  *int64 `json:"ttl_seconds,omitempty"`
}

// MagicLinkSend is the response from SendMagicLink.
type MagicLinkSend struct {
	Sent  bool   `json:"sent"`
	Email string `json:"email,omitempty"`
	Raw   map[string]any `json:"-"`
}

// MagicLinkVerify is the response from VerifyMagicLink.
type MagicLinkVerify struct {
	Valid  bool   `json:"valid"`
	Email  string `json:"email,omitempty"`
	UserID *int   `json:"user_id,omitempty"`
	Raw    map[string]any `json:"-"`
}

// MfaStatus is the response from MfaStatus.
type MfaStatus struct {
	Enrolled bool   `json:"enrolled"`
	Active   bool   `json:"active"`
	Label    string `json:"label,omitempty"`
	Raw      map[string]any `json:"-"`
}

// MfaEnrollment is the response from MfaEnroll.
type MfaEnrollment struct {
	Secret    string `json:"secret,omitempty"`
	OtpauthURL string `json:"otpauth_url,omitempty"`
	Label     string `json:"label,omitempty"`
	Raw       map[string]any `json:"-"`
}

// MfaStatusResponse is the response from MfaActivate.
type MfaStatusResponse struct {
	Active bool   `json:"active"`
	Label  string `json:"label,omitempty"`
	Raw    map[string]any `json:"-"`
}

// OrgSignResponse is the response from OrgSign.
type OrgSignResponse struct {
	Token string `json:"token"`
	Kid   string `json:"kid,omitempty"`
	Raw   map[string]any `json:"-"`
}

// JWKSResponse is the response from OrgJWKS.
type JWKSResponse struct {
	Keys []map[string]any `json:"keys"`
	Raw  map[string]any   `json:"-"`
}

// SecretGet is the response from GetSecret.
type SecretGet struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Raw         map[string]any `json:"-"`
}

// SecretSummary is the response from PutSecret.
type SecretSummary struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
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
	ActiveUsers       int64 `json:"active_users,omitempty"`
	ActiveSessions    int64 `json:"active_sessions,omitempty"`
	PendingElevations int64 `json:"pending_elevations,omitempty"`
	SecretsCount      int64 `json:"secrets_count,omitempty"`
	SigningKeysCount  int64 `json:"signing_keys_count,omitempty"`
	Raw               map[string]any `json:"-"`
}

// ----- Credentials -----

// Credential represents an API credential (client ID / secret pair).
// client_secret is only present on create and rotate-secret responses.
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
	Reset   bool   `json:"reset"`
	Message string `json:"message,omitempty"`
	Raw     map[string]any `json:"-"`
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
