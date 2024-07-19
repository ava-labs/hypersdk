use borsh::{BorshDeserialize, BorshSerialize};
use wasmlanche_sdk::{
    public, state_keys, Address, Context, DeferDeserialize, ExternalCallError, Program,
};

pub const MIN_VOTES: u64 = 2;

#[derive(Debug, BorshSerialize, BorshDeserialize)]
pub struct Proposal {
    to: Program,
    method: String,
    args: Vec<u8>,
    value: u64,
    executed: bool,
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
    ExecutionFailed(ExternalCallError),
}

#[state_keys]
pub enum StateKeys {
    LastProposalId,
    Proposals(u64),
    Voter(u64, usize),
    Yeas(u64),
    Nays(u64),
}

#[public]
pub fn propose(
    context: Context<StateKeys>,
    voters_addr: Vec<Address>,
    to: Program,
    method: String,
    args: Vec<u8>,
    value: u64,
) -> u64 {
    let voters_len = voters_addr.len().try_into().expect("too much voters");
    assert!(voters_len >= MIN_VOTES);
    assert!(_is_not_duplicate(&voters_addr));
    let mut voters: Vec<_> = voters_addr
        .iter()
        .map(|address| Voter {
            address: *address,
            voted: false,
        })
        .collect();
    let i = _ensure_voter(&context.actor(), &voters).unwrap();
    voters[i as usize].voted = true;

    let last_id = _last_proposal_id(&context);

    context
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
        context
            .store_by_key(StateKeys::Voter(last_id, i), &voter)
            .unwrap();
    }

    context
        .store([
            (StateKeys::Nays(last_id), &0u64),
            (StateKeys::Yeas(last_id), &1),
        ])
        .unwrap();

    context
        .store_by_key(StateKeys::LastProposalId, &(last_id + 1))
        .unwrap();

    last_id
}

#[public]
pub fn vote(context: Context<StateKeys>, id: u64, yea: bool) -> Result<(), ProposalError> {
    let proposal = _proposal_at(&context, id).ok_or(ProposalError::InexistentProposal)?;

    if proposal.executed {
        return Err(ProposalError::AlreadyExecuted);
    }

    let actor = context.actor();
    let voters: Vec<_> = (0..proposal.voters_len)
        .map(|i| {
            context
                .get(StateKeys::Voter(id, i as usize))
                .unwrap()
                .unwrap()
        })
        .collect();

    let i = _ensure_voter(&actor, &voters)?;

    if yea {
        context
            .store_by_key(
                StateKeys::Yeas(id),
                &(context.get::<u64>(StateKeys::Yeas(id)).unwrap().unwrap() + 1),
            )
            .unwrap();
    } else {
        context
            .store_by_key(
                StateKeys::Nays(id),
                &(context.get::<u64>(StateKeys::Nays(id)).unwrap().unwrap() + 1),
            )
            .unwrap();
    }

    context
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
    proposal_id: u64,
    max_units: u64,
) -> Result<DeferDeserialize, ProposalError> {
    let mut proposal =
        _proposal_at(&context, proposal_id).ok_or(ProposalError::InexistentProposal)?;

    if proposal.executed {
        return Err(ProposalError::AlreadyExecuted);
    }

    let yeas = context.get(StateKeys::Yeas(proposal_id)).unwrap().unwrap();
    let nays = context.get(StateKeys::Nays(proposal_id)).unwrap().unwrap();
    if !_quorum_reached(proposal.voters_len, yeas, nays) {
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
pub fn last_proposal_id(context: Context<StateKeys>) -> u64 {
    _last_proposal_id(&context)
}

#[public]
pub fn quorum_reached(
    context: Context<StateKeys>,
    proposal_id: u64,
) -> Result<bool, ProposalError> {
    let proposal = _proposal_at(&context, proposal_id).ok_or(ProposalError::InexistentProposal)?;
    let yeas = context.get(StateKeys::Yeas(proposal_id)).unwrap().unwrap();
    let nays = context.get(StateKeys::Nays(proposal_id)).unwrap().unwrap();

    Ok(_quorum_reached(proposal.voters_len, yeas, nays))
}

fn _proposal_at(context: &Context<StateKeys>, proposal_id: u64) -> Option<Proposal> {
    context
        .get(StateKeys::Proposals(proposal_id))
        .expect("state corrupt")
}

fn _last_proposal_id(context: &Context<StateKeys>) -> u64 {
    context
        .get(StateKeys::LastProposalId)
        .expect("state corrupt")
        .unwrap_or_default()
}

fn _quorum_reached(max_votes: u64, yeas: u64, nays: u64) -> bool {
    yeas + nays >= MIN_VOTES && yeas > nays && yeas >= (max_votes + 1) / 2
}

fn _is_not_duplicate(voters: &[Address]) -> bool {
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
fn _ensure_voter(actor: &Address, voters: &[Voter]) -> Result<u64, ProposalError> {
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

    Ok(i as u64)
}

#[cfg(test)]
mod tests {
    use super::ProposalError;
    use simulator::{Endpoint, Param, Step, StepResponseError, TestContext};
    use wasmlanche_sdk::{Address, DeferDeserialize};

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
                    String::from("last_proposal_id").into(),
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
            .response::<Result<DeferDeserialize, ProposalError>>()
            .unwrap()
            .unwrap();

        let id: u64 = res.deserialize().unwrap();

        assert_eq!(id, 1);
    }
}
