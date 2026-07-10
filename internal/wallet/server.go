package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/shared/grpcmeta"
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

type userIDRequest interface {
	GetUserId() string
}

// AuthInterceptor is a gRPC unary interceptor that requires every incoming
// request to include the authenticated caller's identity as metadata. For
// user-scoped requests, the metadata user id must also match the protobuf
// request's user id field.
func AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing caller metadata")
	}
	userIDs := md.Get(grpcmeta.UserIDHeader)
	if len(userIDs) == 0 || userIDs[0] == "" {
		return nil, status.Error(codes.Unauthenticated, "missing caller user id")
	}
	callerID := userIDs[0]

	isAdmin := false
	if adminVals := md.Get(grpcmeta.IsAdminHeader); len(adminVals) > 0 {
		isAdmin = adminVals[0] == "true"
	}

	switch info.FullMethod {
	case pb.WalletService_ListPendingWithdrawals_FullMethodName,
		pb.WalletService_ReviewWithdrawal_FullMethodName:
		if !isAdmin {
			return nil, status.Error(codes.PermissionDenied, "admin metadata required")
		}
	case pb.WalletService_GetRates_FullMethodName,
		pb.WalletService_ProcessDepositWebhook_FullMethodName:
		// System methods are allowed without a user_id match.
	default:
		if reqWithUser, ok := req.(userIDRequest); ok {
			if reqUserID := reqWithUser.GetUserId(); reqUserID != "" && reqUserID != callerID {
				return nil, status.Error(codes.PermissionDenied, "caller user id does not match request user id")
			}
		}
	}

	return handler(ctx, req)
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

func (s *GRPCServer) DebitWallet(ctx context.Context, req *pb.DebitWalletRequest) (*pb.DebitWalletResponse, error) {
	tx, err := s.service.DebitWallet(ctx, req.UserId, req.Currency, req.Amount, req.ReferenceId)
	if err != nil {
		return nil, fmt.Errorf("debit wallet: %w", err)
	}
	return &pb.DebitWalletResponse{TransactionId: tx.ID, Status: tx.Status}, nil
}

func (s *GRPCServer) CreditWallet(ctx context.Context, req *pb.CreditWalletRequest) (*pb.CreditWalletResponse, error) {
	var metadata map[string]any
	if req.Metadata != "" {
		if err := json.Unmarshal([]byte(req.Metadata), &metadata); err != nil {
			return nil, fmt.Errorf("credit wallet metadata: %w", err)
		}
	}
	tx, err := s.service.CreditWallet(ctx, req.UserId, req.Currency, req.Amount, req.ReferenceId, metadata)
	if err != nil {
		return nil, fmt.Errorf("credit wallet: %w", err)
	}
	return &pb.CreditWalletResponse{TransactionId: tx.ID, Status: tx.Status}, nil
}
