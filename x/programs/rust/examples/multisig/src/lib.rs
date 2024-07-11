use borsh::{BorshDeserialize, BorshSerialize};
use std::collections::HashSet;
#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::ExternalCallError;
use wasmlanche_sdk::{public, state_keys, types::Address, Program};

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
    let Context { program, actor, .. } = context;
    let last_id = proposal_id(&program);
    program
        .state()
        .store(
            StateKeys::Proposals(last_id),
            &Proposal {
                to,
                method,
                data,
                yea: 1,
                nay: 0,
                executed: false,
                voters: HashSet::from([actor]),
            },
        )
        .expect("state corrupt");
    last_id
}

#[public]
pub fn vote(context: Context<StateKeys>, id: u32, yea: bool) -> Result<(), ProposalError> {
    let Context { program, actor, .. } = context;
    let mut proposal = proposal_at(&program, id).ok_or(ProposalError::InexistentProposal)?;

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
        .store(StateKeys::Proposals(id), &proposal)
        .expect("state corrupt");

    Ok(())
}

#[public]
pub fn execute(
    context: Context<StateKeys>,
    proposal_id: u32,
    max_units: u64,
) -> Result<T, ProposalError> {
    let Context { program, .. } = context;
    let mut proposal =
        proposal_at(&program, proposal_id).ok_or(ProposalError::InexistentProposal)?;

    if proposal.executed {
        return Err(ProposalError::AlreadyExecuted);
    }
    if !quorum_reached(&proposal) {
        return Err(ProposalError::QuorumNotReached);
    }

    proposal.executed = true;

    // proposal
    //     .to
    //     .call_function(&proposal.method, &proposal.data, max_units)
    //     .map_err(ProposalError::ExecutionFailed)
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
    // TODO add rules
    true
}

#[cfg(test)]
mod tests {
    use simulator::{Endpoint, Key, Param, Step};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");
}
