use wasmlanche_sdk::Context;
use wasmlanche_sdk::{Gas, public, state_keys, types::Address, Program};

pub struct Token(Program);

trait ExternalToken {
    fn transfer(&self, to: Address, amount: u64, fuel: Gas) -> bool;
    fn balance_of(&self, account: Address, fuel: Gas) -> u64;
    fn allowance(&self, owner: Address, spender: Address, fuel: Gas) -> u64;
    fn approve(&self, spender: Address, amount: u64, fuel: Gas) -> bool;
    fn transfer_from(&self, sender: Address, recipient: Address, amount: u64, fuel: Gas) -> bool;
    fn total_supply(&self, fuel: Gas) -> u64;
}

impl ExternalToken for Token {
    fn allowance(&self, owner: Address, spender: Address, fuel: Gas) -> u64 {
        self.0
            .call_function("allowance", (owner, spender), fuel)
            .expect("call to allowance failed")
    }

    fn transfer(&self, to: Address, amount: u64, fuel: Gas) -> bool {
        self.0
            .call_function("transfer", (to, amount), fuel)
            .expect("call to transfer failed")
    }

    fn balance_of(&self, account: Address, fuel: Gas) -> u64 {
        self.0
            .call_function("balance_of", account, fuel)
            .expect("call to account failed")
    }

    fn approve(&self, spender: Address, amount: u64, fuel: Gas) -> bool {
        self.0
            .call_function("approve", (spender, amount), fuel)
            .expect("call to approve failed")
    }

    fn transfer_from(&self, sender: Address, recipient: Address, amount: u64, fuel: Gas) -> bool {
        self.0
            .call_function("transfer_from", (sender, recipient, amount), fuel)
            .expect("call to transfer_from failed")
    }

    fn total_supply(&self, fuel: Gas) -> u64 {
        self.0
            .call_function("total_supply", (), fuel)
            .expect("call to total_supply failed")
    }
}

/// The program state keys.
#[state_keys]
pub enum StateKey {
    Owner,
    Token1,
    Reserve1,
    Token2,
    Reserve2,

    TotalSupply,
    Name,
    Symbol,
    Balance(Address),
    Allowance(Address, Address),
}

pub fn get_owner(program: &Program<StateKey>) -> Option<Address> {
    program
        .state()
        .get::<Address>(StateKey::Owner)
        .expect("failure")
}

pub fn owner_check(program: &Program<StateKey>, actor: Address) {
    assert!(
        match get_owner(program) {
            None => true,
            Some(owner) => owner == actor,
        },
        "caller is required to be owner"
    )
}

pub fn get_token1(program: &Program<StateKey>) -> Token {
    Token(
        program
            .state()
            .get::<Program>(StateKey::Token1)
            .expect("failed to load token 1")
            .expect("token 1 doesn't exist"),
    )
}

