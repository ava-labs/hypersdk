use borsh::{BorshDeserialize, BorshSerialize};
use std::collections::HashSet;
#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, types::Address, Program};
use wasmlanche_sdk::{DeferDeserialize, ExternalCallError};

const MIN_VOTES: u32 = 2;

const MIN_VOTES: u32 = 2;

#[derive(Debug, BorshSerialize, BorshDeserialize)]
pub struct Proposal {
    to: Program,
    method: String,
    data: Vec<u8>,
    yea: u32,
    nay: u32,
    executed: bool,
    voters: HashSet<Address>,
}

#[derive(Debug, BorshSerialize)]
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
pub fn propose(context: Context<StateKeys>, to: Program, method: String, data: Vec<u8>) -> u32 {
    let program = context.program();
    let last_id = proposal_id(&program);

    program
        .state()
        .store_by_key(
            StateKeys::Proposals(last_id),
            &Proposal {
                to,
                method,
                data,
                yea: 1,
                nay: 0,
                executed: false,
                voters: HashSet::from([context.actor()]),
            },
        )
        .expect("state corrupt");

    last_id
}

#[public]
pub fn vote(context: Context<StateKeys>, id: u32, yea: bool) -> Result<(), ProposalError> {
    let program = context.program();
    let mut proposal = proposal_at(&program, id).ok_or(ProposalError::InexistentProposal)?;

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
) -> Result<Vec<u8>, ProposalError> {
    let program = context.program();
    let mut proposal =
        proposal_at(&program, proposal_id).ok_or(ProposalError::InexistentProposal)?;

    if proposal.executed {
        return Err(ProposalError::AlreadyExecuted);
    }
    if !quorum_reached(&proposal) {
        return Err(ProposalError::QuorumNotReached);
    }

    proposal.executed = true;

    Ok(proposal
        .to
        .call_function::<DeferDeserialize>(&proposal.method, &proposal.data, max_units)
        .map_err(ProposalError::ExecutionFailed)?
        .into_inner())
}

#[public]
pub fn pub_proposal_at(context: Context<StateKeys>, proposal_id: u32) -> Option<Proposal> {
    proposal_at(context.program(), proposal_id)
}

#[public]
pub fn pub_proposal_id(context: Context<StateKeys>) -> u32 {
    proposal_id(context.program())
}

fn proposal_at(program: &Program<StateKeys>, proposal_id: u32) -> Option<Proposal> {
    program
        .state()
        .get(StateKeys::Proposals(proposal_id))
        .expect("state corrupt")
        .flatten()
}

fn proposal_id(program: &Program<StateKeys>) -> u32 {
    program
        .state()
        .get(StateKeys::LastProposalId)
        .expect("state corrupt")
        .unwrap_or_default()
}

fn quorum_reached(proposal: &Proposal) -> bool {
    proposal.yea + proposal.nay >= MIN_VOTES && proposal.yea > proposal.nay
}

#[cfg(test)]
mod tests {
    use simulator::{Endpoint, Key, Param, Step, TestContext};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn can_propose() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner = String::from("owner");

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
                    String::from("asdasdasd").into(),
                    Vec::new().into(),
                ],
            })
            .unwrap()
            .result
            .response()
            .unwrap();

        assert_eq!(pid, 0);

        let pid: u32 = simulator
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

        assert_eq!(pid, 0);
    }
}
