package wallet

import (
	"strings"

	"github.com/shopspring/decimal"
)

func addDecimal(a, b string) string {
	da, _ := decimal.NewFromString(a)
	db, _ := decimal.NewFromString(b)
	return da.Add(db).String()
}

func subDecimal(a, b string) string {
	da, _ := decimal.NewFromString(a)
	db, _ := decimal.NewFromString(b)
	return da.Sub(db).String()
}

// MulDecimalStrings multiplies two decimal strings and returns a string.
// It uses the maximum precision of the two inputs.
func MulDecimalStrings(a, b string) (string, error) {
	left, err := decimal.NewFromString(a)
	if err != nil {
		return "", err
	}
	right, err := decimal.NewFromString(b)
	if err != nil {
		return "", err
	}
	scale := max(decimalPlaces(a), decimalPlaces(b))
	return left.Mul(right).StringFixedBank(int32(scale)), nil
}

func decimalPlaces(s string) int {
	idx := strings.IndexByte(s, '.')
	if idx == -1 {
		return 0
	}
	return len(s) - idx - 1
}
