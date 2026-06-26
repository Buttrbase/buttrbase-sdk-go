package buttrbase

// Verifier provides JWKS-backed RS256 signature verification for buttrbase
// access tokens. Construct one at startup, share it across handlers (it is
// safe for concurrent use), and call VerifyToken / VerifyBearer from every
// authenticated endpoint.
//
// The Verifier owns a live JWKS cache that automatically refreshes from the
// configured URL (backed by MicahParks/keyfunc + MicahParks/jwkset).
// Signature verification mirrors the Rust SDK's Verifier.

import (
	"context"
	"fmt"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// VerifierConfig holds the public discovery configuration for a Verifier.
// All fields are public-configuration (no secrets).
//
// Audience is optional. buttrbase access tokens do not always carry a stable
// aud claim (magic-link tokens set aud to the org name; client-credential
// tokens omit it). Leave Audience empty to skip audience validation and rely
// on the issuer + RS256 signature + org/sub claims alone. Set it only when
// you mint tokens with a known, fixed audience.
type VerifierConfig struct {
	// JWKSURL is the URL of the JWKS discovery endpoint, e.g.
	// "https://auth.buttrbase.com/.well-known/jwks.json".
	JWKSURL string
	// Issuer is the expected iss claim value, e.g. "https://auth.buttrbase.com".
	Issuer string
	// Audience is the expected aud claim value. Leave empty to skip aud
	// validation (the common case for buttrbase tokens).
	Audience string
}

// Verifier verifies buttrbase RS256 JWTs against a live JWKS. It is safe
// for concurrent use. The JWKS cache is started on construction and runs
// until the context passed to NewVerifierCtx is cancelled (or indefinitely
// when using NewVerifier).
type Verifier struct {
	config  VerifierConfig
	keyfunc keyfunc.Keyfunc
	ctx     context.Context
}

// jwtClaims is the internal parse target used with jwt.ParseWithClaims.
// It embeds jwt.RegisteredClaims (which satisfies jwt.Claims) and adds the
// buttrbase-specific fields. After parsing we convert to the public
// TokenClaims type so callers never see this type.
type jwtClaims struct {
	jwt.RegisteredClaims

	// buttrbase custom claims
	Org   string   `json:"org"`
	Scope []string `json:"scope"`
	// Data carries the optional identity-enrichment envelope.
	Data *TokenClaimsData `json:"data,omitempty"`
}

// toTokenClaims converts the internal parse target to the public TokenClaims.
func (j jwtClaims) toTokenClaims() TokenClaims {
	var exp, iat int64
	if j.ExpiresAt != nil {
		exp = j.ExpiresAt.Unix()
	}
	if j.IssuedAt != nil {
		iat = j.IssuedAt.Unix()
	}
	scope := j.Scope
	if scope == nil {
		scope = []string{}
	}
	return TokenClaims{
		Sub:   j.Subject,
		Org:   j.Org,
		Exp:   exp,
		Iat:   iat,
		Scope: scope,
		Data:  j.Data,
	}
}

// NewVerifier creates a Verifier with a background JWKS refresh goroutine
// that runs until the process exits. Equivalent to NewVerifierCtx with a
// background context.
func NewVerifier(cfg VerifierConfig) (*Verifier, error) {
	return NewVerifierCtx(context.Background(), cfg)
}

// NewVerifierCtx creates a Verifier. The context controls the lifetime of the
// JWKS refresh goroutine — cancel it to stop background fetches.
func NewVerifierCtx(ctx context.Context, cfg VerifierConfig) (*Verifier, error) {
	kf, err := keyfunc.NewDefaultCtx(ctx, []string{cfg.JWKSURL})
	if err != nil {
		return nil, fmt.Errorf("buttrbase: create JWKS keyfunc: %w", err)
	}
	return &Verifier{config: cfg, keyfunc: kf, ctx: ctx}, nil
}

// newVerifierFromKeyfunc is used by tests to inject a pre-built keyfunc
// (backed by an httptest.Server JWKS) without real network calls.
func newVerifierFromKeyfunc(cfg VerifierConfig, kf keyfunc.Keyfunc) *Verifier {
	return &Verifier{config: cfg, keyfunc: kf, ctx: context.Background()}
}

// VerifyToken verifies an RS256 JWT string against the JWKS, validates the
// issuer (and audience when configured), and returns the enriched
// TokenClaims on success.
//
// The JWKS key is resolved by kid. If the kid is unknown the cache is
// refreshed once before failing.
func (v *Verifier) VerifyToken(tokenString string) (*TokenClaims, error) {
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.config.Issuer),
	}
	if v.config.Audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.config.Audience))
	}

	claims := &jwtClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, v.keyfunc.Keyfunc, parserOpts...)
	if err != nil {
		return nil, fmt.Errorf("buttrbase: token verification failed: %w", err)
	}

	tc := claims.toTokenClaims()
	return &tc, nil
}

// VerifyBearer extracts the token from an HTTP Authorization header of the
// form "Bearer <token>", verifies it, and returns the derived AuthContext.
//
// Returns an error if the header is absent, malformed, or the token fails
// verification.
func (v *Verifier) VerifyBearer(authHeader string) (*AuthContext, error) {
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader || token == "" {
		return nil, fmt.Errorf("buttrbase: missing or malformed Authorization header (expected 'Bearer <token>')")
	}

	claims, err := v.VerifyToken(token)
	if err != nil {
		return nil, err
	}

	ac := claims.AuthContext()
	return &ac, nil
}

// Issuer returns the configured issuer value. Useful for diagnostics.
func (v *Verifier) Issuer() string { return v.config.Issuer }

// Audience returns the configured audience value, or empty string if none
// is configured (meaning aud validation is disabled). Useful for diagnostics.
func (v *Verifier) Audience() string { return v.config.Audience }
