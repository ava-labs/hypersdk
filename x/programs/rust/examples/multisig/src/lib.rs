use borsh::{BorshDeserialize, BorshSerialize};
use std::collections::HashSet;
use wasmlanche_sdk::types::Address;
#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, DeferDeserialize, ExternalCallError, Program};

pub const MIN_VOTES: u32 = 2;

#[derive(Debug, BorshSerialize, BorshDeserialize)]
pub struct Proposal {
    to: Program,
    method: String,
    data: Vec<u8>,
    value: u64,
    yea: u32,
    nay: u32,
    executed: bool,
    voters: HashSet<Address>,
}

#[derive(Debug, BorshSerialize, BorshDeserialize)]
pub enum ProposalError {
    AlreadyExecuted,
    AlreadyVoted,
    InexistentProposal,
    QuorumNotReached,
    ExecutionFailed(ExternalCallError),
}

#[state_keys]
pub enum StateKeys {
    LastProposalId,
    Proposals(u32),
}

#[public]
pub fn propose(
    context: Context<StateKeys>,
    to: Program,
    method: String,
    data: Vec<u8>,
    value: u64,
) -> u32 {
    let program = context.program();
    let last_id = proposal_id(program);

    program
        .state()
        .store_by_key(
            StateKeys::Proposals(last_id),
            &Proposal {
                to,
                method,
                data,
                value,
                yea: 1,
                nay: 0,
                executed: false,
                voters: HashSet::from([context.actor()]),
            },
        )
        .expect("state corrupt");

    program
        .state()
        .store_by_key(StateKeys::LastProposalId, &(last_id + 1))
        .unwrap();

    last_id
}

#[public]
pub fn vote(context: Context<StateKeys>, id: u32, yea: bool) -> Result<(), ProposalError> {
    let program = context.program();
    let mut proposal = proposal_at(program, id).ok_or(ProposalError::InexistentProposal)?;

    let actor = context.actor();
    if proposal.voters.contains(&actor) {
        return Err(ProposalError::AlreadyVoted);
    }
    if proposal.executed {
        return Err(ProposalError::AlreadyExecuted);
    }

    if yea {
        proposal.yea += 1;
    } else {
        proposal.nay += 1;
    }
    proposal.voters.insert(actor);

    program
        .state()
        .store_by_key(StateKeys::Proposals(id), &proposal)
        .expect("state corrupt");

    Ok(())
}

#[public]
pub fn execute(
    context: Context<StateKeys>,
    proposal_id: u32,
    max_units: u64,
) -> Result<DeferDeserialize, ProposalError> {
    let program = context.program();
    let mut proposal =
        proposal_at(program, proposal_id).ok_or(ProposalError::InexistentProposal)?;

    if proposal.executed {
        return Err(ProposalError::AlreadyExecuted);
    }
    if !quorum_reached(&proposal) {
        return Err(ProposalError::QuorumNotReached);
    }

    proposal.executed = true;

    proposal
        .to
        .call_function::<DeferDeserialize>(
            &proposal.method,
            &proposal.data,
            max_units,
            proposal.value,
        )
        .map_err(ProposalError::ExecutionFailed)
}

#[public]
pub fn pub_proposal_at(context: Context<StateKeys>, proposal_id: u32) -> Option<Proposal> {
    proposal_at(context.program(), proposal_id)
}

#[public]
pub fn pub_proposal_id(context: Context<StateKeys>) -> u32 {
    proposal_id(context.program())
}

#[public]
pub fn pub_quorum_reached(
    context: Context<StateKeys>,
    proposal_id: u32,
) -> Result<bool, ProposalError> {
    let proposal =
        proposal_at(context.program(), proposal_id).ok_or(ProposalError::InexistentProposal)?;
    Ok(quorum_reached(&proposal))
}

pub fn proposal_at(program: &Program<StateKeys>, proposal_id: u32) -> Option<Proposal> {
    program
        .state()
        .get(StateKeys::Proposals(proposal_id))
        .expect("state corrupt")
}

pub fn last_proposal_id(program: &Program<StateKeys>) -> u32 {
    program
        .state()
        .get(StateKeys::LastProposalId)
        .expect("state corrupt")
        .unwrap_or_default()
}

