package xcash

type DepositAddressRequest struct {
	UID    string
	Chain  string
	Crypto string
}

type DepositAddressResponse struct {
	Address string `json:"deposit_address"`
}

type DepositWebhook struct {
	Type string `json:"type"`
	Data struct {
		SysNo     string `json:"sys_no"`
		UID       string `json:"uid"`
		Chain     string `json:"chain"`
		Block     int64  `json:"block"`
		Hash      string `json:"hash"`
		Crypto    string `json:"crypto"`
		Amount    string `json:"amount"`
		Confirmed bool   `json:"confirmed"`
		RiskLevel string `json:"risk_level"`
		RiskScore string `json:"risk_score"`
	} `json:"data"`
}
