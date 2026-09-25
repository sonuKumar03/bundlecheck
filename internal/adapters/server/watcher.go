package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

// StartWatcher starts background polling on statsPath to detect file changes on disk.
func (s *Server) StartWatcher() {
	if s.statsPath == "" {
		return
	}

	s.mu.Lock()
	if s.stopWatcher != nil {
		s.mu.Unlock()
		return
	}
	s.stopWatcher = make(chan struct{})
	stopCh := s.stopWatcher
	s.mu.Unlock()

	var lastMod time.Time
	var lastSize int64

	if fi, err := os.Stat(s.statsPath); err == nil {
		lastMod = fi.ModTime()
		lastSize = fi.Size()
	}

	go func() {
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				fi, err := os.Stat(s.statsPath)
				if err != nil {
					continue
				}

				if fi.ModTime() != lastMod || fi.Size() != lastSize {
					lastMod = fi.ModTime()
					lastSize = fi.Size()

					// Debounce to allow full write/flush by build process
					time.Sleep(150 * time.Millisecond)

					// Retry parsing in case file was partially written
					var scanned *core.Bundle
					var scanErr error
					for attempt := 0; attempt < 3; attempt++ {
						scanned, scanErr = s.client.Scan(context.Background(), bundleradar.ScanOptions{
							StatsPath: s.statsPath,
						})
						if scanErr == nil {
							break
						}
						time.Sleep(100 * time.Millisecond)
					}

					if scanErr == nil && scanned != nil {
						s.SetBundle(scanned)
						label := fmt.Sprintf("Build #%d", s.CheckpointCount()+1)
						s.RecordCheckpoint(scanned, label)
					}
				}
			}
		}
	}()
}

// StopWatcher stops the background file watcher if active.
func (s *Server) StopWatcher() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopWatcher != nil {
		close(s.stopWatcher)
		s.stopWatcher = nil
	}
}
