use wasmlanche_sdk::store::ProgramContext;
use wasmlanche_sdk::types::Address;

#[link(wasm_import_module = "token")]
extern "C" {
    fn _transfer(ctx: ProgramContext, sender: Address, recipient: Address, amount: i64) -> bool;
}
