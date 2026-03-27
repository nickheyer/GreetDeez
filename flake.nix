{
  description = "GreetDeez - Hackable display manager for greetd, powered by Go + webkit2gtk";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    gomod2nix = {
      url = "github:tweag/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    { self, nixpkgs, gomod2nix, ... }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forEachSystem = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forEachSystem (
        system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ gomod2nix.overlays.default ];
          };
        in
        {
          default = pkgs.callPackage ./nix/package.nix {
            version = self.shortRev or self.dirtyShortRev or "dev";
          };
        }
      );

      nixosModules.default = import ./nix/module.nix self;

      devShells = forEachSystem (
        system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ gomod2nix.overlays.default ];
          };
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
              gomod2nix.packages.${system}.default
            ];
          };
        }
      );
    };
}
