{
  description = "A flake that builds a repo";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    inputs@{
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        go = pkgs.go_1_24;
        buildInputs = with pkgs; [
          go
          coreutils
          curl
          xmlstarlet

          # Protobuf + gRPC
          protobuf
          protoc-gen-go
          protoc-gen-go-grpc
          grpc
        ];

        defaultShellHook = ''
          export SHELL="${pkgs.bashInteractive}/bin/bash"

          export FLAKE_ROOT="$(nix flake metadata | grep 'Resolved URL' | awk '{print $3}' | sed 's/^path://' | sed 's/^git+file:\/\///')"
          export HISTFILE="$FLAKE_ROOT/.nix_bash_history"

          export GOROOT="${go}/share/go"
        '';
      in
      {
        # run: `nix develop`
        devShells = {
          default = pkgs.mkShell {
            inherit buildInputs;
            shellHook = defaultShellHook;
          };

          # Update IDEA paths. Use only if nix installed in whole system.
          # run: `nix develop .#idea`
          idea = pkgs.mkShell {
            inherit buildInputs;

            shellHook = pkgs.lib.concatLines [
              defaultShellHook
              ''
                cd "$FLAKE_ROOT"

                echo "Replace GOPATH"
                xmlstarlet ed -L -u '//project/component[@name="GOROOT"]/@url' -v 'file://${go}/share/go' .idea/workspace.xml

                exit 0
              ''
            ];
          };
        };
      }
    );
}
