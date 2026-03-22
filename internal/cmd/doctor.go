package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

type doctorLevel string

const (
	doctorOK    doctorLevel = "OK"
	doctorWarn  doctorLevel = "WARN"
	doctorError doctorLevel = "ERROR"
)

type doctorCheck struct {
	Level  doctorLevel
	Name   string
	Detail string
}

var doctorCmd = &cobra.Command{
	Use:           "doctor",
	Short:         "Run diagnostics on the current repository layout",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := doctorRepoRoot()
		if err != nil {
			return err
		}

		checks, hasErrors := runDoctorChecks(repoRoot)
		fmt.Println(renderDoctorChecks(checks))
		if hasErrors {
			return fmt.Errorf("doctor found issues")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func doctorRepoRoot() (string, error) {
	commonDir, err := git.Query("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}

	commonDir, err = normalizeDoctorPath(commonDir)
	if err != nil {
		return "", err
	}

	switch filepath.Base(commonDir) {
	case ".bare", ".git":
		return filepath.Dir(commonDir), nil
	default:
		repoRoot, err := git.Query("rev-parse", "--show-toplevel")
		if err == nil {
			return normalizeDoctorPath(repoRoot)
		}
		return commonDir, nil
	}
}

func normalizeDoctorPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func renderDoctorChecks(checks []doctorCheck) string {
	rows := make([][]string, 0, len(checks))
	okCount, warnCount, errorCount := 0, 0, 0
	for _, check := range checks {
		switch check.Level {
		case doctorOK:
			okCount++
		case doctorWarn:
			warnCount++
		case doctorError:
			errorCount++
		}
		rows = append(rows, []string{
			renderDoctorLevel(check.Level),
			check.Name,
			check.Detail,
		})
	}

	body := ui.RenderTable([]ui.TableColumn{
		{Title: "STATUS", MinWidth: 8, MaxWidth: 10},
		{Title: "CHECK", MinWidth: 20, MaxWidth: 26},
		{Title: "DETAIL", MinWidth: 24, MaxWidth: 80},
	}, rows)
	summary := strings.Join([]string{
		ui.Green(fmt.Sprintf("%d ok", okCount)),
		ui.Yellow(fmt.Sprintf("%d warning(s)", warnCount)),
		ui.Red(fmt.Sprintf("%d error(s)", errorCount)),
	}, " • ")
	return ui.Section("", body, "", summary)
}

func renderDoctorLevel(level doctorLevel) string {
	switch level {
	case doctorOK:
		return ui.Green("✓ OK")
	case doctorWarn:
		return ui.Yellow("! WARN")
	default:
		return ui.Red("✗ ERROR")
	}
}

func runDoctorChecks(repoRoot string) ([]doctorCheck, bool) {
	checks := []doctorCheck{{Level: doctorOK, Name: "Repository", Detail: repoRoot}}
	hasErrors := false

	gitPath := filepath.Join(repoRoot, ".git")
	gitInfo, err := os.Stat(gitPath)
	if err != nil {
		return append(checks, doctorCheck{Level: doctorError, Name: "Repository layout", Detail: err.Error()}), true
	}

	isBareLayout := false
	if gitInfo.IsDir() {
		checks = append(checks, doctorCheck{Level: doctorWarn, Name: "Repository layout", Detail: "standard git layout (.git directory)"})
		if warnings, err := preflightMigrateRepo(repoRoot); err != nil {
			checks = append(checks, doctorCheck{Level: doctorError, Name: "Migration readiness", Detail: err.Error()})
			hasErrors = true
		} else if len(warnings) > 0 {
			for _, warning := range warnings {
				checks = append(checks, doctorCheck{Level: doctorWarn, Name: "Migration readiness", Detail: warning})
			}
		} else {
			checks = append(checks, doctorCheck{Level: doctorOK, Name: "Migration readiness", Detail: "ready for migrate"})
		}
	} else {
		content, readErr := os.ReadFile(gitPath)
		if readErr != nil {
			checks = append(checks, doctorCheck{Level: doctorError, Name: "Repository layout", Detail: readErr.Error()})
			return checks, true
		}
		if strings.Contains(string(content), ".bare") {
			isBareLayout = true
			checks = append(checks, doctorCheck{Level: doctorOK, Name: "Repository layout", Detail: "bare worktree layout (.git file -> .bare)"})
		} else {
			checks = append(checks, doctorCheck{Level: doctorError, Name: "Repository layout", Detail: "unexpected .git file; expected a .bare pointer"})
			hasErrors = true
		}
	}

	if isBareLayout {
		bareDir := filepath.Join(repoRoot, ".bare")
		if info, err := os.Stat(bareDir); err != nil || !info.IsDir() {
			checks = append(checks, doctorCheck{Level: doctorError, Name: ".bare directory", Detail: "missing or not a directory"})
			hasErrors = true
		} else {
			checks = append(checks, doctorCheck{Level: doctorOK, Name: ".bare directory", Detail: bareDir})
		}
	}

	entries, err := worktree.List()
	if err != nil {
		checks = append(checks, doctorCheck{Level: doctorError, Name: "Worktree list", Detail: err.Error()})
		hasErrors = true
	} else {
		checks = append(checks, doctorCheck{Level: doctorOK, Name: "Worktree list", Detail: fmt.Sprintf("%d worktree(s)", len(entries))})
		for _, entry := range entries {
			if _, err := os.Stat(entry.Path); err != nil {
				checks = append(checks, doctorCheck{Level: doctorError, Name: "Worktree path", Detail: fmt.Sprintf("missing path: %s", entry.Path)})
				hasErrors = true
			}
		}
	}

	remote := worktree.DefaultRemote()
	if remote == "" {
		checks = append(checks, doctorCheck{Level: doctorWarn, Name: "Default remote", Detail: "no remote configured"})
	} else {
		checks = append(checks, doctorCheck{Level: doctorOK, Name: "Default remote", Detail: remote})
		defaultBranch := worktree.DefaultBranch(remote)
		if defaultBranch == "" {
			checks = append(checks, doctorCheck{Level: doctorWarn, Name: "Default branch", Detail: fmt.Sprintf("could not determine default branch for %s", remote)})
		} else {
			checks = append(checks, doctorCheck{Level: doctorOK, Name: "Default branch", Detail: defaultBranch})
		}
	}

	return checks, hasErrors
}
