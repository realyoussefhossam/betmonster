package rates

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
