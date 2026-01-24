package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
)

func setupSyncProject(t *testing.T, channelName string) string {
	t.Helper()

	projectDir := t.TempDir()
	project, err := core.InitProject(projectDir, false)
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	sharedDir := filepath.Join(projectDir, ".fray", "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("mkdir shared: %v", err)
	}

	if _, err := db.UpdateProjectConfig(projectDir, db.ProjectConfig{
		StorageVersion: 2,
		ChannelID:      "ch-sync",
		ChannelName:    channelName,
	}); err != nil {
		t.Fatalf("update config: %v", err)
	}

	dbConn, err := db.OpenDatabase(project)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.InitSchema(dbConn); err != nil {
		_ = dbConn.Close()
		t.Fatalf("init schema: %v", err)
	}
	_ = dbConn.Close()

	return projectDir
}

func TestSyncSetupPathCreatesSymlink(t *testing.T) {
	projectDir := setupSyncProject(t, "sync-path")
	frayDir := filepath.Join(projectDir, ".fray")

	markerPath := filepath.Join(frayDir, "shared", "marker.txt")
	if err := os.WriteFile(markerPath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	baseDir := filepath.Join(t.TempDir(), "sync-root")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	cmd := NewRootCmd("test")
	if _, err := executeCommand(cmd, "sync", "setup", "--path", baseDir); err != nil {
		t.Fatalf("sync setup: %v", err)
	}

	expectedTarget := filepath.Join(baseDir, "sync-path", "shared")
	info, err := os.Lstat(filepath.Join(frayDir, "shared"))
	if err != nil {
		t.Fatalf("stat shared: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected shared to be symlink")
	}
	link, err := os.Readlink(filepath.Join(frayDir, "shared"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if filepath.Clean(link) != filepath.Clean(expectedTarget) {
		t.Fatalf("expected link target %s, got %s", expectedTarget, link)
	}
	if _, err := os.Stat(filepath.Join(expectedTarget, "marker.txt")); err != nil {
		t.Fatalf("expected marker moved to target: %v", err)
	}
}

func TestSyncSetupICloudCreatesSymlink(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	projectDir := setupSyncProject(t, "sync-icloud")
	frayDir := filepath.Join(projectDir, ".fray")

	if err := os.WriteFile(filepath.Join(frayDir, "shared", "marker.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	cmd := NewRootCmd("test")
	if _, err := executeCommand(cmd, "sync", "setup", "--icloud"); err != nil {
		t.Fatalf("sync setup icloud: %v", err)
	}

	expectedTarget := filepath.Join(homeDir, "Library", "Mobile Documents", "com~apple~CloudDocs", "fray-sync", "sync-icloud", "shared")
	link, err := os.Readlink(filepath.Join(frayDir, "shared"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if filepath.Clean(link) != filepath.Clean(expectedTarget) {
		t.Fatalf("expected link target %s, got %s", expectedTarget, link)
	}
	if _, err := os.Stat(filepath.Join(expectedTarget, "marker.txt")); err != nil {
		t.Fatalf("expected marker moved to target: %v", err)
	}
}

func TestSyncStatusShowsConfiguration(t *testing.T) {
	projectDir := setupSyncProject(t, "sync-status")

	config := &db.ProjectSyncConfig{Backend: "path", Path: "/tmp/sync/shared"}
	if _, err := db.UpdateProjectConfig(projectDir, db.ProjectConfig{Sync: config}); err != nil {
		t.Fatalf("update sync config: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	cmd := NewRootCmd("test")
	output, err := executeCommand(cmd, "--json", "sync", "status")
	if err != nil {
		t.Fatalf("sync status: %v", err)
	}
	var result syncStatusResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !result.Configured || result.Backend != "path" || result.Path != "/tmp/sync/shared" {
		t.Fatalf("unexpected status: %#v", result)
	}
}
