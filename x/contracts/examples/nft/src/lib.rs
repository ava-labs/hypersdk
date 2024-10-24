// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, state_schema, Address, Context};

pub type Units = u64;
pub type TokenId = usize;

// code comments from https://github.com/OpenZeppelin/openzeppelin-contracts/blob/28aed34dc5e025e61ea0390c18cac875bfde1a78/contracts/token/ERC721/ERC721.sol
// for comparison to EVM implementation
// not all functions are provided here

state_schema! {
    // string private _name;
    Name => String,
    // string private _symbol;
    Symbol => String,
    // mapping(uint256 tokenId => address) private _owners;
    Owner(TokenId) => Address,
    // mapping(address owner => uint256) private _balances;
    Balance(Address) => Units,
    // mapping(uint256 tokenId => address) private _tokenApprovals;
    Approval(TokenId) => Address,
    // mapping(address owner => mapping(address operator => bool)) private _operatorApprovals;
    OperatorApproval(Address, Address) => bool,
}

// /**
//  * @dev Initializes the contract by setting a `name` and a `symbol` to the token collection.
//  */
// constructor(string memory name_, string memory symbol_) {
//     _name = name_;
//     _symbol = symbol_;
// }
#[public]
pub fn init(ctx: &mut Context, name: String, symbol: String) {
    match (ctx.get(Name), ctx.get(Symbol)) {
        (Ok(None), Ok(None)) => {}
        _ => panic!("name and symbol already initialized"),
    }

    ctx.store(((Name, name), (Symbol, symbol)))
        .expect("failed to serialize name and symbol");
}

// /**
//  * @dev See {IERC721Metadata-name}.
//  */
// function name() public view virtual returns (string memory) {
//     return _name;
// }
#[public]
pub fn name(ctx: &mut Context) -> String {
    ctx.get(Name)
        .expect("failed to deserialize")
        .expect("name not set")
}

// /**
//  * @dev See {IERC721Metadata-symbol}.
//  */
// function symbol() public view virtual returns (string memory) {
//     return _symbol;
// }
#[public]
pub fn symbol(ctx: &mut Context) -> String {
    ctx.get(Symbol)
        .expect("failed to deserialize")
        .expect("symbol not set")
}

// /**
//  * @dev See {IERC721-balanceOf}.
//  */
// function balanceOf(address owner) public view virtual returns (uint256) {
//     if (owner == address(0)) {
//         revert ERC721InvalidOwner(address(0));
//     }
//     return _balances[owner];
// }
#[public]
pub fn balance_of(ctx: &mut Context, owner: Address) -> Units {
    if owner == Address::ZERO {
        panic!("invalid owner");
    }

    ctx.get(Balance(owner))
        .expect("failed to deserialize")
        .unwrap_or_default()
}

// /**
//  * @dev See {IERC721-ownerOf}.
//  */
// function ownerOf(uint256 tokenId) public view virtual returns (address) {
//     return _requireOwned(tokenId);
// }
#[public]
pub fn owner_of(ctx: &mut Context, token_id: TokenId) -> Address {
    ctx.get(Owner(token_id))
        .expect("failed to deserialize")
        .unwrap_or_default()
}

// /**
//  * @dev See {IERC721-approve}.
//  */
// function approve(address to, uint256 tokenId) public virtual {
//     _approve(to, tokenId, _msgSender());
// }
#[public]
pub fn approve(ctx: &mut Context, to: Address, token_id: TokenId) {
    let actor = ctx.actor();
    let owner = owner_of(ctx, token_id);

    if owner != actor && !is_approved_for_all(ctx, owner, actor) {
        panic!("not the owner");
    }

    ctx.store_by_key(Approval(token_id), to)
        .expect("failed to serialize")
}

// /**
//  * @dev See {IERC721-setApprovalForAll}.
//  */
// function setApprovalForAll(address operator, bool approved) public virtual {
//     _setApprovalForAll(_msgSender(), operator, approved);
// }
#[public]
pub fn set_approval_for_all(ctx: &mut Context, operator: Address, approved: bool) {
    let actor = ctx.actor();

    ctx.store_by_key(OperatorApproval(actor, operator), approved)
        .expect("failed to serialize approval");
}

// /**
//  * @dev See {IERC721-isApprovedForAll}.
//  */
// function isApprovedForAll(address owner, address operator) public view virtual returns (bool) {
//     return _operatorApprovals[owner][operator];
// }
#[public]
pub fn is_approved_for_all(ctx: &mut Context, owner: Address, operator: Address) -> bool {
    ctx.get(OperatorApproval(owner, operator))
        .expect("failed to deserialize")
        .unwrap_or_default()
}

