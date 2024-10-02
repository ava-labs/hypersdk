package spam

import (
	"strings"
)

const (
	ed25519Key   = "ed25519"
	secp256r1Key = "secp256r1"
	blsKey       = "bls"
	// Auth TypeIDs
	ED25519ID   uint8 = 0
	SECP256R1ID uint8 = 1
	BLSID       uint8 = 2
)



func onlyAPIs(m map[string]string) []string {
	apis := make([]string, 0, len(m))
	for k := range m {
		if !strings.Contains(strings.ToLower(k), "api") {
			continue
		}
		apis = append(apis, k)
	}
	return apis
}
