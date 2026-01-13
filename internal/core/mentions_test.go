package core

import "testing"

func TestExtractMentionsWithBases(t *testing.T) {
	bases := map[string]struct{}{
		"alice": {},
		"bob":   {},
	}

	body := "hey @alice and @bob.1 and email test@test.com @all @unknown"
	mentions := ExtractMentions(body, bases)

	if len(mentions) != 3 {
		t.Fatalf("expected 3 mentions, got %d", len(mentions))
	}
	assertMention(t, mentions, "alice")
	assertMention(t, mentions, "bob.1")
	assertMention(t, mentions, "all")
}

func assertMention(t *testing.T, mentions []string, value string) {
	t.Helper()
	for _, mention := range mentions {
		if mention == value {
			return
		}
	}
	t.Fatalf("expected mention %s", value)
}

func TestExtractMentionsWithWorkerIDs(t *testing.T) {
	bases := map[string]struct{}{
		"dev": {},
		"pm":  {},
	}

	tests := []struct {
		name     string
		body     string
		expected []string
	}{
		{
			name:     "regular agent",
			body:     "hey @dev",
			expected: []string{"dev"},
		},
		{
			name:     "worker ID simple",
			body:     "hey @dev[abc1-0]",
			expected: []string{"dev[abc1-0]"},
		},
		{
			name:     "worker ID with dots",
			body:     "hey @pm.frontend[xyz9-3]",
			expected: []string{"pm.frontend[xyz9-3]"},
		},
		{
			name:     "mixed regular and worker",
			body:     "@dev and @pm[job1-2]",
			expected: []string{"dev", "pm[job1-2]"},
		},
		{
			name:     "multiple workers",
			body:     "@dev[abc1-0] @dev[abc1-1] @dev[abc1-2]",
			expected: []string{"dev[abc1-0]", "dev[abc1-1]", "dev[abc1-2]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mentions := ExtractMentions(tt.body, bases)
			if len(mentions) != len(tt.expected) {
				t.Fatalf("expected %d mentions, got %d: %v", len(tt.expected), len(mentions), mentions)
			}
			for _, exp := range tt.expected {
				assertMention(t, mentions, exp)
			}
		})
	}
}

func TestParseJobWorkerName(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantBase      string
		wantSuffix    string
		wantIdx       int
		wantIsWorker  bool
	}{
		{
			name:         "regular agent",
			input:        "dev",
			wantBase:     "dev",
			wantSuffix:   "",
			wantIdx:      -1,
			wantIsWorker: false,
		},
		{
			name:         "regular agent with dot",
			input:        "pm.frontend",
			wantBase:     "pm.frontend",
			wantSuffix:   "",
			wantIdx:      -1,
			wantIsWorker: false,
		},
		{
			name:         "worker ID simple",
			input:        "dev[abc1-0]",
			wantBase:     "dev",
			wantSuffix:   "abc1",
			wantIdx:      0,
			wantIsWorker: true,
		},
		{
			name:         "worker ID higher index",
			input:        "dev[abc1-3]",
			wantBase:     "dev",
			wantSuffix:   "abc1",
			wantIdx:      3,
			wantIsWorker: true,
		},
		{
			name:         "worker ID with dots",
			input:        "pm.frontend[xyz9-3]",
			wantBase:     "pm.frontend",
			wantSuffix:   "xyz9",
			wantIdx:      3,
			wantIsWorker: true,
		},
		{
			name:         "worker ID double digit index",
			input:        "dev[abcd-12]",
			wantBase:     "dev",
			wantSuffix:   "abcd",
			wantIdx:      12,
			wantIsWorker: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, suffix, idx, isWorker := ParseJobWorkerName(tt.input)
			if base != tt.wantBase {
				t.Errorf("baseAgent: got %q, want %q", base, tt.wantBase)
			}
			if suffix != tt.wantSuffix {
				t.Errorf("jobSuffix: got %q, want %q", suffix, tt.wantSuffix)
			}
			if idx != tt.wantIdx {
				t.Errorf("workerIdx: got %d, want %d", idx, tt.wantIdx)
			}
			if isWorker != tt.wantIsWorker {
				t.Errorf("isWorker: got %v, want %v", isWorker, tt.wantIsWorker)
			}
		})
	}
}

func TestExtractInterruptSyntax(t *testing.T) {
	bases := map[string]struct{}{
		"alice": {},
		"bob":   {},
	}

	tests := []struct {
		name           string
		body           string
		expectAgents   []string
		expectDouble   map[string]bool
		expectNoSpawn  map[string]bool
	}{
		{
			name:         "single interrupt",
			body:         "!@alice need your help",
			expectAgents: []string{"alice"},
			expectDouble: map[string]bool{"alice": false},
			expectNoSpawn: map[string]bool{"alice": false},
		},
		{
			name:         "double interrupt fresh start",
			body:         "!!@bob start fresh",
			expectAgents: []string{"bob"},
			expectDouble: map[string]bool{"bob": true},
			expectNoSpawn: map[string]bool{"bob": false},
		},
		{
			name:         "interrupt no spawn",
			body:         "!@alice! stop now",
			expectAgents: []string{"alice"},
			expectDouble: map[string]bool{"alice": false},
			expectNoSpawn: map[string]bool{"alice": true},
		},
		{
			name:         "double interrupt no spawn",
			body:         "!!@bob! force end",
			expectAgents: []string{"bob"},
			expectDouble: map[string]bool{"bob": true},
			expectNoSpawn: map[string]bool{"bob": true},
		},
		{
			name:         "multiple interrupts",
			body:         "!@alice and !!@bob!",
			expectAgents: []string{"alice", "bob"},
			expectDouble: map[string]bool{"alice": false, "bob": true},
			expectNoSpawn: map[string]bool{"alice": false, "bob": true},
		},
		{
			name:         "mixed regular and interrupt",
			body:         "@alice regular mention and !@bob interrupt",
			expectAgents: []string{"alice", "bob"},
			expectDouble: map[string]bool{"bob": false},
			expectNoSpawn: map[string]bool{"bob": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractMentionsWithSession(tt.body, bases)

			// Check all expected agents are mentioned
			for _, agent := range tt.expectAgents {
				found := false
				for _, m := range result.Mentions {
					if m == agent {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected mention for %s", agent)
				}
			}

			// Check interrupt info
			for agent, expectedDouble := range tt.expectDouble {
				info, ok := result.Interrupts[agent]
				if !ok {
					t.Errorf("expected interrupt info for %s", agent)
					continue
				}
				if info.Double != expectedDouble {
					t.Errorf("agent %s: expected Double=%v, got %v", agent, expectedDouble, info.Double)
				}
			}

			for agent, expectedNoSpawn := range tt.expectNoSpawn {
				info, ok := result.Interrupts[agent]
				if !ok {
					continue // Already reported above
				}
				if info.NoSpawn != expectedNoSpawn {
					t.Errorf("agent %s: expected NoSpawn=%v, got %v", agent, expectedNoSpawn, info.NoSpawn)
				}
			}
		})
	}
}
