{
  # To use:
  #  - install nix: https://github.com/DeterminateSystems/nix-installer?tab=readme-ov-file#install-nix
  #  - run `nix develop` or use direnv (https://direnv.net/)
  #    - for quieter direnv output, set `export DIRENV_LOG_FORMAT=`

  description = "HyperSDK development environment";

  inputs = {
    nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.2405.*.tar.gz";
    avalanchego.url = "github:ava-labs/avalanchego?ref=8551e1d96221c8a5c2c034c2bad0cd571610ad35";
  };

  outputs = { self, nixpkgs, avalanchego, ... }:
    let
      allSystems = builtins.attrNames avalanchego.devShells;
      forAllSystems = nixpkgs.lib.genAttrs allSystems;
    in {
      # Define the development shells for this repository
      devShells = forAllSystems (system: {
        default = avalanchego.devShells.${system}.default;
      });
    };
}