#[cfg(not(feature = "bindings"))]
fn is_approved(ctx: &mut Context, operator: Address, token_id: TokenId) -> bool {
    let owner = owner_of(ctx, token_id);

    operator == owner
        || is_approved_for_token(ctx, operator, token_id)
        || is_approved_for_all(ctx, owner, operator)
}

#[cfg(not(feature = "bindings"))]
fn is_approved_for_token(ctx: &mut Context, actor: Address, token_id: TokenId) -> bool {
    ctx.get(Approval(token_id))
        .expect("failed to deserialize")
        .map(|approved| actor == approved)
        .unwrap_or_default()
}

// /**
//  * @dev See {IERC721-transferFrom}.
//  */
// function transferFrom(address from, address to, uint256 tokenId) public virtual {
//     if (to == address(0)) {
//         revert ERC721InvalidReceiver(address(0));
//     }
//     // Setting an "auth" arguments enables the `_isAuthorized` check which verifies that the token exists
//     // (from != 0). Therefore, it is not needed to verify that the return value is not 0 here.
//     address previousOwner = _update(to, tokenId, _msgSender());
//     if (previousOwner != from) {
//         revert ERC721IncorrectOwner(from, tokenId, previousOwner);
//     }
// }
#[public]
pub fn transfer_from(ctx: &mut Context, from: Address, to: Address, token_id: TokenId) {
    let operator = ctx.actor();
    let owner = owner_of(ctx, token_id);

    if from != owner {
        panic!("not the owner");
    }

    if !is_approved(ctx, operator, token_id) {
        panic!("not approved");
    }

    let from_balance = balance_of(ctx, from);

    ctx.store((
        (Owner(token_id), to),
        (Approval(token_id), Address::ZERO),
        (Balance(from), from_balance - 1),
    ))
    .expect("failed to serialize");

    if to != Address::ZERO {
        let to_balance = balance_of(ctx, to);
        ctx.store_by_key(Balance(to), to_balance + 1)
            .expect("failed to serialize");
    }
}

// /**
//  * @dev Mints `tokenId` and transfers it to `to`.
//  *
//  * WARNING: Usage of this method is discouraged, use {_safeMint} whenever possible
//  *
//  * Requirements:
//  *
//  * - `tokenId` must not exist.
//  * - `to` cannot be the zero address.
//  *
//  * Emits a {Transfer} event.
//  */
// function _mint(address to, uint256 tokenId) internal {
//     if (to == address(0)) {
//         revert ERC721InvalidReceiver(address(0));
//     }
//     address previousOwner = _update(to, tokenId, address(0));
//     if (previousOwner != address(0)) {
//         revert ERC721InvalidSender(address(0));
//     }
// }
#[public]
pub fn mint(ctx: &mut Context, to: Address, token_id: TokenId) {
    let owner = owner_of(ctx, token_id);

    if owner != Address::ZERO {
        panic!("token already exists");
    }

    if to == Address::ZERO {
        panic!("invalid receiver");
    }

    let to_balance = balance_of(ctx, to);

    let new_owner_pair = (Owner(token_id), to);
    let new_owner_balance_pair = (Balance(to), to_balance + 1);

    ctx.store((new_owner_pair, new_owner_balance_pair))
        .expect("failed to serialize")
}

// /**
//  * @dev Destroys `tokenId`.
//  * The approval is cleared when the token is burned.
//  * This is an internal function that does not check if the sender is authorized to operate on the token.
//  *
//  * Requirements:
//  *
//  * - `tokenId` must exist.
//  *
//  * Emits a {Transfer} event.
//  */
// function _burn(uint256 tokenId) internal {
//     address previousOwner = _update(address(0), tokenId, address(0));
//     if (previousOwner == address(0)) {
//         revert ERC721NonexistentToken(tokenId);
//     }
// }
#[public]
pub fn burn(ctx: &mut Context, token_id: TokenId) {
    let owner = owner_of(ctx, token_id);

    if owner == Address::ZERO {
        panic!("token does not exist");
    }

    transfer_from(ctx, owner, Address::ZERO, token_id);
}

#[cfg(all(test, not(feature = "bindings")))]
mod tests {
    use super::*;

