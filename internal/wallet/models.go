package wallet

import "time"

type Wallet struct {
	ID        string
	UserID    string
	Currency  string
	Balance   string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Transaction struct {
	ID            string
	UserID        string
	WalletID      string
	Currency      string
	Type          string
	Amount        string
	BalanceBefore string
	BalanceAfter  string
	Status        string
	ReferenceID   string
	Metadata      string
	CreatedAt     time.Time
}

type DepositAddress struct {
	ID             string
	UserID         string
	Currency       string
	Chain          string
	Address        string
	XCashDepositID string
	Status         string
	CreatedAt      time.Time
}

type WithdrawalRequest struct {
	ID                 string
	UserID             string
	WalletID           string
	Amount             string
	Currency           string
	DestinationAddress string
	Chain              string
	Status             string
	TxHash             string
	ReviewedBy         string
	CreatedAt          time.Time
}
