// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var prometheusCmd = &cobra.Command{
	Use: "prometheus",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

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

var generatePrometheusCmd = &cobra.Command{
	Use: "generate",
	RunE: func(_ *cobra.Command, args []string) error {
		// Generate Prometheus-compatible endpoints
		chainID, uris, err := promptChain("select chainID", nil)
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
		utils.Outf("{{green}}prometheus config file created:{{/}} %s\n", prometheusFile)

		// Log useful queries
		utils.Outf("\n{{cyan}}common prometheus queries:{{/}}\n")
		utils.Outf("{{yellow}}blocks processing:{{/}} avalanche_%s_blks_processing\n", chainID)
		utils.Outf("{{yellow}}blocks accepted:{{/}} avalanche_%s_blks_accepted_count\n", chainID)
		utils.Outf("{{yellow}}blocks rejected:{{/}} avalanche_%s_blks_rejected_count\n", chainID)
		utils.Outf("{{yellow}}transactions per second:{{/}} increase(avalanche_%s_vm_hyper_sdk_vm_txs_accepted[30s])/30\n", chainID)
		utils.Outf("{{yellow}CPU usage:{{/}} avalanche_resource_tracker_cpu_usage\n")
		utils.Outf("{{yellow}}consensus engine processing (ms/s):{{/}} increase(avalanche_%s_handler_chits_sum[30s])/1000000/30 + increase(avalanche_%s_handler_notify_sum[30s])/1000000/30 + increase(avalanche_%s_handler_get_sum[30s])/1000000/30 + increase(avalanche_%s_handler_push_query_sum[30s])/1000000/30 + increase(avalanche_%s_handler_put_sum[30s])/1000000/30 + increase(avalanche_%s_handler_pull_query_sum[30s])/1000000/30 + increase(avalanche_%s_handler_query_failed_sum[30s])/1000000/30\n", chainID, chainID, chainID, chainID, chainID, chainID, chainID)
		return nil
	},
}
