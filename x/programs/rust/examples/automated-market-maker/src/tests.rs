
#[cfg(test)]
mod tests {
    use simulator::{SimpleState, Simulator};
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
        let simulator = Simulator::new(&mut state);
        let gas = 1000000;
        let token_path = PROGRAM_PATH
        .replace("automated_market_maker", "token")
        .replace("automated_market_maker", "token");

        // Setup the tokens
        // TODO: would be a good simulator test if we check token_x and token_y ID to be the same
        let token_x = simulator.create_program(&token_path);
        let token_y = simulator.create_program(&token_path).program().unwrap();
        let token_id = token_x.program_id().unwrap();
        let token_x = token_x.program().unwrap();

        // initialize tokens
        let result = simulator.call_program(token_x, "init", ("CoinX", "CX"), gas);
        let result = simulator.call_program(token_x, "init", ("YCoin", "YC"), gas);

        let amm_program = simulator.create_program(PROGRAM_PATH).program().unwrap();
        let result = simulator.call_program(program, "init", (token_x, token_y, token_id), gas);
        assert!(!result.has_error(), "Init AMM errored");
    }

    #[test]
    fn add_liquidity_same_ratio() {
        
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
}
