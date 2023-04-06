// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var metricsCmd = &cobra.Command{
	Use: "metrics",
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

var prometheusCmd = &cobra.Command{
	Use: "prometheus [path] [output]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return ErrInvalidArgs
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		// Load HTTP Endpoints
		var opsConfig AvalancheOpsConfig
		yamlFile, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(yamlFile, &opsConfig)
		if err != nil {
			return err
		}
		endpoints := make([]string, len(opsConfig.Resources.CreatedNodes))
		for i, node := range opsConfig.Resources.CreatedNodes {
			endpoints[i] = strings.TrimPrefix(node.HTTPEndpoint, "http://")
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
		return os.WriteFile(args[1], yamlData, fsModeWrite)
	},
}
