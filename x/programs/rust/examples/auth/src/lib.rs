use wasmlanche_sdk::types::Address;
use wasmlanche_sdk::{public, state_keys, Context};

#[macro_export]
macro_rules! auth_role {
    // TODO add "+"
    ($( $name:ident ),*) => {
        use borsh::BorshDeserialize;
        use paste::paste;

        #[state_keys]
        pub enum StateKeys {
            Initialized,
            $($name),*
        }

        #[derive(Debug, BorshDeserialize)]
        pub enum Role {
            $($name),*
        }

        impl Into<StateKeys> for Role {
            fn into(self) -> StateKeys {
                match self {
                    $(Role::$name => StateKeys::$name),*
                }
            }
        }

        paste! {
            #[public]
            pub fn init_roles(context: Context<StateKeys>, $([<$name:lower>]: Address),*) {
                let program = context.program();
                if program
                    .state()
                    .get(StateKeys::Initialized)
                    .unwrap()
                    .is_some_and(|init| init)
                {
                    panic!("already initialized");
                }

                paste! {
                    program
                        .state()
                        .store([$((StateKeys::$name, &[<$name:lower>])),*])
                        .unwrap();
                }
            }
        }
    };
}

auth_role!(Master, Executor);

#[public]
pub fn ensure_auth(context: Context<StateKeys>, actor: Address, role: Role) {
    let state = context.program().state();
    assert_eq!(actor, state.get(role.into()).unwrap().unwrap());
}
