{
  description = "GreetDeez — hackable webkit2gtk greeter for greetd";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-linux" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Pre-fetch npm dependencies for offline build in the Nix sandbox.
        # To update after changing package-lock.json:
        #   nix run nixpkgs#prefetch-npm-deps -- ui/greetdeez/package-lock.json
        npmDeps = pkgs.fetchNpmDeps {
          src = ./ui/greetdeez;
          name = "greetdeez-ui-npm-deps";
          # Replace with real hash from prefetch-npm-deps:
          hash = pkgs.lib.fakeHash;
        };

        # Build the Svelte UI as a standalone derivation.
        ui = pkgs.stdenv.mkDerivation {
          pname = "greetdeez-ui";
          version = "0.1.0";
          src = ./ui/greetdeez;

          nativeBuildInputs = with pkgs; [
            nodejs
            npmHooks.npmConfigHook
          ];

          inherit npmDeps;

          buildPhase = ''
            npm run build
          '';

          installPhase = ''
            cp -r build $out
          '';
        };
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "greetdeez";
          version = "0.1.0";
          src = ./.;

          # Replace with real hash after first build attempt:
          #   nix build .# 2>&1 | grep 'got:' | awk '{print $2}'
          vendorHash = pkgs.lib.fakeHash;

          nativeBuildInputs = with pkgs; [
            pkg-config
          ];

          buildInputs = with pkgs; [
            gtk3
            webkitgtk_4_1
          ];

          CGO_ENABLED = "1";

          # Copy pre-built UI into the source tree before Go embeds it.
          preBuild = ''
            cp -r ${ui} ui/greetdeez/build
          '';

          postInstall = ''
            mkdir -p $out/etc/greetd
            cp greetd.toml $out/etc/greetd/greetd.toml
            cp greetdeez.conf $out/etc/greetd/greetdeez.conf
          '';

          meta = with pkgs.lib; {
            description = "Minimal, hackable display manager greeter for greetd";
            homepage = "https://github.com/nickheyer/greetdeez";
            license = licenses.mit;
            platforms = platforms.linux;
            mainProgram = "greetdeez";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            nodejs
            pkg-config
            gtk3
            webkitgtk_4_1
            goreleaser
            nfpm
          ];
        };
      }
    ) // {
      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.programs.greetdeez;
        in
        {
          options.programs.greetdeez = {
            enable = lib.mkEnableOption "GreetDeez greeter for greetd";

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.default;
              description = "The greetdeez package to use.";
            };

            cage = {
              package = lib.mkOption {
                type = lib.types.package;
                default = pkgs.cage;
                description = "The cage package to use.";
              };
            };

            settings = lib.mkOption {
              type = lib.types.attrs;
              default = {};
              description = "greetdeez.conf settings (TOML attrs).";
            };
          };

          config = lib.mkIf cfg.enable {
            services.greetd = {
              enable = true;
              settings.default_session = {
                command = "${cfg.cage.package}/bin/cage -s -- ${cfg.package}/bin/greetdeez";
                user = "greetdeez";
              };
            };

            users.users.greetdeez = {
              isSystemUser = true;
              group = "greetdeez";
              home = "/var/lib/greetdeez";
              createHome = true;
            };
            users.groups.greetdeez = {};

            environment.etc."greetd/greetdeez.conf".source =
              "${cfg.package}/etc/greetd/greetdeez.conf";
          };
        };
    };
}
