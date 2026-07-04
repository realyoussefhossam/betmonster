package wallet

import "context"

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error) {
	return s.store.CreditWallet(ctx, userID, currency, amount, referenceID, metadata)
}

func (s *Service) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string) (*Transaction, error) {
	return s.store.DebitWallet(ctx, userID, currency, amount, referenceID)
}

func (s *Service) GetBalance(ctx context.Context, userID, currency string) (*Wallet, error) {
	return s.store.GetWallet(ctx, userID, currency)
}
