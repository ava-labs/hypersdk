use borsh::{BorshDeserialize, BorshSerialize};
use wasmlanche_sdk::{public, state_keys, Address, Context, DeferDeserialize, Program};

pub const MIN_VOTES: u64 = 2;

#[derive(Debug, BorshSerialize, BorshDeserialize)]
pub struct Proposal {
    to: Program,
    method: String,
    args: Vec<u8>,
    value: u64,
    executed_result: Option<Vec<u8>>,
    voters_len: u64,
}

#[derive(Debug, BorshSerialize, BorshDeserialize)]
pub struct Voter {
    address: Address,
    voted: bool,
}

#[derive(Debug, BorshSerialize, BorshDeserialize)]
pub enum ProposalError {
    AlreadyExecuted,
    AlreadyVoted,
    NonexistentProposal,
    QuorumNotReached,
    NotVoter,
}

#[state_keys]
pub enum StateKeys {
    NextProposalId,
    Proposals(u64),
    Voter(u64, usize),
    Yeas(u64),
    Nays(u64),
}

/// Add a new proposal with a permissioned set of voters.
/// If the vote is in favor of the proposal, it will be possible to execute an arbitrary call to the passed program address.
/// The caller is expected to be included in the set and should thus be the first voter.
#[public]
pub fn propose(
    context: Context<StateKeys>,
    voters: Vec<Address>,
    to: Program,
    method: String,
    args: Vec<u8>,
    value: u64,
) -> u64 {
    let voters_len = voters.len().try_into().expect("too many voters");
    assert!(voters_len >= MIN_VOTES);

    if voters[0] != context.actor() {
        panic!("the proposer is expected to be the first in the voters set");
    }

    assert!(internal::is_not_duplicate(&voters));

    let mut voters: Vec<_> = voters
        .iter()
        .copied()
        .map(|address| Voter {
            address,
            voted: false,
        })
        .collect();

    voters[0].voted = true;

    let next_id = internal::next_proposal_id(&context);

    context
        .store_by_key(
            StateKeys::Proposals(next_id),
            &Proposal {
                to,
                method,
                args,
                value,
                executed_result: None,
                voters_len,
            },
        )
        .expect("failed to store proposal");

    for (i, voter) in voters.into_iter().enumerate() {
        context
            .store_by_key(StateKeys::Voter(next_id, i), &voter)
            .expect("failed to store voter");
    }

    context
        .store([
            (StateKeys::Nays(next_id), &0u64),
            (StateKeys::Yeas(next_id), &1),
        ])
        .expect("failed to store nays and yeas");

    context
        .store_by_key(StateKeys::NextProposalId, &(next_id + 1))
        .expect("failed to store last proposal id");

    next_id
}

/// Cast a vote to the proposal with the given id.
#[public]
pub fn vote(
    context: Context<StateKeys>,
    voter_id: u64,
    proposal_id: u64,
    yea: bool,
) -> Result<(), ProposalError> {
    let proposal =
        internal::get_proposal(&context, proposal_id).ok_or(ProposalError::NonexistentProposal)?;

    if proposal.executed_result.is_some() {
        return Err(ProposalError::AlreadyExecuted);
    }

    let voter = context
        .get::<Voter>(StateKeys::Voter(proposal_id, voter_id as usize))
        .expect("failed to get voter")
        .ok_or(ProposalError::NotVoter)?;

    if voter.voted {
        return Err(ProposalError::AlreadyVoted);
    }

    if yea {
        let yeas = context
            .get::<u64>(StateKeys::Yeas(proposal_id))
            .expect("failed to get yeas")
            .expect("votes should be initialized");

        context
            .store_by_key(StateKeys::Yeas(proposal_id), &(yeas + 1))
            .expect("failed to store yeas");
    } else {
        let nays = context
            .get::<u64>(StateKeys::Nays(proposal_id))
            .expect("failed to get nays")
            .expect("votes should be initialized");

        context
            .store_by_key(StateKeys::Nays(proposal_id), &(nays + 1))
            .expect("failed to store nays");
    }

    context
        .store_by_key(
            StateKeys::Voter(proposal_id, voter_id as usize),
            &Voter {
                address: context.actor(),
                voted: true,
            },
        )
        .expect("failed to store voter");

    Ok(())
}

