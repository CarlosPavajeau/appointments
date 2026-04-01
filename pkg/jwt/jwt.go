package jwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
	"wappiz/pkg/logger"

	"github.com/gin-gonic/gin"
	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	// AccessTokenDuration is kept for callers that reference it externally.
	AccessTokenDuration = 15 * time.Minute

	jwksCacheTTL     = 15 * time.Minute
	jwksFetchTimeout = 10 * time.Second
	jwksMaxBodyBytes = 64 * 1024 // 64 KB — prevent memory exhaustion
	minForceRefresh  = 5 * time.Second
)

// Claims holds the JWT payload expected from the external auth service.
// Field names must match what the external issuer embeds.
type Claims struct {
	UserID string `json:"id"`
	Role   string `json:"role"`
	gojwt.RegisteredClaims
}

// TenantIDLookup resolves a tenant UUID from a user ID.
// It is called by AuthMiddleware after token verification to populate the
// tenant_id context value. Set it via InitTenantFinder at startup.
type TenantIDLookup func(ctx context.Context, userID string) (uuid.UUID, error)

// ─── JWKS verifier ───────────────────────────────────────────────────────────

// Verifier fetches and caches public keys from a JWKS endpoint, then uses them
// to validate JWTs. It is safe for concurrent use.
type Verifier struct {
	jwksURL    string
	issuer     string // optional; empty = skip iss validation
	httpClient *http.Client

	fetchMu   sync.Mutex   // serialises HTTP fetches
	cacheMu   sync.RWMutex // protects cache and fetchedAt
	cache     map[string]any
	fetchedAt time.Time
}

// NewVerifier creates a Verifier backed by the given JWKS URL.
// When issuer is non-empty, the "iss" JWT claim must match it.
func NewVerifier(jwksURL, issuer string) *Verifier {
	return &Verifier{
		jwksURL:    jwksURL,
		issuer:     issuer,
		httpClient: &http.Client{Timeout: jwksFetchTimeout},
		cache:      make(map[string]any),
	}
}

// jwkEntry is the wire representation of a single JSON Web Key.
type jwkEntry struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	// RSA
	N string `json:"n"`
	E string `json:"e"`
	// EC / OKP
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"` // not present in OKP
}

type jwksResponse struct {
	Keys []jwkEntry `json:"keys"`
}

// doHTTPFetch performs the actual JWKS HTTP request and replaces the cache.
// Caller must hold fetchMu.
func (v *Verifier) doHTTPFetch() error {
	resp, err := v.httpClient.Get(v.jwksURL)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, jwksMaxBodyBytes))
	if err != nil {
		return fmt.Errorf("read jwks body: %w", err)
	}

	var set jwksResponse
	if err := json.Unmarshal(body, &set); err != nil {
		return fmt.Errorf("parse jwks: %w", err)
	}

	next := make(map[string]any, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kid == "" {
			logger.Info("[jwt] skipping JWKS entry with empty kid",
				"kty", k.Kty,
				"alg", k.Alg)
			continue
		}
		if k.Use != "" && k.Use != "sig" {
			logger.Info("[jwt] skipping JWKS key (not a signing key)",
				"kid", k.Kid,
				"use", k.Use)
			continue // skip non-signing keys (e.g. encryption keys)
		}
		pub, err := parseJWK(k)
		if err != nil {
			logger.Info("[jwt] skipping malformed JWKS key",
				"kid", k.Kid,
				"kty", k.Kty,
				"err", err)
			continue // skip malformed keys; don't fail the whole set
		}
		next[k.Kid] = pub
	}

	logger.Info("[jwt] JWKS refreshed",
		"keys", len(next),
		"url", v.jwksURL)

	v.cacheMu.Lock()
	v.cache = next
	v.fetchedAt = time.Now()
	v.cacheMu.Unlock()

	return nil
}

// refresh fetches new JWKS keys, serialised by fetchMu.
//
// When force is false (TTL expiry), a double-check prevents redundant fetches
// when multiple goroutines race to refresh a stale cache.
//
// When force is true (unknown kid), the double-check is replaced by a
// rate-limit (minForceRefresh) to prevent DoS through fabricated kid values.
func (v *Verifier) refresh(force bool) error {
	v.fetchMu.Lock()
	defer v.fetchMu.Unlock()

	v.cacheMu.RLock()
	age := time.Since(v.fetchedAt)
	v.cacheMu.RUnlock()

	if force {
		if age < minForceRefresh {
			return nil // rate-limit forced refreshes
		}
	} else {
		if age < jwksCacheTTL {
			return nil // another goroutine already refreshed
		}
	}

	return v.doHTTPFetch()
}

