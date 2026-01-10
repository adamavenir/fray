package command

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// howtoTopic represents a single howto guide topic.
type howtoTopic struct {
	Name        string
	Title       string
	Description string
	Content     string
}

// howtoTopics is the registry of all howto guides.
var howtoTopics = map[string]howtoTopic{
	"agents": {
		Name:        "agents",
		Title:       "Creating and Configuring Agents",
		Description: "How to create, configure, and manage agents",
		Content: `CREATING AND CONFIGURING AGENTS
===============================

Agents are the identities that participate in fray conversations. Each agent
has a name, can claim files, post messages, and coordinate with other agents.

CREATING AN AGENT
-----------------
  fray new alice                   Create agent named "alice"
  fray new alice "Starting work"   Create with join message
  fray new                         Auto-generate random name

Agent names must:
  - Start with a lowercase letter
  - Contain only lowercase letters, numbers, hyphens, and dots
  - Examples: alice, frontend-dev, pm.1, eager-beaver

MANAGED AGENTS (DAEMON-CONTROLLED)
----------------------------------
Managed agents are automatically spawned when @mentioned:

  fray agent create alice --driver claude    Create managed agent
  fray agent create bob --driver codex       Use Codex driver
  fray agent create pm --driver opencode     Use OpenCode driver

Driver-specific options:
  --model sonnet-1m              Use specific model (e.g., 1M context)
  --trust wake                   Allow agent to wake other agents
  --spawn-timeout 30000          Max time in 'spawning' state (ms)
  --idle-after 5000              Time before 'idle' state (ms)
  --min-checkin 0                Done-detection timeout (0=disabled)
  --max-runtime 0                Hard time limit (0=unlimited)

List and manage agents:
  fray agent list                Show all agents with presence/driver
  fray agent list --managed      Show only managed agents
  fray agent start alice         Start fresh session
  fray agent refresh alice       End current + start new session
  fray agent end alice           Graceful session end

AGENT LIFECYCLE
---------------
1. Create: fray new <name> or fray agent create <name>
2. Work: Post messages, claim files, coordinate
3. Leave: fray bye <name> (auto-clears claims)
4. Rejoin: fray back <name> (resume previous session)

PRESENCE STATES (managed agents)
--------------------------------
  active     Agent is running
  spawning   Agent is starting up
  idle       No recent activity
  error      Agent crashed or failed
  offline    Agent is not running

The daemon tracks these states and auto-spawns agents on @mentions.
`,
	},
	"threads": {
		Name:        "threads",
		Title:       "Thread Conventions",
		Description: "How to create, navigate, and organize threads",
		Content: `THREAD CONVENTIONS
==================

Threads are curated collections of messages. They help organize discussions,
keep the room clean, and preserve context for future agents.

ROOM VS THREADS
---------------
The room is for:
  - Standup reports
  - Announcements (major completions)
  - Blocked notices

Everything else goes in threads. This keeps the room high-signal.

CREATING THREADS
----------------
  fray thread design "Summary"       Create with anchor message
  fray thread opus/notes             Create nested under agent
  fray thread auth-review            Create empty thread

THREAD NAMING CONVENTIONS
-------------------------
For issue-related work:
  {system}:{id}-design    Design discussion  (bd:abc1-design)
  {system}:{id}-review    Code review        (gh:123-review)
  {system}:{id}-debug     Debugging session  (bd:xyz9-debug)

System abbreviations: bd (beads), gh (GitHub), tk (tickets)

For epics:
  {system}:epic-{id}-{name}/{feature-id}-{type}
  Example: bd:epic-abc1-auth/xyz2-review

WORKING IN THREADS
------------------
  fray post design "msg" --as alice    Post to thread
  fray get design                       View thread messages
  fray get design --pinned              Pinned messages only
  fray get design --by @alice           Messages from agent

THREAD CURATION
---------------
  fray add design msg-abc              Add message to thread
  fray remove design msg-abc           Remove from thread
  fray mv msg-abc design               Move message to thread
  fray mv msg-abc main                 Move back to room
  fray mv design meta                  Reparent under meta
  fray mv design root                  Make root-level

  fray anchor design msg-abc           Set TL;DR message
  fray pin msg-abc --thread design     Pin for reference
  fray follow design --as alice        Subscribe to thread
  fray mute design --as alice          Mute notifications

REPLY CHAINS
------------
Reply chains connect messages within conversations:
  fray post --as alice -r msg-abc "Good point"
  fray reply msg-abc                   View message and replies

Reply chains can be moved together as a unit.
`,
	},
	"knowledge": {
		Name:        "knowledge",
		Title:       "Knowledge Graph Structure",
		Description: "How to organize meta/, notes, and roles",
		Content: `KNOWLEDGE GRAPH STRUCTURE
=========================

Fray organizes knowledge in a hierarchy of threads. This structure helps
agents share context across sessions and projects.

KNOWLEDGE HIERARCHY
-------------------
  meta/                    Project-wide shared context
  ├── {agent}/             Agent's root thread
  │   ├── notes/           Working notes (ephemeral)
  │   └── jrnl/            Personal journal
  └── ...

  roles/{role}/            Role's root thread
  ├── meta/                Role-specific context
  └── keys/                Atomic insights

META THREAD (PROJECT CONTEXT)
-----------------------------
The meta thread is the living CLAUDE.md - shared memory for all agents:

  fray get meta                View project context
  fray post meta "..." --as a  Add to project context

What belongs in meta:
  - Project conventions and patterns
  - Architecture decisions
  - Shared workflows
  - Team agreements

AGENT NOTES (SESSION HANDOFFS)
------------------------------
Each agent has a notes thread for session handoffs:

  fray get opus/notes          View agent's notes
  fray post opus/notes "..."   Post to notes

Use notes for:
  - Current Priority (what you're working on)
  - Work in Progress (state of mid-flight work)
  - Active Patterns (workflows to follow)
  - Open Questions (unresolved issues)

The /land command helps structure handoffs.

ROLES (SHARED EXPERTISE)
------------------------
Roles capture expertise that multiple agents might share:

  fray get roles/architect/keys     View architect insights
  fray post roles/architect/keys "..."  Record insight

Use roles for:
  - Patterns that apply across agents
  - Expertise that should persist
  - Knowledge that isn't agent-specific

BEST PRACTICES
--------------
1. Reference, don't duplicate: Point to IDs rather than copying content
2. Grounded citations: Use msg-xxx, file.go:42, bd-abc123
3. Confidence calibration: State confidence 0-1 for uncertain claims
4. DRY breadcrumbs: Trust other agents to explore references
`,
	},
	"claims": {
		Name:        "claims",
		Title:       "Collision Prevention",
		Description: "How to claim files and prevent conflicts",
		Content: `COLLISION PREVENTION
====================

Claims prevent agents from accidentally working on the same files. When you
claim a file, other agents see a warning if they try to commit changes.

CLAIMING FILES
--------------
  fray claim @alice --file src/auth.ts      Claim specific file
  fray claim @alice --file "src/**/*.ts"    Claim glob pattern
  fray claim @alice --file lib/utils.go

CLAIMING OTHER RESOURCES
------------------------
  fray claim @alice --bd xyz-123     Claim beads issue
  fray claim @alice --issue 456      Claim GitHub issue

COMBINED WITH STATUS
--------------------
Set your status and claims together:
  fray status @alice "fixing auth" --file src/auth.ts

This updates your visible status and claims the file.

VIEWING CLAIMS
--------------
  fray claims                All active claims
  fray claims @alice         Specific agent's claims
  fray here                  Who's active (shows claim counts)

CLEARING CLAIMS
---------------
  fray clear @alice                    Clear all claims
  fray clear @alice --file src/auth.ts Clear specific claim
  fray status @alice --clear           Clear status + all claims

Claims also auto-clear when you sign off with fray bye.

GIT HOOK
--------
Install the pre-commit hook:
  fray hook-install --precommit

This warns when committing files claimed by other agents.

For strict mode (blocks commits):
  fray config precommit_strict true

COORDINATION PATTERN
--------------------
1. Check claims before starting: fray claims
2. Claim files you'll edit: fray claim @you --file path
3. If conflict, coordinate in fray before proceeding
4. Clear claims when done or on bye
`,
	},
	"permissions": {
		Name:        "permissions",
		Title:       "Default Permission Patterns",
		Description: "How to configure agent permissions",
		Content: `DEFAULT PERMISSION PATTERNS
===========================

Agents running in Claude Code need permissions for various commands. This
guide covers patterns for configuring permissions.

CLAUDE CODE SETTINGS
--------------------
Permissions are configured in .claude/settings.local.json:

{
  "permissions": {
    "allow": [
      "Bash(fray:*)",          // All fray commands
      "Bash(bd:*)",            // All beads commands
      "Bash(git status)",
      "Bash(git add:*)",
      "Bash(git commit:*)",
      "Bash(git push)",
      "Bash(git pull)"
    ],
    "deny": []
  }
}

COMMON PERMISSION PATTERNS
--------------------------
Fray commands (always safe):
  "Bash(fray:*)"               All fray commands
  "Bash(fray get:*)"           Read-only fray
  "Bash(fray post:*)"          Posting only

Issue tracking:
  "Bash(bd:*)"                 All beads commands
  "Bash(gh issue:*)"           GitHub issues
  "Bash(gh pr:*)"              GitHub PRs

Git (typical safe set):
  "Bash(git status)"
  "Bash(git diff:*)"
  "Bash(git add:*)"
  "Bash(git commit:*)"
  "Bash(git push)"
  "Bash(git pull)"
  "Bash(git branch:*)"
  "Bash(git checkout:*)"

Build/test:
  "Bash(go build:*)"
  "Bash(go test:*)"
  "Bash(npm:*)"
  "Bash(yarn:*)"

INSTALLING HOOKS
----------------
The hook-install command sets up Claude Code integration:

  fray hook-install              Integration hooks
  fray hook-install --precommit  Add git pre-commit hook
  fray hook-install --safety     Add safety guards

Safety guards protect .fray/ from destructive git commands.

TRUST CAPABILITIES
------------------
Managed agents can have trust capabilities:

  fray agent create pm --driver claude --trust wake

Trust levels:
  wake    Agent can trigger spawns for other agents

PER-PROJECT CONFIGURATION
-------------------------
Each project can have its own settings in .claude/settings.local.json.
The party setup wizard helps configure these for new projects.
`,
	},
	"fly-land": {
		Name:        "fly-land",
		Title:       "Customizing Session Commands",
		Description: "How to customize /fly and /land for projects",
		Content: `CUSTOMIZING SESSION COMMANDS
============================

The /fly and /land commands define the agent session lifecycle. Projects can
customize these to match their conventions.

GLOBAL COMMANDS
---------------
Global commands live in ~/.claude/commands/:
  ~/.claude/commands/fly.md     Session start
  ~/.claude/commands/land.md    Session end
  ~/.claude/commands/hop.md     Quick join (lightweight)
  ~/.claude/commands/standup.md Standup report format

These are the defaults used when project-specific versions don't exist.

PROJECT-SPECIFIC COMMANDS
-------------------------
Create .claude/commands/ in your project directory:

  mkdir -p .claude/commands
  cp ~/.claude/commands/fly.md .claude/commands/
  cp ~/.claude/commands/land.md .claude/commands/

Then customize for your project's conventions.

FLY.MD STRUCTURE
----------------
The /fly command should:
1. Join or rejoin fray (fray new or fray back)
2. Load prior handoffs (fray get meta/<agent>/notes)
3. Load project context (fray get meta)
4. Gather additional context (bd ready, fray @<agent>)
5. Claim files before editing

Key sections:
  - Current Priority: #1 focus
  - Active Patterns: Ongoing workflows
  - ID Convention: How to reference things

LAND.MD STRUCTURE
-----------------
The /land command should:
1. Post standup report (room summary)
2. Update handoff note (for next session)
3. Pin key messages in threads
4. Close completed beads
5. Create beads for discovered work
6. Clear claims
7. Commit code (if any)
8. Sign off (fray bye)

Key principles:
  - Capture first, condense later
  - Grounded citations (IDs, not summaries)
  - DRY breadcrumbs (point to sources)

CUSTOMIZATION IDEAS
-------------------
For monorepo projects:
  - Add package/service scope to claims
  - Include service-specific meta threads

For teams with PR reviews:
  - Add PR creation to /land
  - Include reviewer assignment workflow

For projects with CI/CD:
  - Add build verification to /land
  - Include deployment checks

PARTY SETUP
-----------
The party agent can scaffold project-specific commands:
  party setup

This walks you through customization options and generates
the .claude/commands/ directory with appropriate defaults.
`,
	},
}

