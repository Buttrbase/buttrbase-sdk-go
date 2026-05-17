# Go SDK

## Overview

The official Go SDK for ButtrBase. Standard `net/http`-based client covering every API surface — auth, organizations, billing, RBAC, teams, credentials, search, AI gateway, webhooks, zero-trust, and more.

## Installation

```bash
go get github.com/buttrbase/buttrbase-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    buttrbase "github.com/buttrbase/buttrbase-go"
)

func main() {
    client := buttrbase.New("bb_live_...")
    ctx := context.Background()

    // Login
    resp, err := client.Login(ctx, "user@example.com", "password", "acme")
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
resp, err := client.Register(ctx, "user@example.com", "password", "acme",
    &buttrbase.RegisterOptions{FirstName: "Jane", LastName: "Doe"})
```

### Magic Link

```go
redirectURL := "https://app.example.com"
_, err := client.MagicLinkSendV2(ctx, "user@example.com", &redirectURL)
resp, err := client.MagicLinkVerifyV2(ctx, "token-from-email")
```

### OTP (Passwordless Phone)

```go
_, err := client.OtpSendV2(ctx, "+15551234567")
resp, err := client.OtpVerifyV2(ctx, "+15551234567", "123456")
```

### SSO (OIDC / SAML)

```go
oidcURL, err := client.OidcAuthorizeURL(ctx, "connection-uuid")
samlURL, err := client.SamlAuthorizeURL(ctx, "connection-uuid")
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
// client.APIKey is auto-replaced with the elevated token
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

## API Keys v2

```go
keys, err := client.ListApiKeysV2(ctx, "org-uuid")
newKey, err := client.CreateApiKeyV2(ctx, "org-uuid", "my-api-key")
err = client.DeleteApiKeyV2(ctx, "org-uuid", "key-uuid")
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

## Docs

See https://buttrbase.com/docs for the full API reference.
