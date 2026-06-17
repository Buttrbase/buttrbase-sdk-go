# Go SDK

> **Breaking change in this release — `app_uuid` migration.**
> `Register`, `Login`, `SendMagicLink`, `OtpSend`, and `OtpVerify` now take
> `appUUID string` (a UUID literal) as their first argument. The legacy
> `app` / `appName` / `appId` slug fields are no longer accepted by the
> backend on any auth route. See `CHANGELOG.md` for the full list.

## Overview

The official Go SDK for ButtrBase. Standard `net/http`-based client covering every API surface — auth, organizations, billing, RBAC, teams, credentials, search, AI gateway, webhooks, zero-trust, and more.

## Installation

```bash
go get github.com/buttrbase/buttrbase-sdk-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    buttrbase "github.com/buttrbase/buttrbase-sdk-go"
)

func main() {
    // App-server auth uses OAuth2 client-credentials. Construct the client
    // with your client_id/client_secret and the SDK fetches (and refreshes)
    // the bearer access token automatically — calls just work.
    client := buttrbase.New("",
        buttrbase.WithClientCredentials("<client-id>", "<client-secret>"))
    ctx := context.Background()

    appUUID := "018f1234-5678-7000-8000-000000000001"

    // Login (app_uuid is now required)
    resp, err := client.Login(ctx, appUUID, "user@example.com", "password", "")
    if err != nil { panic(err) }
    fmt.Println(resp.AccessToken)

    // Get profile
    profile, err := client.GetProfile(ctx)
    if err != nil { panic(err) }
    fmt.Println(profile.Email)
}
```

## Authentication

### Register

```go
appUUID := "018f1234-5678-7000-8000-000000000001"
resp, err := client.Register(ctx, appUUID, "user@example.com", "password", "Acme",
    &buttrbase.RegisterOptions{FirstName: "Jane", LastName: "Doe"})
```

### Magic Link

```go
_, err := client.SendMagicLink(ctx, appUUID, "user@example.com",
    &buttrbase.SendMagicLinkOptions{RedirectURL: "https://app.example.com/auth/callback"})
resp, err := client.VerifyMagicLink(ctx, "token-from-email")
fmt.Println(resp.AccessToken) // JWT with sub, org, aud claims
```

### OTP (Passwordless)

```go
// phone OR email — appUUID is required
_, err := client.OtpSend(ctx, appUUID, "+15551234567")
resp, err := client.OtpVerify(ctx, appUUID, "+15551234567", "123456")
```

### App-server auth (OAuth2 client-credentials)

App servers authenticate with an OAuth2 **client-credentials** grant — the
single supported app-server credential. Static API keys (the retired
`wb_live_`/`wb_test_` keys and the `api-key/exchange` endpoint) are no longer
supported.

Construct the client with your `client_id`/`client_secret` via
`WithClientCredentials`. The SDK exchanges them for a bearer access token via
`POST /api/v1/auth/token` automatically — lazily before the first
authenticated request, and again whenever the cached token expires (it tracks
`expires_in` and refreshes a little early). You never handle the token
yourself:

```go
appClient := buttrbase.New("",
    buttrbase.WithClientCredentials("<client-id>", "<client-secret>"))

// Authenticated calls just work — the token is fetched and cached on demand.
profile, err := appClient.GetProfile(ctx)

// Optional: fetch a token eagerly to fail fast on bad credentials.
if err := appClient.Authenticate(ctx); err != nil { /* 401 invalid client credentials */ }
```

Provision and manage `client_id`/`client_secret` pairs with `CreateCredential`,
`RotateCredentialSecret`, `DeleteCredential`, and `ListCredentials`:

```go
cred, err := client.CreateCredential(ctx, buttrbase.CreateCredentialRequest{
    Name: "production-server",
})
fmt.Println(cred.ClientID, cred.ClientSecret) // secret shown once — persist it

rotated, err := client.RotateCredentialSecret(ctx, cred.CredentialsID)
err = client.DeleteCredential(ctx, cred.CredentialsID)
```

You may still pass a pre-obtained access token directly with
`buttrbase.New("<oauth2-access-token>")` if you manage the token grant
yourself.

