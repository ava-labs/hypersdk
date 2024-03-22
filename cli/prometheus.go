// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:gosec
package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/pkg/browser"
	"gopkg.in/yaml.v2"

	"github.com/ava-labs/hypersdk/utils"
)

const (
	InboundBandwidth           = "increase(avalanche_network_get_received_bytes[5s])/5 + increase(avalanche_network_put_received_bytes[5s])/5 + increase(avalanche_network_ping_received_bytes[5s])/5  + increase(avalanche_network_pong_received_bytes[5s])/5 +  increase(avalanche_network_chits_received_bytes[5s])/5 +  increase(avalanche_network_version_received_bytes[5s])/5 +  increase(avalanche_network_accepted_received_bytes[5s])/5 +  increase(avalanche_network_peerlist_received_bytes[5s])/5 + increase(avalanche_network_ancestors_received_bytes[5s])/5 +  increase(avalanche_network_app_gossip_received_bytes[5s])/5 + increase(avalanche_network_pull_query_received_bytes[5s])/5 + increase(avalanche_network_push_query_received_bytes[5s])/5 + increase(avalanche_network_app_request_received_bytes[5s])/5 + increase(avalanche_network_app_response_received_bytes[5s])/5 + increase(avalanche_network_get_accepted_received_bytes[5s])/5 + increase(avalanche_network_peerlist_ack_received_bytes[5s])/5 +increase(avalanche_network_get_ancestors_received_bytes[5s])/5 +increase(avalanche_network_accepted_frontier_received_bytes[5s])/5 +increase(avalanche_network_get_accepted_frontier_received_bytes[5s])/5 +increase(avalanche_network_accepted_state_summary_received_bytes[5s])/5 +increase(avalanche_network_state_summary_frontier_received_bytes[5s])/5 +increase(avalanche_network_get_accepted_state_summary_received_bytes[5s])/5  +increase(avalanche_network_get_state_summary_frontier_received_bytes[5s])/5"
	InboundCompressionSavings  = "increase(avalanche_network_get_compression_saved_received_bytes_sum[5s])/5 + increase(avalanche_network_put_compression_saved_received_bytes_sum[5s])/5 + increase(avalanche_network_ping_compression_saved_received_bytes_sum[5s])/5  + increase(avalanche_network_pong_compression_saved_received_bytes_sum[5s])/5 +  increase(avalanche_network_chits_compression_saved_received_bytes_sum[5s])/5 +  increase(avalanche_network_version_compression_saved_received_bytes_sum[5s])/5 +  increase(avalanche_network_accepted_compression_saved_received_bytes_sum[5s])/5 +  increase(avalanche_network_peerlist_compression_saved_received_bytes_sum[5s])/5 + increase(avalanche_network_ancestors_compression_saved_received_bytes_sum[5s])/5 +  increase(avalanche_network_app_gossip_compression_saved_received_bytes_sum[5s])/5 + increase(avalanche_network_pull_query_compression_saved_received_bytes_sum[5s])/5 + increase(avalanche_network_push_query_compression_saved_received_bytes_sum[5s])/5 + increase(avalanche_network_app_request_compression_saved_received_bytes_sum[5s])/5 + increase(avalanche_network_app_response_compression_saved_received_bytes_sum[5s])/5 + increase(avalanche_network_get_accepted_compression_saved_received_bytes_sum[5s])/5 + increase(avalanche_network_peerlist_ack_compression_saved_received_bytes_sum[5s])/5 +increase(avalanche_network_get_ancestors_compression_saved_received_bytes_sum[5s])/5 +increase(avalanche_network_accepted_frontier_compression_saved_received_bytes_sum[5s])/5 +increase(avalanche_network_get_accepted_frontier_compression_saved_received_bytes_sum[5s])/5 +increase(avalanche_network_accepted_state_summary_compression_saved_received_bytes_sum[5s])/5 +increase(avalanche_network_state_summary_frontier_compression_saved_received_bytes_sum[5s])/5 +increase(avalanche_network_get_accepted_state_summary_compression_saved_received_bytes_sum[5s])/5  +increase(avalanche_network_get_state_summary_frontier_compression_saved_received_bytes_sum[5s])/5"
	DecompressionTime          = "increase(avalanche_network_codec_zstd_get_decompress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_put_decompress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_ping_decompress_time_sum[5s])/1000000/5  + increase(avalanche_network_codec_zstd_pong_decompress_time_sum[5s])/1000000/5 +  increase(avalanche_network_codec_zstd_chits_decompress_time_sum[5s])/1000000/5 +  increase(avalanche_network_codec_zstd_version_decompress_time_sum[5s])/1000000/5 +  increase(avalanche_network_codec_zstd_accepted_decompress_time_sum[5s])/1000000/5 +  increase(avalanche_network_codec_zstd_peerlist_decompress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_ancestors_decompress_time_sum[5s])/1000000/5 +  increase(avalanche_network_codec_zstd_app_gossip_decompress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_pull_query_decompress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_push_query_decompress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_app_request_decompress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_app_response_decompress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_get_accepted_decompress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_peerlist_ack_decompress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_get_ancestors_decompress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_accepted_frontier_decompress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_get_accepted_frontier_decompress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_accepted_state_summary_decompress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_state_summary_frontier_decompress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_get_accepted_state_summary_decompress_time_sum[5s])/1000000/5  +increase(avalanche_network_codec_zstd_get_state_summary_frontier_decompress_time_sum[5s])/1000000/5"
	OutboundBandwidth          = "increase(avalanche_network_get_sent_bytes[5s])/5 + increase(avalanche_network_put_sent_bytes[5s])/5 + increase(avalanche_network_ping_sent_bytes[5s])/5  + increase(avalanche_network_pong_sent_bytes[5s])/5 +  increase(avalanche_network_chits_sent_bytes[5s])/5 +  increase(avalanche_network_version_sent_bytes[5s])/5 +  increase(avalanche_network_accepted_sent_bytes[5s])/5 +  increase(avalanche_network_peerlist_sent_bytes[5s])/5 + increase(avalanche_network_ancestors_sent_bytes[5s])/5 +  increase(avalanche_network_app_gossip_sent_bytes[5s])/5 + increase(avalanche_network_pull_query_sent_bytes[5s])/5 + increase(avalanche_network_push_query_sent_bytes[5s])/5 + increase(avalanche_network_app_request_sent_bytes[5s])/5 + increase(avalanche_network_app_response_sent_bytes[5s])/5 + increase(avalanche_network_get_accepted_sent_bytes[5s])/5 + increase(avalanche_network_peerlist_ack_sent_bytes[5s])/5 +increase(avalanche_network_get_ancestors_sent_bytes[5s])/5 +increase(avalanche_network_accepted_frontier_sent_bytes[5s])/5 +increase(avalanche_network_get_accepted_frontier_sent_bytes[5s])/5 +increase(avalanche_network_accepted_state_summary_sent_bytes[5s])/5 +increase(avalanche_network_state_summary_frontier_sent_bytes[5s])/5 +increase(avalanche_network_get_accepted_state_summary_sent_bytes[5s])/5  +increase(avalanche_network_get_state_summary_frontier_sent_bytes[5s])/5"
	OutboundCompressionSavings = "increase(avalanche_network_get_compression_saved_sent_bytes_sum[5s])/5 + increase(avalanche_network_put_compression_saved_sent_bytes_sum[5s])/5 + increase(avalanche_network_ping_compression_saved_sent_bytes_sum[5s])/5  + increase(avalanche_network_pong_compression_saved_sent_bytes_sum[5s])/5 +  increase(avalanche_network_chits_compression_saved_sent_bytes_sum[5s])/5 +  increase(avalanche_network_version_compression_saved_sent_bytes_sum[5s])/5 +  increase(avalanche_network_accepted_compression_saved_sent_bytes_sum[5s])/5 +  increase(avalanche_network_peerlist_compression_saved_sent_bytes_sum[5s])/5 + increase(avalanche_network_ancestors_compression_saved_sent_bytes_sum[5s])/5 +  increase(avalanche_network_app_gossip_compression_saved_sent_bytes_sum[5s])/5 + increase(avalanche_network_pull_query_compression_saved_sent_bytes_sum[5s])/5 + increase(avalanche_network_push_query_compression_saved_sent_bytes_sum[5s])/5 + increase(avalanche_network_app_request_compression_saved_sent_bytes_sum[5s])/5 + increase(avalanche_network_app_response_compression_saved_sent_bytes_sum[5s])/5 + increase(avalanche_network_get_accepted_compression_saved_sent_bytes_sum[5s])/5 + increase(avalanche_network_peerlist_ack_compression_saved_sent_bytes_sum[5s])/5 +increase(avalanche_network_get_ancestors_compression_saved_sent_bytes_sum[5s])/5 +increase(avalanche_network_accepted_frontier_compression_saved_sent_bytes_sum[5s])/5 +increase(avalanche_network_get_accepted_frontier_compression_saved_sent_bytes_sum[5s])/5 +increase(avalanche_network_accepted_state_summary_compression_saved_sent_bytes_sum[5s])/5 +increase(avalanche_network_state_summary_frontier_compression_saved_sent_bytes_sum[5s])/5 +increase(avalanche_network_get_accepted_state_summary_compression_saved_sent_bytes_sum[5s])/5  +increase(avalanche_network_get_state_summary_frontier_compression_saved_sent_bytes_sum[5s])/5"
	CompressionTime            = "increase(avalanche_network_codec_zstd_get_compress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_put_compress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_ping_compress_time_sum[5s])/1000000/5  + increase(avalanche_network_codec_zstd_pong_compress_time_sum[5s])/1000000/5 +  increase(avalanche_network_codec_zstd_chits_compress_time_sum[5s])/1000000/5 +  increase(avalanche_network_codec_zstd_version_compress_time_sum[5s])/1000000/5 +  increase(avalanche_network_codec_zstd_accepted_compress_time_sum[5s])/1000000/5 +  increase(avalanche_network_codec_zstd_peerlist_compress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_ancestors_compress_time_sum[5s])/1000000/5 +  increase(avalanche_network_codec_zstd_app_gossip_compress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_pull_query_compress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_push_query_compress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_app_request_compress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_app_response_compress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_get_accepted_compress_time_sum[5s])/1000000/5 + increase(avalanche_network_codec_zstd_peerlist_ack_compress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_get_ancestors_compress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_accepted_frontier_compress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_get_accepted_frontier_compress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_accepted_state_summary_compress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_state_summary_frontier_compress_time_sum[5s])/1000000/5 +increase(avalanche_network_codec_zstd_get_accepted_state_summary_compress_time_sum[5s])/1000000/5  +increase(avalanche_network_codec_zstd_get_state_summary_frontier_compress_time_sum[5s])/1000000/5"
	OutboundFailed             = "increase(avalanche_network_get_failed[5s])/5 + increase(avalanche_network_put_failed[5s])/5 + increase(avalanche_network_ping_failed[5s])/5  + increase(avalanche_network_pong_failed[5s])/5 +  increase(avalanche_network_chits_failed[5s])/5 +  increase(avalanche_network_version_failed[5s])/5 +  increase(avalanche_network_accepted_failed[5s])/5 +  increase(avalanche_network_peerlist_failed[5s])/5 + increase(avalanche_network_ancestors_failed[5s])/5 +  increase(avalanche_network_app_gossip_failed[5s])/5 + increase(avalanche_network_pull_query_failed[5s])/5 + increase(avalanche_network_push_query_failed[5s])/5 + increase(avalanche_network_app_request_failed[5s])/5 + increase(avalanche_network_app_response_failed[5s])/5 + increase(avalanche_network_get_accepted_failed[5s])/5 + increase(avalanche_network_peerlist_ack_failed[5s])/5 +increase(avalanche_network_get_ancestors_failed[5s])/5 +increase(avalanche_network_accepted_frontier_failed[5s])/5 +increase(avalanche_network_get_accepted_frontier_failed[5s])/5 +increase(avalanche_network_accepted_state_summary_failed[5s])/5 +increase(avalanche_network_state_summary_frontier_failed[5s])/5 +increase(avalanche_network_get_accepted_state_summary_failed[5s])/5  +increase(avalanche_network_get_state_summary_frontier_failed[5s])/5"
)

