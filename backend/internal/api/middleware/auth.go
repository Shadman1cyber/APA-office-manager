package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/user"
)

type Identity struct {
	UserID string
	OrgID  string
	Role   user.Role
	Name   string
	Email  string
}

type contextKey int

const identityKey contextKey = 1

type Claims struct {
	UserID string `json:"uid"`
	OrgID  string `json:"oid"`
	Role   string `json:"role"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: 24 * time.Hour}
}

func (tm *TokenManager) Issue(id Identity) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: id.UserID,
		OrgID:  id.OrgID,
		Role:   string(id.Role),
		Name:   id.Name,
		Email:  id.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.ttl)),
			Issuer:    "apa-backend",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tm.secret)
}

var ErrInvalidToken = errors.New("invalid token")

func (tm *TokenManager) Parse(tokenString string) (*Identity, error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}
		return tm.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid || claims.UserID == "" || claims.OrgID == "" {
		return nil, fmt.Errorf("%w: missing claims", ErrInvalidToken)
	}
	return &Identity{
		UserID: claims.UserID,
		OrgID:  claims.OrgID,
		Role:   user.Role(claims.Role),
		Name:   claims.Name,
		Email:  claims.Email,
	}, nil
}

func Auth(tm *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				writeAuthError(w, "برای ادامه ابتدا وارد شوید.")
				return
			}
			identity, err := tm.Parse(strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				writeAuthError(w, "نشست شما منقضی شده است؛ دوباره وارد شوید.")
				return
			}
			ctx := context.WithValue(r.Context(), identityKey, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": "unauthorized", "message": message},
	})
}

func IdentityFrom(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(identityKey).(*Identity)
	return id, ok
}

func RequireIdentity(ctx context.Context) (*Identity, error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("%w: no authenticated identity in context", domain.ErrUnauthorized)
	}
	return id, nil
}
