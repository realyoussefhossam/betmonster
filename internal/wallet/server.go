package wallet

import (
	"context"
	"os"
	"strings"
	"time"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/wallet/rates"
)

func defaultFiat(reqFiat string) string {
	fiat := strings.ToUpper(strings.TrimSpace(reqFiat))
	if fiat == "" {
		fiat = os.Getenv("DEFAULT_FIAT_CURRENCY")
	}
	if fiat == "" || !rates.IsSupportedFiat(fiat) {
		fiat = "USD"
	}
	return fiat
}

type GRPCServer struct {
	pb.UnimplementedWalletServiceServer
	service *Service
	rates   *rates.Aggregator
}

func NewGRPCServer(service *Service, rates *rates.Aggregator) *GRPCServer {
	return &GRPCServer{service: service, rates: rates}
}

func (s *GRPCServer) fiatValue(ctx context.Context, fiat, currency, amount string) (string, error) {
	rate, err := s.rates.GetRate(ctx, fiat, currency)
	if err != nil {
		return "", err
	}
	return rates.MulDecimalStrings(amount, rate)
}

func (s *GRPCServer) GetRates(ctx context.Context, req *pb.GetRatesRequest) (*pb.GetRatesResponse, error) {
	fiat := defaultFiat(req.FiatCurrency)
	currencies := s.service.supportedCurrencies()
	rs := s.rates.SupportedRates(ctx, fiat, currencies)
	return &pb.GetRatesResponse{
		FiatCurrency: fiat,
		Rates:        rs,
	}, nil
}

func (s *GRPCServer) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	wallet, err := s.service.GetBalance(ctx, req.UserId, req.Currency)
	if err != nil {
		return nil, err
	}
	fiat := defaultFiat(req.FiatCurrency)
	fiatValue := "0"
	if s.rates != nil {
		v, err := s.fiatValue(ctx, fiat, wallet.Currency, wallet.Balance)
		if err == nil {
			fiatValue = v
		}
	}
	return &pb.GetBalanceResponse{
		Currency:     wallet.Currency,
		Balance:      wallet.Balance,
		FiatCurrency: fiat,
		FiatValue:    fiatValue,
	}, nil
}

func (s *GRPCServer) ListTransactions(ctx context.Context, req *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
	txns, err := s.service.ListTransactions(ctx, req.UserId, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, err
	}
	fiat := defaultFiat(req.FiatCurrency)
	out := make([]*pb.Transaction, len(txns))
	for i, t := range txns {
		fiatValue := "0"
		if s.rates != nil {
			v, err := s.fiatValue(ctx, fiat, t.Currency, t.Amount)
			if err == nil {
				fiatValue = v
			}
		}
		out[i] = &pb.Transaction{
			Id: t.ID, UserId: t.UserID, WalletId: t.WalletID, Type: t.Type,
			Currency: t.Currency, Amount: t.Amount, BalanceBefore: t.BalanceBefore, BalanceAfter: t.BalanceAfter,
			Status: t.Status, ReferenceId: t.ReferenceID, Metadata: t.Metadata,
			CreatedAt: t.CreatedAt.Format(time.RFC3339),
			FiatValue: fiatValue,
		}
	}
	return &pb.ListTransactionsResponse{Transactions: out}, nil
}

func (s *GRPCServer) GetDepositAddress(ctx context.Context, req *pb.GetDepositAddressRequest) (*pb.GetDepositAddressResponse, error) {
	addr, err := s.service.GetDepositAddress(ctx, req.UserId, req.Currency, req.Chain)
	if err != nil {
		return nil, err
	}
	return &pb.GetDepositAddressResponse{Address: addr.Address, Chain: addr.Chain, Currency: addr.Currency}, nil
}

func (s *GRPCServer) RequestWithdrawal(ctx context.Context, req *pb.RequestWithdrawalRequest) (*pb.RequestWithdrawalResponse, error) {
	out, err := s.service.RequestWithdrawal(ctx, req.UserId, req.Currency, req.Amount, req.DestinationAddress, req.Chain)
	if err != nil {
		return nil, err
	}
	return &pb.RequestWithdrawalResponse{WithdrawalId: out.ID, Status: out.Status}, nil
}

func (s *GRPCServer) ProcessDepositWebhook(ctx context.Context, req *pb.ProcessDepositWebhookRequest) (*pb.ProcessDepositWebhookResponse, error) {
	body, err := s.service.ProcessDepositWebhook(ctx, []byte(req.Body), req.Headers)
	return &pb.ProcessDepositWebhookResponse{ResponseBody: body}, err
}

func (s *GRPCServer) ListPendingWithdrawals(ctx context.Context, req *pb.ListPendingWithdrawalsRequest) (*pb.ListPendingWithdrawalsResponse, error) {
	list, err := s.service.ListPendingWithdrawals(ctx, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, err
	}
	out := make([]*pb.WithdrawalRequest, len(list))
	for i, w := range list {
		out[i] = &pb.WithdrawalRequest{
			Id: w.ID, UserId: w.UserID, Currency: w.Currency, Amount: w.Amount,
			DestinationAddress: w.DestinationAddress, Chain: w.Chain, Status: w.Status, TxHash: w.TxHash,
			CreatedAt: w.CreatedAt.Format(time.RFC3339),
		}
	}
	return &pb.ListPendingWithdrawalsResponse{Withdrawals: out}, nil
}

func (s *GRPCServer) ReviewWithdrawal(ctx context.Context, req *pb.ReviewWithdrawalRequest) (*pb.ReviewWithdrawalResponse, error) {
	w, err := s.service.ReviewWithdrawal(ctx, req.WithdrawalId, req.Action, req.TxHash, req.ReviewedBy)
	if err != nil {
		return nil, err
	}
	return &pb.ReviewWithdrawalResponse{Status: w.Status}, nil
}
