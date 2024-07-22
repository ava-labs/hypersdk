
#[cfg(test)]
mod tests {
    use simulator::{Endpoint, Param, Step, TestContext};
    use wasmlanche_sdk::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn create_program() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap();
    }


    #[test]
    fn amm_init() {
         let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let amm_program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let 

        let context = TestContext::new(program_id);


        
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
