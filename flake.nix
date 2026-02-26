{
  description = "slack-pipe: Secure CLI for reading Slack via session tokens";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-darwin" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "slack-pipe";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-h39PAVqfgCLFwcAdymmIi3v3MrxxEDoreDjw/EZ2tow=";
          subPackages = [ "cmd/slack-pipe" ];
          ldflags = [
            "-s" "-w"
            "-X main.version=0.1.0"
            "-X main.commit=${self.shortRev or "dirty"}"
          ];
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_25
            golangci-lint
            gotools
            gopls
          ];
        };
      }
    );
}