// lookupKey returns the cached public key for the given kid.
// It refreshes the cache on TTL expiry or when the kid is absent.
func (v *Verifier) lookupKey(kid string) (any, error) {
	v.cacheMu.RLock()
	key, ok := v.cache[kid]
	fresh := time.Since(v.fetchedAt) < jwksCacheTTL
	v.cacheMu.RUnlock()

	if ok && fresh {
		return key, nil // fast path: cache hit with fresh data
	}

	// Slow path: stale cache (force=false) or unknown kid in fresh cache (force=true).
	forceRefresh := !ok && fresh
	if err := v.refresh(forceRefresh); err != nil {
		return nil, fmt.Errorf("jwks refresh: %w", err)
	}

	v.cacheMu.RLock()
	key, ok = v.cache[kid]
	v.cacheMu.RUnlock()

	if !ok {
		v.cacheMu.RLock()
		cachedKids := make([]string, 0, len(v.cache))
		for k := range v.cache {
			cachedKids = append(cachedKids, k)
		}
		v.cacheMu.RUnlock()
		logger.Info("[jwt] signing key not found in JWKS after refresh",
			"kid", kid,
			"kids", strings.Join(cachedKids, ","))
		return nil, fmt.Errorf("signing key %q not found in JWKS", kid)
	}
	return key, nil
}

// VerifyToken validates the JWT string and returns its claims.
func (v *Verifier) VerifyToken(tokenStr string) (*Claims, error) {
	kid, alg, err := extractTokenHeader(tokenStr)
	if err != nil {
		logger.Info("[jwt] malformed token header",
			"err", err)
		return nil, errors.New("malformed token")
	}

	logger.Info("[jwt] verifying token",
		"kid", kid,
		"alg", alg)

	// Enforce algorithm allowlist before touching any key material.
	// This prevents algorithm confusion attacks (e.g. RS256 → HS256, alg:none).
	if !isAllowedAlg(alg) {
		logger.Warn("[jwt] rejected: algorithm not in allowlist",
			"alg", alg)
		return nil, fmt.Errorf("algorithm %q is not permitted", alg)
	}

	key, err := v.lookupKey(kid)
	if err != nil {
		logger.Warn("[jwt] key lookup failed",
			"kid", kid,
			"err", err)
		return nil, err
	}

	opts := []gojwt.ParserOption{
		gojwt.WithLeeway(10 * time.Second), // tolerate small clock skew
	}
	if v.issuer != "" {
		opts = append(opts, gojwt.WithIssuer(v.issuer))
		logger.Info("[jwt] issuer validation enabled",
			"iss", v.issuer)
	}

	token, err := gojwt.ParseWithClaims(tokenStr, &Claims{},
		func(t *gojwt.Token) (any, error) {
			// Re-confirm the algorithm after full header parsing.
			switch t.Method.(type) {
			case *gojwt.SigningMethodRSA, *gojwt.SigningMethodECDSA, *gojwt.SigningMethodEd25519:
				return key, nil
			default:
				logger.Warn("[jwt] rejected: unexpected signing method",
					"method", fmt.Sprintf("%T", t.Method))
				return nil, errors.New("unexpected signing method")
			}
		},
		opts...,
	)
	if err != nil {
		logger.Warn("[jwt] ParseWithClaims failed",
			"kid", kid,
			"alg", alg,
			"err", err)
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		logger.Warn("[jwt] token parsed but claims are invalid",
			"ok", ok,
			"valid", token.Valid)
		return nil, errors.New("invalid token")
	}

	logger.Info("[jwt] token valid",
		"user_id", claims.UserID,
		"role", claims.Role)
	return claims, nil
}

// ─── JWKS key parsing ────────────────────────────────────────────────────────

func parseJWK(k jwkEntry) (any, error) {
	switch k.Kty {
	case "RSA":
		return parseRSAKey(k)
	case "EC":
		return parseECKey(k)
	case "OKP":
		return parseOKPKey(k)
	default:
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
}

func parseRSAKey(k jwkEntry) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	eInt := new(big.Int).SetBytes(eBytes)

	if !eInt.IsInt64() || eInt.Int64() > (1<<31-1) {
		return nil, errors.New("RSA exponent out of range")
	}

	pub := &rsa.PublicKey{N: n, E: int(eInt.Int64())}
	if pub.N.BitLen() < 2048 {
		return nil, fmt.Errorf("RSA key too short: %d bits (minimum 2048)", pub.N.BitLen())
	}
	return pub, nil
}

