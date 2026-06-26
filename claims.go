package buttrbase

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// TokenClaimsData mirrors the buttrbase `data` envelope carried inside an
// access-token JWT. All fields are optional; tokens without a `data` object
// deserialize to a zero-value struct.
//
// roles is a comma/space-delimited string of role identifiers
// (e.g. "owner" or "org_admin,leadership"). Call AuthContext.Roles for the
// already-split slice.
type TokenClaimsData struct {
	// Roles is a comma/space-delimited string of role identifiers.
	// Use AuthContext.Roles for the pre-split []string form.
	Roles    *string `json:"roles,omitempty"`
	Email    *string `json:"email,omitempty"`
	OrgUUID  *string `json:"org_uuid,omitempty"`
	UserUUID *string `json:"user_uuid,omitempty"`
}

// TokenClaims represents the standard buttrbase JWT payload. It is returned
// by ParseTokenClaims and carries all registered claims plus the buttrbase
// extensions (org, scope, data).
//
// ParseTokenClaims only decodes the payload — it does NOT verify the
// signature. Always verify the token via the Buttrbase JWKS before trusting
// these values in a security context.
type TokenClaims struct {
	Sub   string   `json:"sub"`
	Org   string   `json:"org"`
	Exp   int64    `json:"exp"`
	Iat   int64    `json:"iat"`
	Scope []string `json:"scope"`
	// Data carries the optional identity-enrichment envelope.
	Data *TokenClaimsData `json:"data,omitempty"`
}

// AuthContext is the principal representation derived from a TokenClaims.
// It exposes the enriched identity fields (Roles, Email) alongside the
// standard identifiers (UserID, OrgID, Scopes).
type AuthContext struct {
	// UserID is the subject UUID (sub claim).
	UserID string
	// OrgID is the organization UUID (org claim).
	OrgID string
	// Scopes is the list of OAuth2 scopes granted.
	Scopes []string
	// Roles is derived by splitting the data.roles comma/space-delimited
	// string. Empty when the token carries no data.roles.
	Roles []string
	// Email is the user's email from the data envelope. Nil when absent.
	Email *string
}

// AuthContext converts the claims into an AuthContext, splitting the
// comma/space-delimited data.roles string into a []string slice.
func (c TokenClaims) AuthContext() AuthContext {
	var roles []string
	var email *string

	if c.Data != nil {
		if c.Data.Roles != nil && *c.Data.Roles != "" {
			raw := *c.Data.Roles
			parts := strings.FieldsFunc(raw, func(r rune) bool {
				return r == ',' || r == ' '
			})
			for _, p := range parts {
				if p != "" {
					roles = append(roles, p)
				}
			}
		}
		email = c.Data.Email
	}

	if roles == nil {
		roles = []string{}
	}

	return AuthContext{
		UserID: c.Sub,
		OrgID:  c.Org,
		Scopes: c.Scope,
		Roles:  roles,
		Email:  email,
	}
}

// ParseTokenClaims decodes the payload of a JWT string and returns the
// buttrbase-specific claims, including the optional data envelope with Roles
// and Email.
//
// WARNING: This function does NOT verify the token signature or expiry.
// Always verify the token against the Buttrbase JWKS endpoint before using
// the returned claims in a security-sensitive context.
func ParseTokenClaims(tokenString string) (TokenClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return TokenClaims{}, fmt.Errorf("buttrbase: malformed JWT: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return TokenClaims{}, fmt.Errorf("buttrbase: JWT payload base64url decode: %w", err)
	}

	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return TokenClaims{}, fmt.Errorf("buttrbase: JWT payload JSON unmarshal: %w", err)
	}

	return claims, nil
}
