package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	runner "github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"gopkg.in/yaml.v2"
)

func (h *Handler) ImportChain() error {
	chainID, err := h.PromptID("chainID")
	if err != nil {
		return err
	}
	uri, err := h.PromptString("uri", 0, consts.MaxInt)
	if err != nil {
		return err
	}
	if err := h.StoreChain(chainID, uri); err != nil {
		return err
	}
	if err := h.StoreDefaultChain(chainID); err != nil {
		return err
	}
	return nil
}

func (h *Handler) ImportANR() error {
	ctx := context.Background()

	// Delete previous items
	oldChains, err := h.DeleteChains()
	if err != nil {
		return err
	}
	if len(oldChains) > 0 {
		utils.Outf("{{yellow}}deleted old chains:{{/}} %+v\n", oldChains)
	}

	// Load new items from ANR
	anrCli, err := runner.New(runner.Config{
		Endpoint:    "0.0.0.0:12352",
		DialTimeout: 10 * time.Second,
	}, logging.NoLog{})
	if err != nil {
		return err
	}
	status, err := anrCli.Status(ctx)
	if err != nil {
		return err
	}
	subnets := map[ids.ID][]ids.ID{}
	for chain, chainInfo := range status.ClusterInfo.CustomChains {
		chainID, err := ids.FromString(chain)
		if err != nil {
			return err
		}
		subnetID, err := ids.FromString(chainInfo.SubnetId)
		if err != nil {
			return err
		}
		chainIDs, ok := subnets[subnetID]
		if !ok {
			chainIDs = []ids.ID{}
		}
		chainIDs = append(chainIDs, chainID)
		subnets[subnetID] = chainIDs
	}
	var filledChainID ids.ID
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		if len(nodeInfo.WhitelistedSubnets) == 0 {
			continue
		}
		trackedSubnets := strings.Split(nodeInfo.WhitelistedSubnets, ",")
		for _, subnet := range trackedSubnets {
			subnetID, err := ids.FromString(subnet)
			if err != nil {
				return err
			}
			for _, chainID := range subnets[subnetID] {
				uri := fmt.Sprintf("%s/ext/bc/%s", nodeInfo.Uri, chainID)
				if err := h.StoreChain(chainID, uri); err != nil {
					return err
				}
				utils.Outf(
					"{{yellow}}stored chainID:{{/}} %s {{yellow}}uri:{{/}} %s\n",
					chainID,
					uri,
				)
				filledChainID = chainID
			}
		}
	}
	return h.StoreDefaultChain(filledChainID)
}

type AvalancheOpsConfig struct {
	Resources struct {
		CreatedNodes []struct {
			HTTPEndpoint string `yaml:"httpEndpoint"`
		} `yaml:"created_nodes"`
	} `yaml:"resources"`
}

func (h *Handler) ImportOps(schainID string, opsPath string) error {
	oldChains, err := h.DeleteChains()
	if err != nil {
		return err
	}
	if len(oldChains) > 0 {
		utils.Outf("{{yellow}}deleted old chains:{{/}} %+v\n", oldChains)
	}

	// Load chainID
	chainID, err := ids.FromString(schainID)
	if err != nil {
		return err
	}

	// Load yaml file
	var opsConfig AvalancheOpsConfig
	yamlFile, err := os.ReadFile(opsPath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, &opsConfig)
	if err != nil {
		return err
	}

	// Add chains
	for _, node := range opsConfig.Resources.CreatedNodes {
		uri := fmt.Sprintf("%s/ext/bc/%s", node.HTTPEndpoint, chainID)
		if err := h.StoreChain(chainID, uri); err != nil {
			return err
		}
		utils.Outf(
			"{{yellow}}stored chainID:{{/}} %s {{yellow}}uri:{{/}} %s\n",
			chainID,
			uri,
		)
	}
	return h.StoreDefaultChain(chainID)
}

func (h *Handler) SetDefaultChain() error {
	chainID, _, err := h.PromptChain("set default chain", nil)
	if err != nil {
		return err
	}
	return h.StoreDefaultChain(chainID)
}

func (h *Handler) PrintChainInfo() error {
	_, uris, err := h.PromptChain("select chainID", nil)
	if err != nil {
		return err
	}
	cli := rpc.NewJSONRPCClient(uris[0])
	networkID, subnetID, chainID, err := cli.Network(context.Background())
	if err != nil {
		return err
	}
	utils.Outf(
		"{{cyan}}networkID:{{/}} %d {{cyan}}subnetID:{{/}} %s {{cyan}}chainID:{{/}} %s",
		networkID,
		subnetID,
		chainID,
	)
	return nil
}
