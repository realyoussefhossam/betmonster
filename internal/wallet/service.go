package wallet

import (
	"context"
	"errors"
	"strings"

	"github.com/realyoussefhossam/betmonster/internal/wallet/xcash"
)

type Service struct {
	store              Store
	xcash              *xcash.Client
	xcashValidator     *xcash.WebhookValidator
	supportedPairs     map[string]struct{}
}

func NewService(store Store, xcashClient *xcash.Client, validator *xcash.WebhookValidator, supportedPairs []string) *Service {
	pairs := make(map[string]struct{}, len(supportedPairs))
	for _, p := range supportedPairs {
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
		pairs[currency+":"+chain] = struct{}{}
	}
	return &Service{
		store:          store,
		xcash:          xcashClient,
		xcashValidator: validator,
		supportedPairs: pairs,
	}
}

func (s *Service) isSupportedPair(currency, chain string) bool {
	_, ok := s.supportedPairs[currency+":"+chain]
	return ok
}

func (s *Service) validatePair(currency, chain string) error {
	if !s.isSupportedPair(currency, chain) {
		return errors.New("unsupported currency-chain pair")
	}
	return nil
}

func (s *Service) supportedCurrencies() []string {
	out := make([]string, 0, len(s.supportedPairs))
	seen := map[string]struct{}{}
	for pair := range s.supportedPairs {
		parts := strings.SplitN(pair, ":", 2)
		if _, ok := seen[parts[0]]; !ok {
			seen[parts[0]] = struct{}{}
			out = append(out, parts[0])
		}
	}
	return out
}

func (s *Service) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error) {
	return s.store.CreditWallet(ctx, userID, currency, amount, referenceID, metadata)
}

func (s *Service) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string) (*Transaction, error) {
	return s.store.DebitWallet(ctx, userID, currency, amount, referenceID)
}

func (s *Service) GetBalance(ctx context.Context, userID, currency string) (*Wallet, error) {
	if !s.currencyInPairs(currency) {
		return nil, errors.New("unsupported currency")
	}
	wallet, err := s.store.GetWallet(ctx, userID, currency)
	if err == nil {
		return wallet, nil
	}
	if errors.Is(err, ErrWalletNotFound) {
		return s.store.CreateWallet(ctx, userID, currency)
	}
	return nil, err
}

func (s *Service) currencyInPairs(currency string) bool {
	for pair := range s.supportedPairs {
		parts := strings.SplitN(pair, ":", 2)
		if parts[0] == currency {
			return true
		}
	}
	return false
}

func (s *Service) GetDepositAddress(ctx context.Context, userID, currency, chain string) (*DepositAddress, error) {
	if err := s.validatePair(currency, chain); err != nil {
		return nil, err
	}
	addr, err := s.store.GetDepositAddress(ctx, userID, currency, chain)
	if err == nil && addr != nil {
		return addr, nil
	}

	resp, err := s.xcash.GetDepositAddress(ctx, xcash.DepositAddressRequest{
		UID:    userID,
		Chain:  chain,
		Crypto: currency,
	})
	if err != nil {
		return nil, err
	}

	addr = &DepositAddress{
		UserID:   userID,
		Currency: currency,
		Chain:    chain,
		Address:  resp.Address,
		Status:   "active",
	}
	return s.store.CreateDepositAddress(ctx, addr)
}

func (s *Service) ProcessDepositWebhook(ctx context.Context, body []byte, headers map[string]string) (string, error) {
	webhook, err := s.xcashValidator.Validate(body, headers)
	if err != nil {
		return "", err
	}
	if !webhook.Data.Confirmed {
		return "ok", nil
	}
	if err := s.validatePair(webhook.Data.Crypto, webhook.Data.Chain); err != nil {
		return "", err
	}
	_, err = s.CreditWallet(ctx, webhook.Data.UID, webhook.Data.Crypto, webhook.Data.Amount, webhook.Data.SysNo, map[string]any{
		"chain": webhook.Data.Chain,
		"hash":  webhook.Data.Hash,
		"block": webhook.Data.Block,
	})
	if err != nil {
		return "", err
	}
	return "ok", nil
}

func (s *Service) RequestWithdrawal(ctx context.Context, userID, currency, amount, destinationAddress, chain string) (*WithdrawalRequest, error) {
	if err := s.validatePair(currency, chain); err != nil {
		return nil, err
	}
	return s.store.RequestWithdrawal(ctx, userID, currency, amount, destinationAddress, chain)
}

func (s *Service) ReviewWithdrawal(ctx context.Context, id, action, txHash, reviewedBy string) (*WithdrawalRequest, error) {
	switch action {
	case "approve":
		return s.store.ApproveWithdrawal(ctx, id, txHash, reviewedBy)
	case "reject":
		return s.store.RejectWithdrawal(ctx, id, reviewedBy)
	default:
		return nil, errors.New("invalid action")
	}
}

func (s *Service) ListTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error) {
	return s.store.ListTransactions(ctx, userID, page, pageSize)
}

func (s *Service) ListPendingWithdrawals(ctx context.Context, page, pageSize int) ([]WithdrawalRequest, error) {
	return s.store.ListPendingWithdrawals(ctx, page, pageSize)
}
