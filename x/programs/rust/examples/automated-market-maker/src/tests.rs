
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
