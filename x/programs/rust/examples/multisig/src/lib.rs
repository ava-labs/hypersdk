use borsh::{BorshDeserialize, BorshSerialize};
use wasmlanche_sdk::types::Address;
#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, DeferDeserialize, ExternalCallError, Program};

pub const MIN_VOTES: u32 = 2;

#[derive(Debug, BorshSerialize, BorshDeserialize)]
pub struct Proposal {
    to: Program,
    method: String,
    args: Vec<u8>,
    value: u64,
    executed: bool,
    voters_len: u32,
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
    InexistentProposal,
    QuorumNotReached,
    NotVoter,
    ExecutionFailed(ExternalCallError),
}

#[state_keys]
pub enum StateKeys {
    LastProposalId,
    Proposals(u32),
    Voter(u32, usize),
    Yeas(u32),
    Nays(u32),
}

#[public]
pub fn propose(
    context: Context<StateKeys>,
    voters_addr: Vec<Address>,
    to: Program,
    method: String,
    args: Vec<u8>,
    value: u64,
) -> u32 {
    let voters_len = voters_addr.len().try_into().expect("too much voters");
    assert!(voters_len >= MIN_VOTES);
    assert!(is_not_duplicate(&voters_addr));
    let voters: Vec<_> = voters_addr
        .iter()
        .map(|address| Voter {
            address: *address,
            voted: false,
        })
        .collect();
    ensure_voter(&context.actor(), &voters).unwrap();

    let program = context.program();
    let last_id = last_proposal_id(program);

    program
        .state()
        .store_by_key(
            StateKeys::Proposals(last_id),
            &Proposal {
                to,
                method,
                args,
                value,
                executed: false,
                voters_len,
            },
        )
        .unwrap();

    for (i, voter) in voters.into_iter().enumerate() {
        program
            .state()
            .store_by_key(StateKeys::Voter(last_id, i), &voter)
            .unwrap();
    }

    program
        .state()
        .store([
            (StateKeys::Nays(last_id), &0),
            (StateKeys::Yeas(last_id), &1),
        ])
        .unwrap();

    program
        .state()
        .store_by_key(StateKeys::LastProposalId, &(last_id + 1))
        .unwrap();

    last_id
}

