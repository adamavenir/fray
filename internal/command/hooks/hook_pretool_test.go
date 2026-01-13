package hooks

import (
	"os"
	"testing"
)

func TestIsFrayPostCommand(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"fray post hello --as opus", true},
		{"fray post \"hello world\" --as opus", true},
		{"  fray post meta/notes \"note\" --as opus", true},
		{"fray get --as opus", false},
		{"echo fray post", false},
		{"git commit -m \"fray post test\"", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := isFrayPostCommand(tt.command)
			if got != tt.want {
				t.Errorf("isFrayPostCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestBuildPostReminder(t *testing.T) {
	// Save original env var
	origTriggerHome := os.Getenv("FRAY_TRIGGER_HOME")
	defer func() {
		os.Setenv("FRAY_TRIGGER_HOME", origTriggerHome)
	}()

	tests := []struct {
		name         string
		command      string
		triggerHome  string
		wantReminder bool
	}{
		{
			name:         "room post from room trigger - no reminder",
			command:      "fray post hello --as opus",
			triggerHome:  "room",
			wantReminder: false,
		},
		{
			name:         "room post from thread trigger - should remind",
			command:      "fray post hello --as opus",
			triggerHome:  "design-thread",
			wantReminder: true,
		},
		{
			name:         "post with reply-to from thread - no reminder",
			command:      "fray post hello --as opus --reply-to msg-xyz",
			triggerHome:  "design-thread",
			wantReminder: false,
		},
		{
			name:         "post with -r flag from thread - no reminder",
			command:      "fray post -r msg-xyz hello --as opus",
			triggerHome:  "design-thread",
			wantReminder: false,
		},
		{
			name:         "thread post from thread trigger - no reminder",
			command:      "fray post meta/notes hello --as opus",
			triggerHome:  "design-thread",
			wantReminder: false,
		},
		{
			name:         "no trigger context - no reminder",
			command:      "fray post hello --as opus",
			triggerHome:  "",
			wantReminder: false,
		},
		{
			name:         "force flag bypasses reminder",
			command:      "fray post hello --as opus --force",
			triggerHome:  "design-thread",
			wantReminder: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("FRAY_TRIGGER_HOME", tt.triggerHome)

			reminder := buildPostReminder(tt.command)
			gotReminder := reminder != ""

			if gotReminder != tt.wantReminder {
				t.Errorf("buildPostReminder(%q) returned reminder=%v, want %v\nReminder: %s",
					tt.command, gotReminder, tt.wantReminder, reminder)
			}
		})
	}
}
