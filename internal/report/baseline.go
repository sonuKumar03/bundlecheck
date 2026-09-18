package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/baseline"
)

// BaselineListText renders a tabular overview of saved baselines.
func BaselineListText(w io.Writer, baselines []baseline.BaselineInfo, activeName string) error {
	if len(baselines) == 0 {
		fmt.Fprintln(w, "No saved baselines found.")
		fmt.Fprintln(w, "Hint: create one with 'bundlecheck baseline save' or 'bundlecheck baseline save --ref <git-branch>'")
		return nil
	}

	var text strings.Builder
	table := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Saved Baselines (.bundlecheck/baselines/):")
	fmt.Fprintln(table, "")
	fmt.Fprintln(table, "  NAME\tREF / SOURCE\tINITIAL JS\tTOTAL JS\tCREATED\tSTATUS")
	fmt.Fprintln(table, "  ────\t────────────\t──────────\t────────\t───────\t──────")

	for _, b := range baselines {
		refStr := b.GitRef
		if refStr == "" {
			refStr = "(local build)"
		} else if b.CommitSHA != "" {
			refStr = fmt.Sprintf("%s (%s)", b.GitRef, b.CommitSHA)
		}

		status := ""
		prefix := "  "
		if b.IsActive {
			status = "✓ active"
			prefix = "* "
		}

		createdStr := formatRelativeTime(b.CreatedAt)

		fmt.Fprintf(table, "%s%s\t%s\t%s\t%s\t%s\t%s\n",
			prefix,
			b.Name,
			refStr,
			formatBytes(b.InitialJS),
			formatBytes(b.TotalJS),
			createdStr,
			status,
		)
	}

	if err := table.Flush(); err != nil {
		return err
	}

	fmt.Fprintln(w, text.String())
	fmt.Fprintln(w, "Run 'bundlecheck baseline use <name>' to switch the active baseline.")
	return nil
}

// BaselineShowText renders the detailed breakdown of a single baseline snapshot.
func BaselineShowText(w io.Writer, bf *baseline.BaselineFile, opts TextOptions) error {
	if bf == nil {
		return fmt.Errorf("cannot render nil baseline")
	}

	fmt.Fprintln(w, "Baseline Snapshot Details")
	if bf.Metadata != nil {
		if bf.Metadata.Name != "" {
			fmt.Fprintf(w, "  Name:       %s\n", bf.Metadata.Name)
		}
		if bf.Metadata.GitRef != "" {
			fmt.Fprintf(w, "  Git Ref:    %s\n", bf.Metadata.GitRef)
		}
		if bf.Metadata.CommitSHA != "" {
			fmt.Fprintf(w, "  Commit:     %s\n", bf.Metadata.CommitSHA)
		}
		if bf.Metadata.BuildCmd != "" {
			fmt.Fprintf(w, "  Build Cmd:  %s\n", bf.Metadata.BuildCmd)
		}
		if !bf.Metadata.CreatedAt.IsZero() {
			fmt.Fprintf(w, "  Captured:   %s\n", bf.Metadata.CreatedAt.Format(time.RFC3339))
		}
		fmt.Fprintln(w)
	}

	res := &analysis.AnalysisResult{
		SchemaVersion: bf.SchemaVersion,
		ToolVersion:   bf.ToolVersion,
		Command:       bf.Command,
		Summary:       bf.Summary,
		Packages:      bf.Packages,
	}

	return TextWithOptions(w, res, opts)
}

func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	diff := time.Since(t)
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(diff.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	if days < 30 {
		return fmt.Sprintf("%d days ago", days)
	}
	return t.Format("2006-01-02")
}
