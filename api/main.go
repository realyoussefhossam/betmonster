package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/rs/cors"

	"github.com/realyoussefhossam/better-auth-go/api/auth"
	"github.com/realyoussefhossam/better-auth-go/api/middleware"
)

type AuthResponse struct {
	Status  string     `json:"status"`
	Message string     `json:"message"`
	User    *auth.User `json:"user,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type contextKey string

const UserContextKey contextKey = "user"

var logger *slog.Logger

func verifyAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Get user from request
	user, err := auth.UserFromRequest(r)
	if err != nil {
		slog.Error("failed to get user", slog.Any("error", err))

		// Return 401 Unauthorized with appropriate error message
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)

		errorMessage := "Authentication failed"

		errorResponse := ErrorResponse{
			Error: errorMessage,
		}
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	response := AuthResponse{
		Status:  "success",
		Message: "Token is valid",
		User:    &user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	router := http.NewServeMux()

	router.HandleFunc("/health", healthHandler)
	router.Handle("/api/auth/verify", http.HandlerFunc(verifyAuthHandler))
	router.Handle("/api/me", http.HandlerFunc(verifyAuthHandler))

	// Enable CORS
	handler := cors.AllowAll().Handler(middleware.Logging(logger, router))

	port := ":8080"

	fmt.Println("Server starting on port", port)

	if err := http.ListenAndServe(port, handler); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
