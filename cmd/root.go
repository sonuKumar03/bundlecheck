package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundleradar/internal/analysis"
)

// Numeric exit codes conforming to automation contract
const (
	ExitCodeSuccess         = 0 // Command succeeded, analysis passed, policy satisfied
	ExitCodePolicyViolation = 1 // Explicit budget breached or disallowed package detected
	ExitCodeUsage           = 2 // Invalid flags, syntax, config file, or argument error
	ExitCodeExecution       = 3 // Runtime failure, missing build artifacts, I/O error
)

// PolicyViolationError indicates a budget breach or disallowed package policy violation.
type PolicyViolationError struct {
	Err error
}

func (e *PolicyViolationError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "bundle policy violation"
}

func (e *PolicyViolationError) Unwrap() error {
	return e.Err
}

// UsageError indicates invalid flags, CLI arguments, or user configuration.
type UsageError struct {
	Err error
}

func (e *UsageError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "usage error"
}

func (e *UsageError) Unwrap() error {
	return e.Err
}

// MapErrorToExitCode maps an error to its documented numeric exit code.
func MapErrorToExitCode(err error) int {
	if err == nil {
		return ExitCodeSuccess
	}
	var pErr *PolicyViolationError
	if errors.As(err, &pErr) {
		return ExitCodePolicyViolation
	}
	var uErr *UsageError
	if errors.As(err, &uErr) {
		return ExitCodeUsage
	}

	msg := err.Error()
	if strings.Contains(msg, "unknown flag") ||
		strings.Contains(msg, "unknown shorthand flag") ||
		strings.Contains(msg, "flag needs an argument") ||
		strings.Contains(msg, "invalid argument") ||
		strings.Contains(msg, "accepts ") ||
		strings.Contains(msg, "requires ") ||
		strings.Contains(msg, "required") ||
		strings.Contains(msg, "unsupported format") ||
		strings.Contains(msg, "cannot be empty") ||
		strings.Contains(msg, "invalid --") ||
		strings.Contains(msg, "must be positive") ||
		strings.Contains(msg, "config ") && strings.Contains(msg, "unknown field") ||
		strings.Contains(msg, "ambiguous") {
		return ExitCodeUsage
	}

	if strings.Contains(msg, "budget breached") ||
		strings.Contains(msg, "regression limits breached") ||
		strings.Contains(msg, "policy violation") ||
		strings.Contains(msg, "rule violation") {
		return ExitCodePolicyViolation
	}

	return ExitCodeExecution
}

// NewRootCommand creates and configures the root bundleradar cobra.Command.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "bundleradar",
		Short:   "Summarize, inspect, measure, compare, trace, advise, and check Angular browser JavaScript bundles",
		Version: analysis.ToolVersion,
		Long: `bundleradar is a fast CLI and AI agent skill for Angular esbuild bundle analysis.
It calculates accurate initial vs. lazy JavaScript byte totals, attributes npm package sizes,
estimates Gzip wire transfer sizes, traces dependency import paths, generates optimization
recommendations, tracks baselines across changes, and enforces bundle size budgets in CI.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		summaryCommand(),
		workspaceCommand(),
		compareCommand(),
		baselineCommand(),
		measureCommand(),
		checkCommand(),
		inspectCommand(),
		whyCommand(),
		suggestCommand(),
		benchmarkCommand(),
		mcpCommand(),
		initCommand(),
		newScanCommand(),
		newDiffCommand(),
		newGateCommand(),
	)
	return root
}

func Execute(args []string, stdout, stderr io.Writer) int {
	root := NewRootCommand()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(stderr, "bundleradar: %v\n", err)
		return MapErrorToExitCode(err)
	}
	return ExitCodeSuccess
}
