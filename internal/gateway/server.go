package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/realyoussefhossam/betmonster/internal/auth"
	"github.com/realyoussefhossam/betmonster/internal/shared/server"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	logger              *slog.Logger
	wallet              *WalletClient
	oddsfeed            *OddsFeedClient
	sportsbook          *SportsbookClient
	jwksClient          *auth.JWKSClient
	limiter             *RateLimiter
	adminUserIDs        map[string]struct{}
	corsAllowedOrigins  map[string]struct{}
	corsAllowAll        bool
	supportedCurrencies []string
	supportedChains     []string
	supportedPairs      map[string]struct{}
	limits              Limits
}

func NewServer(logger *slog.Logger, wallet *WalletClient, oddsfeed *OddsFeedClient, sportsbook *SportsbookClient, jwksClient *auth.JWKSClient, limiter *RateLimiter, adminUserIDs, corsAllowedOrigins, supportedCurrencies, supportedChains, supportedPairs string, limits Limits) *Server {
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

	pairs := parsePairs(supportedPairs)
	currencies := splitTrim(supportedCurrencies)
	chains := splitTrim(supportedChains)
	if len(pairs) > 0 {
		currencies, chains = deriveLists(pairs, currencies, chains)
	}

	return &Server{
		logger:              logger,
		wallet:              wallet,
		oddsfeed:            oddsfeed,
		sportsbook:          sportsbook,
		jwksClient:          jwksClient,
		limiter:             limiter,
		adminUserIDs:        admins,
		corsAllowedOrigins:  origins,
		corsAllowAll:        allowAll,
		supportedCurrencies: currencies,
		supportedChains:     chains,
		supportedPairs:      pairs,
		limits:              limits,
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

func parsePairs(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		parts := strings.SplitN(p, ":", 2)
		if len(parts) != 2 {
			continue
		}
		currency := strings.TrimSpace(parts[0])
		chain := strings.TrimSpace(parts[1])
		if currency == "" || chain == "" {
			continue
		}
		out[currency+":"+chain] = struct{}{}
	}
	return out
}

func deriveLists(pairs map[string]struct{}, currencies, chains []string) ([]string, []string) {
	curSet := map[string]struct{}{}
	chainSet := map[string]struct{}{}
	for pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		curSet[parts[0]] = struct{}{}
		chainSet[parts[1]] = struct{}{}
	}
	if len(currencies) == 0 {
		for c := range curSet {
			currencies = append(currencies, c)
		}
	}
	if len(chains) == 0 {
		for c := range chainSet {
			chains = append(chains, c)
		}
	}
	return currencies, chains
}

func (s *Server) isSupportedCurrency(c string) bool {
	for _, c2 := range s.supportedCurrencies {
		if c2 == c {
			return true
		}
	}
	return false
}

func (s *Server) isSupportedChain(c string) bool {
	for _, c2 := range s.supportedChains {
		if c2 == c {
			return true
		}
	}
	return false
}

func (s *Server) isSupportedPair(currency, chain string) bool {
	if len(s.supportedPairs) == 0 {
		return s.isSupportedCurrency(currency) && s.isSupportedChain(chain)
	}
	_, ok := s.supportedPairs[currency+":"+chain]
	return ok
}