    #[test]
    fn balance_is_zero_when_no_owner() {
        let bob = Address::new([1; Address::LEN]);
        let mut ctx = Context::with_actor(bob);

        let balance = balance_of(&mut ctx, bob);
        assert_eq!(balance, 0);
    }

    #[test]
    fn mint_sets_owner() {
        let alice = Address::new([1; Address::LEN]);
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        let owner = owner_of(&mut ctx, token_id);
        assert_eq!(owner, alice);
    }

    #[test]
    fn mint_increases_balance() {
        let alice = Address::new([1; Address::LEN]);
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        let balance = balance_of(&mut ctx, alice);
        assert_eq!(balance, 1);

        mint(&mut ctx, alice, token_id + 1);
        let balance = balance_of(&mut ctx, alice);
        assert_eq!(balance, 2);
    }

    #[test]
    fn mint_does_not_set_approval() {
        let alice = Address::new([1; Address::LEN]);
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        let approval = ctx.get(Approval(token_id)).expect("failed to deserialize");
        assert_eq!(approval, None);
    }

    // burn
    #[test]
    fn burn_decreases_balance() {
        let alice = Address::new([1; Address::LEN]);
        let mut ctx = Context::with_actor(alice);
        let tokens = [0, 1];

        tokens.into_iter().for_each(|token_id| {
            ctx.store_by_key(Owner(token_id), alice)
                .expect("failed to serialize")
        });

        ctx.store_by_key(Balance(alice), tokens.len() as u64)
            .expect("failed to serialize");

        let balance = balance_of(&mut ctx, alice);
        assert_eq!(balance, tokens.len() as u64);

        tokens
            .into_iter()
            .enumerate()
            .map(|(i, id)| (i + 1, id))
            .map(|(burn_count, id)| (tokens.len() - burn_count, id))
            .for_each(|(expected_balance, token_id)| {
                burn(&mut ctx, token_id);
                let balance = balance_of(&mut ctx, alice);
                assert_eq!(balance, expected_balance as u64);
            });
    }

    #[test]
    fn burn_results_in_owner_of_zero_address() {
        let alice = Address::new([1; Address::LEN]);
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        burn(&mut ctx, token_id);

        let owner = owner_of(&mut ctx, token_id);
        assert_eq!(owner, Address::ZERO);
    }

