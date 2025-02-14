// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:gosec
package prometheus

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

func GeneratePrometheus(
	endpointURIs []string,
	baseURI string,
	chainID ids.ID,
	openBrowser bool,
	startPrometheus bool,
	prometheusFile string,
	prometheusData string,
) error {
	endpoints := make([]string, len(endpointURIs))
	for i, uri := range endpointURIs {
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
	for i, panel := range generateChainPanels(chainID) {
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

func generateChainPanels(chainID ids.ID) []string {
	panels := []string{}

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_empty_block_built[5s])", chainID))
	utils.Outf("{{yellow}}empty blocks built (5s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_build_capped[5s])", chainID))
	utils.Outf("{{yellow}}build time capped (5s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("avalanche_%s_blks_processing", chainID))
	utils.Outf("{{yellow}}blocks processing:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_blks_accepted_count[5s])/5", chainID))
	utils.Outf("{{yellow}}blocks accepted per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_blks_rejected_count[5s])/5", chainID))
	utils.Outf("{{yellow}}blocks rejected per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_deleted_blocks[5s])/5", chainID))
	utils.Outf("{{yellow}}blocks deleted per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_bandwidth_price", chainID))
	utils.Outf("{{yellow}}bandwidth unit price:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_compute_price", chainID))
	utils.Outf("{{yellow}}compute unit price:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_storage_read_price", chainID))
	utils.Outf("{{yellow}}storage read unit price:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_storage_create_price", chainID))
	utils.Outf("{{yellow}}storage create unit price:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_storage_modify_price", chainID))
	utils.Outf("{{yellow}}storage modify unit price:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_submitted[5s])/5", chainID))
	utils.Outf("{{yellow}}transactions submitted per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_gossiped[5s])/5", chainID))
	utils.Outf("{{yellow}}transactions gossiped per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_received[5s])/5", chainID))
	utils.Outf("{{yellow}}transactions received per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_seen_txs_received[5s])/5", chainID))
	utils.Outf("{{yellow}}seen transactions received per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_verified[5s])/5", chainID))
	utils.Outf("{{yellow}}transactions verified per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_accepted[5s])/5", chainID))
	utils.Outf("{{yellow}}transactions accepted per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_state_operations[5s])/5", chainID))
	utils.Outf("{{yellow}}state operations per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_state_changes[5s])/5", chainID))
	utils.Outf("{{yellow}}state changes per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_root_calculated_sum[5s])/1000000/5", chainID))
	utils.Outf("{{yellow}}root calculation (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_root_sum[5s])/1000000/5", chainID))
	utils.Outf("{{yellow}}wait root calculation (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_signatures_sum[5s])/1000000/5", chainID))
	utils.Outf("{{yellow}}signature verification wait (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_cleared_mempool[5s])/5", chainID))
	utils.Outf("{{yellow}}cleared mempool per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_mempool_size", chainID))
	utils.Outf("{{yellow}}mempool size:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, "avalanche_resource_tracker_cpu_usage")
	utils.Outf("{{yellow}}CPU usage:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, "avalanche_go_memstats_alloc_bytes")
	utils.Outf("{{yellow}}memory (avalanchego) usage:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("avalanche_%s_vm_go_memstats_alloc_bytes", chainID))
	utils.Outf("{{yellow}}memory (morpheusvm) usage:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_handler_chits_sum[5s])/1000000/5 + increase(avalanche_%s_handler_notify_sum[5s])/1000000/5 + increase(avalanche_%s_handler_get_sum[5s])/1000000/5 + increase(avalanche_%s_handler_push_query_sum[5s])/1000000/5 + increase(avalanche_%s_handler_put_sum[5s])/1000000/5 + increase(avalanche_%s_handler_pull_query_sum[5s])/1000000/5 + increase(avalanche_%s_handler_query_failed_sum[5s])/1000000/5", chainID, chainID, chainID, chainID, chainID, chainID, chainID))
	utils.Outf("{{yellow}}consensus engine processing (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, InboundBandwidth)
	utils.Outf("{{yellow}}inbound bandwidth (B):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, InboundCompressionSavingsPercent)
	utils.Outf("{{yellow}}inbound compression savings (%):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, DecompressionTime)
	utils.Outf("{{yellow}}inbound decompression time (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, OutboundBandwidth)
	utils.Outf("{{yellow}}outbound bandwidth (B):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, OutboundCompressionSavingsPercent)
	utils.Outf("{{yellow}}outbound compression savings (%):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, CompressionTime)
	utils.Outf("{{yellow}}outbound compression time (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, OutboundFailed)
	utils.Outf("{{yellow}}outbound failed:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_build_sum[5s])/1000000/5", chainID))
	utils.Outf("{{yellow}}block build (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_parse_sum[5s])/1000000/5", chainID))
	utils.Outf("{{yellow}}block parse (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_verify_sum[5s])/1000000/5", chainID))
	utils.Outf("{{yellow}}block verify (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_accept_sum[5s])/1000000/5", chainID))
	utils.Outf("{{yellow}}block accept (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_process_sum[5s])/1000000/5", chainID))
	utils.Outf("{{yellow}}block process [async] (ms/s):{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_state_merkleDB_intermediate_node_cache_hit[5s])/(increase(avalanche_%s_vm_state_merkleDB_intermediate_node_cache_miss[5s]) + increase(avalanche_%s_vm_state_merkleDB_intermediate_node_cache_hit[5s]))", chainID, chainID, chainID))
	utils.Outf("{{yellow}}intermediate node cache hit rate:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_state_merkleDB_value_node_cache_hit[5s])/(increase(avalanche_%s_vm_state_merkleDB_value_node_cache_miss[5s]) + increase(avalanche_%s_vm_state_merkleDB_value_node_cache_hit[5s]))", chainID, chainID, chainID))
	utils.Outf("{{yellow}}value node cache hit rate:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_executor_build_executable[5s]) / (increase(avalanche_%s_vm_hypersdk_chain_executor_build_blocked[5s]) + increase(avalanche_%s_vm_hypersdk_chain_executor_build_executable[5s]))", chainID, chainID, chainID))
	utils.Outf("{{yellow}}build txs executable (%%) per second:{{/}} %s\n", panels[len(panels)-1])

	panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_executor_verify_executable[5s]) / (increase(avalanche_%s_vm_hypersdk_chain_executor_verify_blocked[5s]) + increase(avalanche_%s_vm_hypersdk_chain_executor_verify_executable[5s]))", chainID, chainID, chainID))
	utils.Outf("{{yellow}}verify txs executable (%%) per second:{{/}} %s\n", panels[len(panels)-1])

	return panels
}