### OAuth Start URL

```go
url := client.OAuthStartURL(
    buttrbase.OAuthProviderGoogle,
    appUUID,
    "https://app.example.com/auth/google/callback",
)
// 302 the browser to `url`; the backend signs a state token and bounces
// to the provider's authorize URL.
```

### Passkey support (WebAuthn)

Thin wrappers around the four passkey ceremony endpoints. The WebAuthn
challenge / credential blobs are pass-through `json.RawMessage` — no
webauthn helper library is pulled in. The browser does the actual ceremony
via `navigator.credentials.create / .get`.

```go
// Registration (requires an authenticated caller — passkey is added to the
// user's existing account).
challenge, err := client.PasskeyRegisterBegin(ctx)
// ... hand challenge.Challenge to the browser, get back a credential JSON ...
result, err := client.PasskeyRegisterComplete(ctx, buttrbase.PasskeyRegistrationComplete{
    RegistrationState: challenge.RegistrationState,
    Credential:        browserCredentialJSON,
})
fmt.Println(result.CredentialID)

// Authentication (anonymous):
ach, err := client.PasskeyAuthenticateBegin(ctx)
// ... browser produces an assertion ...
session, err := client.PasskeyAuthenticateComplete(ctx, buttrbase.PasskeyAuthComplete{
    AuthState:  ach.AuthState,
    Credential: browserAssertionJSON,
})

// List the signed-in user's enrolled passkeys (descending by CreatedAt):
passkeys, err := client.ListMyPasskeys(ctx)
for _, p := range passkeys {
    name := p.CredentialIDPrefix
    if p.Nickname != nil { name = *p.Nickname }
    fmt.Println(name, p.CredentialUUID)
}

// Revoke one by its CredentialUUID (owner check enforced server-side):
if err := client.DeleteMyPasskey(ctx, passkeys[0].CredentialUUID); err != nil { /* ... */ }
```

### SSO (OIDC / SAML)

```go
oidcURL, err := client.OidcAuthorizeURL(ctx, "connection-uuid")
samlURL, err := client.SamlAuthorizeURL(ctx, "connection-uuid")
```

### Windowed scope re-mint (scope-context)

Re-mint the caller's access token windowed to an explicit, gate-checked scope
subset (least-privilege "windowed" strategy). The caller must already hold a
valid access token; the granted set is always a subset of their effective
scopes. Only the access token is re-minted (the refresh token is unchanged).

```go
res, err := client.ScopeContext(ctx, buttrbase.ScopeContextRequest{
    RequestedScopes: []string{"orders:read", "orders:write"},
})
fmt.Println(res.Token)  // freshly minted, windowed access token
fmt.Println(res.Scopes) // deduped, sorted granted set
```

### Tenant home discovery (public)

Resolve an active tenant's public routing info for an org (and optionally an
app) so a client can target it directly. Returns a 404 error for unknown or
non-active tenants. No bearer token required.

```go
home, err := client.GetTenantHome(ctx, "org-uuid", nil) // omit app_id
fmt.Println(home.TenancyMode)
if home.HomeBaseURL != nil {
    fmt.Println(*home.HomeBaseURL)
}
```

## MFA / TOTP

```go
status, err := client.MfaStatusFull(ctx)
enrollment, err := client.MfaTotpEnroll(ctx)
_, err = client.MfaTotpActivate(ctx, "123456")
_, err = client.MfaTotpVerify(ctx, "123456")
_, err = client.MfaTotpChallenge(ctx)
codes, err := client.MfaGenerateRecoveryCodes(ctx)
_, err = client.MfaRedeemRecoveryCode(ctx, "recovery-code")
_, err = client.MfaTotpDisable(ctx)
```

## Step-Up Auth

```go
resp, err := client.AuthStepUp(ctx, "totp-code", false)
// client.AccessToken is auto-replaced with the elevated token
```

## Organization Security

```go
settings, err := client.GetSecuritySettings(ctx, "org-uuid")
_, err = client.UpdateSecuritySettings(ctx, "org-uuid", map[string]any{"mfa_required": true})

connections, err := client.ListSsoConnections(ctx, "org-uuid")
conn, err := client.CreateSsoConnection(ctx, "org-uuid", map[string]any{"provider": "okta"})

events, err := client.ListAuditEvents(ctx, "org-uuid")
```

