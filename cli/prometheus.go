// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"github.com/ava-labs/hypersdk/cli/prompt"
	"github.com/ava-labs/hypersdk/internal/prometheus"
)

func (h *Handler) GeneratePrometheus(baseURI string, openBrowser bool, startPrometheus bool, prometheusFile string, prometheusData string) error {
	chains, err := h.GetChains()
	if err != nil {
		return err
	}
	chainID, uris, err := prompt.SelectChain("select chainID", chains)
	if err != nil {
		return err
	}
	return prometheus.GeneratePrometheus(uris, baseURI, chainID, openBrowser, startPrometheus, prometheusFile, prometheusData)
}
