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

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_included[5s])/5", chainID))
			utils.Outf("{{yellow}}all transactions processed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_blks_processing", chainID))
			utils.Outf("{{yellow}}blocks processing:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_blks_accepted_count[5s])/5", chainID))
			utils.Outf("{{yellow}}blocks accepted per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_blks_rejected_count[5s])/5", chainID))
			utils.Outf("{{yellow}}blocks rejected per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_last_accepted_epoch", chainID))
			utils.Outf("{{yellow}}last accepted epoch:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_last_executed_epoch", chainID))
			utils.Outf("{{yellow}}last executed epoch:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_submitted[5s])/5", chainID))
			utils.Outf("{{yellow}}transactions submitted per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_gossiped[5s])/5", chainID))
			utils.Outf("{{yellow}}transactions gossiped per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_tx_gossip_dropped[5s])/5", chainID))
			utils.Outf("{{yellow}}transaction gossip dropped per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_received[5s])/5", chainID))
			utils.Outf("{{yellow}}transactions received per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_txs_invalid[5s])/5", chainID))
			utils.Outf("{{yellow}}invalid transactions processed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_state_changes[5s])/5", chainID))
			utils.Outf("{{yellow}}state changes per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_repeat_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_wait_repeat_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}repeat wait (ms/block):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_queue_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_wait_queue_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}queue wait (ms/block):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_auth_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_wait_auth_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}auth wait (ms/block):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_precheck_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_wait_precheck_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}precheck wait (ms/block):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_exec_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_wait_exec_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}execution wait (ms/block):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_commit_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_wait_commit_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}commit wait (ms/block):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_trie_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_wait_trie_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}trie wait (ms/attempt):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_trie_node_changes_sum[5s])/increase(avalanche_%s_vm_hypersdk_chain_trie_node_changes_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}trie node changes (changes/attempt):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_trie_value_changes_sum[5s])/increase(avalanche_%s_vm_hypersdk_chain_trie_value_changes_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}trie value changes (changes/attempt):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_trie_skipped_value_changes_sum[5s])/increase(avalanche_%s_vm_hypersdk_chain_trie_skipped_value_changes_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}skipped trie value changes (changes/attempt):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_wait_root_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_wait_root_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}root wait (ms/attempt):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_remaining_mempool[5s])/5", chainID))
			utils.Outf("{{yellow}}remaining mempool per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_mempool_len", chainID))
			utils.Outf("{{yellow}}mempool count:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_mempool_size", chainID))
			utils.Outf("{{yellow}}mempool size (bytes):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, "avalanche_resource_tracker_cpu_usage")
			utils.Outf("{{yellow}}CPU usage:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, "avalanche_go_memstats_alloc_bytes")
			utils.Outf("{{yellow}}memory (avalanchego) usage:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_go_memstats_alloc_bytes", chainID))
			utils.Outf("{{yellow}}memory (morpheusvm) usage:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_data_size", chainID))
			utils.Outf("{{yellow}}chainData size:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_state_size", chainID))
			utils.Outf("{{yellow}}state size:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_state_len", chainID))
			utils.Outf("{{yellow}}state items:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_handler_chits_sum[5s])/1000000/5 + increase(avalanche_%s_handler_notify_sum[5s])/1000000/5 + increase(avalanche_%s_handler_get_sum[5s])/1000000/5 + increase(avalanche_%s_handler_push_query_sum[5s])/1000000/5 + increase(avalanche_%s_handler_put_sum[5s])/1000000/5 + increase(avalanche_%s_handler_pull_query_sum[5s])/1000000/5 + increase(avalanche_%s_handler_query_failed_sum[5s])/1000000/5", chainID, chainID, chainID, chainID, chainID, chainID, chainID))
			utils.Outf("{{yellow}}consensus engine processing (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunk_build_count[5s])/5", chainID))
			utils.Outf("{{yellow}}chunks built per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunk_build_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}chunk build (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunk_build_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_chunk_build_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}chunk build (ms/chunk):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunk_build_txs_dropped[5s])/5", chainID))
			utils.Outf("{{yellow}}chunk build txs dropped per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunk_bytes_built[5s])/5", chainID))
			utils.Outf("{{yellow}}chunk bytes built per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunk_auth_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_chunk_auth_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}chunk authorization (ms/chunk):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunk_auth_count[5s])/5", chainID))
			utils.Outf("{{yellow}}chunk authorizations per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_useless_chunk_auth[5s])/5", chainID))
			utils.Outf("{{yellow}}useless chunk authorizations per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunk_auth_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}chunk authorization (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_build_count[5s])/5", chainID))
			utils.Outf("{{yellow}}blocks built per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_build_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}block build (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_build_certs_dropped[5s])/5", chainID))
			utils.Outf("{{yellow}}block build certs dropped per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_parse_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}block parse (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_verify_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}block verify (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_verify_failed[5s])/5", chainID))
			utils.Outf("{{yellow}}blocks verify failed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_accept_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}block accept (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_process_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}block process [async] (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_block_execute_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_block_execute_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}block execute [async] (ms/block):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_units_executed_bandwidth[5s])/5", chainID))
			utils.Outf("{{yellow}}bandwidth units executed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_units_executed_compute[5s])/5", chainID))
			utils.Outf("{{yellow}}compute units executed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_units_executed_read[5s])/5", chainID))
			utils.Outf("{{yellow}}read units executed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_units_executed_allocate[5s])/5", chainID))
			utils.Outf("{{yellow}}allocate units executed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_units_executed_write[5s])/5", chainID))
			utils.Outf("{{yellow}}write units executed per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_optimistic_certified_gossip[5s])/5", chainID))
			utils.Outf("{{yellow}}optimistic certified gossip per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_fetch_missing_chunks_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_fetch_missing_chunks_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}missing chunk fetch (ms/block):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_fetch_chunk_attempts[5s])/5", chainID))
			utils.Outf("{{yellow}}chunk fetch attempts per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_useless_fetch_chunk_attempts[5s])/5", chainID))
			utils.Outf("{{yellow}}useless chunk fetch attempts per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_collect_chunk_signatures_sum[5s])/1000000/increase(avalanche_%s_vm_hypersdk_chain_collect_chunk_signatures_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}collect chunk signatures (ms/chunk):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_chunks_executed[5s])/increase(avalanche_%s_vm_hypersdk_chain_block_execute_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}chunks per executed block:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_gossip_tx_msg_invalid[5s])/5", chainID))
			utils.Outf("{{yellow}}invalid tx msgs received over gossip per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_gossip_tx_invalid[5s])/5", chainID))
			utils.Outf("{{yellow}}invalid txs received over gossip per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_gossip_chunk_invalid[5s])/5", chainID))
			utils.Outf("{{yellow}}invalid chunks received over gossip per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_gossip_chunk_sig_invalid[5s])/5", chainID))
			utils.Outf("{{yellow}}invalid chunk signatures over gossip per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_gossip_cert_invalid[5s])/5", chainID))
			utils.Outf("{{yellow}}invalid certs received over gossip per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_engine_backlog", chainID))
			utils.Outf("{{yellow}}block execution backlog:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_gossip_tx_backlog", chainID))
			utils.Outf("{{yellow}}gossip transaction backlog:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_rpc_tx_backlog", chainID))
			utils.Outf("{{yellow}}RPC transaction backlog:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_websocket_connections", chainID))
			utils.Outf("{{yellow}}websocket connections:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_rpc_tx_invalid[5s])/5", chainID))
			utils.Outf("{{yellow}}invalid txs received over RPC per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_tx_time_remaining_mempool_sum[5s])/increase(avalanche_%s_vm_hypersdk_chain_tx_time_remaining_mempool_count[5s])", chainID, chainID))
			utils.Outf("{{yellow}}validity time remaining when tx added to mempool (ms):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_mempool_expired[5s])/5", chainID))
			utils.Outf("{{yellow}}expired txs from mempool per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hypersdk_chain_executed_processing_backlog", chainID))
			utils.Outf("{{yellow}}executed chunk processing backlog:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_tx_rpc_authorized[5s])/5", chainID))
			utils.Outf("{{yellow}}RPC pre-authorized per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_executed_chunk_process_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}chunk process [async] (ms/s):{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_executed_block_process_sum[5s])/1000000/5", chainID))
			utils.Outf("{{yellow}}executed block process [async] (ms/s):{{/}} %s\n", panels[len(panels)-1])

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

			panels = append(panels, "increase(avalanche_network_app_gossip_sent_bytes[5s])/5")
			utils.Outf("{{yellow}}sent app gossip bytes:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, "increase(avalanche_network_app_gossip_received_bytes[5s])/5")
			utils.Outf("{{yellow}}received app gossip bytes:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, "avalanche_go_goroutines")
			utils.Outf("{{yellow}}avalanchego goroutines:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_deleted_blocks[5s])/5", chainID))
			utils.Outf("{{yellow}}blocks deleted per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_deleted_useless_chunks[5s])/5", chainID))
			utils.Outf("{{yellow}}useless chunks deleted per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_expired_built_chunks[5s])/5", chainID))
			utils.Outf("{{yellow}}expired chunks per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_chain_expired_certs[5s])/5", chainID))
			utils.Outf("{{yellow}}expired certs per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_deleted_included_chunks[5s])/5", chainID))
			utils.Outf("{{yellow}}included chunks deleted per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hypersdk_vm_deleted_filtered_chunks[5s])/5", chainID))
			utils.Outf("{{yellow}}filtered chunks deleted per second:{{/}} %s\n", panels[len(panels)-1])

			panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_statedb_pebble_l0_compactions[5s])/5 + increase(avalanche_%s_vm_statedb_pebble_other_compactions[5s])/5", chainID, chainID))
			utils.Outf("{{yellow}}statedb compactions per second:{{/}} %s\n", panels[len(panels)-1])

			return panels
		})
	},
}
