package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/skills"
)

func TestInstallAgentSkillCreatesSkill(t *testing.T) {
	skillRoot := t.TempDir()

	target, installed, err := installAgentSkill(skillRoot, false)
	if err != nil {
		t.Fatalf("installAgentSkill returned error: %v", err)
	}
	if !installed {
		t.Fatal("installAgentSkill reported installed=false for a new skill")
	}

	wantTarget := filepath.Join(skillRoot, agentSkillName, "SKILL.md")
	if target != wantTarget {
		t.Fatalf("target = %q, want %q", target, wantTarget)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read installed skill: %v", err)
	}
	if string(got) != skills.GitWT {
		t.Fatal("installed skill content does not match embedded skill")
	}
}

func TestInstallAgentSkillIsIdempotent(t *testing.T) {
	skillRoot := t.TempDir()

	if _, _, err := installAgentSkill(skillRoot, false); err != nil {
		t.Fatalf("first installAgentSkill returned error: %v", err)
	}

	target, installed, err := installAgentSkill(skillRoot, false)
	if err != nil {
		t.Fatalf("second installAgentSkill returned error: %v", err)
	}
	if installed {
		t.Fatal("installAgentSkill reported installed=true for an already installed skill")
	}
	if target == "" {
		t.Fatal("installAgentSkill returned an empty target")
	}
}

func TestInstallAgentSkillRefusesDifferentExistingSkill(t *testing.T) {
	skillRoot := t.TempDir()
	targetDir := filepath.Join(skillRoot, agentSkillName)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}
	target := filepath.Join(targetDir, "SKILL.md")
	if err := os.WriteFile(target, []byte("different skill\n"), 0o644); err != nil {
		t.Fatalf("failed to write existing skill: %v", err)
	}

	gotTarget, installed, err := installAgentSkill(skillRoot, false)
	if err == nil {
		t.Fatal("installAgentSkill succeeded with different existing skill")
	}
	if installed {
		t.Fatal("installAgentSkill reported installed=true after refusing overwrite")
	}
	if gotTarget != target {
		t.Fatalf("target = %q, want %q", gotTarget, target)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error %q does not mention --force", err)
	}
}

func TestInstallAgentSkillForceOverwritesDifferentExistingSkill(t *testing.T) {
	skillRoot := t.TempDir()
	targetDir := filepath.Join(skillRoot, agentSkillName)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}
	target := filepath.Join(targetDir, "SKILL.md")
	if err := os.WriteFile(target, []byte("different skill\n"), 0o644); err != nil {
		t.Fatalf("failed to write existing skill: %v", err)
	}

	gotTarget, installed, err := installAgentSkill(skillRoot, true)
	if err != nil {
		t.Fatalf("installAgentSkill returned error: %v", err)
	}
	if !installed {
		t.Fatal("installAgentSkill reported installed=false after force overwrite")
	}
	if gotTarget != target {
		t.Fatalf("target = %q, want %q", gotTarget, target)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read overwritten skill: %v", err)
	}
	if string(got) != skills.GitWT {
		t.Fatal("force overwrite did not write embedded skill content")
	}
}

func TestExpandAgentSkillRootExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := expandAgentSkillRoot("~/custom-skills")
	if err != nil {
		t.Fatalf("expandAgentSkillRoot returned error: %v", err)
	}

	want := filepath.Join(home, "custom-skills")
	if got != want {
		t.Fatalf("expandAgentSkillRoot = %q, want %q", got, want)
	}
}

func TestEmbeddedAgentSkillFrontmatter(t *testing.T) {
	if !strings.HasPrefix(skills.GitWT, "---\nname: git-wt\n") {
		t.Fatal("embedded skill is missing git-wt frontmatter")
	}
	if !strings.Contains(skills.GitWT, "description: Use the `git-wt` CLI") {
		t.Fatal("embedded skill is missing a useful description")
	}
	if !strings.Contains(skills.GitWT, "git wt doctor") {
		t.Fatal("embedded skill is missing git-wt usage guidance")
	}
	if !strings.HasSuffix(skills.GitWT, "\n") {
		t.Fatal("embedded skill should end with a newline")
	}
}
