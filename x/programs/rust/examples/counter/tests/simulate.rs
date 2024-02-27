//mod simulate;
use async_std;
use nix::sys::signal::{kill, Signal};
use nix::unistd::Pid;
use serial_test::serial;
use simulator_client::{CreateKeyRequest, PublicKey, SimulatorClient};
use std::borrow::Borrow;
use std::env;
use std::sync::WaitTimeoutResult;
use std::{process::Child, process::Command};
use std::{thread, time};
use tokio;
use tonic::{self, IntoRequest};
use wasmlanche_sdk::simulator;

// export SIMULATOR_PATH=/path/to/simulator
// export PROGRAM_PATH=/path/to/program.wasm
// cargo cargo test --package token --lib nocapture -- tests::test_token_plan --exact --nocapture --ignored
struct SimulatorLife {
    child: Child,
    cleaned: bool,
}

impl SimulatorLife {
    fn new() -> SimulatorLife {
        let s_path = env::var(simulator::PATH_KEY).expect("SIMULATOR_PATH not set");
        let server_process = Command::new(s_path)
            .spawn()
            .expect("failed to spawn simulator");
        SimulatorLife {
            child: server_process,
            cleaned: false,
        }
    }
    fn cleanup(&mut self) {
        let pid = Pid::from_raw(self.child.id() as i32);
        kill(pid, Signal::SIGTERM).unwrap();
        println!("stoped simulator");
        self.cleaned = true;
    }
}

impl Drop for SimulatorLife {
    fn drop(&mut self) {
        if !self.cleaned {
            self.cleanup()
        }
    }
}

#[tokio::test]
#[serial]
//#[ignore = "requires SIMULATOR_PATH and PROGRAM_PATH to be set"]
async fn test_counter_plan() -> Result<(), Box<dyn std::error::Error>> {
    let mut simulator_life = SimulatorLife::new();
    //let ten_millis = time::Duration::from_millis(5000);
    thread::sleep(time::Duration::from_millis(1000));
    let mut client = SimulatorClient::connect("http://127.0.0.1:50051").await?;

    let request = tonic::Request::new(CreateKeyRequest {
        name: "alice".into(),
    });
    let response = client.create_key(request).await?.into_inner();
    assert_ne!(response.value, "");
    simulator_life.cleanup();
    Ok(())
}
