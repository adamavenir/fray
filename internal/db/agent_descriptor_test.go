package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adamavenir/fray/internal/types"
)

func TestAppendMessageEmitsAgentDescriptor(t *testing.T) {
	projectDir := t.TempDir()
	if _, err := UpdateProjectConfig(projectDir, ProjectConfig{StorageVersion: 2}); err != nil {
		t.Fatalf("update config: %v", err)
	}

	localDir := filepath.Join(projectDir, ".fray", "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("mkdir local: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "machine-id"), []byte(`{"id":"laptop","seq":0,"created_at":1}`), 0o644); err != nil {
		t.Fatalf("write machine-id: %v", err)
	}

	msg := types.Message{
		ID:        "msg-1",
		TS:        100,
		FromAgent: "alice",
		Body:      "hello",
		Mentions:  []string{},
		Type:      types.MessageTypeAgent,
	}
	if err := AppendMessage(projectDir, msg); err != nil {
		t.Fatalf("append message: %v", err)
	}
	msg.ID = "msg-2"
	if err := AppendMessage(projectDir, msg); err != nil {
		t.Fatalf("append message: %v", err)
	}

	statePath := filepath.Join(projectDir, ".fray", "shared", "machines", "laptop", agentStateFile)
	lines, err := readJSONLLines(statePath)
	if err != nil {
		t.Fatalf("read agent-state: %v", err)
	}
	descriptorCount := 0
	for _, line := range lines {
		raw, typ := parseRawEnvelope(line)
		if raw == nil || typ != "agent_descriptor" {
			continue
		}
		var record struct {
			AgentID string `json:"agent_id"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record.AgentID == "alice" {
			descriptorCount++
		}
	}
	if descriptorCount != 1 {
		t.Fatalf("expected 1 descriptor for alice, got %d", descriptorCount)
	}
}

func TestReadAgentDescriptorsMergedDedupes(t *testing.T) {
	projectDir := t.TempDir()
	if _, err := UpdateProjectConfig(projectDir, ProjectConfig{StorageVersion: 2}); err != nil {
		t.Fatalf("update config: %v", err)
	}

	machineDir := filepath.Join(projectDir, ".fray", "shared", "machines", "laptop")
	if err := os.MkdirAll(machineDir, 0o755); err != nil {
		t.Fatalf("mkdir machine: %v", err)
	}

	first := AgentDescriptorJSONLRecord{
		Type:        "agent_descriptor",
		AgentID:     "alice",
		DisplayName: strPtr("Alice"),
		TS:          10,
	}
	second := AgentDescriptorJSONLRecord{
		Type:        "agent_descriptor",
		AgentID:     "alice",
		DisplayName: strPtr("Alice Updated"),
		TS:          20,
	}
	other := AgentDescriptorJSONLRecord{
		Type:    "agent_descriptor",
		AgentID: "bob",
		TS:      15,
	}
	lines := []AgentDescriptorJSONLRecord{first, second, other}
	var payload []string
	for _, record := range lines {
		data, _ := json.Marshal(record)
		payload = append(payload, string(data))
	}
	if err := os.WriteFile(filepath.Join(machineDir, agentStateFile), []byte(strings.Join(payload, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write agent-state: %v", err)
	}

	descriptors, err := ReadAgentDescriptors(projectDir)
	if err != nil {
		t.Fatalf("read descriptors: %v", err)
	}
	if len(descriptors) != 2 {
		t.Fatalf("expected 2 descriptors, got %d", len(descriptors))
	}
	found := false
	for _, descriptor := range descriptors {
		if descriptor.AgentID == "alice" {
			found = true
			if descriptor.DisplayName == nil || *descriptor.DisplayName != "Alice Updated" {
				t.Fatalf("expected latest descriptor, got %#v", descriptor.DisplayName)
			}
		}
	}
	if !found {
		t.Fatalf("expected alice descriptor")
	}
}

func TestRebuildPopulatesAgentDescriptors(t *testing.T) {
	projectDir := t.TempDir()
	if _, err := UpdateProjectConfig(projectDir, ProjectConfig{StorageVersion: 2}); err != nil {
		t.Fatalf("update config: %v", err)
	}

	machineDir := filepath.Join(projectDir, ".fray", "shared", "machines", "laptop")
	if err := os.MkdirAll(machineDir, 0o755); err != nil {
		t.Fatalf("mkdir machine: %v", err)
	}

	descriptor := AgentDescriptorJSONLRecord{
		Type:         "agent_descriptor",
		AgentID:      "alice",
		DisplayName:  strPtr("Alice"),
		Capabilities: []string{"code"},
		TS:           10,
	}
	data, _ := json.Marshal(descriptor)
	if err := os.WriteFile(filepath.Join(machineDir, agentStateFile), append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write agent-state: %v", err)
	}

	dbConn := openTestDB(t)
	if err := RebuildDatabaseFromJSONL(dbConn, projectDir); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	descriptors, err := GetAgentDescriptors(dbConn)
	if err != nil {
		t.Fatalf("get descriptors: %v", err)
	}
	if len(descriptors) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
	}
	if descriptors[0].AgentID != "alice" {
		t.Fatalf("expected alice descriptor, got %#v", descriptors[0])
	}
	if descriptors[0].DisplayName == nil || *descriptors[0].DisplayName != "Alice" {
		t.Fatalf("expected display name Alice, got %#v", descriptors[0].DisplayName)
	}
	if len(descriptors[0].Capabilities) != 1 || descriptors[0].Capabilities[0] != "code" {
		t.Fatalf("expected capabilities [code], got %#v", descriptors[0].Capabilities)
	}

	agent, err := GetAgent(dbConn, "alice")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if agent == nil {
		t.Fatalf("expected agent record for alice")
	}
}