pub fn quorum_reached(proposal: &Proposal) -> bool {
    proposal.yea + proposal.nay >= MIN_VOTES && proposal.yea > proposal.nay
}

#[cfg(test)]
mod tests {
    use super::ProposalError;
    use simulator::{Endpoint, Key, Step, TestContext};
    use wasmlanche_sdk::DeferDeserialize;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn can_propose() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let test_context = TestContext::from(program_id);

        let pid: u32 = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "propose".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    program_id.into(),
                    String::new().into(),
                    Vec::new().into(),
                    0u64.into(),
                ],
            })
            .unwrap()
            .result
            .response()
            .unwrap();

        assert_eq!(pid, 0);

        let new_pid: u32 = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "pub_proposal_id".to_string(),
                max_units: 0,
                params: vec![test_context.clone().into()],
            })
            .unwrap()
            .result
            .response()
            .unwrap();

        assert_eq!(new_pid, 1);
    }

    #[test]
    fn cannot_double_vote() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let test_context = TestContext::from(program_id);

        let pid: u32 = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "propose".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    program_id.into(),
                    String::new().into(),
                    Vec::new().into(),
                    0u64.into(),
                ],
            })
            .unwrap()
            .result
            .response()
            .unwrap();

        let voter = "voter";
        let voter_key = Key::Ed25519(voter.to_string());
        simulator
            .run_step(&Step::create_key(voter_key.clone()))
            .unwrap();

        let voter_context = TestContext {
            actor_key: Some(voter_key),
            ..test_context.clone()
        };

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "vote".to_string(),
                max_units: u64::MAX,
                params: vec![voter_context.clone().into(), pid.into(), true.into()],
            })
            .unwrap()
            .result
            .response::<Result<(), ProposalError>>()
            .unwrap();

        assert!(res.is_ok());

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "vote".to_string(),
                max_units: u64::MAX,
                params: vec![voter_context.clone().into(), pid.into(), true.into()],
            })
            .unwrap()
            .result
            .response::<Result<(), ProposalError>>()
            .unwrap();

        assert!(matches!(res, Err(ProposalError::AlreadyVoted)));
    }

    #[test]
    fn execute() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let test_context = TestContext::from(program_id);

        let pid: u32 = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "propose".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    program_id.into(),
                    String::from("pub_proposal_id").into(),
                    Vec::new().into(),
                    0u64.into(),
                ],
            })
            .unwrap()
            .result
            .response()
            .unwrap();

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "execute".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.clone().into(), pid.into(), u64::MAX.into()],
            })
            .unwrap()
            .result
            .response::<Result<(), ProposalError>>()
            .unwrap();

        assert!(matches!(res, Err(ProposalError::QuorumNotReached)));

        let voter = "voter";
        let voter_key = Key::Ed25519(voter.to_string());
        simulator
            .run_step(&Step::create_key(voter_key.clone()))
            .unwrap();

        let voter_context = TestContext {
            actor_key: Some(voter_key),
            ..test_context.clone()
        };

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "vote".to_string(),
                max_units: u64::MAX,
                params: vec![voter_context.clone().into(), pid.into(), true.into()],
            })
            .unwrap()
            .result
            .response::<Result<(), ProposalError>>()
            .unwrap();

        assert!(res.is_ok());

        let reached_res = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "pub_quorum_reached".to_string(),
                max_units: 0,
                params: vec![test_context.clone().into(), pid.into()],
            })
            .unwrap()
            .result
            .response::<Result<bool, ProposalError>>()
            .unwrap();

        assert!(reached_res.is_ok_and(|reached| reached));

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "execute".to_string(),
                max_units: u64::MAX,
                params: vec![voter_context.clone().into(), pid.into(), 1000000u64.into()],
            })
            .unwrap()
            .result
            .response::<Result<DeferDeserialize, ProposalError>>()
            .unwrap()
            .unwrap();

        let bytes: Vec<u8> = res.deserialize().unwrap();

        assert_eq!(borsh::from_slice::<u32>(&bytes).unwrap(), 1);
    }
}
