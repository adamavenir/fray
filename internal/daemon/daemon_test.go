package daemon

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
)

// testHarness provides a temp fray project for integration tests.
type testHarness struct {
	t           *testing.T
	projectDir  string
	projectPath string
	db          *sql.DB
	debouncer   *MentionDebouncer
}

// newTestHarness creates a temp fray project and returns the harness.
func newTestHarness(t *testing.T) *testHarness {
	t.Helper()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	projectDir := t.TempDir()
	frayDir := filepath.Join(projectDir, ".fray")
	if err := os.MkdirAll(frayDir, 0755); err != nil {
		t.Fatalf("mkdir .fray: %v", err)
	}

	// Write minimal config
	configPath := filepath.Join(frayDir, "fray-config.json")
	if err := os.WriteFile(configPath, []byte(`{"channel_id":"ch-test","channel_name":"test"}`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Create empty JSONL files so DiscoverProject finds the project
	for _, name := range []string{"messages.jsonl", "agents.jsonl"} {
		path := filepath.Join(frayDir, name)
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	project, err := core.DiscoverProject(projectDir)
	if err != nil {
		t.Fatalf("discover project: %v", err)
	}

	database, err := db.OpenDatabase(project)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := db.InitSchema(database); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
	})

	return &testHarness{
		t:           t,
		projectDir:  projectDir,
		projectPath: project.DBPath,
		db:          database,
		debouncer:   NewMentionDebouncer(database, project.DBPath),
	}
}

// createAgent creates a test agent.
func (h *testHarness) createAgent(agentID string, managed bool) types.Agent {
	h.t.Helper()

	now := time.Now().Unix()
	agent := types.Agent{
		AgentID:      agentID,
		RegisteredAt: now,
		LastSeen:     now,
		Managed:      managed,
		Presence:     types.PresenceOffline,
	}
	if managed {
		agent.Invoke = &types.InvokeConfig{
			Driver:         "claude",
			PromptDelivery: types.PromptDeliveryStdin,
		}
	}

	if err := db.CreateAgent(h.db, agent); err != nil {
		h.t.Fatalf("create agent %s: %v", agentID, err)
	}

	created, err := db.GetAgent(h.db, agentID)
	if err != nil {
		h.t.Fatalf("get agent %s: %v", agentID, err)
	}
	return *created
}

// postMessage creates a test message.
func (h *testHarness) postMessage(fromAgent, body string, msgType types.MessageType) types.Message {
	h.t.Helper()

	msg := types.Message{
		TS:        time.Now().Unix(),
		FromAgent: fromAgent,
		Body:      body,
		Type:      msgType,
		Home:      "room",
	}

	// Extract mentions
	bases, _ := db.GetAgentBases(h.db)
	msg.Mentions = core.ExtractMentions(body, bases)

	created, err := db.CreateMessage(h.db, msg)
	if err != nil {
		h.t.Fatalf("create message: %v", err)
	}
	return created
}

// postReply creates a reply to an existing message.
func (h *testHarness) postReply(fromAgent, body, replyTo string, msgType types.MessageType) types.Message {
	h.t.Helper()

	msg := types.Message{
		TS:        time.Now().Unix(),
		FromAgent: fromAgent,
		Body:      body,
		Type:      msgType,
		Home:      "room",
		ReplyTo:   &replyTo,
	}

	bases, _ := db.GetAgentBases(h.db)
	msg.Mentions = core.ExtractMentions(body, bases)

	created, err := db.CreateMessage(h.db, msg)
	if err != nil {
		h.t.Fatalf("create reply: %v", err)
	}
	return created
}

// --- Helper Function Tests (Unit-style) ---

func TestIsSelfMention(t *testing.T) {
	tests := []struct {
		name     string
		msgFrom  string
		agentID  string
		expected bool
	}{
		{"self mention", "alice", "alice", true},
		{"different agent", "bob", "alice", false},
		{"sub-agent not self", "alice.1", "alice", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := types.Message{FromAgent: tt.msgFrom}
			result := IsSelfMention(msg, tt.agentID)
			if result != tt.expected {
				t.Errorf("IsSelfMention(%q, %q) = %v, want %v", tt.msgFrom, tt.agentID, result, tt.expected)
			}
		})
	}
}

