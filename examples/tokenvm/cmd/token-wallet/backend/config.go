package backend

type Config struct {
	TokenRPC    string `json:"tokenRPC"`
	FaucetRPC   string `json:"faucetRPC"`
	SearchCores int    `json:"searchCores"`
}