    #[test]
    #[should_panic = "not the owner"]
    fn burn_results_in_old_owner_not_being_able_to_approve() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);
        approve(&mut ctx, bob, token_id);

        burn(&mut ctx, token_id);

        approve(&mut ctx, bob, token_id);
    }

    #[test]
    #[should_panic = "not the owner"]
    fn burn_results_in_old_owner_not_being_able_to_tranfer() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);
        transfer_from(&mut ctx, alice, bob, token_id);

        ctx.set_actor(bob);

        burn(&mut ctx, token_id);

        transfer_from(&mut ctx, bob, alice, token_id);
    }

    #[test]
    #[should_panic = "not approved"]
    fn burn_can_only_be_done_by_owner() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        ctx.set_actor(bob);
        burn(&mut ctx, token_id);
    }

    #[test]
    fn burn_can_be_done_by_token_approved() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);
        approve(&mut ctx, bob, token_id);

        ctx.set_actor(bob);
        burn(&mut ctx, token_id);

        let new_owner = owner_of(&mut ctx, token_id);
        assert_eq!(new_owner, Address::ZERO);
    }

    #[test]
    fn burn_can_be_done_by_approved_all() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);
        set_approval_for_all(&mut ctx, bob, true);

        ctx.set_actor(bob);
        burn(&mut ctx, token_id);

        let new_owner = owner_of(&mut ctx, token_id);
        assert_eq!(new_owner, Address::ZERO);
    }

    #[test]
    fn transfer_increases_receiver_balance_by_one() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        let bob_balance = balance_of(&mut ctx, bob);
        assert_eq!(bob_balance, 0);

        transfer_from(&mut ctx, alice, bob, token_id);

        let bob_balance = balance_of(&mut ctx, bob);
        assert_eq!(bob_balance, 1);
    }

    #[test]
    fn transfer_decreases_sender_balance_by_one() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        let alice_balance = balance_of(&mut ctx, alice);
        assert_eq!(alice_balance, 1);

        transfer_from(&mut ctx, alice, bob, token_id);

        let alice_balance = balance_of(&mut ctx, alice);
        assert_eq!(alice_balance, 0);
    }

    #[test]
    fn transfer_sets_new_owner() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        let owner = owner_of(&mut ctx, token_id);
        assert_eq!(owner, alice);

        transfer_from(&mut ctx, alice, bob, token_id);

        let owner = owner_of(&mut ctx, token_id);
        assert_eq!(owner, bob);
    }

    #[test]
    fn transfer_clears_approval() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);
        approve(&mut ctx, bob, token_id);

        transfer_from(&mut ctx, alice, bob, token_id);

        let approval = ctx.get(Approval(token_id)).expect("failed to deserialize");
        assert_eq!(approval, Some(Address::ZERO));
    }

    #[test]
    #[should_panic = "not the owner"]
    fn transfer_prevents_old_owner_from_approving() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);
        transfer_from(&mut ctx, alice, bob, token_id);

        approve(&mut ctx, bob, token_id);
    }

    #[test]
    #[should_panic = "not approved"]
    fn transfer_prevents_old_owner_from_transfering() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);
        transfer_from(&mut ctx, alice, bob, token_id);

        transfer_from(&mut ctx, bob, alice, token_id);
    }

    #[test]
    fn transfer_is_allowed_by_approved_actor() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);
        approve(&mut ctx, bob, token_id);

        ctx.set_actor(bob);
        transfer_from(&mut ctx, alice, bob, token_id);

        let owner = owner_of(&mut ctx, token_id);
        assert_eq!(owner, bob);
    }

    #[test]
    #[should_panic = "not approved"]
    fn transfer_is_not_allowed_by_non_approved_actor() {
        let [alice, bob, charlie] = [1, 2, 3].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);
        approve(&mut ctx, bob, token_id);

        ctx.set_actor(charlie);
        transfer_from(&mut ctx, alice, bob, token_id);
    }

    #[test]
    fn approve_can_be_set_by_owner() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        approve(&mut ctx, bob, token_id);

        let approval = ctx.get(Approval(token_id)).expect("failed to deserialize");
        assert_eq!(approval, Some(bob));
    }

    #[test]
    #[should_panic = "not the owner"]
    fn approve_cannot_be_changed_by_approved() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        approve(&mut ctx, bob, token_id);

        ctx.set_actor(bob);
        approve(&mut ctx, alice, token_id);
    }

    #[test]
    #[should_panic = "not the owner"]
    fn approve_cannot_be_changed_by_random() {
        let [alice, bob, carol] = [1, 2, 3].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);

        approve(&mut ctx, bob, token_id);

        ctx.set_actor(carol);

        approve(&mut ctx, alice, token_id);
    }

    #[test]
    fn approve_all_can_be_set_by_actor() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);

        let approval_result = ctx
            .get(OperatorApproval(alice, bob))
            .expect("failed to deserialize");
        assert_eq!(approval_result, None);

        let approval = true;
        set_approval_for_all(&mut ctx, bob, approval);

        let approval_result = ctx
            .get(OperatorApproval(alice, bob))
            .expect("failed to deserialize");
        assert_eq!(approval_result, Some(approval));

        let approval = false;
        set_approval_for_all(&mut ctx, bob, approval);

        let approval_result = ctx
            .get(OperatorApproval(alice, bob))
            .expect("failed to deserialize");
        assert_eq!(approval_result, Some(approval));
    }

    #[test]
    fn approve_all_can_transfer_one() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let mut ctx = Context::with_actor(alice);
        let token_id = 0;

        mint(&mut ctx, alice, token_id);
        let owner = owner_of(&mut ctx, token_id);
        assert_eq!(owner, alice);

        set_approval_for_all(&mut ctx, bob, true);

        ctx.set_actor(bob);
        transfer_from(&mut ctx, alice, bob, token_id);

        let owner = owner_of(&mut ctx, token_id);
        assert_eq!(owner, bob);
    }

    #[test]
    fn approve_all_can_transfer_many() {
        let [alice, bob] = [1, 2].map(|i| Address::new([i; Address::LEN]));
        let tokens = [0, 1];
        let mut ctx = Context::with_actor(alice);

        tokens.into_iter().for_each(|token_id| {
            mint(&mut ctx, alice, token_id);
        });

        set_approval_for_all(&mut ctx, bob, true);

        ctx.set_actor(bob);

        tokens.into_iter().for_each(|token_id| {
            transfer_from(&mut ctx, alice, bob, token_id);
        });

        tokens.into_iter().for_each(|token_id| {
            let owner = owner_of(&mut ctx, token_id);
            assert_eq!(owner, bob);
        });
    }
}