func TestIsDirectAddress(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		agentID  string
		expected bool
	}{
		// Direct address cases
		{"direct single", "@alice hey", "alice", true},
		{"direct multiple first", "@alice @bob hey", "alice", true},
		{"direct multiple second", "@alice @bob hey", "bob", true},
		{"direct with punctuation", "@alice, what do you think?", "alice", true},
		{"direct subagent", "@alice hey", "alice.1", true},
		{"direct to subagent", "@alice.1 hey", "alice.1", true},
		{"direct parent gets subagent mention", "@alice.1 hey", "alice", true},

		// NOT direct address
		{"mid-sentence mention", "hey @alice what's up", "alice", false},
		{"no @ prefix", "alice hey", "alice", false},
		{"FYI pattern", "FYI @alice this happened", "alice", false},
		{"fyi lowercase", "fyi @alice this happened", "alice", false},
		{"CC pattern", "CC @alice @bob", "alice", false},
		{"cc lowercase", "cc @alice", "alice", false},
		{"heads up pattern", "heads up @alice", "alice", false},
		{"wrong agent", "@bob hey", "alice", false},
		{"mention after text", "check this @alice", "alice", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := types.Message{Body: tt.body}
			result := IsDirectAddress(msg, tt.agentID)
			if result != tt.expected {
				t.Errorf("IsDirectAddress(%q, %q) = %v, want %v", tt.body, tt.agentID, result, tt.expected)
			}
		})
	}
}

func TestMatchesMention(t *testing.T) {
	tests := []struct {
		name     string
		mention  string
		agentID  string
		expected bool
	}{
		{"exact match", "alice", "alice", true},
		{"mention matches subagent", "alice", "alice.1", true},
		{"mention matches deep subagent", "alice", "alice.frontend.1", true},
		{"subagent mention matches parent", "alice.1", "alice", true},
		{"no match different base", "bob", "alice", false},
		{"partial no match", "ali", "alice", false},
		{"different subagent", "alice.2", "alice.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesMention(tt.mention, tt.agentID)
			if result != tt.expected {
				t.Errorf("matchesMention(%q, %q) = %v, want %v", tt.mention, tt.agentID, result, tt.expected)
			}
		})
	}
}

