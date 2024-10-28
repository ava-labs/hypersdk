// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use counter::Count;
use multisig::{Proposal, ProposalId, Voters};
use std::{collections::HashSet, env, num::NonZeroU32};
use wasmlanche::{
    simulator::{SimpleState, Simulator},
    Address,
};

const CONTRACT_PATH: &str = env!("CONTRACT_PATH");
const GAS: u64 = 10_000_000_000;

#[test]
fn propose_and_execute() {
    let [bob, alice, charlie] = [1, 2, 3].map(|i| Address::new([i; 33]));

    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);

    simulator.set_actor(bob);

    let multisig_address = simulator.create_contract(CONTRACT_PATH).unwrap().address;

    let counter_path = CONTRACT_PATH.replace("multisig", "counter");
    let counter_address = simulator.create_contract(&counter_path).unwrap().address;

    let bob_count: Count = simulator
        .call_contract(counter_address, "get_value", bob, GAS)
        .unwrap();

    assert_eq!(bob_count, 0);

    let expected_bob_count = 5;

    let proposal = Proposal {
        contract_address: counter_address,
        function_name: "inc".to_string(),
        args: wasmlanche::borsh::to_vec(&(bob, expected_bob_count)).unwrap(),
    };

    let voters = Voters::Addresses(HashSet::from([bob, alice, charlie]));
    let quorum = NonZeroU32::new(2).unwrap();

    let proposal_id: ProposalId = simulator
        .call_contract(multisig_address, "propose", (proposal, voters, quorum), GAS)
        .unwrap();

    simulator.set_actor(alice);

    let resultant_bob_count = simulator
        .call_contract::<Option<Vec<u8>>, _>(multisig_address, "vote", (proposal_id, true), GAS)
        .unwrap()
        .unwrap();

    let success: bool = wasmlanche::borsh::from_slice(&resultant_bob_count).unwrap();

    assert!(success);

    let bob_count: Count = simulator
        .call_contract(counter_address, "get_value", bob, GAS)
        .unwrap();

    assert_eq!(bob_count, expected_bob_count);
}