#[public]
pub fn vote(context: Context<StateKeys>, id: u32, yea: bool) -> Result<(), ProposalError> {
    let program = context.program();
    let proposal = proposal_at(program, id).ok_or(ProposalError::InexistentProposal)?;

    if proposal.executed {
        return Err(ProposalError::AlreadyExecuted);
    }

    let actor = context.actor();
    let voters: Vec<_> = (0..proposal.voters_len)
        .map(|i| {
            program
                .state()
                .get(StateKeys::Voter(id, i as usize))
                .unwrap()
                .unwrap()
        })
        .collect();

    let i = ensure_voter(&actor, &voters)?;

    if yea {
        program
            .state()
            .store_by_key(
                StateKeys::Yeas(id),
                &(program
                    .state()
                    .get::<u32>(StateKeys::Yeas(id))
                    .unwrap()
                    .unwrap()
                    + 1),
            )
            .unwrap();
    } else {
        program
            .state()
            .store_by_key(
                StateKeys::Nays(id),
                &(program
                    .state()
                    .get::<u32>(StateKeys::Nays(id))
                    .unwrap()
                    .unwrap()
                    + 1),
            )
            .unwrap();
    }

    program
        .state()
        .store_by_key(
            StateKeys::Voter(id, i as usize),
            &Voter {
                address: actor,
                voted: true,
            },
        )
        .unwrap();

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

    let yeas = program
        .state()
        .get(StateKeys::Yeas(proposal_id))
        .unwrap()
        .unwrap();
    let nays = program
        .state()
        .get(StateKeys::Nays(proposal_id))
        .unwrap()
        .unwrap();
    if !quorum_reached(proposal.voters_len, yeas, nays) {
        return Err(ProposalError::QuorumNotReached);
    }

    proposal.executed = true;

    proposal
        .to
        .call_function::<DeferDeserialize>(
            &proposal.method,
            &proposal.args,
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
pub fn pub_last_proposal_id(context: Context<StateKeys>) -> u32 {
    last_proposal_id(context.program())
}

#[public]
pub fn pub_quorum_reached(
    context: Context<StateKeys>,
    proposal_id: u32,
) -> Result<bool, ProposalError> {
    let program = context.program();

    let proposal = proposal_at(program, proposal_id).ok_or(ProposalError::InexistentProposal)?;
    let yeas = program
        .state()
        .get(StateKeys::Yeas(proposal_id))
        .unwrap()
        .unwrap();
    let nays = program
        .state()
        .get(StateKeys::Nays(proposal_id))
        .unwrap()
        .unwrap();

    Ok(quorum_reached(proposal.voters_len, yeas, nays))
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

pub fn quorum_reached(max_votes: u32, yeas: u32, nays: u32) -> bool {
    yeas + nays >= MIN_VOTES && yeas > nays && yeas >= (max_votes + 1) / 2
}

pub fn is_not_duplicate(voters: &[Address]) -> bool {
    if voters.is_empty() {
        panic!("there should be at least one voter");
    }

    for (i, vi) in voters.iter().enumerate() {
        for (j, vj) in voters[1..].iter().enumerate() {
            if i != j + 1 && vi == vj {
                return false;
            }
        }
    }

    true
}

/// Check if the passed actor is part of the voters and return its index on success
pub fn ensure_voter(actor: &Address, voters: &[Voter]) -> Result<u32, ProposalError> {
    let mut i = 0;
    loop {
        if i < voters.len() {
            let voter = &voters[i];

            if &voter.address == actor {
                if voter.voted {
                    return Err(ProposalError::AlreadyVoted);
                } else {
                    break;
                }
            }

            i += 1;
        } else {
            return Err(ProposalError::NotVoter);
        }
    }

    Ok(i as u32)
}

#[cfg(test)]
mod tests {
    use super::ProposalError;
    use simulator::{Endpoint, Param, Step, StepResponseError, TestContext};
    use wasmlanche_sdk::types::Address;
    use wasmlanche_sdk::DeferDeserialize;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn cannot_propose_invalid_with_params() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let test_context = TestContext::from(program_id);

        assert!(matches!(
            simulator
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
                .response::<()>(),
            Err(StepResponseError::ExternalCall(_))
        ));

        let voters = vec![
            Address::new([1; Address::LEN]),
            Address::new([1; Address::LEN]),
        ];

        assert!(matches!(
            simulator
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
                .response::<()>(),
            Err(StepResponseError::ExternalCall(_))
        ));
    }

    #[test]
    fn cannot_double_vote() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let mut test_context = TestContext::from(program_id);

        let voter1 = Address::from_str("voter1");
        let voter2 = Address::from_str("voter2");

        let voters = vec![voter1, voter2];

        test_context.actor = voter1;

        let pid: u32 = simulator
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

        let res = dbg!(simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "vote".to_string(),
                max_units: u64::MAX,
                params: vec![voter_context.clone().into(), pid.into(), true.into()],
            })
            .unwrap()
            .result
            .response::<Result<(), ProposalError>>()
            .unwrap());

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
    fn execute_proposal() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let mut test_context = TestContext::from(program_id);

        let voter1 = Address::from_str("voter1");
        let voter2 = Address::from_str("voter2");

        let voters = vec![voter1, voter2];

        test_context.actor = voter1;

        let pid: u32 = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "propose".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    Param::FixedBytes(borsh::to_vec(&voters).unwrap()),
                    program_id.into(),
                    String::from("pub_last_proposal_id").into(),
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

        let id: u32 = res.deserialize().unwrap();

        assert_eq!(id, 1);
    }
}
