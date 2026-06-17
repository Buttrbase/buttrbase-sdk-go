# Changelog

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
