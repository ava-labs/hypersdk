// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:lll
package cmd

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

var prometheusCmd = &cobra.Command{
	Use: "prometheus",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var generatePrometheusCmd = &cobra.Command{
	Use: "generate",
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().GeneratePrometheus(prometheusBaseURI, prometheusOpenBrowser, startPrometheus, prometheusFile, prometheusData, func(chainID ids.ID) []string {
			panels := []string{}

			panels = append(panels, fmt.Sprintf("avalanche_%s_blks_processing", chainID))
			utils.Outf("{{yellow}}blocks processing:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_blks_accepted_count[5s])/5", chainID))
			utils.Outf("{{yellow}}blocks accepted per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_blks_rejected_count[5s])/5", chainID))
			utils.Outf("{{yellow}}blocks rejected per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_submitted[5s])/5", chainID))
			utils.Outf("{{yellow}}transactions submitted per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_gossiped[5s])/5", chainID))
			utils.Outf("{{yellow}}transactions gossiped per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_received[5s])/5", chainID))
			utils.Outf("{{yellow}}transactions received per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_included[5s])/5", chainID))
			utils.Outf("{{yellow}}all transactions processed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_valid[5s])/5", chainID))
			utils.Outf("{{yellow}}valid transactions executed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("(increase(avalanche_%s_vm_hypersdk_vm_txs_valid[5s])/5)/(increase(avalanche_%s_vm_hypersdk_vm_txs_included[5s])/5) * 100", chainID, chainID))
			utils.Outf("{{yellow}}valid transactions executed (%):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_state_operations[5s])/5", chainID))
			utils.Outf("{{yellow}}state operations per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_state_changes[5s])/5", chainID))
			utils.Outf("{{yellow}}state changes per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_root_calculated_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}root calcuation (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_root_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}wait root calculation (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_exec_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}exec wait (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("(increase(avalanche_%s_vm_hypersdk_chain_wait_exec_count[5s])/5)/(increase(avalanche_%s_vm_hypersdk_chain_block_verify_count[5s])/5) * 100", chainID, chainID))
			utils.Outf("{{yellow}}execution waits per verified block (%):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_cleared_mempool[5s])/5", chainID))
			utils.Outf("{{yellow}}cleared mempool per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, "avalanche_resource_tracker_cpu_usage")
			utils.Outf("{{yellow}}CPU usage:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, "avalanche_go_memstats_alloc_bytes")
			utils.Outf("{{yellow}}memory (avalanchego) usage:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_go_memstats_alloc_bytes", chainID))
			utils.Outf("{{yellow}}memory (morpheusvm) usage:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_handler_chits_sum[5s])/1000000/5 + increase(avalanche_%s_handler_notify_sum[5s])/1000000/5 + increase(avalanche_%s_handler_get_sum[5s])/1000000/5 + increase(avalanche_%s_handler_push_query_sum[5s])/1000000/5 + increase(avalanche_%s_handler_put_sum[5s])/1000000/5 + increase(avalanche_%s_handler_pull_query_sum[5s])/1000000/5 + increase(avalanche_%s_handler_query_failed_sum[5s])/1000000/5", chainID, chainID, chainID, chainID, chainID, chainID, chainID))
			utils.Outf("{{yellow}}consensus engine processing (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunk_build_count[5s])/5", chainID))
			utils.Outf("{{yellow}}chunks built per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunk_build_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}chunk build (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_build_count[5s])/5", chainID))
			utils.Outf("{{yellow}}blocks built per second:{{/}} %s\n", panels[len(panels)-1])

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

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_execute_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}block execute [async] (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_state_merkleDB_intermediate_node_cache_hit[5s])/(increase(avalanche_%s_vm_state_merkleDB_intermediate_node_cache_miss[5s]) + increase(avalanche_%s_vm_state_merkleDB_intermediate_node_cache_hit[5s]))", chainID, chainID, chainID))
			utils.Outf("{{yellow}}intermediate node cache hit rate:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_state_merkleDB_value_node_cache_hit[5s])/(increase(avalanche_%s_vm_state_merkleDB_value_node_cache_miss[5s]) + increase(avalanche_%s_vm_state_merkleDB_value_node_cache_hit[5s]))", chainID, chainID, chainID))
			utils.Outf("{{yellow}}value node cache hit rate:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_executor_executable[5s]) / (increase(avalanche_%s_vm_hypersdk_chain_executor_blocked[5s]) + increase(avalanche_%s_vm_hypersdk_chain_executor_executable[5s]))", chainID, chainID, chainID))
			utils.Outf("{{yellow}}txs executable (%%) per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_handler_unprocessed_msgs_len", chainID))
			utils.Outf("{{yellow}}unprocessed messages:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_handler_async_unprocessed_msgs_len", chainID))
			utils.Outf("{{yellow}}async unprocessed messages:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_handler_app_gossip_msg_handling_count[5s])/5", chainID))
			utils.Outf("{{yellow}}app gossip messages processed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunks_received[5s])/5", chainID))
			utils.Outf("{{yellow}}chunks received per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_sigs_received[5s])/5", chainID))
			utils.Outf("{{yellow}}signatures received per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_certs_received[5s])/5", chainID))
			utils.Outf("{{yellow}}certificates received per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_go_goroutines", chainID))
			utils.Outf("{{yellow}}goroutines:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, "avalanche_go_goroutines")
			utils.Outf("{{yellow}}avalanchego goroutines:{{/}} %s\n", panels[len(panels)-1])

			return panels
		})
	},
}