/// Execute a proposal whose quorum has been reached.
#[public]
pub fn execute(
    context: Context<StateKeys>,
    proposal_id: u64,
    max_units: u64,
) -> Result<(), ProposalError> {
    let mut proposal =
        internal::get_proposal(&context, proposal_id).ok_or(ProposalError::NonexistentProposal)?;

    if proposal.executed_result.is_some() {
        return Err(ProposalError::AlreadyExecuted);
    }

    let yeas = context
        .get(StateKeys::Yeas(proposal_id))
        .expect("failed to get yeas")
        .expect("votes should be initialized");
    let nays = context
        .get(StateKeys::Nays(proposal_id))
        .expect("failed to get nays")
        .expect("votes should be initialized");

    if !internal::quorum_reached(proposal.voters_len, yeas, nays) {
        return Err(ProposalError::QuorumNotReached);
    }

    let result = proposal
        .to
        .call_function::<DeferDeserialize>(
            &proposal.method,
            &proposal.args,
            max_units,
            proposal.value,
        )
        .expect("the external execution failed");

    proposal.executed_result = Some(result.into());

    context
        .store_by_key(StateKeys::Proposals(proposal_id), &proposal)
        .expect("failed to store proposal");

    Ok(())
}

#[public]
pub fn get_proposal(context: Context<StateKeys>, proposal_id: u64) -> Proposal {
    internal::get_proposal(&context, proposal_id).expect("nonexistent proposal")
}

/// Return the id of the next proposal.
#[public]
pub fn next_proposal_id(context: Context<StateKeys>) -> u64 {
    internal::next_proposal_id(&context)
}

/// Return wether or not the quorum of the passed proposal id has been reached.
#[public]
pub fn quorum_reached(
    context: Context<StateKeys>,
    proposal_id: u64,
) -> Result<bool, ProposalError> {
    let proposal =
        internal::get_proposal(&context, proposal_id).ok_or(ProposalError::NonexistentProposal)?;

    let yeas = context
        .get(StateKeys::Yeas(proposal_id))
        .expect("failed to get yeas")
        .expect("votes should be initialized");
    let nays = context
        .get(StateKeys::Nays(proposal_id))
        .expect("failed to get nays")
        .expect("votes should be initialized");

    Ok(internal::quorum_reached(proposal.voters_len, yeas, nays))
}

mod internal {
    use super::*;

    pub fn get_proposal(context: &Context<StateKeys>, proposal_id: u64) -> Option<Proposal> {
        context
            .get(StateKeys::Proposals(proposal_id))
            .expect("state corrupt")
    }

    pub fn next_proposal_id(context: &Context<StateKeys>) -> u64 {
        context
            .get(StateKeys::NextProposalId)
            .expect("state corrupt")
            .unwrap_or_default()
    }

    pub fn quorum_reached(max_votes: u64, yeas: u64, nays: u64) -> bool {
        yeas + nays >= MIN_VOTES && yeas > nays && yeas >= (max_votes + 1) / 2
    }

    pub fn is_not_duplicate(voters: &[Address]) -> bool {
        for (i, vi) in voters.iter().enumerate() {
            for (j, vj) in voters[1..].iter().enumerate() {
                if i != j + 1 && vi == vj {
                    return false;
                }
            }
        }

        true
    }
}

#[cfg(test)]
mod tests {
    use super::{Proposal, ProposalError};
    use simulator::{Endpoint, Param, Step, StepResponseError, TestContext};
    use wasmlanche_sdk::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn cannot_propose_invalid_with_params() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let test_context = TestContext::from(program_id);

