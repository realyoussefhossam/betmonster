package gateway

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/realyoussefhossam/betmonster/internal/auth"
	"github.com/realyoussefhossam/betmonster/internal/shared/server"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	logger              *slog.Logger
	wallet              *WalletClient
	jwksClient          *auth.JWKSClient
	limiter             *RateLimiter
	adminUserIDs        map[string]struct{}
	corsAllowedOrigins  map[string]struct{}
	corsAllowAll        bool
	supportedCurrencies []string
	supportedChains     []string
}

func NewServer(logger *slog.Logger, wallet *WalletClient, jwksClient *auth.JWKSClient, limiter *RateLimiter, adminUserIDs, corsAllowedOrigins, supportedCurrencies, supportedChains string) *Server {
	admins := map[string]struct{}{}
	for _, id := range strings.Split(adminUserIDs, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			admins[id] = struct{}{}
		}
	}

	origins := map[string]struct{}{}
	allowAll := false
	for _, o := range strings.Split(corsAllowedOrigins, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			allowAll = true
		} else {
			origins[o] = struct{}{}
		}
	}

	return &Server{
		logger:              logger,
		wallet:              wallet,
		jwksClient:          jwksClient,
		limiter:             limiter,
		adminUserIDs:        admins,
		corsAllowedOrigins:  origins,
		corsAllowAll:        allowAll,
		supportedCurrencies: splitTrim(supportedCurrencies),
		supportedChains:     splitTrim(supportedChains),
	}
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", server.WithRoutePattern("/health", http.HandlerFunc(s.handleHealth)))
	mux.Handle("/metrics", server.WithRoutePattern("/metrics", server.MetricsHandler()))
	mux.Handle("/api/wallet/supported", server.WithRoutePattern("/api/wallet/supported", http.HandlerFunc(s.handleSupported)))
	mux.Handle("/api/wallet/balance", server.WithRoutePattern("/api/wallet/balance", s.auth(http.HandlerFunc(s.handleBalance))))
	mux.Handle("/api/wallet/transactions", server.WithRoutePattern("/api/wallet/transactions", s.auth(http.HandlerFunc(s.handleTransactions))))
	mux.Handle("/api/wallet/deposit-address", server.WithRoutePattern("/api/wallet/deposit-address", s.auth(http.HandlerFunc(s.handleDepositAddress))))
	mux.Handle("/api/wallet/withdraw", server.WithRoutePattern("/api/wallet/withdraw", s.auth(http.HandlerFunc(s.handleWithdraw))))
	mux.Handle("/api/admin/withdrawals", server.WithRoutePattern("/api/admin/withdrawals", s.auth(s.admin(http.HandlerFunc(s.handleListPendingWithdrawals)))))
	mux.Handle("/api/admin/withdrawals/review", server.WithRoutePattern("/api/admin/withdrawals/review", s.auth(s.admin(http.HandlerFunc(s.handleReviewWithdrawal)))))
	mux.Handle("/webhooks/xcash/deposit", server.WithRoutePattern("/webhooks/xcash/deposit", http.HandlerFunc(s.handleXcashWebhook)))
	return server.RequestID(server.Logging(s.logger, server.Metrics(s.limiter.Middleware(s.cors(mux)))))
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		allowedOrigin := ""
		if s.corsAllowAll {
			allowed = true
			allowedOrigin = "*"
		} else if origin != "" {
			if _, ok := s.corsAllowedOrigins[origin]; ok {
				allowed = true
				allowedOrigin = origin
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"gateway"}`))
}

func (s *Server) handleSupported(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"currencies": s.supportedCurrencies,
		"chains":     s.supportedChains,
	})
}

func (s *Server) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.jwksClient.UserFromRequest(r.Context(), r)
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, err)
			return
		}
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next(w, r.WithContext(ctx))
	})
}

func (s *Server) admin(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(UserContextKey).(auth.User)
		if !ok {
			s.writeError(w, http.StatusUnauthorized, nil)
			return
		}
		if _, ok := s.adminUserIDs[user.ID]; !ok {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

func (s *Server) userFromContext(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := r.Context().Value(UserContextKey).(auth.User)
	if !ok {
		s.writeError(w, http.StatusUnauthorized, nil)
		return auth.User{}, false
	}
	return user, true
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	user, ok := s.userFromContext(w, r)
	if !ok {
		return
	}
	currency := r.URL.Query().Get("currency")
	if currency == "" {
		currency = "USDT"
	}
	resp, err := s.wallet.GetBalance(r.Context(), user.ID, currency)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleTransactions(w http.ResponseWriter, r *http.Request) {
	user, ok := s.userFromContext(w, r)
	if !ok {
		return
	}
	resp, err := s.wallet.ListTransactions(r.Context(), user.ID, 1, 20)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDepositAddress(w http.ResponseWriter, r *http.Request) {
	user, ok := s.userFromContext(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	currency := q.Get("currency")
	chain := q.Get("chain")
	if currency == "" {
		currency = "USDT"
	}
	if chain == "" {
		chain = "base"
	}
	resp, err := s.wallet.GetDepositAddress(r.Context(), user.ID, currency, chain)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	user, ok := s.userFromContext(w, r)
	if !ok {
		return
	}
	var body struct {
		Currency           string `json:"currency"`
		Amount             string `json:"amount"`
		DestinationAddress string `json:"destinationAddress"`
		Chain              string `json:"chain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Currency == "" {
		body.Currency = "USDT"
	}
	if body.Chain == "" {
		body.Chain = "base"
	}
	resp, err := s.wallet.RequestWithdrawal(r.Context(), user.ID, body.Currency, body.Amount, body.DestinationAddress, body.Chain)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListPendingWithdrawals(w http.ResponseWriter, r *http.Request) {
	resp, err := s.wallet.ListPendingWithdrawals(r.Context(), 1, 20)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleReviewWithdrawal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WithdrawalID string `json:"withdrawalId"`
		Action       string `json:"action"`
		TxHash       string `json:"txHash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	user, ok := s.userFromContext(w, r)
	if !ok {
		return
	}
	resp, err := s.wallet.ReviewWithdrawal(r.Context(), body.WithdrawalID, body.Action, body.TxHash, user.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleXcashWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	headers := map[string]string{}
	for _, name := range []string{"XC-Nonce", "XC-Timestamp", "XC-Signature"} {
		headers[name] = r.Header.Get(name)
	}
	resp, err := s.wallet.ProcessDepositWebhook(r.Context(), string(body), headers)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resp.ResponseBody))
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if msg, ok := v.(proto.Message); ok {
		b, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(msg)
		if err != nil {
			s.logger.Error("marshal proto json", "error", err)
			return
		}
		w.Write(b)
		return
	}

	json.NewEncoder(w).Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
