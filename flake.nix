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
          git-wt = pkgs.stdenvNoCC.mkDerivation {
            pname = "git-wt";
            version = "0.1.0";

            src = ./.;

            nativeBuildInputs = with pkgs; [
              makeWrapper
              installShellFiles
            ];

            installPhase = ''
              runHook preInstall

              mkdir -p $out/bin
              cp git-wt $out/bin/git-wt
              chmod +x $out/bin/git-wt
              wrapProgram $out/bin/git-wt \
                --prefix PATH : ${
                pkgs.lib.makeBinPath [
                  pkgs.git
                  pkgs.fzf
                ]
              }

              runHook postInstall
            '';

            postInstall = ''
              installShellCompletion --bash ${./completions/git-wt.bash}
              installShellCompletion --zsh ${./completions/git-wt.zsh}
              installShellCompletion --fish ${./completions/git-wt.fish}
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
            shfmt = {
              enable = true;
              indent_size = 0; # 0 = tabs
            };
            prettier = {
              enable = true;
              includes = [
                "*.md"
                "*.yml"
                "*.yaml"
                "*.json"
              ];
            };
            alejandra.enable = true;
          };

          settings.formatter.shfmt.includes = [
            "git-wt"
            "completions/*.bash"
          ];
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            git
            fzf
            nixd
            shellcheck
            lefthook
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
