// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"gopkg.in/yaml.v2"

	"github.com/ava-labs/hypersdk/utils"
)

type ClusterInfo struct {
	ChainID  string `yaml:"CHAIN_ID"` // ids.ID requires "first and last characters to be quotes"
	SubnetID string `yaml:"SUBNET_ID"`
	APIs     []struct {
		CloudID string `yaml:"CLOUD_ID"`
		IP      string `yaml:"IP"`
		Region  string `yaml:"REGION"`
	} `yaml:"API"`
	Validators []struct {
		CloudID string `yaml:"CLOUD_ID"`
		IP      string `yaml:"IP"`
		Region  string `yaml:"REGION"`
		NodeID  string `yaml:"NODE_ID"`
	} `yaml:"VALIDATOR"`
}

func ReadClusterInfoFile(cliPath string) (ids.ID, map[string]string, error) {
	// Load yaml file
	yamlFile, err := os.ReadFile(cliPath)
	if err != nil {
		return ids.Empty, nil, err
	}
	var yamlContents ClusterInfo
	if err := yaml.Unmarshal(yamlFile, &yamlContents); err != nil {
		return ids.Empty, nil, fmt.Errorf("%w: unable to unmarshal YAML", err)
	}
	chainID, err := ids.FromString(yamlContents.ChainID)
	if err != nil {
		return ids.Empty, nil, err
	}

	// Load nodes
	nodes := make(map[string]string)
	for i, api := range yamlContents.APIs {
		name := fmt.Sprintf("%s-%d (%s)", "API", i, api.Region)
		uri := fmt.Sprintf("http://%s:9650/ext/bc/%s", api.IP, chainID)
		nodes[name] = uri
	}
	for i, validator := range yamlContents.Validators {
		name := fmt.Sprintf("%s-%d (%s)", "Validator", i, validator.Region)
		uri := fmt.Sprintf("http://%s:9650/ext/bc/%s", validator.IP, chainID)
		nodes[name] = uri
	}
	return chainID, nodes, nil
}

func (h *Handler) ImportClusterInfo(cliPath string) error {
	oldChains, err := h.DeleteChains()
	if err != nil {
		return err
	}
	if len(oldChains) > 0 {
		utils.Outf("{{yellow}}deleted old chains:{{/}} %+v\n", oldChains)
	}

	// Load yaml file
	chainID, nodes, err := ReadClusterInfoFile(cliPath)
	if err != nil {
		return err
	}
	for name, uri := range nodes {
		if err := h.StoreChain(chainID, name); err != nil {
			return err
		}
		utils.Outf(
			"{{yellow}}[%s] stored chainID:{{/}} %s {{yellow}}uri:{{/}} %s\n",
			name,
			chainID,
			uri,
		)
	}
	return h.StoreDefaultChain(chainID)
}

func OnlyAPIs(uris map[string]string) []string {
	apis := make([]string, 0, len(uris))
	for k := range uris {
		if !strings.Contains(strings.ToLower(k), "api") {
			continue
		}

		apis = append(apis, uris[k])
	}
	return apis
}
