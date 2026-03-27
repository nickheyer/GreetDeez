{
  lib,
  buildGoApplication,
  pkg-config,
  nodejs,
  buf,
  protoc-gen-go,
  protoc-gen-es,
  gnumake,
  gtk3,
  webkitgtk_4_1,
  version,
}:

buildGoApplication {
  pname = "greetdeez";
  inherit version;
  src = lib.cleanSource ./..;
  modules = ../gomod2nix.toml;

  nativeBuildInputs = [ pkg-config nodejs buf protoc-gen-go protoc-gen-es gnumake ];
  buildInputs = [ gtk3 webkitgtk_4_1 ];

  buildPhase = ''
    export HOME=$TMPDIR
    export GOBIN=$TMPDIR/bin
    export PATH=$GOBIN:$PATH
    make build
  '';

  installPhase = ''
    install -Dm755 bin/greetdeez $out/bin/greetdeez
  '';

  meta = {
    description = "Hackable display manager for greetd, powered by Go + webkit2gtk";
    homepage = "https://github.com/nickheyer/greetdeez";
    license = lib.licenses.mit;
    platforms = lib.platforms.linux;
    mainProgram = "greetdeez";
  };
}
