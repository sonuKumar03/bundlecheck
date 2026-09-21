package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundlecheck/internal/advisor"
	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/build"
	"github.com/sonuKumar03/bundlecheck/internal/compression"
	"github.com/sonuKumar03/bundlecheck/internal/report"
	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func summaryCommand() *cobra.Command {
	var (
		stats       string
		dist        string
		project     string
		entry       string
		format      string
		output      string
		filter      string
		top         int
		all         bool
		showGzip    bool
		showSuggest bool
	)

	c := &cobra.Command{
		Use:   "summary [stats.json] [dist]",
		Short: "Report initial and lazy JS sizes and npm contributors",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) > 0 && stats == "" {
				stats = args[0]
			}
			if len(args) > 1 && dist == "" {
				dist = args[1]
			}

			if format != "text" && format != "json" && !report.IsMarkdownFormat(format) {
				return fmt.Errorf("unsupported format %q: use text, json, or markdown", format)
			}

			sFile, dDir, err := resolveBuildArtifacts(stats, dist, project)
			if err != nil {
				return err
			}

			needCompression := format == "json" || showGzip
			result, snap, err := runAnalysisWithEntry(sFile, dDir, entry, needCompression)
			if err != nil {
				return err
			}

			getAdvisorResult := func() (*advisor.AdvisorResult, error) {
				if entry != "" {
					return advisor.AnalyzeWithEntry(snap, advisor.AdvisorOptions{}, entry)
				}
				return advisor.Analyze(snap, advisor.AdvisorOptions{}), nil
			}

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() {
				_ = cleanup()
			}()

			if format == "json" {
				if showSuggest {
					advisorRes, err := getAdvisorResult()
					if err != nil {
						return err
					}
					combined := struct {
						*analysis.AnalysisResult
						Suggestions []advisor.Suggestion `json:"suggestions"`
					}{
						AnalysisResult: result,
						Suggestions:    advisorRes.Suggestions,
					}
					return report.JSON(w, combined)
				}
				return report.JSON(w, result)
			}

			opts := report.TextOptions{
				Top:    top,
				Filter: filter,
				All:    all,
				Gzip:   showGzip,
			}

			if report.IsMarkdownFormat(format) {
				if err := report.SummaryMarkdown(w, result, opts); err != nil {
					return err
				}
				if showSuggest {
					fmt.Fprintln(w)
					advisorRes, err := getAdvisorResult()
					if err != nil {
						return err
					}
					return report.SuggestMarkdown(w, advisorRes, showGzip)
				}
				return nil
			}

			if err := report.TextWithOptions(w, result, opts); err != nil {
				return err
			}

			if showSuggest {
				fmt.Fprintln(w)
				advisorRes, err := getAdvisorResult()
				if err != nil {
					return err
				}
				return report.SuggestText(w, advisorRes, showGzip)
			}

			return nil
		},
	}

	c.Flags().StringVarP(&stats, "stats", "s", "", "Path to Angular/esbuild stats.json (auto-detected if omitted)")
	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html (auto-detected if omitted)")
	c.Flags().StringVarP(&project, "project", "p", "", "Project name for multi-project workspaces when auto-detecting")
	c.Flags().StringVarP(&entry, "entry", "e", "", "Scope analysis to a specific entrypoint file or chunk name")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json, or markdown")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to specified file path instead of stdout")
	c.Flags().IntVar(&top, "top", 10, "Number of top packages to display in text mode")
	c.Flags().StringVar(&filter, "filter", "", "Filter packages by name substring in text mode")
	c.Flags().BoolVar(&all, "all", false, "Display all packages in text mode")
	c.Flags().BoolVarP(&showGzip, "gzip", "g", false, "Display estimated Gzip wire transfer sizes")
	c.Flags().BoolVar(&showSuggest, "suggest", false, "Include actionable optimization recommendations")

	return c
}

func resolveBuildArtifacts(stats, dist, project string) (string, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("get working directory: %w", err)
	}
	return resolveBuildArtifactsInDir(wd, stats, dist, project)
}

func runAnalysis(stats, dist string) (*analysis.AnalysisResult, error) {
	res, _, err := runAnalysisWithOptions(stats, dist, true)
	return res, err
}

func runAnalysisWithOptions(stats, dist string, withCompression bool) (*analysis.AnalysisResult, *snapshot.BundleSnapshot, error) {
	return runAnalysisWithEntry(stats, dist, "", withCompression)
}

func runAnalysisWithEntry(stats, dist, entry string, withCompression bool) (*analysis.AnalysisResult, *snapshot.BundleSnapshot, error) {
	var (
		s   *snapshot.BundleSnapshot
		err error
	)
	if entry != "" {
		s, err = build.LoadWithEntry(stats, dist, entry)
	} else {
		s, err = build.Load(stats, dist)
	}
	if err != nil {
		return nil, nil, err
	}
	result, err := analysis.Analyze(s)
	if err != nil {
		return nil, nil, err
	}
	if withCompression {
		compression.AttachCompression(s, dist)
		result.Summary = s.Totals
		result.Packages = s.Packages
	}
	return result, s, nil
}

func getOutputWriter(c *cobra.Command, output string) (io.Writer, func() error, error) {
	if output == "" {
		return c.OutOrStdout(), func() error { return nil }, nil
	}
	dir := filepath.Dir(output)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, nil, fmt.Errorf("create directory %q: %w", dir, err)
		}
	}
	f, err := os.Create(output)
	if err != nil {
		return nil, nil, fmt.Errorf("create output file %q: %w", output, err)
	}
	return f, f.Close, nil
}
