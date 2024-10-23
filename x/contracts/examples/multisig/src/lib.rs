// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::{collections::HashSet, num::NonZeroU32};

use wasmlanche::{
    borsh::{BorshDeserialize, BorshSerialize},
    public, state_schema, Address, Context,
};

pub type ProposalId = usize;

state_schema! {
    /// Number of total proposals, doubles as the next `ProposalId`
    ProposalCount => ProposalId,
    /// The proposal itself
    ProposalItem(ProposalId) => Proposal,
    /// Voting information include who can vote and how many yeas left until quorum is reached
    VoteInfo(ProposalId) => ProposalMeta,
    /// Votes cast by a voter
    /// Any address can vote for any proposal, but that doesn't mean that the proposal will count toward quorum
    Vote(ProposalId, Address) => bool,
}

/// Create a proposal
#[public]
pub fn propose(
    ctx: &mut Context,
    proposal: Proposal,
    voters: Voters,
    quorum: NonZeroU32,
) -> ProposalId {
    let actor = ctx.actor();
    let mut quorum = quorum.get();

    if voters.contains(actor) {
        quorum -= 1;
    }

    let quorum =
        NonZeroU32::new(quorum).expect("cannot create a proposal that immediately executes");

    let meta = voters
        .with_quorum(quorum)
        .expect("quorum larger than eligible voters");

    let id = ctx
        .get(ProposalCount)
        .expect("failed to deserialize")
        .unwrap_or_default();

    ctx.store((
        (ProposalCount, id + 1),
        (ProposalItem(id), proposal),
        (VoteInfo(id), meta),
        (Vote(id, actor), true),
    ))
    .expect("failed to store proposal");

    id
}

/// Vote on a proposal
#[public]
pub fn vote(ctx: &mut Context, proposal_id: ProposalId, vote: bool) -> Option<Vec<u8>> {
    let actor = ctx.actor();

    let previous_vote = ctx
        .get(Vote(proposal_id, actor))
        .expect("failed to deserialize");

    if previous_vote.is_some() {
        return None;
    }

    ctx.store_by_key(Vote(proposal_id, actor), vote)
        .expect("failed to serialize");

    if !vote {
        return None;
    }

    let ProposalMeta {
        voters,
        votes_remaining,
    } = ctx
        .get(VoteInfo(proposal_id))
        .expect("failed to deserialize")
        .expect("proposal not found");

    if !voters.contains(actor) || votes_remaining == 0 {
        return None;
    }

    let votes_remaining = votes_remaining - 1;

    ctx.store_by_key(
        VoteInfo(proposal_id),
        ProposalMeta {
            voters,
            votes_remaining,
        },
    )
    .expect("failed to store votes remaining");

    match votes_remaining {
        0 => {
            let Proposal {
                contract_address,
                function_name,
                args,
            } = ctx
                .get(ProposalItem(proposal_id))
                .expect("failed to deserialize")
                .expect("proposal not found");
            // TODO: fix this
            // let remaining_fuel = ctx.remaining_fuel();
            let remaining_fuel = 1_000_000;

            let result: DeferDeserialization = ctx
                .call_contract(contract_address, &function_name, &args, remaining_fuel, 0)
                .expect("failed to call contract");

            Some(result.into())
        }

        _ => None,
    }
}

/// A proposal consists of an address of a contract to call
/// a function name to call on that contract
/// and a set of serialized arguments to pass to that function
#[derive(BorshSerialize, BorshDeserialize)]
// TODO: re-derive these traits to automatically inject the crate
#[borsh(crate = "wasmlanche::borsh")]
pub struct Proposal {
    pub contract_address: Address,
    pub function_name: String,
    pub args: Vec<u8>,
}

/// Contains a set of `Voters` as well as `votes_remaining` (until quorum is reached)
#[derive(BorshSerialize, BorshDeserialize)]
#[borsh(crate = "wasmlanche::borsh")]
struct ProposalMeta {
    voters: Voters,
    votes_remaining: u32,
    // TODO: add expiry
}

/// Either a specific set of addresses OR an open vote in which the quorum voter will actually have to pay
/// for the final execution
// TODO:
// reward the final voter with to refund their fuel contributions
#[cfg_attr(test, derive(Debug))]
pub enum Voters {
    Any,
    Addresses(HashSet<Address>),
}

impl Voters {
    fn with_quorum(self, quorum: NonZeroU32) -> Option<ProposalMeta> {
        let votes_remaining = quorum.get();

        if votes_remaining <= self.len() as u32 {
            Some(ProposalMeta {
                voters: self,
                votes_remaining,
            })
        } else {
            None
        }
    }

