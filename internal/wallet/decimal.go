package wallet

import (
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
