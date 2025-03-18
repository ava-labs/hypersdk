{
  # To use:
  #  - install nix: https://github.com/DeterminateSystems/nix-installer?tab=readme-ov-file#install-nix
  #  - run `nix develop` or use direnv (https://direnv.net/)
  #    - for quieter direnv output, set `export DIRENV_LOG_FORMAT=`

  description = "HyperSDK development environment";

  inputs = {
    nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.2405.*.tar.gz";
    avalanchego.url = "github:ava-labs/avalanchego?ref=6626b41b013f2365b96bd66dfc8499ee4861d25b";
  };

  outputs = { self, nixpkgs, avalanchego, ... }:
    let
      allSystems = builtins.attrNames avalanchego.devShells;
      forAllSystems = nixpkgs.lib.genAttrs allSystems;
    in {
      # Define the development shells for this repository
      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in {
          default = avalanchego.devShells.${system}.default.overrideAttrs (oldAttrs: {
            buildInputs = oldAttrs.buildInputs ++ [
              # Required to ensure scripts/update_avalanchego_version.sh works on darwin
              # TODO(marun) Convert the update script to golang to avoid this dependency
              pkgs.gnused
            ];
          });
        }
      );
    };
}
