// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package backend

type Config struct {
	TokenRPC    string `json:"tokenRPC"`
	FaucetRPC   string `json:"faucetRPC"`
	SearchCores int    `json:"searchCores"`
	FeedRPC     string `json:"feedRPC"`
}
