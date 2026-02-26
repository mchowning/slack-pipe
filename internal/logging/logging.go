// Package logging configures structured logging to file and optionally stderr.
//
// Log files are written to $XDG_STATE_HOME/slack-pipe/logs/ (defaulting to
// ~/.local/state/slack-pipe/logs/) with one file per day. Files older than
// the configured retention period are pruned on startup.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	appName        = "slack-pipe"
	logSubdir      = "logs"
	retentionDays  = 7
	dirPermissions = 0o700
)

// Setup configures slog to write debug-level logs to a daily log file, and
// optionally to stderr when verbose is true. It returns a cleanup function
// that should be deferred by the caller to flush and close the log file.
func Setup(verbose bool) (cleanup func(), err error) {
	logDir, err := logDir()
	if err != nil {
		return nil, fmt.Errorf("determine log directory: %w", err)
	}

	if err := os.MkdirAll(logDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("create log directory %s: %w", logDir, err)
	}

	pruneOldLogs(logDir)

	filename := fmt.Sprintf("%s-%s.log", appName, time.Now().Format("2006-01-02"))
	logPath := filepath.Join(logDir, filename)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // logPath is derived from internal app paths, not user input
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", logPath, err)
	}

	var writers []io.Writer
	writers = append(writers, f)
	if verbose {
		writers = append(writers, os.Stderr)
	}

	w := io.MultiWriter(writers...)
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	cleanup = func() {
		_ = f.Close()
	}
	return cleanup, nil
}

// logDir returns the directory for log files, respecting XDG_STATE_HOME.
func logDir() (string, error) {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, appName, logSubdir), nil
}

// pruneOldLogs removes log files older than retentionDays.
func pruneOldLogs(dir string) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	prefix := appName + "-"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // best-effort
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		// Parse date from filename: slack-pipe-2006-01-02.log
		dateStr := strings.TrimPrefix(name, prefix)
		dateStr = strings.TrimSuffix(dateStr, ".log")
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}
