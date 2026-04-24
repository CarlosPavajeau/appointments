package jwt

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// DBVerifier validates JWTs using public keys fetched from the database jwks table.
// It is safe for concurrent use.
type DBVerifier struct {
	dbtx   db.DBTX
	issuer string // optional; empty = skip iss validation
}

// NewDBVerifier creates a DBVerifier backed by the given database connection.
// When issuer is non-empty, the "iss" JWT claim must match it.
func NewDBVerifier(dbtx db.DBTX, issuer string) *DBVerifier {
	return &DBVerifier{dbtx: dbtx, issuer: issuer}
}

// lookupKey fetches the public key for the given kid from the database.
func (v *DBVerifier) lookupKey(ctx context.Context, kid string) (any, error) {
	row, err := db.Query.FindJWKByID(ctx, v.dbtx, kid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("signing key %q not found", kid)
	}
	if err != nil {
		return nil, fmt.Errorf("jwk lookup: %w", err)
	}

	var entry jwkEntry
	if err := json.Unmarshal([]byte(row.PublicKey), &entry); err != nil {
		return nil, fmt.Errorf("parse public key for kid %q: %w", kid, err)
	}
	return parseJWK(entry)
}

// VerifyToken validates the JWT string and returns its claims.
func (v *DBVerifier) VerifyToken(ctx context.Context, tokenStr string) (*Claims, error) {
	kid, alg, err := extractTokenHeader(tokenStr)
	if err != nil {
		return nil, errors.New("malformed token")
	}

	// Enforce algorithm allowlist before touching any key material.
	// This prevents algorithm confusion attacks (e.g. RS256 → HS256, alg:none).
	if !isAllowedAlg(alg) {
		return nil, fmt.Errorf("algorithm %q is not permitted", alg)
	}

	key, err := v.lookupKey(ctx, kid)
	if err != nil {
		logger.Warn("[jwt] key lookup failed", "kid", kid, "err", err)
		return nil, err
	}

	opts := []gojwt.ParserOption{
		gojwt.WithLeeway(10 * time.Second), // tolerate small clock skew
	}

	if v.issuer != "" {
		opts = append(opts, gojwt.WithIssuer(v.issuer))
	}

	token, err := gojwt.ParseWithClaims(tokenStr, &Claims{},
		func(t *gojwt.Token) (any, error) {
			// Re-confirm the algorithm after full header parsing.
			switch t.Method.(type) {
			case *gojwt.SigningMethodRSA, *gojwt.SigningMethodECDSA, *gojwt.SigningMethodEd25519:
				return key, nil
			default:
				return nil, errors.New("unexpected signing method")
			}
		},
		opts...,
	)
	if err != nil {
		logger.Warn("[jwt] ParseWithClaims failed", "kid", kid, "alg", alg, "err", err)
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		logger.Warn("[jwt] token parsed but claims are invalid", "ok", ok, "valid", token.Valid)
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
