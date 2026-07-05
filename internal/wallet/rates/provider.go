package rates

import "context"

// RateProvider fetches the exchange rate between a fiat currency and a crypto currency.
type RateProvider interface {
	// GetRate returns the rate for 1 unit of crypto expressed in fiat.
	// For example, if crypto=BTC and fiat=USD, a rate of 60000 means 1 BTC = 60000 USD.
	GetRate(ctx context.Context, fiat, crypto string) (string, error)
	Name() string
}
