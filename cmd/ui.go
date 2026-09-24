package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

var browserLauncher = func(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // Linux / BSD
		c = exec.Command("xdg-open", url)
	}
	return c.Start()
}

func newUICommand() *cobra.Command {
	var (
		port  int
		host  string
		open  bool
		watch bool
	)

	cmd := &cobra.Command{
		Use:   "ui [stats.json]",
		Short: "Launch the interactive BundleRadar Studio web UI",
		Long: `Start an interactive local web studio to visually inspect bundle composition,
trace package bloat with directed BFS ingress chains, and monitor size metrics.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			statsPath := ""
			if len(args) > 0 {
				statsPath = args[0]
				if _, err := os.Stat(statsPath); os.IsNotExist(err) {
					return &UsageError{Err: fmt.Errorf("stats file not found: %s", statsPath)}
				}
			} else {
				statsPath = autoDiscoverStatsFile(".")
			}

			_ = watch // reserved for live re-scan watcher
			return runUIServer(host, port, statsPath, open)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 4200, "Port to serve the web UI on")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host address to bind HTTP server to")
	cmd.Flags().BoolVar(&open, "open", true, "Open default browser automatically")
	cmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch stats file for changes")

	return cmd
}

func runUIServer(host string, port int, statsPath string, openBrowser bool) error {
	return runUIServerWithContext(context.Background(), host, port, statsPath, openBrowser)
}

func runUIServerWithContext(parentCtx context.Context, host string, port int, statsPath string, openBrowser bool) error {
	client := bundleradar.New()

	srv, err := server.New(server.Config{
		Host:      host,
		Port:      port,
		StatsPath: statsPath,
		Client:    client,
	})
	if err != nil {
		return &UsageError{Err: fmt.Errorf("failed to initialize studio server: %w", err)}
	}

	if err := srv.Start(); err != nil {
		return &UsageError{Err: fmt.Errorf("failed to start studio server: %w", err)}
	}
	defer func() { _ = srv.Close() }()

	url := srv.URL()
	fmt.Printf("\n⚡ BundleRadar Studio running at: %s\n", url)
	if statsPath != "" {
		fmt.Printf("   Active stats file: %s\n", statsPath)
	} else {
		fmt.Printf("   No stats.json found in current directory. Studio running in workbench mode.\n")
	}
	fmt.Println("   Press Ctrl+C to stop.")

	if openBrowser {
		openInBrowser(url)
	}

	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, stop := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	fmt.Println("\nStopping BundleRadar Studio...")
	return nil
}

func autoDiscoverStatsFile(startDir string) string {
	candidates := []string{
		"stats.json",
		"dist/stats.json",
		"build/stats.json",
		"apps/portal/dist/stats.json",
	}

	for _, c := range candidates {
		p := filepath.Join(startDir, c)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func openInBrowser(url string) {
	// 0.0.0.0 is non-routable in browser address bars on macOS/Safari/Windows; rewrite to 127.0.0.1
	targetURL := strings.Replace(url, "://0.0.0.0:", "://127.0.0.1:", 1)
	_ = browserLauncher(targetURL)
}
