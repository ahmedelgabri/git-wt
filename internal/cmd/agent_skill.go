package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ahmedelgabri/git-wt/skills"
	"github.com/spf13/cobra"
)

const (
	agentSkillName        = "git-wt"
	defaultAgentSkillRoot = "~/.agents/skills"
)

var agentSkillCmd = &cobra.Command{
	Use:   "agent-skill",
	Short: "Install the git-wt agent skill",
	Long: `Install an Agent Skills-compatible git-wt skill so coding agents can
load git-wt workflows on demand.

By default, this writes ~/.agents/skills/git-wt/SKILL.md. Use --dir to target a
different skill root, such as ~/.claude/skills, or --print to review the skill
without installing it.`,
	Example: `  git wt agent-skill
  git wt agent-skill --dir ~/.claude/skills
  git wt agent-skill --print
  git wt agent-skill --force`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runAgentSkill,
}

func init() {
	agentSkillCmd.Flags().String("dir", defaultAgentSkillRoot, "Skill root directory that will contain git-wt/SKILL.md")
	agentSkillCmd.Flags().Bool("force", false, "Overwrite an existing git-wt skill")
	agentSkillCmd.Flags().Bool("print", false, "Print the skill markdown instead of installing")
	rootCmd.AddCommand(agentSkillCmd)
}

func runAgentSkill(cmd *cobra.Command, args []string) error {
	if printSkill, _ := cmd.Flags().GetBool("print"); printSkill {
		_, err := fmt.Fprint(cmd.OutOrStdout(), skills.GitWT)
		return err
	}

	skillRoot, _ := cmd.Flags().GetString("dir")
	force, _ := cmd.Flags().GetBool("force")

	target, installed, err := installAgentSkill(skillRoot, force)
	if err != nil {
		return err
	}

	if installed {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Installed %s agent skill to %s\n", agentSkillName, target)
	} else {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s agent skill already installed at %s\n", agentSkillName, target)
	}
	return err
}

func installAgentSkill(skillRoot string, force bool) (target string, installed bool, err error) {
	skillRoot, err = expandAgentSkillRoot(skillRoot)
	if err != nil {
		return "", false, err
	}

	targetDir := filepath.Join(skillRoot, agentSkillName)
	target = filepath.Join(targetDir, "SKILL.md")

	existing, readErr := os.ReadFile(target)
	switch {
	case readErr == nil && string(existing) == skills.GitWT:
		return target, false, nil
	case readErr == nil && !force:
		return target, false, fmt.Errorf("agent skill already exists at %s; use --force to overwrite", target)
	case readErr != nil && !errors.Is(readErr, os.ErrNotExist):
		return target, false, readErr
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return target, false, err
	}
	if err := os.WriteFile(target, []byte(skills.GitWT), 0o644); err != nil {
		return target, false, err
	}
	return target, true, nil
}

func expandAgentSkillRoot(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("skill root directory cannot be empty")
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
