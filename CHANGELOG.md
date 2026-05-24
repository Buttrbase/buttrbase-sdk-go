# Changelog

## Unreleased — app_uuid migration

### Breaking
- Methods taking the old `app` slug now take `appUUID string` (UUID literal): `Register`, `Login`, `SendMagicLink`, `OtpSend`, `OtpVerify`.
- `SendMagicLink` now hits `POST /api/auth/magic-link/send` (was `POST /v1/magic-link/send`) and sends the redirect target as `redirect_to`, the field the backend accepts.
- `VerifyMagicLink` now hits `POST /api/auth/magic-link/verify` (was `POST /v1/magic-link/verify`) and is anonymous (no Authorization header).
- `OtpSend` now hits `POST /api/auth/otp` (was `POST /api/auth/otp/send`) and `OtpVerify` sends the code as `otp` to match the new request shape.
- `Login` no longer requires `org_name` — it is sent only when non-empty. `app_uuid` is now the required disambiguator.

### Added
- `ExchangeAPIKey(ctx, apiKey)` — anonymous initial exchange against `POST /api/v1/auth/api-key/exchange`.
- `ExchangeRefreshToken(ctx, refreshToken)` — refresh-mode exchange; revokes the presented refresh token.
- `OAuthStartURL(provider, appUUID, returnTo)` — builds the `GET /api/v1/auth/oauth/{provider}/start` URL.
- App-level API key admin: `ListAppAPIKeys`, `CreateAppAPIKey`, `RevokeAppAPIKey`, `RotateAppAPIKey` against `/api/v1/apps/{app_uuid}/api-keys[/...]`.
- OAuth config admin: `ListOAuthConfigs`, `CreateOAuthConfig`, `UpdateOAuthConfig`, `DeleteOAuthConfig` against `/api/v1/apps/{app_uuid}/oauth-configs[/...]`.
- `ReadAuditLog(ctx, appUUID, opts)` against `GET /api/v1/apps/{app_uuid}/audit-log`.
- Types: `ExchangeResponse`, `APIKeySummary`, `CreatedKeyResponse`, `CreateAPIKeyInput`, `ExpiryInput`, `OAuthConfigSummary`, `CreateOAuthConfigInput`, `UpdateOAuthConfigInput`, `AuditLogQuery`, `AuditRow`, `OAuthProvider`, `APIKeyType`, `APIKeyEnv`.

### Unchanged
- Org-scoped `/api/v2/api-keys` surface (`ListAPIKeysV2`, `CreateAPIKeyV2`, `DeleteAPIKeyV2`) is untouched — the new app-level endpoints are a parallel surface.

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