## Sessions & Devices

```go
sessions, err := client.OrgSessionInventory(ctx, "org-uuid")
_, err = client.OrgRevokeAllSessions(ctx, "org-uuid")

accounts, err := client.ListDeviceAccounts(ctx, "device-uuid")
_, err = client.SwitchDeviceActiveAccount(ctx, "device-uuid", "account-uuid")
```

The signed-in end user can list and revoke their own registered device keys
(public-safe fields only; no private key material). Requires a bearer token.

```go
devices, err := client.ListDevices(ctx)
for _, d := range devices {
    fmt.Println(d.DeviceUUID, d.JKT)
}

// Revoke one by its device UUID (owner check enforced server-side):
res, err := client.RevokeDevice(ctx, devices[0].DeviceUUID)
fmt.Println(res.Revoked)
```

## App-Level OAuth Provider Configs

```go
// Register Google for the app
cfg, err := client.CreateOAuthConfig(ctx, appUUID, buttrbase.CreateOAuthConfigInput{
    Provider:     buttrbase.OAuthProviderGoogle,
    ClientID:     "google-client-id",
    ClientSecret: "google-client-secret",
    RedirectURIs: []string{"https://app.example.com/auth/google/callback"},
    Scopes:       []string{"openid", "email", "profile"},
    Enabled:      true,
})

// Patch — only non-nil fields are sent
enabled := false
_, err = client.UpdateOAuthConfig(ctx, appUUID, "google", buttrbase.UpdateOAuthConfigInput{
    Enabled: &enabled,
})

configs, err := client.ListOAuthConfigs(ctx, appUUID)
err = client.DeleteOAuthConfig(ctx, appUUID, "google")
```

## App-Level Audit Log

```go
rows, err := client.ReadAuditLog(ctx, appUUID, buttrbase.AuditLogQuery{
    Limit:        500,
    ActionPrefix: "oauth_config.",
})
for _, row := range rows {
    fmt.Println(row.CreatedAt, row.Action, row.TargetID)
}
```

## Entitlements

```go
check, err := client.EntitlementsCheck(ctx, "advanced-analytics", "org-uuid")
effective, err := client.EntitlementsEffective(ctx)
```

## Teams

```go
team, err := client.CreateTeam(ctx, map[string]any{"name": "Engineering"})
teams, err := client.ListOrgTeams(ctx, "org-uuid")
members, err := client.ListTeamMembers(ctx, "team-uuid")
_, err = client.AddTeamMember(ctx, "team-uuid", "user-uuid")
err = client.RemoveTeamMember(ctx, "team-uuid", "user-uuid")
```

## Admin: Signing Keys

```go
keys, err := client.ListSigningKeys(ctx, "org-uuid")
_, err = client.RotateSigningKeys(ctx, "org-uuid")
audit, err := client.ListSigningAudit(ctx, "org-uuid")
```

## Admin: mTLS CA

```go
ca, err := client.GetCA(ctx, "org-uuid")
ca, err = client.InitCA(ctx, "org-uuid", map[string]any{"common_name": "My CA"})
certs, err := client.ListCertificates(ctx, "org-uuid")
cert, err := client.IssueCertificate(ctx, "org-uuid", map[string]any{"csr": "..."})
```

## Admin: Secrets Vault

```go
secrets, err := client.ListSecrets(ctx, "org-uuid")
_, err = client.PutSecretAdmin(ctx, "org-uuid", "DB_URL", "postgres://...")
secret, err := client.GetSecretByName(ctx, "org-uuid", "DB_URL")
err = client.DeleteSecret(ctx, "org-uuid", "DB_URL")
```

## Admin: Domains & Webhooks

```go
domains, err := client.ListDomains(ctx, "org-uuid")
domain, err := client.CreateDomain(ctx, "org-uuid", "example.com")
_, err = client.VerifyDomain(ctx, "org-uuid", 1)

endpoints, err := client.ListWebhookEndpoints(ctx, "org-uuid")
ep, err := client.CreateWebhookEndpoint(ctx, "org-uuid", "https://hook.example.com", []string{"user.created"})
```

