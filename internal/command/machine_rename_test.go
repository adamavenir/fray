package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/adamavenir/fray/internal/db"
)

func TestMachineRenameUpdatesLocalMachine(t *testing.T) {
	projectDir := setupMachinesProject(t)

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
	if _, err := executeCommand(cmd, "machine", "rename", "laptop", "desk"); err != nil {
		t.Fatalf("machine rename: %v", err)
	}

	frayDir := filepath.Join(projectDir, ".fray")
	if _, err := os.Stat(filepath.Join(frayDir, "shared", "machines", "desk")); err != nil {
		t.Fatalf("expected desk machine dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(frayDir, "shared", "machines", "laptop")); err == nil {
		t.Fatalf("expected laptop machine dir to be renamed")
	}

	data, err := os.ReadFile(filepath.Join(frayDir, "local", "machine-id"))
	if err != nil {
		t.Fatalf("read machine-id: %v", err)
	}
	var record struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode machine-id: %v", err)
	}
	if record.ID != "desk" {
		t.Fatalf("expected machine id desk, got %s", record.ID)
	}

	config, err := db.ReadProjectConfig(projectDir)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if config == nil || config.MachineAliases["laptop"] != "desk" {
		t.Fatalf("expected alias laptop -> desk")
	}
}