func firstOrEmpty(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", server.WithRoutePattern("/health", http.HandlerFunc(s.handleHealth)))
	mux.Handle("/metrics", server.WithRoutePattern("/metrics", server.MetricsHandler()))
	mux.Handle("/api/wallet/supported", server.WithRoutePattern("/api/wallet/supported", http.HandlerFunc(s.handleSupported)))
	mux.Handle("/api/wallet/rates", server.WithRoutePattern("/api/wallet/rates", http.HandlerFunc(s.handleRates)))
	mux.Handle("/api/wallet/balance", server.WithRoutePattern("/api/wallet/balance", s.auth(http.HandlerFunc(s.handleBalance))))
	mux.Handle("/api/wallet/transactions", server.WithRoutePattern("/api/wallet/transactions", s.auth(http.HandlerFunc(s.handleTransactions))))
	mux.Handle("/api/wallet/deposit-address", server.WithRoutePattern("/api/wallet/deposit-address", s.auth(http.HandlerFunc(s.handleDepositAddress))))
	mux.Handle("/api/wallet/withdraw", server.WithRoutePattern("/api/wallet/withdraw", s.auth(http.HandlerFunc(s.handleWithdraw))))
	mux.Handle("/api/admin/withdrawals", server.WithRoutePattern("/api/admin/withdrawals", s.auth(s.admin(http.HandlerFunc(s.handleListPendingWithdrawals)))))
	mux.Handle("/api/admin/withdrawals/review", server.WithRoutePattern("/api/admin/withdrawals/review", s.auth(s.admin(http.HandlerFunc(s.handleReviewWithdrawal)))))
	mux.Handle("/webhooks/xcash/deposit", server.WithRoutePattern("/webhooks/xcash/deposit", http.HandlerFunc(s.handleXcashWebhook)))

	mux.Handle("/api/sports", server.WithRoutePattern("/api/sports", http.HandlerFunc(s.handleListSports)))
	mux.Handle("/api/sports/{sport_id}/leagues", server.WithRoutePattern("/api/sports/{sport_id}/leagues", http.HandlerFunc(s.handleListLeagues)))
	mux.Handle("/api/events", server.WithRoutePattern("/api/events", http.HandlerFunc(s.handleListEvents)))
	mux.Handle("/api/events/{event_id}", server.WithRoutePattern("/api/events/{event_id}", http.HandlerFunc(s.handleGetEvent)))
	mux.Handle("/api/events/{event_id}/markets", server.WithRoutePattern("/api/events/{event_id}/markets", http.HandlerFunc(s.handleListMarkets)))
	mux.Handle("/api/markets/{market_id}/outcomes", server.WithRoutePattern("/api/markets/{market_id}/outcomes", http.HandlerFunc(s.handleListOutcomes)))
	mux.Handle("/api/live/events", server.WithRoutePattern("/api/live/events", http.HandlerFunc(s.handleListLiveEvents)))

	mux.Handle("/api/bets", server.WithRoutePattern("/api/bets", s.auth(http.HandlerFunc(s.handleBets))))
	mux.Handle("/api/bets/{bet_id}", server.WithRoutePattern("/api/bets/{bet_id}", s.auth(http.HandlerFunc(s.handleGetBet))))
	mux.Handle("/api/admin/bets/settle", server.WithRoutePattern("/api/admin/bets/settle", s.auth(s.admin(http.HandlerFunc(s.handleSettleBet)))))

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
	pairs := make([]string, 0, len(s.supportedPairs))
	for pair := range s.supportedPairs {
		pairs = append(pairs, pair)
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"currencies": s.supportedCurrencies,
		"chains":     s.supportedChains,
		"pairs":      pairs,
	})
}

