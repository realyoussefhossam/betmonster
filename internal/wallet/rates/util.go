package rates

import "strings"

func isStablecoin(crypto string) bool {
	switch crypto {
	case "USDT", "USDC", "BUSD", "DAI":
		return true
	}
	return false
}

func normalizeSymbol(crypto string) string {
	switch crypto {
	case "MATIC":
		return "POL"
	case "TON":
		return "GRAM"
	case "ARB-TOKEN":
		return "ARB"
	case "OP-TOKEN":
		return "OP"
	}
	return crypto
}

var supportedFiatCurrencies = []string{
	"USD", "EUR", "JPY", "INR", "CAD", "CNY", "IDR", "KRW", "PHP", "RUB",
	"MXN", "PLN", "TRY", "VND", "ARS", "PEN", "CLP", "NGN", "AED", "BHD",
	"CRC", "KWD", "MAD", "MYR", "QAR", "SAR", "SGD", "TND", "TWD", "GHS",
	"KES", "BOB", "XOF", "PKR", "NZD", "ISK", "BAM", "TZS", "EGP", "LKR",
	"UGX", "KZT", "BDT", "UAH", "GEL", "MNT", "GTQ", "KGS", "ZAR", "TMT",
	"ZMW", "TJS", "MRU", "TTD", "GMD", "MGA", "JMD", "NIO", "HNL", "MZN",
	"XAF", "RWF", "GNF", "BWP", "KMF", "LSL", "ERN", "BIF", "MWK", "PGK",
}

// IsSupportedFiat returns true if fiat is in the supported list.
func IsSupportedFiat(fiat string) bool {
	fiat = strings.ToUpper(fiat)
	for _, c := range supportedFiatCurrencies {
		if c == fiat {
			return true
		}
	}
	return false
}

// SupportedFiatCurrencies returns the list of supported fiat currencies.
func SupportedFiatCurrencies() []string {
	return append([]string(nil), supportedFiatCurrencies...)
}
