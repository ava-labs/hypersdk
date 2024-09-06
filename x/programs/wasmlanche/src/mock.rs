#[cfg(feature = "test")]
mod functions {
    use crate::context::CallContractArgs;
    use crate::{types::Gas, Id};
    use crate::{Address, Context, ExternalCallError};
    use borsh::BorshSerialize;

    impl Context {
        /// Mocks an external function call.
        /// # Panics
        /// Panics if serialization fails.
        pub fn mock_call_function<T, U>(
            &self,
            target: Address,
            function: &str,
            args: T,
            max_units: Gas,
            max_value: u64,
            result: U,
        ) where
            T: BorshSerialize,
            U: BorshSerialize,
        {
            use crate::host::CALL_FUNCTION_PREFIX;

            let args = borsh::to_vec(&args).expect("error serializing result");

            let contract_args =
                CallContractArgs::new(&target, function, &args, max_units, max_value);
            let contract_args = borsh::to_vec(&(CALL_FUNCTION_PREFIX, contract_args))
                .expect("error serializing result");

            // serialize the result as Ok(result)
            let result: Result<U, ExternalCallError> = Ok(result);
            let result = borsh::to_vec(&result).expect("error serializing result");
            self.host_accessor().state().put(&contract_args, result);
        }

        /// Mocks a deploy call.
        /// # Panics
        /// Panics if serialization fails.
        pub fn mock_deploy(&self, program_id: Id, account_creation_data: &[u8]) -> Address {
            use crate::host::DEPLOY_PREFIX;

            let key = borsh::to_vec(&(DEPLOY_PREFIX, program_id, account_creation_data))
                .expect("failed to serialize args");

            let val = self.host_accessor().new_deploy_address();

            self.host_accessor()
                .state()
                .put(&key, val.as_ref().to_vec());

            val
        }
    }
}
