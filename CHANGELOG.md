# Changelog

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
