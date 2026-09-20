package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundlecheck/internal/analysis"
	"github.com/sonuKumar03/bundlecheck/internal/baseline"
	"github.com/sonuKumar03/bundlecheck/internal/discovery"
	"github.com/sonuKumar03/bundlecheck/internal/report"
	"github.com/sonuKumar03/bundlecheck/internal/worktree"
)

func baselineCommand() *cobra.Command {
	var (
		stats    string
		dist     string
		project  string
		format   string
		output   string
		ref      string
		fromGit  string
		name     string
		buildCmd string
		noBuild  bool
	)

	runSave := func(c *cobra.Command, args []string) error {
		if len(args) > 0 && name == "" {
			name = args[0]
		}
		if ref == "" && fromGit != "" {
			ref = fromGit
		}
		if format != "text" && format != "json" {
			return fmt.Errorf("unsupported format %q: use text or json", format)
		}

		var result *analysis.AnalysisResult
		meta := &baseline.Metadata{
			Name:      name,
			GitRef:    ref,
			BuildCmd:  buildCmd,
			Project:   project,
			CreatedAt: time.Now().UTC(),
		}

		if ref != "" {
			// Worktree-based build from specified git ref
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			if !worktree.IsGitRepo(wd) {
				return fmt.Errorf("--ref requires the current directory to be inside a git repository")
			}

			commitSHA, _ := worktree.GetCommitSHA(wd, ref)
			meta.CommitSHA = commitSHA

			if format != "json" {
				fmt.Fprintf(c.OutOrStdout(), "Checking out git ref %q in temporary worktree...\n", ref)
			}

			wtDir, cleanup, err := worktree.Create(wd, ref)
			if err != nil {
				return fmt.Errorf("create worktree for %q: %w", ref, err)
			}
			defer cleanup()

			if !noBuild {
				if format != "json" {
					cmdToRun := buildCmd
					if cmdToRun == "" {
						cmdToRun = "npm run build"
					}
					fmt.Fprintf(c.OutOrStdout(), "Building %q in worktree (%s)...\n", ref, cmdToRun)
				}

				if err := worktree.RunBuild(wtDir, buildCmd); err != nil {
					return err
				}
			}

			sFile, dDir, err := resolveBuildArtifactsInDir(wtDir, stats, dist, project)
			if err != nil {
				return fmt.Errorf("locate build artifacts in worktree: %w", err)
			}

			result, err = runAnalysis(sFile, dDir)
			if err != nil {
				return fmt.Errorf("analyze worktree build: %w", err)
			}
		} else {
			// Local build analysis
			sFile, dDir, err := resolveBuildArtifacts(stats, dist, project)
			if err != nil {
				return err
			}

			result, err = runAnalysis(sFile, dDir)
			if err != nil {
				return err
			}

			// Capture local git commit if available
			if wd, err := os.Getwd(); err == nil && worktree.IsGitRepo(wd) {
				if sha, err := worktree.GetCommitSHA(wd, "HEAD"); err == nil {
					meta.CommitSHA = sha
				}
			}
		}

		var targetFile string
		if output != "" && output != baseline.DefaultBaselineFilename {
			targetFile = baseline.ResolvePath(output)
			if err := baseline.SaveWithMetadata(targetFile, result, meta); err != nil {
				return fmt.Errorf("save baseline: %w", err)
			}
		} else {
			baseName := name
			if baseName == "" {
				if ref != "" {
					baseName = sanitizeBaselineName(ref)
				} else {
					baseName = "default"
				}
			}
			savedPath, err := baseline.SaveNamed(baseline.DefaultDir, baseName, result, meta)
			if err != nil {
				return fmt.Errorf("save baseline: %w", err)
			}
			targetFile = savedPath
		}

		if format == "json" {
			return report.JSON(c.OutOrStdout(), result)
		}

		fmt.Fprintf(c.OutOrStdout(), "Successfully captured and saved baseline to %s\n\n", targetFile)
		return report.Text(c.OutOrStdout(), result)
	}

	c := &cobra.Command{
		Use:   "baseline",
		Short: "Capture, manage, and inspect baseline metrics across branches",
		Long: `Capture, save, list, switch, and rebuild baseline bundle metrics across git branches or local builds.
Baselines are stored in .bundlecheck/baselines/ and compared against during 'bundlecheck measure'.`,
		Args: cobra.NoArgs,
		RunE: runSave,
	}

	c.Flags().StringVarP(&stats, "stats", "s", "", "Path to Angular/esbuild stats.json (auto-detected if omitted)")
	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html (auto-detected if omitted)")
	c.Flags().StringVarP(&project, "project", "p", "", "Project name for multi-project workspaces when auto-detecting")
	c.Flags().StringVarP(&format, "format", "f", "text", "Output format: text or json")
	c.Flags().StringVarP(&output, "output", "o", "", "Custom path to save baseline JSON file")
	c.Flags().StringVarP(&ref, "ref", "r", "", "Git branch, tag, or commit ref to build in an isolated worktree")
	c.Flags().StringVar(&fromGit, "from-git", "", "Git branch, tag, or commit ref to build in an isolated worktree (alias for --ref)")
	c.Flags().StringVarP(&name, "name", "n", "", "Custom identifier name for the baseline snapshot")
	c.Flags().StringVar(&buildCmd, "build-cmd", "", "Custom build command to execute in worktree (default: npm run build)")
	c.Flags().BoolVar(&noBuild, "no-build", false, "Skip executing build command in worktree (use pre-existing artifacts)")

	// Subcommand: save
	saveCmd := &cobra.Command{
		Use:   "save [name]",
		Short: "Capture and save baseline metrics from current build or git branch",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runSave,
	}
	saveCmd.Flags().AddFlagSet(c.Flags())

	// Subcommand: create
	createCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Capture, build, and save a named baseline from current build or git ref",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runSave,
	}
	createCmd.Flags().AddFlagSet(c.Flags())

	// Subcommand: list / ls
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all saved baseline snapshots",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			listFormat, _ := cmd.Flags().GetString("format")
			baselines, active, err := baseline.List(baseline.DefaultDir)
			if err != nil {
				return err
			}

			if listFormat == "json" {
				return report.JSON(cmd.OutOrStdout(), map[string]any{
					"active":    active,
					"baselines": baselines,
				})
			}

			return report.BaselineListText(cmd.OutOrStdout(), baselines, active)
		},
	}
	listCmd.Flags().StringVarP(&format, "format", "f", "text", "Output format: text or json")

	// Subcommand: use
	useCmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Set the active baseline snapshot for 'bundlecheck measure'",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetName := args[0]
			if err := baseline.SetActive(baseline.DefaultDir, targetName); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Switched active baseline to %q\n", targetName)
			return nil
		},
	}

	// Subcommand: rebuild / update
	rebuildCmd := &cobra.Command{
		Use:     "rebuild <name>",
		Aliases: []string{"update"},
		Short:   "Rebuild and update a git-backed baseline snapshot",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetName := args[0]
			snapshotFile, err := baseline.LoadSnapshot(targetName)
			if err != nil {
				return err
			}

			targetRef := ""
			customBuild := buildCmd
			targetProject := project
			if snapshotFile.Metadata != nil {
				targetRef = snapshotFile.Metadata.GitRef
				if customBuild == "" {
					customBuild = snapshotFile.Metadata.BuildCmd
				}
				if targetProject == "" {
					targetProject = snapshotFile.Metadata.Project
				}
			}

			if targetRef == "" {
				return fmt.Errorf("baseline %q is not linked to a git ref and cannot be rebuilt automatically\nHint: capture from current build with 'bundlecheck baseline save --name %s'", targetName, targetName)
			}

			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Rebuilding baseline %q from git ref %q...\n", targetName, targetRef)

			commitSHA, _ := worktree.GetCommitSHA(wd, targetRef)
			wtDir, cleanup, err := worktree.Create(wd, targetRef)
			if err != nil {
				return fmt.Errorf("create worktree for %q: %w", targetRef, err)
			}
			defer cleanup()

			if err := worktree.RunBuild(wtDir, customBuild); err != nil {
				return err
			}

			sFile, dDir, err := resolveBuildArtifactsInDir(wtDir, stats, dist, targetProject)
			if err != nil {
				return fmt.Errorf("locate build artifacts in worktree: %w", err)
			}

			result, err := runAnalysis(sFile, dDir)
			if err != nil {
				return fmt.Errorf("analyze worktree build: %w", err)
			}

			meta := &baseline.Metadata{
				Name:      targetName,
				GitRef:    targetRef,
				CommitSHA: commitSHA,
				BuildCmd:  customBuild,
				Project:   targetProject,
				CreatedAt: time.Now().UTC(),
			}

			savedPath, err := baseline.SaveNamed(baseline.DefaultDir, targetName, result, meta)
			if err != nil {
				return fmt.Errorf("save updated baseline: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated baseline %q at %s\n\n", targetName, savedPath)
			return report.Text(cmd.OutOrStdout(), result)
		},
	}
	rebuildCmd.Flags().StringVar(&buildCmd, "build-cmd", "", "Custom build command override")

	// Subcommand: show
	showCmd := &cobra.Command{
		Use:   "show [name]",
		Short: "Inspect the details and metrics of a saved baseline snapshot",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetName := ""
			if len(args) > 0 {
				targetName = args[0]
			}
			bf, err := baseline.LoadSnapshot(targetName)
			if err != nil {
				return err
			}

			showFormat, _ := cmd.Flags().GetString("format")
			topVal, _ := cmd.Flags().GetInt("top")
			filterVal, _ := cmd.Flags().GetString("filter")
			allVal, _ := cmd.Flags().GetBool("all")

			if showFormat == "json" {
				return report.JSON(cmd.OutOrStdout(), bf)
			}

			opts := report.TextOptions{
				Top:    topVal,
				Filter: filterVal,
				All:    allVal,
			}
			return report.BaselineShowText(cmd.OutOrStdout(), bf, opts)
		},
	}
	var (
		showFormat string
		showTop    int
		showFilter string
		showAll    bool
	)
	showCmd.Flags().StringVarP(&showFormat, "format", "f", "text", "Output format: text or json")
	showCmd.Flags().IntVar(&showTop, "top", 10, "Number of top packages to display")
	showCmd.Flags().StringVar(&showFilter, "filter", "", "Filter packages by name substring")
	showCmd.Flags().BoolVar(&showAll, "all", false, "Display all packages")

	// Subcommand: delete / rm
	deleteCmd := &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a saved baseline snapshot",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetName := args[0]
			if err := baseline.Delete(baseline.DefaultDir, targetName); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted baseline snapshot %q\n", targetName)
			return nil
		},
	}

	c.AddCommand(saveCmd, createCmd, listCmd, useCmd, rebuildCmd, showCmd, deleteCmd)

	return c
}

func resolveBuildArtifactsInDir(rootDir, stats, dist, project string) (string, string, error) {
	return discovery.Resolve(rootDir, stats, dist, project)
}

func sanitizeBaselineName(ref string) string {
	s := strings.TrimPrefix(ref, "origin/")
	s = strings.TrimPrefix(s, "refs/heads/")
	s = strings.TrimPrefix(s, "refs/tags/")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	return s
}
