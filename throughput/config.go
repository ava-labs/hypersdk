package throughput

import "github.com/ava-labs/hypersdk/auth"

type SpamConfig struct {
	uris []string
	key *auth.PrivateKey
	sZipf float64
	vZipf float64
	txsPerSecond int
	minTxsPerSecond int
	txsPerSecondStep int
	numClients int
	numAccounts int	
}

func DefaultSpamConfig(
	uris []string,
	key *auth.PrivateKey,
) *SpamConfig {
	return &SpamConfig{
		uris: uris,
		key: key,
		sZipf: 1.01,
		vZipf: 2.7,
		txsPerSecond: 500,
		minTxsPerSecond: 100,
		txsPerSecondStep: 200,
		numClients: 10,
		numAccounts: 25,
	}
}

func NewSpamConfig(
	uris []string,
	key *auth.PrivateKey,
	sZipf float64,
	vZipf float64,
	txsPerSecond int,
	minTxsPerSecond int,
	txsPerSecondStep int,
	numClients int,
	numAccounts int,
) *SpamConfig {
	return &SpamConfig{
		uris,
		key,
		sZipf,
		vZipf,
		txsPerSecond,
		minTxsPerSecond,
		txsPerSecondStep,
		numClients,
		numAccounts,
	}
}