    fn len(&self) -> usize {
        match self {
            Voters::Any => usize::MAX,
            Voters::Addresses(addresses) => addresses.len(),
        }
    }

    fn contains(&self, address: Address) -> bool {
        match self {
            Voters::Any => true,
            Voters::Addresses(addresses) => addresses.contains(&address),
        }
    }
}

impl BorshDeserialize for Voters {
    fn deserialize_reader<R: std::io::Read>(reader: &mut R) -> std::io::Result<Self> {
        let addresses: HashSet<Address> = BorshDeserialize::deserialize_reader(reader)?;

        if addresses.is_empty() {
            Ok(Voters::Any)
        } else {
            Ok(Voters::Addresses(addresses))
        }
    }
}

impl BorshSerialize for Voters {
    fn serialize<W: std::io::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        const EMPTY: &[Address] = &[];

        match self {
            Voters::Any => BorshSerialize::serialize(EMPTY, writer),
            Voters::Addresses(addresses) => BorshSerialize::serialize(addresses, writer),
        }
    }
}

use de::*;

mod de {
    use super::*;

    pub struct DeferDeserialization(Vec<u8>);

    impl From<DeferDeserialization> for Vec<u8> {
        fn from(val: DeferDeserialization) -> Self {
            val.0
        }
    }

    impl BorshDeserialize for DeferDeserialization {
        fn deserialize_reader<R: std::io::Read>(reader: &mut R) -> std::io::Result<Self> {
            let mut bytes = Vec::new();

            reader.read_to_end(&mut bytes)?;

            Ok(DeferDeserialization(bytes))
        }
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn deserialize_primitive() {
            let bytes = wasmlanche::borsh::to_vec(&42).expect("failed to serialize");
            let defer: DeferDeserialization =
                wasmlanche::borsh::from_slice(&bytes).expect("failed to deserialize");

            assert_eq!(defer.0, bytes);
        }

        #[test]
        fn deserialize_collection() {
            let bytes = wasmlanche::borsh::to_vec(&vec![42]).expect("failed to serialize");
            let defer: DeferDeserialization =
                wasmlanche::borsh::from_slice(&bytes).expect("failed to deserialize");

            assert_eq!(defer.0, bytes);
        }

