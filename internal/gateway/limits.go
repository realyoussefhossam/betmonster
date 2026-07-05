package gateway

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Limits configures per-transaction and daily caps for deposits and withdrawals.
// An empty string for any field means "no limit" for that bound.
// Daily limits are currently enforced as a per-transaction cap; aggregate
// per-user daily tracking will be added once the gateway maintains limit state.
type Limits struct {
	MinDeposit      string
	MaxDeposit      string
	DailyDeposit    string
	MinWithdrawal   string
	MaxWithdrawal   string
	DailyWithdrawal string
}

// ValidateDeposit checks the requested deposit amount against the configured bounds.
func (l Limits) ValidateDeposit(amount string) error {
	return l.validate(amount, l.MinDeposit, l.MaxDeposit, l.DailyDeposit, "deposit")
}

// ValidateWithdrawal checks the requested withdrawal amount against the configured bounds.
func (l Limits) ValidateWithdrawal(amount string) error {
	return l.validate(amount, l.MinWithdrawal, l.MaxWithdrawal, l.DailyWithdrawal, "withdrawal")
}

func (l Limits) validate(amount, min, max, daily, op string) error {
	if amount == "" {
		return fmt.Errorf("%s amount is required", op)
	}

	d, err := decimal.NewFromString(amount)
	if err != nil {
		return fmt.Errorf("invalid %s amount: %w", op, err)
	}

	if min != "" {
		m, err := decimal.NewFromString(min)
		if err != nil {
			return fmt.Errorf("invalid %s minimum limit: %w", op, err)
		}
		if d.LessThan(m) {
			return fmt.Errorf("%s below minimum", op)
		}
	}

	if max != "" {
		m, err := decimal.NewFromString(max)
		if err != nil {
			return fmt.Errorf("invalid %s maximum limit: %w", op, err)
		}
		if d.GreaterThan(m) {
			return fmt.Errorf("%s above maximum", op)
		}
	}

	if daily != "" {
		m, err := decimal.NewFromString(daily)
		if err != nil {
			return fmt.Errorf("invalid %s daily limit: %w", op, err)
		}
		if d.GreaterThan(m) {
			return fmt.Errorf("%s above daily limit", op)
		}
	}

	return nil
}
