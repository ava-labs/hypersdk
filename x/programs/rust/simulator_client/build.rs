fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
         .build_server(false)
         .out_dir("src/")
         .compile(
             &["../../cmd/simulator_api/api/service.proto"],
             &["../../cmd/simulator_api/api/"]
         )?;
    Ok(())
}