// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"fmt"
	"net/url"
	"os"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/utils"
	"gopkg.in/yaml.v2"
)

const fsModeWrite = 0o600

type PrometheusStaticConfig struct {
	Targets []string `yaml:"targets"`
}

type PrometheusScrapeConfig struct {
	JobName       string                    `yaml:"job_name"`
	StaticConfigs []*PrometheusStaticConfig `yaml:"static_configs"`
	MetricsPath   string                    `yaml:"metrics_path"`
}

type PrometheusConfig struct {
	Global struct {
		ScrapeInterval     string `yaml:"scrape_interval"`
		EvaluationInterval string `yaml:"evaluation_interval"`
	} `yaml:"global"`
	ScrapeConfigs []*PrometheusScrapeConfig `yaml:"scrape_configs"`
}

func (h *Handler) GeneratePrometheus(prometheusFile string, prometheusData string, getPanels func(ids.ID) []string) error {
	chainID, uris, err := h.PromptChain("select chainID", nil)
	if err != nil {
		return err
	}
	endpoints := make([]string, len(uris))
	for i, uri := range uris {
		host, err := utils.GetHost(uri)
		if err != nil {
			return err
		}
		port, err := utils.GetPort(uri)
		if err != nil {
			return err
		}
		endpoints[i] = fmt.Sprintf("%s:%s", host, port)
	}

	// Create Prometheus YAML
	var prometheusConfig PrometheusConfig
	prometheusConfig.Global.ScrapeInterval = "15s"
	prometheusConfig.Global.EvaluationInterval = "15s"
	prometheusConfig.ScrapeConfigs = []*PrometheusScrapeConfig{
		{
			JobName: "prometheus",
			StaticConfigs: []*PrometheusStaticConfig{
				{
					Targets: endpoints,
				},
			},
			MetricsPath: "/ext/metrics",
		},
	}
	yamlData, err := yaml.Marshal(&prometheusConfig)
	if err != nil {
		return err
	}
	if err := os.WriteFile(prometheusFile, yamlData, fsModeWrite); err != nil {
		return err
	}

	// Generated dashboard link
	//
	// We must manually encode the params because prometheus skips any panels
	// that are not numerically sorted and `url.params` only sorts
	// lexicographically.
	dashboard := "http://localhost:9090/graph"
	for i, panel := range getPanels(chainID) {
		appendChar := "&"
		if i == 0 {
			appendChar = "?"
		}
		dashboard = fmt.Sprintf("%s%sg%d.expr=%s&g%d.tab=0", dashboard, appendChar, i, url.QueryEscape(panel), i)
	}
	utils.Outf("{{orange}}pre-built dashboard:{{/}} %s\n", dashboard)

	// Emit command to run prometheus
	utils.Outf("{{green}}prometheus cmd:{{/}} /tmp/prometheus --config.file=%s --storage.tsdb.path=%s\n", prometheusFile, prometheusData)
	return nil
}