## Payments

```go
session, err := client.CreatePaymentCheckout(ctx, map[string]any{"amount": 5000})
invoice, err := client.SendInvoice(ctx, map[string]any{"amount": 5000, "customer_email": "buyer@example.com"})
```

## AI Gateway

```go
resp, err := client.AiChatCompletions(ctx, "org-uuid", "openai", map[string]any{
    "model": "gpt-4",
    "messages": []map[string]any{{"role": "user", "content": "Hello!"}},
})
```

## SMS & Email

```go
_, err := client.SendSms(ctx, "+15551234567", "Hello from ButtrBase!")
_, err = client.VerifyEmailIdentity(ctx, "user@example.com")
```

## Webhook Verification

```go
valid := buttrbase.VerifyWebhookSignature(
    body,
    signatureHeader,
    timestampHeader,
    "whsec_...",
    300, // tolerance in seconds
)
```

## Errors

Non-2xx responses return `*ButtrbaseError` with `StatusCode`, `Detail`, and `Body`.

## Recipes

### Complete Onboarding

```go
client := buttrbase.New("",
    buttrbase.WithClientCredentials("<client-id>", "<client-secret>"))
ctx := context.Background()

appUUID := "018f1234-5678-7000-8000-000000000001"

// 1. Register and login
_, err := client.Register(ctx, appUUID, "admin@acme.com", "s3cur3!", "Acme Corp",
    &buttrbase.RegisterOptions{FirstName: "Alice"})
login, err := client.Login(ctx, appUUID, "admin@acme.com", "s3cur3!", "")

// 2. Get profile
profile, err := client.GetProfile(ctx)

// 3. Create a team and add a member
team, err := client.CreateTeam(ctx, "org-uuid", map[string]any{"name": "Engineering"})
_, err = client.AddTeamMember(ctx, "org-uuid", team.UUID, map[string]any{"user_uuid": "colleague-uuid"})
```

### MFA Enrollment

```go
// 1. Check MFA status
status, err := client.MfaStatus(ctx)

// 2. Enroll in TOTP — returns secret + QR URL
enrollment, err := client.MfaEnroll(ctx, "")
fmt.Println("Scan this QR:", enrollment.QrCodeURL)

// 3. Activate with code from authenticator app
_, err = client.MfaActivate(ctx, "123456")

// 4. Generate recovery codes
codes, err := client.MfaGenerateRecoveryCodes(ctx)
```

### Checkout Flow

```go
// 1. Preview pricing
preview, err := client.PricingPreview(ctx, map[string]any{"plan": "pro", "seats": 10})

// 2. Check entitlement
check, err := client.CheckEntitlement(ctx, map[string]any{
    "feature_name": "advanced-analytics", "org_uuid": "org-uuid",
})

// 3. Create checkout session
session, err := client.PricingCheckoutSession(ctx, map[string]any{"plan": "pro", "seats": 10})
```

### SSO Setup

```go
// 1. Create an OIDC connection
conn, err := client.CreateSsoConnection(ctx, "org-uuid", map[string]any{
    "provider": "okta", "name": "Okta SSO",
})

// 2. Get the authorize URL
url, err := client.OidcAuthorizeURL(ctx, conn.ConnectionUUID)

// 3. Handle callback
resp, err := client.OidcCallback(ctx, conn.ConnectionUUID, map[string]any{"code": "auth-code"})
```

### Secrets & Key Management

```go
// 1. Store a secret
_, err := client.PutSecret(ctx, "org-uuid", "DATABASE_URL", "postgres://...", "DB connection")

// 2. List and retrieve secrets
secrets, err := client.ListSecrets(ctx, "org-uuid")
secret, err := client.GetSecret(ctx, "org-uuid", "DATABASE_URL")

// 3. Rotate signing keys — create new, revoke old
newKey, err := client.CreateSigningKey(ctx, "org-uuid", map[string]any{"algorithm": "ES256"})
audit, err := client.ListSigningKeys(ctx, "org-uuid")
```

## Docs

See https://buttrbase.com/docs for the full API reference.
