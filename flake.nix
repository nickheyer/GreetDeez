{
  description = "GreetDeez - Hackable display manager for greetd, powered by Go + webkit2gtk";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { nixpkgs, ... }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      devShells.${system}.default = pkgs.mkShell {
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
          wlr-randr
          cage
        ];
      };
    };
}
