package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
)

func TestInitJoinFlowRegistersDescriptorAgents(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	projectDir := t.TempDir()
	frayDir := filepath.Join(projectDir, ".fray")

	existingMachine := "remote"
	if existingMachine == defaultMachineID() {
		existingMachine = "remote-1"
	}
	machineDir := filepath.Join(frayDir, "shared", "machines", existingMachine)
	if err := os.MkdirAll(machineDir, 0o755); err != nil {
		t.Fatalf("mkdir shared machine: %v", err)
	}

	descriptors := []db.AgentDescriptorJSONLRecord{
		{Type: "agent_descriptor", AgentID: "alice", Capabilities: []string{"code"}, TS: 10},
		{Type: "agent_descriptor", AgentID: "bob", Capabilities: []string{"review"}, TS: 20},
	}
	var lines []string
	for _, descriptor := range descriptors {
		data, _ := json.Marshal(descriptor)
		lines = append(lines, string(data))
	}
	if err := os.WriteFile(filepath.Join(machineDir, "agent-state.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write agent-state: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frayDir, "shared", ".v2"), []byte(""), 0o644); err != nil {
		t.Fatalf("write v2 sentinel: %v", err)
	}

	if _, err := db.UpdateProjectConfig(projectDir, db.ProjectConfig{
		StorageVersion: 2,
		ChannelID:      "ch-join",
		ChannelName:    "join-test",
	}); err != nil {
		t.Fatalf("update config: %v", err)
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
	if _, err := executeCommand(cmd, "init", "--defaults"); err != nil {
		t.Fatalf("init join flow: %v", err)
	}

	machineIDPath := filepath.Join(frayDir, "local", "machine-id")
	data, err := os.ReadFile(machineIDPath)
	if err != nil {
		t.Fatalf("read machine-id: %v", err)
	}
	var machineRecord struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &machineRecord); err != nil {
		t.Fatalf("decode machine-id: %v", err)
	}
	if machineRecord.ID != defaultMachineID() {
		t.Fatalf("expected machine id %s, got %s", defaultMachineID(), machineRecord.ID)
	}

	agents, err := db.ReadAgents(projectDir)
	if err != nil {
		t.Fatalf("read agents: %v", err)
	}
	found := map[string]bool{}
	for _, agent := range agents {
		found[agent.AgentID] = true
	}
	if !found["alice"] || !found["bob"] {
		t.Fatalf("expected alice and bob in runtime, got %#v", found)
	}

	project, err := core.DiscoverProject(projectDir)
	if err != nil {
		t.Fatalf("discover project: %v", err)
	}
	dbConn, err := db.OpenDatabase(project)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer dbConn.Close()

	agent, err := db.GetAgent(dbConn, "alice")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if agent == nil {
		t.Fatalf("expected alice in db")
	}
}
