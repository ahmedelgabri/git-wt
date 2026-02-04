class GitWt < Formula
  desc "Git custom command that enhances Git's native worktree functionality"
  homepage "https://github.com/ahmedelgabri/git-wt"
  url "https://github.com/ahmedelgabri/git-wt.git",
      branch: "main"
  version "0.1.0"
  license "MIT"
  head "https://github.com/ahmedelgabri/git-wt.git", branch: "main"

  depends_on "fzf"
  depends_on "git"

  def install
    bin.install "git-wt"
    bash_completion.install "completions/git-wt.bash" => "git-wt"
    zsh_completion.install "completions/git-wt.zsh" => "_git-wt"
    fish_completion.install "completions/git-wt.fish"
  end

  test do
    system bin/"git-wt", "help"
  end
end