pub fn get_token2(program: &Program<StateKey>) -> Token {
    Token(
        program
            .state()
            .get::<Program>(StateKey::Token2)
            .expect("failed to load token 2")
            .expect("token 2 doesn't exist"),
    )
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn init(context: Context<StateKey>, token1: Program, token2: Program) {
    let Context { program, actor, .. } = context;

    program
        .state()
        .store(StateKey::Owner, &actor)
        .expect("failed to store total supply");

    program
        .state()
        .store(StateKey::Token1, &token1)
        .expect("failed to store token1");

    program
        .state()
        .store(StateKey::Token2, &token2)
        .expect("failed to store token2");

    program
        .state()
        .store(StateKey::Owner, &actor)
        .expect("failed to store total supply");

    // set total supply
    program
        .state()
        .store::<u64>(StateKey::TotalSupply, &0)
        .expect("failed to store total supply");

    // set token name
    program
        .state()
        .store(StateKey::Name, &"gov")
        .expect("failed to store coin name");

    // set token symbol
    program
        .state()
        .store(StateKey::Symbol, &"GOV")
        .expect("failed to store symbol");
}

#[public]
pub fn add_liquidity(context: Context<StateKey>, supplied_token1: u64, supplied_token2: u64) {
    let Context { program, actor } = context;
    let token1 = get_token1(&program);
    let token2 = get_token2(&program);

    let balance_token1 = token1.balance_of(program.id(), 10000000);
    let balance_token2 = token2.balance_of(program.id(), 10000000);

    assert!(
        supplied_token1 * balance_token2 == supplied_token2 * balance_token1,
        "Invalid ratio"
    );

    token1.transfer_from(actor, program, supplied_token1, 20000000);
    token2.transfer_from(actor, program, supplied_token2, 20000000);

    // Mint LP tokens based on the amount of liquidity provided
    let liquidity = calculate_liquidity_amount(supplied_token1, supplied_token2);
    _mint(&program, actor, liquidity);

    // Reward the liquidity provider with governance tokens
    let reserve1 = supplied_token1 + get_reserve1(&program);
    program
        .state()
        .store(StateKey::Reserve1, &reserve1)
        .expect("failed to store reserve 1");

    let reserve2 = supplied_token2 + get_reserve2(&program);
    program
        .state()
        .store(StateKey::Reserve2, &reserve2)
        .expect("failed to store reserve 2");
}

fn get_reserve1(program: &Program<StateKey>) -> u64 {
    program
        .state()
        .get(StateKey::Reserve1)
        .expect("failure")
        .unwrap_or_default()
}

fn get_reserve2(program: &Program<StateKey>) -> u64 {
    program
        .state()
        .get(StateKey::Reserve2)
        .expect("failure")
        .unwrap_or_default()
}

#[public]
pub fn remove_liquidity(context: Context<StateKey>, amount: u64) -> (u64, u64) {
    let Context { program, actor, .. } = context;
    let total_liquidity = _total_supply(&program);
    assert!(amount <= total_liquidity, "Invalid amount");

    let token1 = get_token1(&program);
    let token2 = get_token2(&program);

    let reserve1 = get_reserve1(&program);
    let reserve2 = get_reserve2(&program);
    let amount_token1 = amount * reserve1 / total_liquidity;
    let amount_token2 = amount * reserve2 / total_liquidity;
    _burn(&program, actor, amount);

    token1.transfer(actor, amount_token1, 2000000);
    token2.transfer(actor, amount_token2, 2000000);

    let reserve1 = reserve1 - amount_token1;
    program
        .state()
        .store(StateKey::Reserve1, &reserve1)
        .expect("failed to store reserve1");
    let reserve2 = reserve2 - amount_token2;
    program
        .state()
        .store(StateKey::Reserve2, &reserve2)
        .expect("failed to store reserve2");

    (amount_token1, amount_token2)
}

fn calculate_liquidity_amount(amount1: u64, amount2: u64) -> u64 {
    amount1 + amount2
}

#[public]
pub fn total_supply(context: Context<StateKey>) -> u64 {
    let Context { program, .. } = context;
    _total_supply(&program)
}

fn _total_supply(program: &Program<StateKey>) -> u64 {
    program
        .state()
        .get(StateKey::TotalSupply)
        .expect("failure")
        .unwrap_or_default()
}

/// Transfers balance from the token owner to the recipient.
#[public]
pub fn mint(context: Context<StateKey>, recipient: Address, amount: u64) -> bool {
    let Context { program, actor, .. } = context;
    owner_check(&program, actor);
    _mint(&program, recipient, amount)
}

fn _mint(program: &Program<StateKey>, recipient: Address, amount: u64) -> bool {
    let balance = program
        .state()
        .get::<u64>(StateKey::Balance(recipient))
        .expect("failed to get balance")
        .unwrap_or_default();

    program
        .state()
        .store(StateKey::Balance(recipient), &(balance + amount))
        .expect("failed to store balance");

    let total = _total_supply(program);
    program
        .state()
        .store(StateKey::TotalSupply, &(total + amount))
        .expect("failed to store total supply");

    true
}

/// Burn the token from the recipient.
#[public]
pub fn burn(context: Context<StateKey>, recipient: Address, amount: u64) -> u64 {
    let Context { program, actor, .. } = context;
    owner_check(&program, actor);
    _burn(&program, recipient, amount)
}

fn _burn(program: &Program<StateKey>, recipient: Address, amount: u64) -> u64 {
    let user_total = _balance_of(program, recipient);
    assert!(
        amount <= user_total,
        "address doesn't have enough tokens to burn"
    );
    let new_amount = user_total - amount;
    program
        .state()
        .store::<u64>(StateKey::Balance(recipient), &new_amount)
        .expect("failed to burn recipient tokens");

    let total = _total_supply(program);
    program
        .state()
        .store::<u64>(StateKey::TotalSupply, &(total - amount))
        .expect("failed to reduce tokens total");

    new_amount
}

/// Gets the balance of the recipient.
#[public]
pub fn balance_of(context: Context<StateKey>, account: Address) -> u64 {
    let Context { program, .. } = context;
    _balance_of(&program, account)
}

fn _balance_of(program: &Program<StateKey>, account: Address) -> u64 {
    program
        .state()
        .get(StateKey::Balance(account))
        .expect("failure")
        .unwrap_or_default()
}

#[public]
pub fn allowance(context: Context<StateKey>, owner: Address, spender: Address) -> u64 {
    let Context { program, .. } = context;
    _allowance(&program, owner, spender)
}

pub fn _allowance(program: &Program<StateKey>, owner: Address, spender: Address) -> u64 {
    program
        .state()
        .get::<u64>(StateKey::Allowance(owner, spender))
        .expect("failure")
        .unwrap_or_default()
}

#[public]
pub fn approve(context: Context<StateKey>, spender: Address, amount: u64) -> bool {
    let Context { program, actor, .. } = context;
    assert_ne!(actor, spender, "actor and spender must be different");
    program
        .state()
        .store(StateKey::Allowance(actor, spender), &amount)
        .expect("failed to store allowance");
    true
}

/// Transfers balance from the sender to the the recipient.
#[public]
pub fn transfer(context: Context<StateKey>, recipient: Address, amount: u64) -> bool {
    let Context { program, actor, .. } = context;
    _transfer(&program, actor, recipient, amount)
}

fn _transfer(
    program: &Program<StateKey>,
    sender: Address,
    recipient: Address,
    amount: u64,
) -> bool {
    assert_ne!(sender, recipient, "sender and recipient must be different");

    // ensure the sender has adequate balance
    let sender_balance = program
        .state()
        .get::<u64>(StateKey::Balance(sender))
        .expect("failed to get sender balance")
        .unwrap_or_default();

    assert!(sender_balance >= amount, "invalid input");

    let recipient_balance = program
        .state()
        .get::<u64>(StateKey::Balance(recipient))
        .expect("failed to store recipient balance")
        .unwrap_or_default();

    // update balances
    program
        .state()
        .store(StateKey::Balance(sender), &(sender_balance - amount))
        .expect("failed to store sender balance");

    program
        .state()
        .store(StateKey::Balance(recipient), &(recipient_balance + amount))
        .expect("failed to store recipient balance");

    true
}

#[public]
pub fn transfer_from(
    context: Context<StateKey>,
    sender: Address,
    recipient: Address,
    amount: u64,
) -> bool {
    let Context {
        ref program, actor, ..
    } = context;
    let total_allowance = _allowance(program, sender, actor);
    assert!(total_allowance > amount);
    program
        .state()
        .store(
            StateKey::Allowance(sender, actor),
            &(total_allowance - amount),
        )
        .expect("failed to store allowance");
    _transfer(program, sender, recipient, amount)
}
