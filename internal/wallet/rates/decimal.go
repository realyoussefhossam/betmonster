package rates

import (
	"strings"

	"github.com/shopspring/decimal"
)

// MulDecimalStrings multiplies two decimal strings and returns the result as a string.
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
