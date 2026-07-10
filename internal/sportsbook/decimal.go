package sportsbook

import (
	"fmt"

	"github.com/shopspring/decimal"
)

func parseDecimal(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("parse decimal %q: %w", s, err)
	}
	return d, nil
}

func multiplyDecimal(a, b string) (string, error) {
	da, err := parseDecimal(a)
	if err != nil {
		return "", err
	}
	db, err := parseDecimal(b)
	if err != nil {
		return "", err
	}
	return da.Mul(db).String(), nil
}

func decimalGreaterThanZero(s string) (bool, error) {
	d, err := parseDecimal(s)
	if err != nil {
		return false, err
	}
	return d.GreaterThan(decimal.Zero), nil
}
