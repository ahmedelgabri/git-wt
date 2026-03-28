package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/ui"
)

type commandHint struct {
	Action  string
	Command string
}

func renderTableSection(columns []ui.TableColumn, rows [][]string, notes []string, summary string) string {
	return renderTableSectionWithFooter(columns, rows, notes, summary, "")
}

func renderTableSectionWithFooter(columns []ui.TableColumn, rows [][]string, notes []string, summary string, footer string) string {
	parts := []string{ui.RenderTable(columns, rows)}
	if len(notes) > 0 {
		parts = append(parts, "", strings.Join(notes, "\n"))
	}
	if summary != "" {
		parts = append(parts, "", summary)
	}
	if strings.TrimSpace(footer) != "" {
		parts = append(parts, "", footer)
	}
	return ui.Section("", parts...)
}

func renderRepoLayoutSection(rootDir string, branches []treeBranch) string {
	rows := [][]string{
		{ui.Path(layoutPath(rootDir, ".bare")), ui.Subtle("git database")},
		{ui.Path(layoutPath(rootDir, ".git")), ui.Subtle("gitdir pointer")},
	}
	for _, branch := range branches {
		rows = append(rows, []string{
			ui.Path(layoutPath(rootDir, branch.Name)),
			branch.Desc,
		})
	}

	summary := ui.Subtle(fmt.Sprintf("%d worktree(s) ready", len(branches)))
	return renderTableSection([]ui.TableColumn{
		{Title: "PATH", MinWidth: 18, MaxWidth: 48},
		{Title: "ROLE", MinWidth: 16, MaxWidth: 32},
	}, rows, nil, summary)
}

func renderCommandHintsSection(hints []commandHint) string {
	rows := make([][]string, 0, len(hints))
	for _, hint := range hints {
		rows = append(rows, []string{hint.Action, ui.Muted(hint.Command)})
	}

	return renderTableSection([]ui.TableColumn{
		{Title: "NEXT", MinWidth: 18, MaxWidth: 28},
		{Title: "COMMAND", MinWidth: 24},
	}, rows, nil, "")
}

func layoutPath(rootDir, entry string) string {
	if rootDir == "." {
		return "." + string(filepath.Separator) + entry
	}
	return filepath.Join(rootDir, entry)
}