func parseECKey(k jwkEntry) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported EC curve %q", k.Crv)
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}

	pub := &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}
	if !curve.IsOnCurve(pub.X, pub.Y) {
		return nil, errors.New("EC point is not on curve")
	}
	return pub, nil
}

func parseOKPKey(k jwkEntry) (ed25519.PublicKey, error) {
	if k.Crv != "Ed25519" {
		return nil, fmt.Errorf("unsupported OKP curve %q (only Ed25519 is supported)", k.Crv)
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	if len(xBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("ed25519 public key must be %d bytes, got %d", ed25519.PublicKeySize, len(xBytes))
	}
	return ed25519.PublicKey(xBytes), nil
}

// extractTokenHeader decodes the JWT header segment to read kid and alg
// without performing any signature verification.
func extractTokenHeader(tokenStr string) (kid, alg string, err error) {
	dot := strings.IndexByte(tokenStr, '.')
	if dot < 0 {
		return "", "", errors.New("missing separator")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(tokenStr[:dot])
	if err != nil {
		return "", "", fmt.Errorf("decode header: %w", err)
	}

	var h struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &h); err != nil {
		return "", "", fmt.Errorf("parse header: %w", err)
	}
	return h.Kid, h.Alg, nil
}

// isAllowedAlg returns true for asymmetric algorithms only.
// HMAC algorithms and "none" are intentionally excluded to prevent
// algorithm confusion attacks.
func isAllowedAlg(alg string) bool {
	switch alg {
	case "RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "EdDSA":
		return true
	default:
		return false
	}
}

// ─── Package-level initialisation ────────────────────────────────────────────

var defaultVerifier *Verifier
var defaultTenantFinder TenantIDLookup

// InitTenantFinder registers the function used by AuthMiddleware to resolve
// a tenant UUID from the authenticated user ID. Call this at startup after
// the tenant use-case is initialised.
func InitTenantFinder(f TenantIDLookup) {
	defaultTenantFinder = f
}

// Init initialises the package-level JWKS verifier.
// It must be called once at application startup, before any requests are served.
// The initial JWKS fetch is performed eagerly so the service fails fast if the
// authentication endpoint is unreachable.
func Init(jwksURL, issuer string) error {
	v := NewVerifier(jwksURL, issuer)
	if err := v.refresh(true); err != nil {
		return fmt.Errorf("initial jwks fetch failed: %w", err)
	}
	defaultVerifier = v
	return nil
}

// ─── Gin middleware ───────────────────────────────────────────────────────────

// AuthMiddleware is a Gin middleware that validates Bearer JWTs issued by the
// external authentication service. Init must be called before routes are served.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if defaultVerifier == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				gin.H{"error": "auth verifier not initialised"})
			return
		}

		header := c.GetHeader("Authorization")
		if len(header) < 8 || header[:7] != "Bearer " {
			logger.Info("[jwt] missing or malformed Authorization header",
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"len", len(header))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		claims, err := defaultVerifier.VerifyToken(header[7:])
		if err != nil {
			if errors.Is(err, gojwt.ErrTokenExpired) {
				logger.Warn("[jwt] token expired",
					"method", c.Request.Method,
					"path", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "token_expired",
					"hint":  "obtain a new token from the authentication service",
				})
				return
			}
			logger.Warn("[jwt] invalid token",
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"err", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)

		if defaultTenantFinder != nil {
			tenantID, err := defaultTenantFinder(c.Request.Context(), claims.UserID)
			if err != nil {
				logger.Warn("[jwt] tenant lookup failed",
					"user_id", claims.UserID,
					"err", err)
			} else {
				c.Set("tenant_id", tenantID)
			}
		}

		c.Next()
	}
}

// ─── Context helpers ─────────────────────────────────────────────────────────

func TenantIDFromContext(c *gin.Context) uuid.UUID {
	return c.MustGet("tenant_id").(uuid.UUID)
}

// TenantIDFromContextOK returns the tenant UUID and whether it was present.
// Use this when the tenant may not exist yet (e.g. first-time registration).
func TenantIDFromContextOK(c *gin.Context) (uuid.UUID, bool) {
	v, exists := c.Get("tenant_id")
	if !exists {
		return uuid.UUID{}, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

func UserIDFromContext(c *gin.Context) string {
	return c.MustGet("user_id").(string)
}
