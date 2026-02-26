package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetup_CreatesLogFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	cleanup, err := Setup(false)
	if err != nil {
		t.Fatalf("Setup() error: %v", err)
	}
	defer cleanup()

	// Write a log line so there's content
	slog.Info("test message")

	today := time.Now().Format("2006-01-02")
	logPath := filepath.Join(tmp, appName, logSubdir, appName+"-"+today+".log")

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("expected log file at %s: %v", logPath, err)
	}
	if info.Size() == 0 {
		t.Error("log file is empty, expected content")
	}
}

func TestSetup_AppendsToExistingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	cleanup1, err := Setup(false)
	if err != nil {
		t.Fatalf("Setup() first call error: %v", err)
	}
	slog.Info("first message")
	cleanup1()

	cleanup2, err := Setup(false)
	if err != nil {
		t.Fatalf("Setup() second call error: %v", err)
	}
	slog.Info("second message")
	cleanup2()

	today := time.Now().Format("2006-01-02")
	logPath := filepath.Join(tmp, appName, logSubdir, appName+"-"+today+".log")

	content, err := os.ReadFile(logPath) //nolint:gosec // test path is deterministic from TempDir + fixed filename
	if err != nil {
		t.Fatal(err)
	}

	s := string(content)
	if !contains(s, "first message") || !contains(s, "second message") {
		t.Errorf("log file should contain both messages, got:\n%s", s)
	}
}

func TestLogDir_RespectsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/state")
	dir, err := logDir()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join("/custom/state", appName, logSubdir)
	if dir != expected {
		t.Errorf("logDir() = %q, want %q", dir, expected)
	}
}

func TestLogDir_DefaultsToHomeLocalState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	dir, err := logDir()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "state", appName, logSubdir)
	if dir != expected {
		t.Errorf("logDir() = %q, want %q", dir, expected)
	}
}

func TestPruneOldLogs(t *testing.T) {
	tmp := t.TempDir()

	// Create an old log file (10 days ago)
	old := time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	oldFile := filepath.Join(tmp, appName+"-"+old+".log")
	if err := os.WriteFile(oldFile, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create a recent log file (1 day ago)
	recent := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	recentFile := filepath.Join(tmp, appName+"-"+recent+".log")
	if err := os.WriteFile(recentFile, []byte("recent"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create a non-log file that should be ignored
	otherFile := filepath.Join(tmp, "other.txt")
	if err := os.WriteFile(otherFile, []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}

	pruneOldLogs(tmp)

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("expected old log file to be pruned")
	}
	if _, err := os.Stat(recentFile); err != nil {
		t.Error("expected recent log file to be kept")
	}
	if _, err := os.Stat(otherFile); err != nil {
		t.Error("expected non-log file to be kept")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
