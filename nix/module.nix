# NixOS module for GreetDeez.
#
# In your NixOS configuration:
#
#   # flake.nix
#   inputs.greetdeez.url = "github:nickheyer/GreetDeez";
#
#   # configuration.nix
#   imports = [ inputs.greetdeez.nixosModules.default ];
#
#   services.greetdeez.enable = true;
#
self:
{ config, lib, pkgs, ... }:

let
  cfg = config.services.greetdeez;
  tomlFormat = pkgs.formats.toml { };

  desktops = config.services.displayManager.sessionData.desktops;

  defaultSettings = {
    sessions.dirs = [
      { path = "${desktops}/share/wayland-sessions"; type = "wayland"; }
      { path = "${desktops}/share/xsessions"; type = "x11"; }
    ];
  };

  greetdeezConf = tomlFormat.generate "greetdeez.conf"
    (lib.recursiveUpdate defaultSettings cfg.settings);

  greetdeezCmd = "${cfg.package}/bin/greetdeez -config ${greetdeezConf}";
in
{
  options.services.greetdeez = {
    enable = lib.mkEnableOption "GreetDeez - Hackable display manager for greetd, powered by Go + webkit2gtk";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      description = "The greetdeez package to use.";
    };

    settings = lib.mkOption {
      type = tomlFormat.type;
      default = { };
      description = ''
        Settings written to greetdeez.conf (TOML).
        Session directories are auto-configured from NixOS session packages
        and merged with any values you set here.

        See https://github.com/nickheyer/GreetDeez for available options.
      '';
      example = lib.literalExpression ''
        {
          window.title = "Login";
          ui.theme = "cyber";
          power.enabled = true;
        }
      '';
    };

    cage = {
      enable = lib.mkOption {
        type = lib.types.bool;
        default = true;
        description = "Run greetdeez inside cage (Wayland kiosk compositor).";
      };

      extraArgs = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [ "-s" "-m" "last" ];
        description = "Extra arguments passed to cage.";
      };
    };

    vt = lib.mkOption {
      type = lib.types.int;
      default = 7;
      description = "Virtual terminal to run the greeter on.";
    };
  };

  config = lib.mkIf cfg.enable {
    services.greetd = {
      enable = true;
      settings = {
        terminal.vt = lib.mkDefault cfg.vt;
        default_session = {
          command =
            if cfg.cage.enable then
              "${pkgs.cage}/bin/cage ${lib.escapeShellArgs cfg.cage.extraArgs} -- ${greetdeezCmd}"
            else
              greetdeezCmd;
          user = "greetdeez";
        };
      };
    };

    users.users.greetdeez = {
      isSystemUser = true;
      group = "greetdeez";
      home = "/var/lib/greetdeez";
    };
    users.groups.greetdeez = { };

    systemd.tmpfiles.rules = [
      "d /var/lib/greetdeez 0750 greetdeez greetdeez -"
    ];
  };
}
