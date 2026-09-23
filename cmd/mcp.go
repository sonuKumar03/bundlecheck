package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundleradar/internal/mcp"
)

func mcpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run BundleRadar as a Model Context Protocol (MCP) server over standard I/O",
		Long: `Start a Model Context Protocol (MCP) server over stdio for AI coding assistants (Claude Code, Antigravity, Cursor, etc.).
Provides bundle inspection tools (bundle_summary, bundle_why, bundle_suggest, bundle_check, bundle_measure, workspace_summary)
and optimization rules resource.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s := mcp.NewServer()
			stdioServer := server.NewStdioServer(s)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if err := stdioServer.Listen(ctx, os.Stdin, os.Stdout); err != nil {
				return fmt.Errorf("mcp server: %w", err)
			}
			return nil
		},
	}
	return cmd
}
