package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const (
	jwksRefreshInterval = 15 * time.Minute
)

type User struct {
	ID    string
	Email string
	Name  string
}

var (
	ErrMissingUserID = errors.New("missing user id")
)

type JWKSClient struct {
	cache   *jwk.Cache
	jwksURL string
}

func NewJWKSClient(ctx context.Context, jwksURL string) (*JWKSClient, error) {
	client := httprc.NewClient()
	cache, err := jwk.NewCache(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("create jwk cache: %w", err)
	}
	if err := cache.Register(ctx, jwksURL, jwk.WithConstantInterval(jwksRefreshInterval)); err != nil {
		return nil, fmt.Errorf("register jwks url: %w", err)
	}
	return &JWKSClient{cache: cache, jwksURL: jwksURL}, nil
}

func (c *JWKSClient) UserFromRequest(ctx context.Context, r *http.Request) (User, error) {
	keyset, err := c.cache.Lookup(ctx, c.jwksURL)
	if err != nil {
		return User{}, fmt.Errorf("fetch jwks: %w", err)
	}

	token, err := jwt.ParseRequest(r, jwt.WithKeySet(keyset))
	if err != nil {
		return User{}, fmt.Errorf("parse request: %w", err)
	}

	userID, exists := token.Subject()
	if !exists {
		return User{}, ErrMissingUserID
	}

	var email string
	var name string

	token.Get("email", &email)
	token.Get("name", &name)

	return User{
		ID:    userID,
		Email: email,
		Name:  name,
	}, nil
}

// UserFromRequest is kept for backward compatibility but fetches uncached keys.
// Prefer constructing a JWKSClient and using UserFromRequest on it.
func UserFromRequest(r *http.Request) (User, error) {
	jwksURL := os.Getenv("JWKS_URL")
	if jwksURL == "" {
		jwksURL = "http://localhost:3000/api/auth/jwks"
	}
	keyset, err := jwk.Fetch(r.Context(), jwksURL)
	if err != nil {
		return User{}, fmt.Errorf("fetch jwks: %w", err)
	}

	token, err := jwt.ParseRequest(r, jwt.WithKeySet(keyset))
	if err != nil {
		return User{}, fmt.Errorf("parse request: %w", err)
	}

	userID, exists := token.Subject()
	if !exists {
		return User{}, ErrMissingUserID
	}

	var email string
	var name string

	token.Get("email", &email)
	token.Get("name", &name)

	return User{
		ID:    userID,
		Email: email,
		Name:  name,
	}, nil
}
