package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const (
	jwksRefreshInterval = 15 * time.Minute
	jwksFetchTimeout    = 5 * time.Second
)

var (
	ErrMissingUserID = errors.New("missing user id")
)

type User struct {
	ID    string
	Email string
	Name  string
}

type JWKSClient struct {
	mu       sync.RWMutex
	jwksURL  string
	keyset   jwk.Set
	fetched  time.Time
	httpClient *http.Client
}

func NewJWKSClient(ctx context.Context, jwksURL string) (*JWKSClient, error) {
	c := &JWKSClient{
		jwksURL:    jwksURL,
		httpClient: &http.Client{Timeout: jwksFetchTimeout},
	}
	// Attempt an initial fetch but do not block startup on it.
	// The first request will retry if the fetch failed or if keys are stale.
	if err := c.refresh(ctx); err != nil {
		// Continue; keys will be fetched lazily on the first request.
		_ = err
	}
	return c, nil
}

func (c *JWKSClient) refresh(ctx context.Context) error {
	keyset, err := jwk.Fetch(ctx, c.jwksURL, jwk.WithHTTPClient(c.httpClient))
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keyset = keyset
	c.fetched = time.Now()
	return nil
}

func (c *JWKSClient) currentKeyset(ctx context.Context) (jwk.Set, error) {
	c.mu.RLock()
	if c.keyset != nil && time.Since(c.fetched) < jwksRefreshInterval {
		ks := c.keyset
		c.mu.RUnlock()
		return ks, nil
	}
	c.mu.RUnlock()

	if err := c.refresh(ctx); err != nil {
		c.mu.RLock()
		if c.keyset != nil {
			ks := c.keyset
			c.mu.RUnlock()
			return ks, nil
		}
		c.mu.RUnlock()
		return nil, err
	}
	return c.keyset, nil
}

func (c *JWKSClient) UserFromRequest(ctx context.Context, r *http.Request) (User, error) {
	keyset, err := c.currentKeyset(ctx)
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
	keyset, err := jwk.Fetch(r.Context(), jwksURL, jwk.WithHTTPClient(&http.Client{Timeout: jwksFetchTimeout}))
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
