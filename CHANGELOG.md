# Changelog

## Unreleased — commerce/management parity with Rust SDK

Closes all hard-missing Go gaps identified in the 2026-06-26 parity audit.
No existing exported methods or types were changed or removed.

### Added — Wallet
- `Wallet(ctx) (*WalletSummary, error)` — GET /api/wallet. Mirrors Rust SDK `wallet(bearer)`.
- `WalletTransactions(ctx, limit, offset uint32) ([]WalletTransaction, error)` — GET /api/wallet/transactions. Mirrors Rust SDK `wallet_transactions(bearer, limit, offset)`.
- New types: `WalletSummary`, `WalletTransaction`.

### Added — Subscriptions
- `Subscriptions(ctx) ([]SubscriptionItem, error)` — GET /api/subscriptions.
- `CreateSubscription(ctx, CreateSubscriptionRequest) (*SubscriptionItem, error)` — POST /api/subscriptions.
- `CancelSubscription(ctx, subscriptionID int) error` — DELETE /api/subscriptions/{id}.
- New types: `SubscriptionItem`, `CreateSubscriptionRequest`.

### Added — Usage
- `ReportUsage(ctx, UsageEvent) error` — POST /api/usage/report. Mirrors Rust SDK `report_usage`. Uses bearer auth (vs Rust SDK's HTTP Basic; see parity report).
- New type: `UsageEvent` (metric, quantity, org_uuid?, app_uuid?, timestamp?).

### Added — Teams
- `OrgTeams(ctx, orgUUID string) ([]TeamItem, error)` — GET /api/organizations/{orgUUID}/teams.
- `UserTeams(ctx, userUUID string) ([]TeamItem, error)` — GET /api/users/{userUUID}/teams.
- New type: `TeamItem` (id, team_uuid, org_uuid, name, description?).

### Added — App management
- `MyApps(ctx) ([]AppEntry, error)` — GET /api/me/apps.
- `AppOrgs(ctx, appUUID string) ([]OrgEntry, error)` — GET /api/apps/{appUUID}/organizations.
- `AppCredentials(ctx, appUUID string) (*AppCredentialsResponse, error)` — GET /api/apps/{appUUID}/credentials.
- `EnableSandbox(ctx, appUUID string) error` — PATCH /api/apps/{appUUID} with body {sandbox_enabled: true}.
- `RotateCredentials(ctx, appUUID, environment string) (map[string]any, error)` — POST /api/apps/{appUUID}/credentials/{env}/rotate.
- New types: `AppEntry`, `OrgEntry`, `AppCredentialInfo`, `AppCredentialsResponse`.

### Added — Auth
- `RefreshToken(ctx, refreshToken string) (*AccessToken, error)` — POST /api/app/auth/refresh. Also updates `Client.AccessToken` on success. New type: `AccessToken`.

### Added — Typed shapes (shape divergences resolved, old methods preserved)
- `CheckEntitlementTyped(ctx, featureKey string) (*EntitlementResult, error)` — canonical `feature_key` body + typed response. Old `CheckEntitlement(ctx, map[string]any)` is unchanged.
- `BatchCheckEntitlementsTyped(ctx, featureKeys []string) (map[string]EntitlementResult, error)` — canonical `feature_keys` body + `map[string]EntitlementResult` response. Routes to `/api/entitlements/check/batch`. Old `BatchCheckEntitlements(ctx, map[string]any)` is unchanged.
- `GetEffectiveEntitlementsTyped(ctx) ([]EffectiveEntitlement, error)` — typed `[]EffectiveEntitlement` slice. Old `GetEffectiveEntitlements(ctx, map[string]string)` is unchanged.
- `PricingPreviewTyped(ctx, PricingPreviewRequest) (*PricingPreviewResponse, error)` — typed request. Old `PricingPreview(ctx, map[string]any)` is unchanged.
- `PricingQuoteTyped(ctx, PricingPreviewRequest) (map[string]any, error)` — typed request. Old `PricingQuote(ctx, map[string]any)` is unchanged.
- `CheckoutSessionTyped(ctx, CheckoutSessionTypedRequest) (*CheckoutSessionResponse, error)` — typed request + typed response. Old `PricingCheckoutSession(ctx, map[string]any)` is unchanged.
- New types: `EntitlementResult`, `EffectiveEntitlement`, `PricingPreviewRequest`, `PricingPreviewResponse`, `CheckoutSessionTypedRequest`, `CheckoutSessionResponse`.

### Design notes
- `ReportUsage` uses bearer auth. The Rust SDK uses HTTP Basic (client_id:client_secret) for this endpoint. The Go client model is OAuth2 client-credentials bearer-first; callers using `WithClientCredentials` attach the right token automatically. The endpoint accepts both auth models.
- `TeamItem` is a new typed struct in `commerce.go`; it does not collide with the existing untyped `Team` in `types.go` (different name).
- All new `{"data": ...}` unwrapping uses private envelope types (not exported).

### Intended version
`v0.8.0` — to be tagged on merge to main.

---

## Unreleased — JWKS verifier (mirrors Rust SDK Verifier)

Adds real RS256 signature verification on top of the existing claim structs.
No existing types or method signatures changed.

### Added
- `VerifierConfig` — `{ JWKSURL, Issuer, Audience string }`. `Audience` is
  optional; leave empty to skip `aud` validation (mirrors Rust SDK's
  `VerifierConfig.audience: Option<String>`).
- `Verifier` — owns a live JWKS cache backed by
  `MicahParks/keyfunc` + `MicahParks/jwkset`. Safe for concurrent use.
- `NewVerifier(cfg VerifierConfig) (*Verifier, error)` — constructs a Verifier
  with a background JWKS refresh goroutine running until process exit.
- `NewVerifierCtx(ctx context.Context, cfg VerifierConfig) (*Verifier, error)`
  — same, but the refresh goroutine stops when ctx is cancelled.
- `(*Verifier).VerifyToken(token string) (*TokenClaims, error)` — verifies an
  RS256 JWT against the JWKS (kid → fetch/cache → validate), checks issuer and
  optionally audience, returns the enriched `TokenClaims` (including
  `data.roles`/`data.email`). Mirrors `Verifier::verify` in the Rust SDK.
- `(*Verifier).VerifyBearer(authHeader string) (*AuthContext, error)` — strips
  `"Bearer "`, calls `VerifyToken`, returns an `AuthContext` via the existing
  `TokenClaims.AuthContext()` method. Mirrors `Verifier::verify_bearer` in the
  Rust SDK.
- `(*Verifier).Issuer() string` and `(*Verifier).Audience() string` — read-only
  accessors for diagnostics.

### Dependencies added
- `github.com/golang-jwt/jwt/v5 v5.3.1`
- `github.com/MicahParks/keyfunc/v3 v3.8.0`
- `github.com/MicahParks/jwkset v0.11.0` (transitive)
- `golang.org/x/time v0.9.0` (transitive)

### Design note
`jwt.ParseWithClaims` requires its claims struct to implement `jwt.Claims`
(six getter methods). Rather than alter the public `TokenClaims` struct — which
has `exp int64` / `iat int64` conflicting with `jwt.RegisteredClaims`'s
`*NumericDate` fields — an unexported `jwtClaims` wrapper embeds
`jwt.RegisteredClaims` plus the buttrbase-specific fields. After parsing,
`jwtClaims.toTokenClaims()` converts to the public `TokenClaims` so callers
see no new types.

### Intended version
`v0.7.0` — to be tagged on merge to main.

## Unreleased — token claims enrichment (mirrors Rust SDK 0.6.0)

Additively exposes the buttrbase `data` envelope carried inside access-token
JWTs. No existing types or method signatures changed.

### Added
- `TokenClaimsData` — struct representing the `data` object inside a
  buttrbase JWT (`roles`, `email`, `org_uuid`, `user_uuid`; all optional).
- `TokenClaims` — the full JWT payload (`sub`, `org`, `exp`, `iat`, `scope`,
  optional `data`), returned by `ParseTokenClaims`.
- `AuthContext` — the derived principal (`UserID`, `OrgID`, `Scopes`,
  `Roles []string`, `Email *string`).
- `(TokenClaims).AuthContext()` — converts `TokenClaims` to `AuthContext`,
  splitting `data.roles` (comma/space-delimited string) into a `[]string`
  slice and forwarding `data.email`.
- `ParseTokenClaims(tokenString string) (TokenClaims, error)` — decodes the
  JWT payload (base64url only; **does not verify the signature**). Always
  verify against the Buttrbase JWKS before trusting claims in a security
  context.

### Intended version
`v0.6.0` — to be tagged on merge to main.

## Unreleased — static API-key removal

Static API-key auth is retired. OAuth2 client-credentials (`client_id` +
`client_secret`) is now the only supported app-server credential, and the SDK
performs the token grant for you.

### Added
- `WithClientCredentials(clientID, clientSecret)` option — construct the client
  with a client-credentials pair and the SDK fetches/refreshes the bearer
  access token automatically (lazily before the first authenticated request,
  refreshing a bit before `expires_in`).
- `Authenticate(ctx)` — exchanges the configured `client_id`/`client_secret`
  for an access token via `POST /api/v1/auth/token` and caches it. Called
  automatically before authenticated requests; call it directly to fail fast on
  bad credentials.

### Breaking / Removed
- Removed the `wb_live_`/`wb_test_` static-key auth path. `New` now takes an
  OAuth2 bearer access token; the `Client.APIKey` field is renamed to
  `Client.AccessToken`.
- Removed `ExchangeAPIKey` and `ExchangeRefreshToken` (the
  `POST /api/v1/auth/api-key/exchange` endpoint).
- Removed app-level API-key admin: `ListAppAPIKeys`, `CreateAppAPIKey`,
  `RevokeAppAPIKey`, `RotateAppAPIKey`.
- Removed the org-scoped v2 API-key surface: `ListAPIKeysV2`, `CreateAPIKeyV2`,
  `DeleteAPIKeyV2`.
- Removed types: `ExchangeResponse`, `APIKeySummary`, `CreatedKeyResponse`,
  `CreateAPIKeyInput`, `ExpiryInput`, `APIKeyType` (+consts), `APIKeyEnv`
  (+consts), `ApiKey`.

App-server callers manage client-credentials pairs with `CreateCredential`,
`RotateCredentialSecret`, `DeleteCredential`, `ListCredentials`, then construct
the client with `WithClientCredentials` — the SDK handles the token grant. A
pre-obtained access token may still be passed directly to `New`.

## Unreleased — magic-link contract fix

### Breaking
- `SendMagicLink` signature changed to `SendMagicLink(ctx, email string, *SendMagicLinkOptions)`. `app_uuid` moved out of the positional args and into `SendMagicLinkOptions` (it is optional per the backend contract).
- `SendMagicLinkOptions` now exposes `AppUUID`, `RedirectTo`, and `OrgUUID` (with `omitempty` JSON tags). The old `RedirectURL` and unsupported `TTLSeconds` fields were removed.
- `MagicLinkSend` now matches the response contract: `Sent bool`, `DevToken string` (raw one-time token, non-prod dev-echo only; empty in prod), `ExpiresInSeconds int64`. The bogus `Email` field was removed.
- `MagicLinkVerify` now matches the response contract: `AccessToken string` (JWKS-verifiable RS256), `TokenType string`, `User MagicLinkUser{UserUUID, Email}`, `RedirectTo string`. The bogus `Valid`/`Email`/`UserID` fields were removed. New `MagicLinkUser` type added.

### Notes
- Magic-link is the only browser flow that yields a JWKS-verifiable RS256 access token; the generic email-OTP endpoints issue HS256 tokens the public JWKS cannot verify. Cross-app federation: passing `AppUUID` + an allowlisted `RedirectTo` origin makes the emailed link target the app's own callback so the app verifies the RS256 token itself.

## Unreleased — app_uuid migration

### Breaking
- Methods taking the old `app` slug now take `appUUID string` (UUID literal): `Register`, `Login`, `SendMagicLink`, `OtpSend`, `OtpVerify`.
- `SendMagicLink` now hits `POST /api/auth/magic-link/send` (was `POST /v1/magic-link/send`) and sends the redirect target as `redirect_to`, the field the backend accepts.
- `VerifyMagicLink` now hits `POST /api/auth/magic-link/verify` (was `POST /v1/magic-link/verify`) and is anonymous (no Authorization header).
- `OtpSend` now hits `POST /api/auth/otp` (was `POST /api/auth/otp/send`) and `OtpVerify` sends the code as `otp` to match the new request shape.
- `Login` no longer requires `org_name` — it is sent only when non-empty. `app_uuid` is now the required disambiguator.

### Added
- `OAuthStartURL(provider, appUUID, returnTo)` — builds the `GET /api/v1/auth/oauth/{provider}/start` URL.
- OAuth config admin: `ListOAuthConfigs`, `CreateOAuthConfig`, `UpdateOAuthConfig`, `DeleteOAuthConfig` against `/api/v1/apps/{app_uuid}/oauth-configs[/...]`.
- `ReadAuditLog(ctx, appUUID, opts)` against `GET /api/v1/apps/{app_uuid}/audit-log`.
- Types: `OAuthConfigSummary`, `CreateOAuthConfigInput`, `UpdateOAuthConfigInput`, `AuditLogQuery`, `AuditRow`, `OAuthProvider`.

### Passkey support
- `PasskeyRegisterBegin(ctx)`, `PasskeyRegisterComplete(ctx, body)`,
  `PasskeyAuthenticateBegin(ctx)`, `PasskeyAuthenticateComplete(ctx, body)`
  — thin wrappers over `POST /api/passkeys/{register,authenticate}/{begin,complete}`.
  WebAuthn challenge / credential blobs are `json.RawMessage` (pass-through);
  no webauthn helper library is pulled in. Begin endpoints unwrap the
  backend's `{"data": ...}` envelope for ergonomics.
- `ListMyPasskeys(ctx)` — `GET /api/v1/me/passkeys`. Returns
  `[]PasskeyListItem` in descending `CreatedAt` order.
- `DeleteMyPasskey(ctx, credentialUUID)` —
  `DELETE /api/v1/me/passkeys/{uuid}`. Owner check enforced server-side.
- Types: `PasskeyRegistrationChallenge`, `PasskeyRegistrationComplete`,
  `PasskeyRegistrationResult`, `PasskeyAuthChallenge`, `PasskeyAuthComplete`,
  `PasskeyListItem`.
