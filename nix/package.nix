{
  lib,
  stdenv,
  go,
  pkg-config,
  nodejs,
  buf,
  protoc-gen-go,
  protoc-gen-es,
  gnumake,
  gtk3,
  webkitgtk_4_1,
}:

stdenv.mkDerivation {
  pname = "greetdeez";
  version = "0.1.0";
  src = lib.cleanSource ./..;

  nativeBuildInputs = [ go pkg-config nodejs buf protoc-gen-go protoc-gen-es gnumake ];
  buildInputs = [ gtk3 webkitgtk_4_1 ];

  buildPhase = ''
    export HOME=$TMPDIR
    export GOPATH=$TMPDIR/go
    export GOCACHE=$TMPDIR/go-cache
    make build
  '';

  installPhase = ''
    install -Dm755 bin/greetdeez $out/bin/greetdeez
  '';

  meta = {
    description = "Hackable display manager greeter for greetd, powered by Go + webkit2gtk";
    homepage = "https://github.com/nickheyer/greetdeez";
    license = lib.licenses.mit;
    platforms = lib.platforms.linux;
    mainProgram = "greetdeez";
  };
}
