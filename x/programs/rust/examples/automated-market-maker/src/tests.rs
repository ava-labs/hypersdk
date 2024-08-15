
#[cfg(test)]
mod tests {
    use std::ops::Add;

    use simulator::{SimpleState, Simulator};
    use wasmlanche_sdk::Address;
    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn init_program() {
        let mut state = SimpleState::new();
        let simulator = Simulator::new(&mut state);

        let error = simulator.create_program(PROGRAM_PATH).has_error();
        assert!(!error, "Create program errored")
    }

    #[test]
    fn amm_init() {
        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);
        let alice = Address::new([1; 33]);
        simulator.set_actor(alice);
        let gas = 1000000000;
       
        init_amm(&mut simulator, gas);
    }

    #[test]
    fn add_liquidity_same_ratio() {
        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);
        let alice = Address::new([1; 33]);
        simulator.set_actor(alice);
        let gas = 1000000000;
       
        let (token_x, token_y, lt, amm) = init_amm(&mut simulator, gas);
        let amount: u64 = 100;

        simulator.call_program(token_x, "mint", (alice, amount), gas).unwrap();
        simulator.call_program(token_y, "mint", (alice, amount), gas).unwrap();

        let balance = simulator.call_program(lt, "balance_of", (alice), gas).result::<u64>().unwrap();
        assert_eq!(balance, 0, "Balance of liquidity token is incorrect");

        simulator.call_program(token_x, "approve", (amm, amount), gas).unwrap();
        simulator.call_program(token_y, "approve", (amm, amount), gas).unwrap();
        // println!("approved tokens to be spent by {:?}", lt);

        let result = simulator.call_program(amm, "add_liquidity", (amount, amount), gas);
        assert!(!result.has_error(), "Add liquidity errored {:?}", result.error());

        let balance = simulator.call_program(lt, "balance_of", (alice), gas).result::<u64>().unwrap();
        assert!(balance > 0, "Balance of liquidity token is incorrect");

        println!("Balance of liquidity token: {:?}", balance);
    }

    #[test]
    fn add_liquidity_different_ratio() {
        
    }

    #[test]
    fn swap_changes_ratio() {
       
    }

    #[test]
    fn swap_insufficient_funds() {
        
    }

    #[test]
    fn swap_incorrect_ratio() {
        
    }

    #[test]
    fn swap_incorrect_token() {
        
    }

    #[test]
    fn remove_liquidity() {
        
    }

    #[test]
    fn remove_liquidity_insufficient_funds() {
        
    }

    fn init_amm (simulator: &mut Simulator, gas: u64) -> (Address, Address, Address, Address) {
        let token_path = PROGRAM_PATH
        .replace("automated_market_maker", "token")
        .replace("automated-market-maker", "token");
        let alice = Address::new([1; 33]);

        simulator.set_actor(alice);
        // Setup the tokens
        // TODO: would be a good simulator test if we check token_x and token_y ID to be the same
        let token_x = simulator.create_program(&token_path);
        let token_program_id = token_x.program_id().unwrap();
        let token_x = token_x.program().unwrap();
        println!("Token X: {:?}", token_x);
        let amount: u64 = 100;



        let token_y = simulator.create_program(&token_path).program().unwrap();

        // initialize tokens
        simulator.call_program(token_x, "init", ("CoinX", "CX"), gas).unwrap();
        simulator.call_program(token_y, "init", ("YCoin", "YC"), gas).unwrap();
        let amm_program = simulator.create_program(PROGRAM_PATH).program().unwrap();

        
        let result = simulator.call_program(amm_program, "init", (token_x, token_y, token_program_id), gas);
        assert!(!result.has_error(), "Init AMM errored");


        // Check if the liquidity token was created
        let lt = simulator.call_program(amm_program, "get_liquidity_token", (), gas);
        assert!(!lt.has_error(), "Get liquidity token errored");
        let lt = lt.result::<Address>().unwrap();
        // grab the name of the liquidity token
        let lt_name = simulator.call_program(lt, "symbol", (), gas);
        assert!(!lt_name.has_error(), "Get liquidity token name errored");
        let lt_name = lt_name.result::<String>().unwrap();
        assert_eq!(lt_name, "LT", "Liquidity token name is incorrect");

        (token_x, token_y, lt, amm_program)
    }
}
