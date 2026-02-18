{
  description = "git-wt - Git worktree management tool";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  outputs = inputs:
    inputs.flake-parts.lib.mkFlake {inherit inputs;} {
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      imports = [
        inputs.treefmt-nix.flakeModule
      ];

      perSystem = {
        pkgs,
        config,
        self',
        lib,
        ...
      }: {
        packages = {
          default = self'.packages.git-wt;
          git-wt = pkgs.buildGoModule {
            pname = "git-wt";
            version = "1.0.0";

            src = lib.cleanSource ./.;

            vendorHash = "sha256-ono3ROh/4JWxHXzEFzqL8NfFurfSw/QiMAroZkmniRM=";

            nativeBuildInputs = with pkgs; [
              installShellFiles
              makeWrapper
            ];

            subPackages = ["cmd/git-wt"];

            postInstall = ''
              # Generate shell completions before wrapping
              $out/bin/git-wt completion bash > git-wt.bash
              $out/bin/git-wt completion zsh > _git-wt
              $out/bin/git-wt completion fish > git-wt.fish
              installShellCompletion --bash git-wt.bash
              installShellCompletion --zsh _git-wt
              installShellCompletion --fish git-wt.fish

              # Generate and install man pages
              mkdir -p $TMPDIR/man
              $out/bin/git-wt man $TMPDIR/man
              installManPage $TMPDIR/man/*.1

              wrapProgram $out/bin/git-wt \
                --prefix PATH : ${pkgs.lib.makeBinPath [pkgs.git]}
            '';

            meta = {
              mainProgram = "git-wt";
              homepage = "https://github.com/ahmedelgabri/git-wt";
              description = "Git worktree management tool";
              license = lib.licenses.mit;
              platforms = lib.platforms.unix;
            };
          };
        };

        treefmt = {
          projectRootFile = "flake.nix";

          programs = {
            gofumpt.enable = true;
            prettier = {
              enable = true;
              includes = [
                "*.md"
                "*.yml"
                "*.yaml"
                "*.json"
                "*.svg"
              ];
            };
            alejandra.enable = true;
          };
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            git
            nixd
            lefthook
            prettier
            bats
            go
            gopls
            gofumpt
            go-tools # staticcheck, etc...
            gomodifytags
            gotools # goimports
          ];

          inputsFrom = [config.treefmt.build.devShell];

          shellHook =
            /*
            bash
            */
            ''
              # avoid overriding global git hooks
              git config core.hooksPath .hooks
              lefthook install
            '';
        };
      };
    };
}
