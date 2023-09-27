use wasmlanche_sdk::{program::Program, public, types::Address};


/// The internal state keys used by the program.
#[repr(u8)]
enum StateKey {
    TotalSupply,
    Name,
    Symbol,
    Balances(Address),
}

impl StateKey {
    pub fn to_vec(self) -> Vec<u8> {
        match self {
            StateKey::TotalSupply => vec![self.into()],
            StateKey::Name => vec![self.into()],
            StateKey::Symbol => vec![self.into()],
            StateKey::Balances(a) => {
                let mut bytes = Vec::with_capacity(1 + 32);
                bytes.push(self.into());
                bytes.extend_from_slice(a.as_bytes());
                bytes
            }
        }
    }
}

impl Into<Vec<u8>> for StateKey {
    fn into(self) -> Vec<u8> {
        self.to_vec()
    }
}

impl Into<u8> for StateKey {
    fn into(self) -> u8 {
        match self {
            StateKey::TotalSupply => 0,
            StateKey::Name => 1,
            StateKey::Symbol => 2,
            StateKey::Balances(_) => 3,
        }
    }
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn init(program: Program) -> bool {
    program
        .state()
        .insert(&StateKey::TotalSupply.to_vec(), &123456789_i64)
        .expect("failed to insert total supply");

    program
        .state()
        .insert(&StateKey::Name.to_vec(), b"WasmCoin")
        .expect("failed to insert total supply");

    program
        .state()
        .insert(&StateKey::Symbol.to_vec(), b"WACK")
        .expect("failed to insert symbol");

    true
}

/// Gets total supply or -1 on error.
#[public]
pub fn get_total_supply(program: Program) -> i64 {
    program
        .state()
        .get_value(&StateKey::TotalSupply.to_vec())
        .expect("failed to get total supply")
}

/// Adds amount coins to the recipients balance.
#[public]
pub fn mint_to(program: Program, recipient: Address, amount: i64) -> bool {
    let balance = program
        .state()
        .get_value(&StateKey::Balances(recipient).to_vec())
        .unwrap_or(0);

    program
        .state()
        .insert(&StateKey::Balances(recipient).to_vec(), &(balance + amount))
        .is_ok()
}

/// Transfers amount coins from the sender to the recipient. Returns whether successful.
#[public]
pub fn transfer(state: State, sender: Address, recipient: Address, amount: i64) -> bool {
    // require sender != recipient
    if sender == recipient {
        return false;
    }
    // ensure the sender has adequate balance
    let sender_balance: i64 = state.get_map_value("balances", &sender).unwrap_or(0);
    if amount < 0 || sender_balance < amount {
        return false;
    }
    let recipient_balance: i64 = state.get_map_value("balances", &recipient).unwrap_or(0);
    state
        .store_map_value("balances", &sender, &(sender_balance - amount))
        .store_map_value("balances", &recipient, &(recipient_balance + amount))
        .is_ok()
}

/// Gets the balance of the recipient.
#[public]
pub fn get_balance(state: State, recipient: Address) -> i64 {
    state.get_map_value("balances", &recipient).unwrap_or(0)
}
