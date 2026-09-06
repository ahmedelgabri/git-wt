package cmd

import (
	"context"
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
	Use:   "doctor",
	Short: "Run diagnostics on the current repository layout",
	Long: `Run repository diagnostics for both standard and bare worktree layouts.

Doctor checks repository layout, the .bare directory, linked worktree paths,
default remote and branch detection, and migration readiness for standard
repositories.`,
	Example:       `  git wt doctor`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := doctorRepoRoot()
		if err != nil {
			return err
		}

		if ui.StdoutTTY() {
			return runDoctorAsync(repoRoot)
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
	commonDir, err := git.QueryPath("rev-parse", "--git-common-dir")
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
		repoRoot, err := git.QueryPath("rev-parse", "--show-toplevel")
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
	return renderDoctorChecksWithFooter(checks, "")
}

func renderDoctorChecksWithFooter(checks []doctorCheck, footer string) string {
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

	summary := strings.Join([]string{
		ui.Subtle(fmt.Sprintf("%d check(s)", len(checks))),
		ui.Green(fmt.Sprintf("%d ok", okCount)),
		ui.Yellow(fmt.Sprintf("%d warning(s)", warnCount)),
		ui.Red(fmt.Sprintf("%d error(s)", errorCount)),
	}, " • ")
	return renderTableSectionWithFooter([]ui.TableColumn{
		{Title: "STATUS", MinWidth: 8, MaxWidth: 10},
		{Title: "CHECK", MinWidth: 20, MaxWidth: 26},
		{Title: "DETAIL", MinWidth: 24, MaxWidth: 80},
	}, rows, nil, summary, footer)
}

func renderDoctorLevel(level doctorLevel) string {
	switch level {
	case doctorOK:
		return ui.Green("✓")
	case doctorWarn:
		return ui.Yellow("! WARN")
	default:
		return ui.Red("✗ ERROR")
	}
}

func runDoctorChecks(repoRoot string) ([]doctorCheck, bool) {
	checks := make([]doctorCheck, 0, 8)
	hasErrors := walkDoctorChecks(context.Background(), repoRoot, func(check doctorCheck) {
		checks = append(checks, check)
	})
	return checks, hasErrors
}

func walkDoctorChecks(ctx context.Context, repoRoot string, emit func(doctorCheck)) bool {
	hasErrors := false
	emitCheck := func(check doctorCheck) {
		emit(check)
		if check.Level == doctorError {
			hasErrors = true
		}
	}
	checkCanceled := func() bool {
		return ctx != nil && ctx.Err() != nil
	}

	emitCheck(doctorCheck{Level: doctorOK, Name: "Repository", Detail: repoRoot})
	if checkCanceled() {
		return hasErrors
	}

	gitPath := filepath.Join(repoRoot, ".git")
	gitInfo, err := os.Stat(gitPath)
	if err != nil {
		emitCheck(doctorCheck{Level: doctorError, Name: "Repository layout", Detail: err.Error()})
		return true
	}

	isBareLayout := false
	if gitInfo.IsDir() {
		emitCheck(doctorCheck{Level: doctorWarn, Name: "Repository layout", Detail: "standard git layout (.git directory)"})
		if err := preflightMigrateRepo(repoRoot); err != nil {
			emitCheck(doctorCheck{Level: doctorError, Name: "Migration readiness", Detail: err.Error()})
		} else {
			emitCheck(doctorCheck{Level: doctorOK, Name: "Migration readiness", Detail: "ready for migrate"})
		}
	} else {
		content, readErr := os.ReadFile(gitPath)
		if readErr != nil {
			emitCheck(doctorCheck{Level: doctorError, Name: "Repository layout", Detail: readErr.Error()})
			return true
		}
		if strings.Contains(string(content), ".bare") {
			isBareLayout = true
			emitCheck(doctorCheck{Level: doctorOK, Name: "Repository layout", Detail: "bare worktree layout (.git file -> .bare)"})
		} else {
			emitCheck(doctorCheck{Level: doctorError, Name: "Repository layout", Detail: "unexpected .git file; expected a .bare pointer"})
		}
	}
	if checkCanceled() {
		return hasErrors
	}

	if isBareLayout {
		bareDir := filepath.Join(repoRoot, ".bare")
		if info, err := os.Stat(bareDir); err != nil || !info.IsDir() {
			emitCheck(doctorCheck{Level: doctorError, Name: ".bare directory", Detail: "missing or not a directory"})
		} else {
			emitCheck(doctorCheck{Level: doctorOK, Name: ".bare directory", Detail: bareDir})
		}
	}
	if checkCanceled() {
		return hasErrors
	}

	entries, err := worktree.ListContext(ctx)
	if err != nil {
		emitCheck(doctorCheck{Level: doctorError, Name: "Worktree list", Detail: err.Error()})
	} else {
		emitCheck(doctorCheck{Level: doctorOK, Name: "Worktree list", Detail: fmt.Sprintf("%d worktree(s)", len(entries))})
		for _, entry := range entries {
			if _, err := os.Stat(entry.Path); err != nil {
				emitCheck(doctorCheck{Level: doctorError, Name: "Worktree path", Detail: fmt.Sprintf("missing path: %s", entry.Path)})
			}
		}
	}
	if checkCanceled() {
		return hasErrors
	}

	remote := worktree.DefaultRemoteInContext(ctx, repoRoot)
	if remote == "" {
		emitCheck(doctorCheck{Level: doctorWarn, Name: "Default remote", Detail: "no remote configured"})
	} else {
		emitCheck(doctorCheck{Level: doctorOK, Name: "Default remote", Detail: remote})
		defaultBranch := worktree.DefaultBranchInContext(ctx, repoRoot, remote)
		if defaultBranch == "" {
			emitCheck(doctorCheck{Level: doctorWarn, Name: "Default branch", Detail: fmt.Sprintf("could not determine default branch for %s", remote)})
		} else {
			emitCheck(doctorCheck{Level: doctorOK, Name: "Default branch", Detail: defaultBranch})
		}
	}

	return hasErrors
}