var (
	InboundCompressionSavingsPercent  = fmt.Sprintf("(%s)/(%s + %s) * 100", InboundCompressionSavings, InboundBandwidth, InboundCompressionSavings)
	OutboundCompressionSavingsPercent = fmt.Sprintf("(%s)/(%s + %s) * 100", OutboundCompressionSavings, OutboundBandwidth, OutboundCompressionSavings)
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

func (h *Handler) GeneratePrometheus(baseURI string, openBrowser bool, startPrometheus bool, prometheusFile string, prometheusData string, getPanels func(ids.ID) []string) error {
	chainID, uris, err := h.PromptChain("select chainID", nil)
	if err != nil {
		return err
	}
	if err := h.CloseDatabase(); err != nil {
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
	prometheusConfig.Global.ScrapeInterval = "1s"
	prometheusConfig.Global.EvaluationInterval = "1s"
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
	dashboard := baseURI + "/graph"
	for i, panel := range getPanels(chainID) {
		appendChar := "&"
		if i == 0 {
			appendChar = "?"
		}
		dashboard = fmt.Sprintf("%s%sg%d.expr=%s&g%d.tab=0&g%d.step_input=1&g%d.range_input=5m", dashboard, appendChar, i, url.QueryEscape(panel), i, i, i)
	}

	if !startPrometheus {
		if !openBrowser {
			utils.Outf("{{orange}}pre-built dashboard:{{/}} %s\n", dashboard)

			// Emit command to run prometheus
			utils.Outf("{{green}}prometheus cmd:{{/}} /tmp/prometheus --config.file=%s --storage.tsdb.path=%s\n", prometheusFile, prometheusData)
			return nil
		}
		return browser.OpenURL(dashboard)
	}

	// Start prometheus and open browser
	//
	// Attempting to exit from the terminal will gracefully
	// stop this process.
	cmd := exec.CommandContext(context.Background(), "/tmp/prometheus", "--config.file="+prometheusFile, "--storage.tsdb.path="+prometheusData)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	errChan := make(chan error)
	go func() {
		select {
		case <-errChan:
			return
		case <-time.After(5 * time.Second):
			if !openBrowser {
				utils.Outf("{{orange}}pre-built dashboard:{{/}} %s\n", dashboard)
				return
			}
			utils.Outf("{{cyan}}opening dashboard{{/}}\n")
			if err := browser.OpenURL(dashboard); err != nil {
				utils.Outf("{{red}}unable to open dashboard:{{/}} %s\n", err.Error())
			}
		}
	}()

	utils.Outf("{{cyan}}starting prometheus (/tmp/prometheus) in background{{/}}\n")
	if err := cmd.Run(); err != nil {
		errChan <- err
		utils.Outf("{{orange}}prometheus exited with error:{{/}} %v\n", err)
		utils.Outf(`install prometheus using the following commands:

rm -f /tmp/prometheus
wget https://github.com/prometheus/prometheus/releases/download/v2.43.0/prometheus-2.43.0.darwin-amd64.tar.gz
tar -xvf prometheus-2.43.0.darwin-amd64.tar.gz
rm prometheus-2.43.0.darwin-amd64.tar.gz
mv prometheus-2.43.0.darwin-amd64/prometheus /tmp/prometheus
rm -rf prometheus-2.43.0.darwin-amd64

`)
		return err
	}
	utils.Outf("{{cyan}}prometheus exited{{/}}\n")
	return nil
}
