{
  description = "GreetDeez - Hackable display manager for greetd, powered by Go + webkit2gtk";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs, ... }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forEachSystem = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forEachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.callPackage ./nix/package.nix { };
        }
      );

      nixosModules.default = import ./nix/module.nix self;

      devShells = forEachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              pkg-config
              go
              nodejs
              buf
              protoc-gen-go
              protoc-gen-es
              gnumake
              gtk3
              webkitgtk_4_1
            ];
          };
        }
      );
    };
}