        #[test]
        fn deserialize_array() {
            let bytes = wasmlanche::borsh::to_vec(&[42]).expect("failed to serialize");
            let defer: DeferDeserialization =
                wasmlanche::borsh::from_slice(&bytes).expect("failed to deserialize");

            assert_eq!(defer.0, bytes);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    #[should_panic = "cannot create a proposal that immediately executes"]
    fn cannot_create_quorum_of_one_when_in_voter_set() {
        let [bob, contract_address] = [1, 2].map(|i| Address::new([i; 33]));
        let proposal = Proposal {
            contract_address,
            function_name: Default::default(),
            args: Default::default(),
        };

        let voters = Voters::Addresses(HashSet::from([bob]));
        let quorum = NonZeroU32::new(1).unwrap();
        let mut ctx = Context::with_actor(bob);

        propose(&mut ctx, proposal, voters, quorum);
    }

    #[test]
    fn can_create_quorum_of_one_when_not_in_voter_set() {
        let [bob, alice, contract_address] = [1, 2, 3].map(|i| Address::new([i; 33]));
        let voters = Voters::Addresses(HashSet::from([alice]));
        let quorum = NonZeroU32::new(1).unwrap();

        let proposal = Proposal {
            contract_address,
            function_name: Default::default(),
            args: Default::default(),
        };

        let mut ctx = Context::with_actor(bob);

        propose(&mut ctx, proposal, voters, quorum);
    }

    #[test]
    fn cant_create_quorum_greater_than_number_of_voters() {
        let voters = [1, 2].map(|i| Address::new([i; 33]));
        let quorum = NonZeroU32::new(voters.len() as u32 + 1).unwrap();
        let voters = Voters::Addresses(HashSet::from(voters));

        let meta = voters.with_quorum(quorum);

        assert!(meta.is_none());
    }

    #[test]
    fn propose_initializes_vote() {
        let [bob, alice, contract_address] = [1, 2, 3].map(|i| Address::new([i; 33]));
        let voters = Voters::Addresses(HashSet::from([bob, alice]));
        let quorum = 2;

        let proposal = Proposal {
            contract_address,
            function_name: "foo".to_string(),
            args: vec![],
        };

        let mut ctx = Context::with_actor(bob);

        let id = propose(&mut ctx, proposal, voters, NonZeroU32::new(quorum).unwrap());

        let ProposalMeta {
            votes_remaining, ..
        } = ctx
            .get(VoteInfo(id))
            .expect("failed to deserialize")
            .expect("proposal not found");

        assert_eq!(votes_remaining, quorum - 1);
    }

    #[test]
    fn cannot_vote_twice() {
        let [bob, contract_address] = [1, 2].map(|i| Address::new([i; 33]));
        let voters = Voters::Any;
        let quorum = NonZeroU32::new(2).unwrap();

        let proposal = Proposal {
            contract_address,
            function_name: "foo".to_string(),
            args: vec![],
        };

        let ctx = &mut Context::with_actor(bob);

        let id = propose(ctx, proposal, voters, quorum);

        let result = vote(ctx, id, true);

        assert!(result.is_none());
    }

    #[test]
    fn reach_2_of_3_quorum() {
        let [bob, alice, charlie, contract_address] = [1, 2, 3, 4].map(|i| Address::new([i; 33]));
        let voters = Voters::Addresses(HashSet::from([bob, alice, charlie]));
        let quorum = NonZeroU32::new(2).unwrap();
        let function_name = "foo";
        let args = &[];

        let proposal = Proposal {
            contract_address,
            function_name: function_name.to_string(),
            args: args.to_vec(),
        };

        let mut ctx = Context::with_actor(bob);

        let id = propose(&mut ctx, proposal, voters, quorum);

        let result = "hello world";
        let expected_result = wasmlanche::borsh::to_vec(result).unwrap();

        ctx.mock_function_call(contract_address, function_name, args, 0, result);

        ctx.set_actor(alice);

        let result = vote(&mut ctx, id, true);

        assert_eq!(result, Some(expected_result));
    }

    #[test]
    fn reach_3_of_5_quorum() {
        let voters = [1, 2, 3, 4, 5].map(|i| Address::new([i; 33]));
        let [bob, alice, charlie, _, _] = voters;
        let voters = Voters::Addresses(HashSet::from(voters));
        let quorum = NonZeroU32::new(3).unwrap();
        let contract_address = Address::new([6; 33]);
        let function_name = "foo";
        let args = &[];

        let proposal = Proposal {
            contract_address,
            function_name: function_name.to_string(),
            args: args.to_vec(),
        };

        let mut ctx = Context::with_actor(bob);

        let id = propose(&mut ctx, proposal, voters, quorum);
        let result = "hello world";
        let expected_result = wasmlanche::borsh::to_vec(result).unwrap();

        ctx.mock_function_call(contract_address, function_name, args, 0, result);

        ctx.set_actor(alice);

        let _ = vote(&mut ctx, id, true);

        ctx.set_actor(charlie);

        let result = vote(&mut ctx, id, true);

        assert_eq!(result, Some(expected_result));
    }

    #[test]
    fn unregistered_voter_doesnt_count() {
        let [bob, alice, charlie] = [1, 2, 3].map(|i| Address::new([i; 33]));
        let voters = Voters::Addresses(HashSet::from([bob, alice]));
        let quorum = NonZeroU32::new(2).unwrap();

        let contract_address = Address::new([4; 33]);
        let function_name = "foo";
        let args = &[];

        let proposal = Proposal {
            contract_address,
            function_name: function_name.to_string(),
            args: args.to_vec(),
        };

        let mut ctx = Context::with_actor(bob);

        let id = propose(&mut ctx, proposal, voters, quorum);
        ctx.set_actor(charlie);

        let result = vote(&mut ctx, id, true);

        assert!(result.is_none());
    }

    #[test]
    fn any_voter_executes() {
        let [bob, alice] = [1, 2].map(|i| Address::new([i; 33]));
        let voters = Voters::Any;
        let quorum = NonZeroU32::new(2).unwrap();

        let contract_address = Address::new([3; 33]);
        let function_name = "foo";
        let args = &[];

        let proposal = Proposal {
            contract_address,
            function_name: function_name.to_string(),
            args: args.to_vec(),
        };

        let mut ctx = Context::with_actor(bob);

        let id = propose(&mut ctx, proposal, voters, quorum);

        let result = "hello world";
        let expected_result = wasmlanche::borsh::to_vec(result).unwrap();

        ctx.mock_function_call(contract_address, function_name, args, 0, result);

        ctx.set_actor(alice);

        let result = vote(&mut ctx, id, true);

        assert_eq!(result, Some(expected_result));
    }
}