        let response = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "propose".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    Vec::new().into(),
                    program_id.into(),
                    String::new().into(),
                    Vec::new().into(),
                    0u64.into(),
                ],
            })
            .unwrap()
            .result
            .response::<()>();
        assert!(matches!(response, Err(StepResponseError::ExternalCall(_))));

        let voters = vec![
            Address::new([1; Address::LEN]),
            Address::new([1; Address::LEN]),
        ];

        let response = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "propose".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    Param::FixedBytes(borsh::to_vec(&voters).unwrap()),
                    program_id.into(),
                    String::new().into(),
                    Vec::new().into(),
                    0u64.into(),
                ],
            })
            .unwrap()
            .result
            .response::<()>();
        assert!(matches!(response, Err(StepResponseError::ExternalCall(_))));

        let test_context = TestContext {
            actor: Address::new([1; Address::LEN]),
            ..test_context.clone()
        };

        let voters = vec![
            Address::new([2; Address::LEN]),
            Address::new([1; Address::LEN]),
        ];

        let response = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "propose".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    Param::FixedBytes(borsh::to_vec(&voters).unwrap()),
                    program_id.into(),
                    String::new().into(),
                    Vec::new().into(),
                    0u64.into(),
                ],
            })
            .unwrap()
            .result
            .response::<()>();
        assert!(matches!(response, Err(StepResponseError::ExternalCall(_))));
    }

    #[test]
    fn cannot_double_vote() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let mut test_context = TestContext::from(program_id);

        let voter1 = Address::new([1; 33]);
        let voter2 = Address::new([2; 33]);

        let voters = vec![voter1, voter2];

        test_context.actor = voter1;

        let pid: u64 = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "propose".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    Param::FixedBytes(borsh::to_vec(&voters).unwrap()),
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

        let voter_context = TestContext {
            actor: voter2,
            ..test_context.clone()
        };

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "vote".to_string(),
                max_units: u64::MAX,
                params: vec![
                    voter_context.clone().into(),
                    1.into(),
                    pid.into(),
                    true.into(),
                ],
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
                params: vec![
                    voter_context.clone().into(),
                    1.into(),
                    pid.into(),
                    true.into(),
                ],
            })
            .unwrap()
            .result
            .response::<Result<(), ProposalError>>()
            .unwrap();

        assert!(matches!(res, Err(ProposalError::AlreadyVoted)));
    }

    #[test]
    fn execute_proposal() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let mut test_context = TestContext::from(program_id);

        let voter1 = Address::new([1; 33]);
        let voter2 = Address::new([2; 33]);

        let voters = vec![voter1, voter2];

        test_context.actor = voter1;

        let pid: u64 = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "propose".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    Param::FixedBytes(borsh::to_vec(&voters).unwrap()),
                    program_id.into(),
                    String::from("next_proposal_id").into(),
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

        let voter_context = TestContext {
            actor: voter2,
            ..test_context.clone()
        };

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "vote".to_string(),
                max_units: u64::MAX,
                params: vec![
                    voter_context.clone().into(),
                    1.into(),
                    pid.into(),
                    true.into(),
                ],
            })
            .unwrap()
            .result
            .response::<Result<(), ProposalError>>()
            .unwrap();

        assert!(res.is_ok());

        let reached_res = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "quorum_reached".to_string(),
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
            .response::<Result<(), ProposalError>>()
            .unwrap();

        assert!(res.is_ok());

        let result = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "get_proposal".to_string(),
                max_units: 0,
                params: vec![voter_context.into(), pid.into()],
            })
            .unwrap()
            .result
            .response::<Proposal>()
            .unwrap();

        let executed_result = result.executed_result.expect("not executed proposal");
        let id: u64 = borsh::from_slice(&executed_result).expect("deserialization failed");

        assert_eq!(id, 1);
    }
}
