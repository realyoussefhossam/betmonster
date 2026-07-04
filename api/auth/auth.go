package auth

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

type User struct {
	ID    string
	Email string
	Name  string
}

var (
	ErrMissingUserID = errors.New("missing user id")
)

func UserFromRequest(r *http.Request) (User, error) {
	keyset, err := jwk.Fetch(r.Context(), "http://localhost:3000/api/auth/jwks")
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
