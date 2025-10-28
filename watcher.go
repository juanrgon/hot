package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Watcher struct {
	config      *Config
	lastModTime map[string]time.Time
}

func NewWatcher(config *Config) *Watcher {
	return &Watcher{
		config:      config,
		lastModTime: make(map[string]time.Time),
	}
}

func (w *Watcher) Watch(ctx context.Context, eventCh chan<- string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.checkFiles(eventCh)
		}
	}
}

func (w *Watcher) checkFiles(eventCh chan<- string) {
	for _, dir := range w.config.WatchDirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// Skip excluded directories
			if info.IsDir() {
				baseName := filepath.Base(path)
				for _, exclude := range w.config.ExcludeDirs {
					if baseName == exclude {
						return filepath.SkipDir
					}
				}
				return nil
			}

			// Check if file has watched extension
			hasWatchedExt := false
			for _, ext := range w.config.WatchExts {
				if strings.HasSuffix(path, ext) {
					hasWatchedExt = true
					break
				}
			}

			if !hasWatchedExt {
				return nil
			}

			// Check if file was modified
			modTime := info.ModTime()
			lastMod, exists := w.lastModTime[path]

			if !exists {
				// First time seeing this file
				w.lastModTime[path] = modTime
			} else if modTime.After(lastMod) {
				// File was modified
				w.lastModTime[path] = modTime
				select {
				case eventCh <- path:
				default:
				}
			}

			return nil
		})
	}
}
