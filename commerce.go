package buttrbase

import (
	"context"
	"fmt"
	"net/url"
)

// ============================================================
// Wallet
// ============================================================

// WalletSummary is the user's wallet balance and budget.
// Mirrors Rust SDK WalletSummary.
type WalletSummary struct {
	BalanceCents    int64   `json:"balance_cents"`
	BudgetLimitCents *int64 `json:"budget_limit_cents,omitempty"`
	BudgetPeriod    *string `json:"budget_period,omitempty"`
}

// walletEnvelope unwraps {"data": WalletSummary} from GET /api/wallet.
type walletEnvelope struct {
	Data WalletSummary `json:"data"`
}

// Wallet returns the wallet balance and budget for the authenticated user.
//
// GET /api/wallet
// Requires a bearer token.
func (c *Client) Wallet(ctx context.Context) (*WalletSummary, error) {
	var env walletEnvelope
	if err := c.do(ctx, "GET", "/api/wallet", nil, true, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// WalletTransaction is a single wallet deposit or withdrawal.
// Mirrors Rust SDK WalletTransaction.
type WalletTransaction struct {
	ID          int    `json:"id"`
	Kind        string `json:"kind"`
	AmountCents int64  `json:"amount_cents"`
	Description *string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// walletTransactionsEnvelope unwraps {"data": [...]} from GET /api/wallet/transactions.
type walletTransactionsEnvelope struct {
	Data []WalletTransaction `json:"data"`
}

// WalletTransactions returns paginated wallet transactions for the authenticated user.
//
// GET /api/wallet/transactions?limit=&offset=
// Requires a bearer token.
func (c *Client) WalletTransactions(ctx context.Context, limit, offset uint32) ([]WalletTransaction, error) {
	path := fmt.Sprintf("/api/wallet/transactions?limit=%d&offset=%d", limit, offset)
	var env walletTransactionsEnvelope
	if err := c.do(ctx, "GET", path, nil, true, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// ============================================================
// Subscriptions
// ============================================================

// SubscriptionItem is one active or past subscription.
// Mirrors Rust SDK SubscriptionItem.
type SubscriptionItem struct {
	ID                     int     `json:"id"`
	UserUUID               *string `json:"user_uuid,omitempty"`
	PriceID                *int    `json:"price_id,omitempty"`
	Provider               string  `json:"provider"`
	ProviderSubscriptionID string  `json:"provider_subscription_id"`
	Status                 string  `json:"status"`
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
}

// subscriptionsEnvelope unwraps {"data": [...]} from GET /api/subscriptions.
type subscriptionsEnvelope struct {
	Data []SubscriptionItem `json:"data"`
}

// subscriptionEnvelope unwraps {"data": SubscriptionItem} from POST /api/subscriptions.
type subscriptionEnvelope struct {
	Data SubscriptionItem `json:"data"`
}

// Subscriptions lists the user's subscriptions.
//
// GET /api/subscriptions
// Requires a bearer token.
func (c *Client) Subscriptions(ctx context.Context) ([]SubscriptionItem, error) {
	var env subscriptionsEnvelope
	if err := c.do(ctx, "GET", "/api/subscriptions", nil, true, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// CreateSubscriptionRequest is the body for CreateSubscription.
// Only PriceID is required; all other fields are optional.
type CreateSubscriptionRequest struct {
	PriceID  int    `json:"price_id"`
	Quantity *int   `json:"quantity,omitempty"`
	QuoteID  string `json:"quote_id,omitempty"`
}

// CreateSubscription creates a subscription for a price.
//
// POST /api/subscriptions
// Requires a bearer token.
func (c *Client) CreateSubscription(ctx context.Context, req CreateSubscriptionRequest) (*SubscriptionItem, error) {
	var env subscriptionEnvelope
	if err := c.do(ctx, "POST", "/api/subscriptions", req, true, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// CancelSubscription cancels a subscription by its integer ID.
//
// DELETE /api/subscriptions/{id}
// Requires a bearer token.
func (c *Client) CancelSubscription(ctx context.Context, subscriptionID int) error {
	path := fmt.Sprintf("/api/subscriptions/%d", subscriptionID)
	return c.do(ctx, "DELETE", path, nil, true, nil)
}

// ============================================================
// Usage reporting
// ============================================================

// UsageEvent is the body for ReportUsage.
// Mirrors Rust SDK UsageEvent.
type UsageEvent struct {
	// Metric is the metered feature key (e.g. "api_calls", "storage_gb").
	Metric string `json:"metric"`
	// Quantity is the amount consumed.
	Quantity float64 `json:"quantity"`
	// OrgUUID scopes the event to a specific organisation (optional).
	OrgUUID *string `json:"org_uuid,omitempty"`
	// AppUUID scopes the event to a specific app (optional).
	AppUUID *string `json:"app_uuid,omitempty"`
	// Timestamp is an ISO-8601 string; omit to use server-side now (optional).
	Timestamp *string `json:"timestamp,omitempty"`
}

// ReportUsage reports a metered usage event for billing reconciliation.
//
// POST /api/usage/report
//
// Unlike most SDK methods, this uses the client's stored access token (bearer).
// The Rust SDK sends HTTP Basic auth here, but the Go client model is
// bearer-token-first — callers using client-credentials (WithClientCredentials)
// will automatically fetch and attach the right token. See audit note in
// .task-parity-report.md for the design divergence.
func (c *Client) ReportUsage(ctx context.Context, event UsageEvent) error {
	return c.do(ctx, "POST", "/api/usage/report", event, true, nil)
}

// ============================================================
// Teams
// ============================================================

// TeamItem is one team returned by the team listing endpoints.
// Mirrors Rust SDK TeamItem.
type TeamItem struct {
	ID          int     `json:"id"`
	TeamUUID    string  `json:"team_uuid"`
	OrgUUID     string  `json:"org_uuid"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// orgTeamsEnvelope unwraps {"data": [...]} from GET /api/organizations/{org_uuid}/teams.
type orgTeamsEnvelope struct {
	Data []TeamItem `json:"data"`
}

// OrgTeams lists active teams in an organisation.
//
// GET /api/organizations/{orgUUID}/teams
// Requires a bearer token.
func (c *Client) OrgTeams(ctx context.Context, orgUUID string) ([]TeamItem, error) {
	path := "/api/organizations/" + url.PathEscape(orgUUID) + "/teams"
	var env orgTeamsEnvelope
	if err := c.do(ctx, "GET", path, nil, true, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// UserTeams lists the teams a user belongs to.
//
// GET /api/users/{userUUID}/teams
// Requires a bearer token.
func (c *Client) UserTeams(ctx context.Context, userUUID string) ([]TeamItem, error) {
	path := "/api/users/" + url.PathEscape(userUUID) + "/teams"
	var env orgTeamsEnvelope
	if err := c.do(ctx, "GET", path, nil, true, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// ============================================================
// App management
// ============================================================

// AppEntry is one app in the list returned by MyApps.
// Mirrors Rust SDK AppEntry.
type AppEntry struct {
	AppUUID string  `json:"app_uuid"`
	AppName string  `json:"app_name"`
	Role    *string `json:"role,omitempty"`
}

// appsEnvelope unwraps {"data": [...]} from GET /api/me/apps.
type appsEnvelope struct {
	Data []AppEntry `json:"data"`
}

// MyApps lists the apps the authenticated user belongs to.
//
// GET /api/me/apps
// Requires a bearer token.
func (c *Client) MyApps(ctx context.Context) ([]AppEntry, error) {
	var env appsEnvelope
	if err := c.do(ctx, "GET", "/api/me/apps", nil, true, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// OrgEntry is one org in the list returned by AppOrgs.
// Mirrors Rust SDK OrgEntry.
type OrgEntry struct {
	OrgUUID string  `json:"org_uuid"`
	OrgName string  `json:"org_name"`
	Role    *string `json:"role,omitempty"`
}

// orgEntriesEnvelope unwraps {"data": [...]} from GET /api/apps/{app_uuid}/organizations.
type orgEntriesEnvelope struct {
	Data []OrgEntry `json:"data"`
}

// AppOrgs lists the organisations within an app that the user belongs to.
//
// GET /api/apps/{appUUID}/organizations
// Requires a bearer token.
func (c *Client) AppOrgs(ctx context.Context, appUUID string) ([]OrgEntry, error) {
	path := "/api/apps/" + url.PathEscape(appUUID) + "/organizations"
	var env orgEntriesEnvelope
	if err := c.do(ctx, "GET", path, nil, true, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// AppCredentialInfo describes live or sandbox credentials for an app.
// Mirrors Rust SDK AppCredentialInfo.
type AppCredentialInfo struct {
	Environment          string  `json:"environment"`
	ClientID             string  `json:"client_id"`
	ClientSecretPrefix   *string `json:"client_secret_prefix,omitempty"`
	IsActive             bool    `json:"is_active"`
	CreatedAt            *string `json:"created_at,omitempty"`
	RotatedAt            *string `json:"rotated_at,omitempty"`
}

// AppCredentialsResponse is the response from AppCredentials.
// Mirrors Rust SDK AppCredentialsResponse.
type AppCredentialsResponse struct {
	AppName        string             `json:"app_name"`
	SandboxEnabled bool               `json:"sandbox_enabled"`
	Live           *AppCredentialInfo `json:"live,omitempty"`
	Sandbox        *AppCredentialInfo `json:"sandbox,omitempty"`
}

// appCredentialsEnvelope unwraps {"data": AppCredentialsResponse}.
type appCredentialsEnvelope struct {
	Data AppCredentialsResponse `json:"data"`
}

// AppCredentials retrieves live/sandbox credential info for an app.
// Admin-only.
//
// GET /api/apps/{appUUID}/credentials
// Requires a bearer token.
func (c *Client) AppCredentials(ctx context.Context, appUUID string) (*AppCredentialsResponse, error) {
	path := "/api/apps/" + url.PathEscape(appUUID) + "/credentials"
	var env appCredentialsEnvelope
	if err := c.do(ctx, "GET", path, nil, true, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// EnableSandbox enables sandbox mode for an app.
//
// PATCH /api/apps/{appUUID}  body: {"sandbox_enabled": true}
// Requires a bearer token.
func (c *Client) EnableSandbox(ctx context.Context, appUUID string) error {
	path := "/api/apps/" + url.PathEscape(appUUID)
	body := map[string]any{"sandbox_enabled": true}
	return c.do(ctx, "PATCH", path, body, true, nil)
}

// RotateCredentials rotates credentials for an app environment.
// environment must be "live" or "sandbox".
//
// POST /api/apps/{appUUID}/credentials/{environment}/rotate
// Requires a bearer token.
func (c *Client) RotateCredentials(ctx context.Context, appUUID, environment string) (map[string]any, error) {
	path := "/api/apps/" + url.PathEscape(appUUID) + "/credentials/" + url.PathEscape(environment) + "/rotate"
	var out map[string]any
	if err := c.do(ctx, "POST", path, nil, true, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ============================================================
// Refresh token
// ============================================================

// AccessToken is returned by RefreshToken.
// Mirrors Rust SDK AccessToken.
type AccessToken struct {
	Token        string  `json:"token"`
	RefreshToken *string `json:"refresh_token,omitempty"`
}

// RefreshToken exchanges a refresh token for a new access token.
//
// POST /api/app/auth/refresh  body: {"refresh": refreshToken}
// Uses the bearer token already on the client for app identification
// (the Go client model uses bearer rather than HTTP Basic for this).
//
// Design note: the Rust SDK sends HTTP Basic auth here; the Go SDK
// sends the stored bearer token because the Go client model authenticates
// via OAuth2 client-credentials grant rather than per-request Basic auth.
// The endpoint accepts both auth styles. Divergence is documented.
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*AccessToken, error) {
	body := map[string]any{"refresh": refreshToken}
	var out AccessToken
	if err := c.do(ctx, "POST", "/api/app/auth/refresh", body, true, &out); err != nil {
		return nil, err
	}
	if out.Token != "" {
		c.AccessToken = out.Token
	}
	return &out, nil
}

// ============================================================
// Typed pricing helpers (shape fix: map[string]any → typed structs)
// ============================================================

// PricingPreviewRequest is the typed body for PricingPreviewTyped and PricingQuoteTyped.
// Mirrors Rust SDK PricingPreviewRequest. Replaces the legacy map[string]any overload.
type PricingPreviewRequest struct {
	PriceID    int     `json:"price_id"`
	CouponCode *string `json:"coupon_code,omitempty"`
	Seats      *int64  `json:"seats,omitempty"`
	Country    *string `json:"country,omitempty"`
}

// PricingPreviewResponse is the decoded pricing preview.
// Mirrors Rust SDK PricingPreview.
type PricingPreviewResponse struct {
	AmountCents    int64   `json:"amount_cents"`
	Currency       string  `json:"currency"`
	DiscountCents  *int64  `json:"discount_cents,omitempty"`
	TaxCents       *int64  `json:"tax_cents,omitempty"`
	FinalCents     int64   `json:"final_cents"`
	RegionResolved *string `json:"region_resolved,omitempty"`
}

// pricingPreviewEnvelope unwraps {"data": PricingPreviewResponse}.
type pricingPreviewEnvelope struct {
	Data PricingPreviewResponse `json:"data"`
}

// CheckoutSessionRequest is the typed body for CheckoutSessionTyped.
// Mirrors Rust SDK CheckoutSessionRequest.
type CheckoutSessionTypedRequest struct {
	PriceID int     `json:"price_id"`
	QuoteID *string `json:"quote_id,omitempty"`
}

// CheckoutSessionResponse is the decoded checkout session.
// Mirrors Rust SDK CheckoutSession.
type CheckoutSessionResponse struct {
	PaymentURL string  `json:"payment_url"`
	SessionID  *string `json:"session_id,omitempty"`
	Provider   string  `json:"provider"`
}

// checkoutSessionEnvelope unwraps {"data": CheckoutSessionResponse}.
type checkoutSessionEnvelope struct {
	Data CheckoutSessionResponse `json:"data"`
}

// PricingPreviewTyped previews the price using a typed request struct.
// This is the canonical shape matching the Rust SDK.
//
// POST /api/pricing/preview
// Requires a bearer token.
//
// The existing PricingPreview(ctx, map[string]any) method is preserved unchanged.
func (c *Client) PricingPreviewTyped(ctx context.Context, req PricingPreviewRequest) (*PricingPreviewResponse, error) {
	var env pricingPreviewEnvelope
	if err := c.do(ctx, "POST", "/api/pricing/preview", req, true, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// PricingQuoteTyped locks a signed price quote (10-minute TTL) using a typed request.
// This is the canonical shape matching the Rust SDK.
//
// POST /api/pricing/quote
// Requires a bearer token.
//
// The existing PricingQuote(ctx, map[string]any) method is preserved unchanged.
func (c *Client) PricingQuoteTyped(ctx context.Context, req PricingPreviewRequest) (map[string]any, error) {
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := c.do(ctx, "POST", "/api/pricing/quote", req, true, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// CheckoutSessionTyped creates a checkout session using a typed request.
// Blocked for sandbox credentials — the backend returns 400 if the bearer
// token carries sandbox:true. This is the canonical shape matching the Rust SDK.
//
// POST /api/pricing/checkout-session
// Requires a bearer token.
//
// The existing PricingCheckoutSession(ctx, map[string]any) method is preserved unchanged.
func (c *Client) CheckoutSessionTyped(ctx context.Context, req CheckoutSessionTypedRequest) (*CheckoutSessionResponse, error) {
	var env checkoutSessionEnvelope
	if err := c.do(ctx, "POST", "/api/pricing/checkout-session", req, true, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// ============================================================
// Typed entitlement helpers (shape fix: map[string]any → typed structs)
// ============================================================

// EntitlementResult is the result of a single entitlement check.
// Mirrors Rust SDK EntitlementResult.
type EntitlementResult struct {
	Granted bool    `json:"granted"`
	Reason  *string `json:"reason,omitempty"`
}

// entitlementResultEnvelope unwraps {"data": EntitlementResult}.
type entitlementResultEnvelope struct {
	Data EntitlementResult `json:"data"`
}

// entitlementBatchEnvelope unwraps {"data": map[string]EntitlementResult}.
type entitlementBatchEnvelope struct {
	Data map[string]EntitlementResult `json:"data"`
}

// EffectiveEntitlement is a single row from GetEffectiveEntitlementsTyped.
// Mirrors Rust SDK EffectiveEntitlement.
type EffectiveEntitlement struct {
	FeatureKey string  `json:"feature_key"`
	Granted    bool    `json:"granted"`
	Reason     *string `json:"reason,omitempty"`
}

// effectiveEntitlementsEnvelope unwraps {"data": [...]}.
type effectiveEntitlementsEnvelope struct {
	Data []EffectiveEntitlement `json:"data"`
}

// CheckEntitlementTyped checks whether the user has access to featureKey.
// This is the canonical shape matching the Rust SDK.
//
// POST /api/entitlements/check  body: {"feature_key": featureKey}
// Requires a bearer token.
//
// Divergence resolved: the old CheckEntitlement(ctx, map[string]any) accepted a
// free-form map and returned map[string]any; the Rust SDK uses a typed
// {"feature_key": string} body and returns EntitlementResult. This new method
// adds the canonical shape; the old method is preserved unchanged.
func (c *Client) CheckEntitlementTyped(ctx context.Context, featureKey string) (*EntitlementResult, error) {
	body := map[string]any{"feature_key": featureKey}
	var env entitlementResultEnvelope
	if err := c.do(ctx, "POST", "/api/entitlements/check", body, true, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// BatchCheckEntitlementsTyped checks multiple feature keys in one call.
// Returns a map of featureKey → EntitlementResult.
// This is the canonical shape matching the Rust SDK.
//
// POST /api/entitlements/check/batch  body: {"feature_keys": [...]}
// Requires a bearer token.
//
// Divergence resolved: the old BatchCheckEntitlements(ctx, map[string]any)
// used a free-form map and routed to /api/entitlements/batch-check. The Rust SDK
// uses {"feature_keys": []string} and routes to /api/entitlements/check/batch.
// This new method adds the canonical shape and path; the old method is preserved.
func (c *Client) BatchCheckEntitlementsTyped(ctx context.Context, featureKeys []string) (map[string]EntitlementResult, error) {
	body := map[string]any{"feature_keys": featureKeys}
	var env entitlementBatchEnvelope
	if err := c.do(ctx, "POST", "/api/entitlements/check/batch", body, true, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// GetEffectiveEntitlementsTyped returns all effective entitlements for the user.
// Mirrors Rust SDK effective_entitlements(bearer) -> Vec<EffectiveEntitlement>.
//
// GET /api/entitlements/effective
// Requires a bearer token.
//
// The old GetEffectiveEntitlements(ctx, filters map[string]string) is preserved unchanged.
func (c *Client) GetEffectiveEntitlementsTyped(ctx context.Context) ([]EffectiveEntitlement, error) {
	var env effectiveEntitlementsEnvelope
	if err := c.do(ctx, "GET", "/api/entitlements/effective", nil, true, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}