func (s *Server) handleRates(w http.ResponseWriter, r *http.Request) {
	fiat := r.URL.Query().Get("fiat_currency")
	resp, err := s.wallet.GetRates(r.Context(), fiat)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"fiat_currency": resp.FiatCurrency,
		"rates":         resp.Rates,
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
	q := r.URL.Query()
	currency := q.Get("currency")
	fiat := q.Get("fiat_currency")
	if currency == "" {
		currency = firstOrEmpty(s.supportedCurrencies)
	}
	if !s.isSupportedCurrency(currency) {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported currency: %s", currency))
		return
	}
	resp, err := s.wallet.GetBalance(r.Context(), user.ID, currency, fiat)
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
	fiat := r.URL.Query().Get("fiat_currency")
	resp, err := s.wallet.ListTransactions(r.Context(), user.ID, fiat, 1, 20)
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
		currency = firstOrEmpty(s.supportedCurrencies)
	}
	if chain == "" {
		chain = firstOrEmpty(s.supportedChains)
	}
	if !s.isSupportedPair(currency, chain) {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported currency-chain pair: %s:%s", currency, chain))
		return
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
		body.Currency = firstOrEmpty(s.supportedCurrencies)
	}
	if body.Chain == "" {
		body.Chain = firstOrEmpty(s.supportedChains)
	}
	if !s.isSupportedPair(body.Currency, body.Chain) {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported currency-chain pair: %s:%s", body.Currency, body.Chain))
		return
	}
	if err := s.limits.ValidateWithdrawal(body.Amount); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
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

	var payload struct {
		Data struct {
			Amount string `json:"amount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.limits.ValidateDeposit(payload.Data.Amount); err != nil {
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

func (s *Server) handleListSports(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pagination(r)
	resp, err := s.oddsfeed.ListSports(r.Context(), page, pageSize)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListLeagues(w http.ResponseWriter, r *http.Request) {
	sportID := r.PathValue("sport_id")
	page, pageSize := pagination(r)
	resp, err := s.oddsfeed.ListLeagues(r.Context(), sportID, page, pageSize)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := pagination(r)
	resp, err := s.oddsfeed.ListEvents(r.Context(), q.Get("sport_id"), q.Get("league_id"), q.Get("status"), page, pageSize)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	resp, err := s.oddsfeed.GetEvent(r.Context(), r.PathValue("event_id"))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if resp.Event == nil {
		s.writeError(w, http.StatusNotFound, fmt.Errorf("event not found"))
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListMarkets(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pagination(r)
	status := r.URL.Query().Get("status")
	resp, err := s.oddsfeed.ListMarkets(r.Context(), r.PathValue("event_id"), status, page, pageSize)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListOutcomes(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pagination(r)
	status := r.URL.Query().Get("status")
	resp, err := s.oddsfeed.ListOutcomes(r.Context(), r.PathValue("market_id"), status, page, pageSize)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListLiveEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := pagination(r)
	resp, err := s.oddsfeed.ListLiveScores(r.Context(), q.Get("sport_id"), q.Get("league_id"), page, pageSize)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func pagination(r *http.Request) (int, int) {
	page := 1
	pageSize := 20
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && v > 0 {
		pageSize = v
	}
	return page, pageSize
}

func (s *Server) handleBets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePlaceBet(w, r)
	case http.MethodGet:
		s.handleListBets(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, nil)
	}
}

func (s *Server) handlePlaceBet(w http.ResponseWriter, r *http.Request) {
	user, ok := s.userFromContext(w, r)
	if !ok {
		return
	}
	var body struct {
		EventID     string `json:"eventId"`
		MarketID    string `json:"marketId"`
		OutcomeID   string `json:"outcomeId"`
		Stake       string `json:"stake"`
		Currency    string `json:"currency"`
		ReferenceID string `json:"referenceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.sportsbook.PlaceBet(r.Context(), user.ID, body.EventID, body.MarketID, body.OutcomeID, body.Stake, body.Currency, body.ReferenceID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListBets(w http.ResponseWriter, r *http.Request) {
	user, ok := s.userFromContext(w, r)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	page, pageSize := pagination(r)
	resp, err := s.sportsbook.ListBets(r.Context(), user.ID, status, page, pageSize)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetBet(w http.ResponseWriter, r *http.Request) {
	user, ok := s.userFromContext(w, r)
	if !ok {
		return
	}
	betID := r.PathValue("bet_id")
	resp, err := s.sportsbook.GetBet(r.Context(), betID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if resp == nil || resp.Bet == nil || resp.Bet.UserId != user.ID {
		s.writeError(w, http.StatusNotFound, fmt.Errorf("bet not found"))
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSettleBet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BetID   string `json:"betId"`
		Outcome string `json:"outcome"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.sportsbook.SettleBet(r.Context(), body.BetID, body.Outcome)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
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