func TestCanTriggerSpawn(t *testing.T) {
	tests := []struct {
		name       string
		msgType    types.MessageType
		fromAgent  string
		threadOwner *string
		expected   bool
	}{
		{"human in room", types.MessageTypeUser, "adam", nil, true},
		{"agent in room", types.MessageTypeAgent, "bob", nil, false},
		{"human in owned thread", types.MessageTypeUser, "adam", strPtr("alice"), true},
		{"owner in own thread", types.MessageTypeAgent, "alice", strPtr("alice"), true},
		{"non-owner agent in thread", types.MessageTypeAgent, "bob", strPtr("alice"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := types.Message{
				Type:      tt.msgType,
				FromAgent: tt.fromAgent,
			}
			var thread *types.Thread
			if tt.threadOwner != nil {
				thread = &types.Thread{OwnerAgent: tt.threadOwner}
			}
			result := CanTriggerSpawn(msg, thread)
			if result != tt.expected {
				t.Errorf("CanTriggerSpawn() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// --- Integration Tests ---

func TestIsReplyToAgent(t *testing.T) {
	h := newTestHarness(t)

	// Create agents
	h.createAgent("alice", true)
	h.createAgent("bob", false)

	// Alice posts a message
	aliceMsg := h.postMessage("alice", "I have a question", types.MessageTypeAgent)

	// Bob replies to Alice
	bobReply := h.postReply("bob", "Here's my answer", aliceMsg.ID, types.MessageTypeUser)

	// Test: bob's reply IS a reply to alice
	if !IsReplyToAgent(h.db, bobReply, "alice") {
		t.Error("expected bob's reply to be detected as reply to alice")
	}

	// Test: bob's reply is NOT a reply to bob
	if IsReplyToAgent(h.db, bobReply, "bob") {
		t.Error("bob's reply should not be detected as reply to bob")
	}

	// Test: original message is NOT a reply to anyone
	if IsReplyToAgent(h.db, aliceMsg, "alice") {
		t.Error("original message should not be a reply")
	}
}

func TestIsReplyToAgent_SubagentMatching(t *testing.T) {
	h := newTestHarness(t)

	// Create agents - alice and alice.1 (subagent)
	h.createAgent("alice", true)
	h.createAgent("alice.1", true)
	h.createAgent("bob", false)

	// alice.1 posts a message
	subagentMsg := h.postMessage("alice.1", "From subagent", types.MessageTypeAgent)

	// Bob replies to alice.1
	bobReply := h.postReply("bob", "Reply to subagent", subagentMsg.ID, types.MessageTypeUser)

	// Parent agent "alice" should get notified of replies to "alice.1"
	if !IsReplyToAgent(h.db, bobReply, "alice") {
		t.Error("parent agent should be notified of replies to subagent")
	}

	// Subagent should also match exactly
	if !IsReplyToAgent(h.db, bobReply, "alice.1") {
		t.Error("subagent should match reply to itself")
	}
}

func TestDebouncer_WatermarkTracking(t *testing.T) {
	h := newTestHarness(t)

	agent := h.createAgent("alice", true)

	// Initial watermark should be empty
	watermark := h.debouncer.GetWatermark(agent.AgentID)
	if watermark != "" {
		t.Errorf("expected empty watermark, got %q", watermark)
	}

	// Post a message
	msg := h.postMessage("bob", "@alice hey", types.MessageTypeUser)

	// Update watermark
	if err := h.debouncer.UpdateWatermark(agent.AgentID, msg.ID); err != nil {
		t.Fatalf("update watermark: %v", err)
	}

	// Verify watermark updated
	watermark = h.debouncer.GetWatermark(agent.AgentID)
	if watermark != msg.ID {
		t.Errorf("expected watermark %q, got %q", msg.ID, watermark)
	}
}

func TestDebouncer_PendingMentions(t *testing.T) {
	h := newTestHarness(t)

	h.createAgent("alice", true)

	// Queue some mentions
	h.debouncer.QueueMention("alice", "msg-1")
	h.debouncer.QueueMention("alice", "msg-2")
	h.debouncer.QueueMention("alice", "msg-1") // Duplicate - should be ignored

	// Check pending count
	if count := h.debouncer.PendingCount("alice"); count != 2 {
		t.Errorf("expected 2 pending, got %d", count)
	}

	// Flush pending
	pending := h.debouncer.FlushPending("alice")
	if len(pending) != 2 {
		t.Errorf("expected 2 flushed, got %d", len(pending))
	}

	// Pending should be empty after flush
	if h.debouncer.HasPending("alice") {
		t.Error("expected no pending after flush")
	}
}

func TestShouldSpawn_PresenceStates(t *testing.T) {
	h := newTestHarness(t)

	tests := []struct {
		name     string
		presence types.PresenceState
		expected bool
	}{
		{"offline spawns", types.PresenceOffline, true},
		{"idle spawns", types.PresenceIdle, true},
		{"empty spawns", "", true},
		{"spawning queues", types.PresenceSpawning, false},
		{"active queues", types.PresenceActive, false},
		{"error does not spawn", types.PresenceError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := types.Agent{
				AgentID:  "alice",
				Presence: tt.presence,
			}
			msg := types.Message{
				FromAgent: "bob", // Not self
				Body:      "@alice hey",
			}
			result := h.debouncer.ShouldSpawn(agent, msg)
			if result != tt.expected {
				t.Errorf("ShouldSpawn(presence=%q) = %v, want %v", tt.presence, result, tt.expected)
			}
		})
	}
}

func TestShouldSpawn_SelfMentionNeverSpawns(t *testing.T) {
	h := newTestHarness(t)

	agent := types.Agent{
		AgentID:  "alice",
		Presence: types.PresenceOffline, // Would normally spawn
	}
	msg := types.Message{
		FromAgent: "alice", // Self mention
		Body:      "@alice reminder to myself",
	}

	if h.debouncer.ShouldSpawn(agent, msg) {
		t.Error("self mention should never spawn")
	}
}

// --- End-to-End Mention Detection Tests ---

func TestMentionDetection_DirectAddressWakes(t *testing.T) {
	h := newTestHarness(t)

	alice := h.createAgent("alice", true)
	h.createAgent("bob", false)

	// Bob directly addresses alice
	msg := h.postMessage("bob", "@alice can you help?", types.MessageTypeUser)

	// Should be detected as direct address
	if !IsDirectAddress(msg, alice.AgentID) {
		t.Error("@alice at start should be direct address")
	}

	// Should trigger spawn (alice is offline)
	if !h.debouncer.ShouldSpawn(alice, msg) {
		t.Error("direct address to offline agent should spawn")
	}
}

func TestMentionDetection_FYIDoesNotWake(t *testing.T) {
	h := newTestHarness(t)

	alice := h.createAgent("alice", true)
	h.createAgent("bob", false)

	// Bob FYIs alice
	msg := h.postMessage("bob", "FYI @alice the deploy is done", types.MessageTypeUser)

	// Should NOT be direct address
	if IsDirectAddress(msg, alice.AgentID) {
		t.Error("FYI pattern should not be direct address")
	}
}

func TestMentionDetection_ChainReplyWakes(t *testing.T) {
	h := newTestHarness(t)

	alice := h.createAgent("alice", true)
	h.createAgent("bob", false)

	// Alice posts something
	aliceMsg := h.postMessage("alice", "What do you think about this approach?", types.MessageTypeAgent)

	// Bob replies (without explicit @mention)
	bobReply := h.postReply("bob", "Looks good to me", aliceMsg.ID, types.MessageTypeUser)

	// Reply should wake alice even without @mention
	if !IsReplyToAgent(h.db, bobReply, alice.AgentID) {
		t.Error("reply to alice's message should wake alice")
	}
}

// Helper
func strPtr(s string) *string {
	return &s
}
