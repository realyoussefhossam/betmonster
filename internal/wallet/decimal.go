package wallet

import (
	"fmt"

	"github.com/shopspring/decimal"
)

func addDecimal(a, b string) (string, error) {
	da, err := decimal.NewFromString(a)
	if err != nil {
		return "", fmt.Errorf("parse amount %q: %w", a, err)
	}
	db, err := decimal.NewFromString(b)
	if err != nil {
		return "", fmt.Errorf("parse amount %q: %w", b, err)
	}
	return da.Add(db).String(), nil
}

func subDecimal(a, b string) (string, error) {
	da, err := decimal.NewFromString(a)
	if err != nil {
		return "", fmt.Errorf("parse amount %q: %w", a, err)
	}
	db, err := decimal.NewFromString(b)
	if err != nil {
		return "", fmt.Errorf("parse amount %q: %w", b, err)
	}
	return da.Sub(db).String(), nil
}