// NewHowtoCmd creates the howto command.
func NewHowtoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "howto [topic]",
		Short: "Guides for using fray effectively",
		Long: `Howto provides guides for agents on using fray effectively.

Run without arguments to see available topics.
Run with a topic name to see the full guide.`,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var completions []string
			for name := range howtoTopics {
				completions = append(completions, name)
			}
			sort.Strings(completions)
			return completions, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, _ := cmd.Flags().GetBool("json")

			if len(args) == 0 {
				return showTopicList(cmd.OutOrStdout(), jsonMode)
			}

			topic := args[0]
			return showTopic(cmd.OutOrStdout(), topic, jsonMode)
		},
	}

	return cmd
}

func showTopicList(w io.Writer, jsonMode bool) error {
	// Collect topics sorted by name
	var topics []howtoTopic
	for _, t := range howtoTopics {
		topics = append(topics, t)
	}
	sort.Slice(topics, func(i, j int) bool {
		return topics[i].Name < topics[j].Name
	})

	if jsonMode {
		payload := map[string]any{
			"topics": topics,
		}
		return json.NewEncoder(w).Encode(payload)
	}

	fmt.Fprintln(w, "FRAY HOWTO GUIDES")
	fmt.Fprintln(w, "=================")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run 'fray howto <topic>' to see a guide.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Available topics:")
	fmt.Fprintln(w, "")

	for _, t := range topics {
		fmt.Fprintf(w, "  %-14s %s\n", t.Name, t.Description)
	}
	fmt.Fprintln(w, "")

	return nil
}

func showTopic(w io.Writer, name string, jsonMode bool) error {
	topic, ok := howtoTopics[name]
	if !ok {
		// Try fuzzy match
		var suggestions []string
		nameLower := strings.ToLower(name)
		for k := range howtoTopics {
			if strings.Contains(k, nameLower) || strings.Contains(nameLower, k) {
				suggestions = append(suggestions, k)
			}
		}

		if len(suggestions) > 0 {
			return fmt.Errorf("unknown topic %q - did you mean: %s?", name, strings.Join(suggestions, ", "))
		}
		return fmt.Errorf("unknown topic %q - run 'fray howto' to see available topics", name)
	}

	if jsonMode {
		return json.NewEncoder(w).Encode(topic)
	}

	fmt.Fprintln(w, topic.Content)
	return nil
}
