use borsh::BorshDeserialize;
use wasmlanche::{public, state_schema, Address, Context};

state_schema! {
    Time => u64,
    Program => Address,
    Function => String,
    Args => Vec<u8>,
    Executed => bool,
}

const LOCK_TIME: u64 = 3600;

#[public]
pub fn schedule(ctx: &mut Context, to: Address, function: String, data: Vec<u8>) {
    if ctx.get(Time).expect("failed to deserialize").is_some() {
        panic!("already scheduled");
    }

    ctx.store((
        (Time, ctx.timestamp() + LOCK_TIME),
        (Program, to),
        (Function, function),
        (Args, data),
    ))
    .expect("failed to serialize");
}

#[public]
pub fn execute(ctx: &mut Context, max_units: u64) -> Vec<u8> {
    if ctx
        .get(Executed)
        .expect("failed to deserialize")
        .is_some_and(|executed| executed)
    {
        panic!("already executed");
    }

    let time = ctx
        .get(Time)
        .expect("failed to deserialize")
        .expect("not scheduled");

    if time > ctx.timestamp() {
        panic!("too early");
    }

    let called_program = ctx.get(Program).expect("failed to deserialize").unwrap();

    let function = ctx.get(Function).expect("failed to deserialize").unwrap();

    let args = ctx.get(Args).expect("failed to deserialize").unwrap();

    ctx.store_by_key(Executed, true).unwrap();

    ctx.call_contract(called_program, &function, args.as_slice(), max_units, 0)
        .expect("external call failed")
}

#[public]
pub fn next_execution(ctx: &mut Context) -> Option<u64> {
    ctx.get(Time).expect("failed to deserialize")
}

mod de {
    use super::*;

    pub struct DeferDeserialization(Vec<u8>);

    impl From<DeferDeserialization> for Vec<u8> {
        fn from(val: DeferDeserialization) -> Self {
            val.0
        }
    }

    impl BorshDeserialize for DeferDeserialization {
        fn deserialize_reader<R: std::io::Read>(reader: &mut R) -> std::io::Result<Self> {
            let mut bytes = Vec::new();

            reader.read_to_end(&mut bytes)?;

            Ok(DeferDeserialization(bytes))
        }
    }
}

#[cfg(test)]
mod tests {
    use wasmlanche::simulator::{Error, SimpleState, Simulator};

    use super::*;
    #[test]
    #[should_panic = "not scheduled"]
    fn cannot_pre_execute() {
        let mut ctx = Context::new();
        execute(&mut ctx, u64::MAX);
    }

    #[test]
    fn schedule_in_the_future() {
        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);
        let timelock = Address::new([1; 33]);

        let mut ctx = Context::new();

        let timestamp = 123456;
        simulator.set_timestamp(timestamp);
        schedule(&mut ctx, timelock, String::new(), vec![]);

        let next_execution = next_execution(&mut ctx);
        assert_eq!(next_execution, Some(timestamp + LOCK_TIME));
    }

    #[test]
    #[should_panic = "too early"]
    fn early_execute() {
        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);
        let timelock = Address::new([1; 33]);

        let mut ctx = Context::new();

        let timestamp = 123456;
        simulator.set_timestamp(timestamp);
        schedule(&mut ctx, timelock, "".to_string(), vec![]);

        execute(&mut ctx, u64::MAX);
    }

    #[test]
    fn cannot_re_execute() {
        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);
        let timelock = Address::new([1; 33]);
        simulator.set_actor(Address::new([2; 33]));

        let timestamp = 123456;
        simulator.set_timestamp(timestamp);
        simulator
            .call_contract::<(), _>(
                timelock,
                "schedule",
                (timelock, "next_execution".to_string(), Vec::<u8>::new()),
                u64::MAX,
            )
            .unwrap();

        simulator.set_timestamp(timestamp + LOCK_TIME);
        simulator
            .call_contract::<(), _>(timelock, "execute", (), u64::MAX)
            .unwrap();

        simulator.set_timestamp(timestamp + LOCK_TIME);
        let err = simulator
            .call_contract::<(), _>(timelock, "execute", (), u64::MAX)
            .unwrap_err();
        assert!(matches!(err, Error::CallContract(_)));
    }

    #[test]
    fn timelock_execution() {
        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);
        let timelock = Address::new([1; 33]);

        let mut ctx = Context::new();

        let timestamp = 123456;
        simulator.set_timestamp(timestamp);
        schedule(&mut ctx, timelock, "next_execution".to_string(), vec![]);

        simulator.set_timestamp(timestamp + LOCK_TIME);
        execute(&mut ctx, u64::MAX);
    }
}
