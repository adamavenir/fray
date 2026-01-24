package command

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/llm"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

type initResult struct {
	Initialized    bool     `json:"initialized"`
	AlreadyExisted bool     `json:"already_existed"`
	ChannelID      string   `json:"channel_id"`
	ChannelName    string   `json:"channel_name"`
	Path           string   `json:"path"`
	IssueTracker   string   `json:"issue_tracker,omitempty"`
	AgentsCreated  []string `json:"agents_created,omitempty"`
	Error          string   `json:"error,omitempty"`
}

// stockAgent represents a suggested agent for interactive init.
type stockAgent struct {
	Name        string
	Description string
	Driver      string // default driver
}

// stockAgents is the default set of agents to suggest during init.
var stockAgents = []stockAgent{
	{Name: "dev", Description: "development work", Driver: "claude"},
	{Name: "arch", Description: "architecture review/plans", Driver: "claude"},
	{Name: "desi", Description: "design review", Driver: "claude"},
	{Name: "pm", Description: "project coordination", Driver: "claude"},
}

// NewInitCmd creates the init command.
func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize fray in current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			useDefaults, _ := cmd.Flags().GetBool("defaults")
			jsonMode, _ := cmd.Flags().GetBool("json")

			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()

			if useDefaults && !force {
				if existing, err := core.DiscoverProject(""); err == nil {
					configPath := filepath.Join(existing.Root, ".fray", "fray-config.json")
					if _, err := os.Stat(configPath); err == nil {
						config, err := db.ReadProjectConfig(existing.DBPath)
						if err == nil && config != nil && config.ChannelID != "" && config.ChannelName != "" {
							result := initResult{
								Initialized:    true,
								AlreadyExisted: true,
								ChannelID:      config.ChannelID,
								ChannelName:    config.ChannelName,
								Path:           existing.Root,
							}
							if jsonMode {
								_ = json.NewEncoder(out).Encode(result)
								return nil
							}
							fmt.Fprintf(out, "Already initialized: %s (%s)\n", config.ChannelName, config.ChannelID)
							return nil
						}
					}
				}
			}

			projectRoot, err := os.Getwd()
			if err != nil {
				return writeInitError(errOut, jsonMode, err)
			}
			projectRoot, err = filepath.Abs(projectRoot)
			if err != nil {
				return writeInitError(errOut, jsonMode, err)
			}

			if !force && shouldJoinExistingProject(projectRoot) {
				return joinExistingProject(projectRoot, useDefaults, jsonMode, out, errOut)
			}

			project, err := core.InitProject(projectRoot, force)
			if err != nil {
				return writeInitError(errOut, jsonMode, err)
			}

			// Create llm/ directory and router.mld template
			if err := ensureLLMRouter(project.Root); err != nil {
				return writeInitError(errOut, jsonMode, err)
			}

			existingConfig, err := db.ReadProjectConfig(project.DBPath)
			if err != nil {
				return writeInitError(errOut, jsonMode, err)
			}
			channelID := ""
			channelName := ""
			if existingConfig != nil {
				channelID = existingConfig.ChannelID
				channelName = existingConfig.ChannelName
			}
			alreadyExisted := channelID != "" && channelName != ""

			if channelID == "" {
				defaultName := filepath.Base(project.Root)
				channelName = defaultName
				if !useDefaults {
					channelName = promptChannelName(defaultName)
				}
				generated, genErr := core.GenerateGUID("ch")
				if genErr != nil {
					return writeInitError(errOut, jsonMode, genErr)
				}
				channelID = generated

				update := db.ProjectConfig{
					Version:     1,
					ChannelID:   channelID,
					ChannelName: channelName,
					CreatedAt:   time.Now().UTC().Format(time.RFC3339),
					KnownAgents: map[string]db.ProjectKnownAgent{},
				}
				if existingConfig != nil {
					if existingConfig.Version != 0 {
						update.Version = existingConfig.Version
					}
					update.KnownAgents = existingConfig.KnownAgents
				}
				if _, err := db.UpdateProjectConfig(project.DBPath, update); err != nil {
					return writeInitError(errOut, jsonMode, err)
				}
			} else if channelName == "" {
				channelName = filepath.Base(project.Root)
				if _, err := db.UpdateProjectConfig(project.DBPath, db.ProjectConfig{ChannelName: channelName}); err != nil {
					return writeInitError(errOut, jsonMode, err)
				}
			}

			dbConn, err := db.OpenDatabase(project)
			if err != nil {
				return writeInitError(errOut, jsonMode, err)
			}
			if err := db.InitSchema(dbConn); err != nil {
				_ = dbConn.Close()
				return writeInitError(errOut, jsonMode, err)
			}
			if channelID != "" {
				if err := db.SetConfig(dbConn, "channel_id", channelID); err != nil {
					_ = dbConn.Close()
					return writeInitError(errOut, jsonMode, err)
				}
				if channelName != "" {
					if err := db.SetConfig(dbConn, "channel_name", channelName); err != nil {
						_ = dbConn.Close()
						return writeInitError(errOut, jsonMode, err)
					}
				}
			}
			_ = dbConn.Close()

			if channelID != "" && channelName != "" {
				if _, err := core.RegisterChannel(channelID, channelName, project.Root); err != nil {
					return writeInitError(errOut, jsonMode, err)
				}

				result := initResult{
					Initialized:    true,
					AlreadyExisted: alreadyExisted,
					ChannelID:      channelID,
					ChannelName:    channelName,
					Path:           project.Root,
				}

				// Create agents (interactive or default)
				var agentsCreated []string
				if !useDefaults && isTTY(os.Stdin) {
					// Interactive: let user select agents
					agentsCreated = promptAndCreateAgents(project.DBPath)
				} else {
					// Non-interactive: create all stock agents
					for _, agent := range stockAgents {
						if err := createManagedAgent(project.DBPath, agent.Name, agent.Driver); err != nil {
							fmt.Fprintf(errOut, "Warning: failed to create agent %s: %v\n", agent.Name, err)
							continue
						}
						agentsCreated = append(agentsCreated, agent.Name)
					}
				}
				if len(agentsCreated) > 0 {
					result.AgentsCreated = agentsCreated
				}

				// JSON output (after agents created)
				if jsonMode {
					_ = json.NewEncoder(out).Encode(result)
					return nil
				}

				// Human-readable output
				if !alreadyExisted {
					fmt.Fprintf(out, "✓ Registered channel %s as '%s'\n", channelID, channelName)
				}
				fmt.Fprintln(out, "Initialized .fray/")

				if len(agentsCreated) > 0 {
					fmt.Fprintf(out, "✓ Created %d managed agents: %s\n", len(agentsCreated), strings.Join(agentsCreated, ", "))
				}

				// Interactive: offer to install hooks
				if !useDefaults && isTTY(os.Stdin) {
					fmt.Fprintln(out, "")
					if promptYesNo("Install Claude Code hooks?", true) {
						fmt.Fprintln(out, "")
						fmt.Fprintln(out, "Run: fray hook-install --safety")
						fmt.Fprintln(out, "  (restart Claude Code after installing)")
					}
				} else {
					fmt.Fprintln(out, "")
					fmt.Fprintln(out, "Next steps:")
					fmt.Fprintln(out, "  fray hook-install --safety     # Install hooks with safety guards")
					fmt.Fprintln(out, "  fray hook-install --precommit  # Add git pre-commit hook for claims")
				}
			}

			return nil
		},
	}

	cmd.Flags().Bool("defaults", false, "use default values without prompting (idempotent)")

	return cmd
}

func promptChannelName(defaultName string) string {
	if !isTTY(os.Stdin) {
		return defaultName
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stdout, "Channel name for this project? [%s]: ", defaultName)
	text, _ := reader.ReadString('\n')
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return defaultName
	}
	return trimmed
}

func writeInitError(errOut io.Writer, jsonMode bool, err error) error {
	if jsonMode {
		payload := initResult{Initialized: false, Error: err.Error()}
		data, _ := json.Marshal(payload)
		fmt.Fprintln(errOut, string(data))
		return err
	}
	fmt.Fprintf(errOut, "Error: %s\n", err.Error())
	return err
}

func isTTY(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// promptIssueTracker asks the user to select an issue tracker.
func promptIssueTracker() string {
	if !isTTY(os.Stdin) {
		return ""
	}

	fmt.Println("Issue tracker:")
	fmt.Println("  1. bd (beads - built-in)")
	fmt.Println("  2. gh (GitHub Issues)")
	fmt.Println("  3. tk (tickets)")
	fmt.Println("  4. md (markdown files in todo/)")
	fmt.Println("  5. none")
	fmt.Print("Select [1-5, default=1]: ")

	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	trimmed := strings.TrimSpace(text)

	switch trimmed {
	case "", "1":
		return "bd"
	case "2":
		return "gh"
	case "3":
		return "tk"
	case "4":
		return "md"
	case "5":
		return "none"
	default:
		return "bd"
	}
}

// promptYesNo asks a yes/no question with a default.
func promptYesNo(question string, defaultYes bool) bool {
	if !isTTY(os.Stdin) {
		return defaultYes
	}

	suffix := "[Y/n]"
	if !defaultYes {
		suffix = "[y/N]"
	}
	fmt.Printf("%s %s: ", question, suffix)

	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	trimmed := strings.ToLower(strings.TrimSpace(text))

	if trimmed == "" {
		return defaultYes
	}
	return trimmed == "y" || trimmed == "yes"
}

// promptAndCreateAgents shows stock agents and creates selected ones.
func promptAndCreateAgents(dbPath string) []string {
	if !isTTY(os.Stdin) {
		return nil
	}

	fmt.Println("")
	fmt.Println("Suggested agents (select with numbers, e.g., 1,2,4 or 'all' or 'none'):")
	for i, agent := range stockAgents {
		fmt.Printf("  %d. %s - %s\n", i+1, agent.Name, agent.Description)
	}
	fmt.Print("Select [default=all]: ")

	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	trimmed := strings.TrimSpace(strings.ToLower(text))

	var selectedIndices []int
	if trimmed == "" || trimmed == "all" {
		for i := range stockAgents {
			selectedIndices = append(selectedIndices, i)
		}
	} else if trimmed == "none" {
		return nil
	} else {
		parts := strings.Split(trimmed, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			var idx int
			if _, err := fmt.Sscanf(part, "%d", &idx); err == nil && idx >= 1 && idx <= len(stockAgents) {
				selectedIndices = append(selectedIndices, idx-1)
			}
		}
	}

	if len(selectedIndices) == 0 {
		return nil
	}

	// Ask about driver customization
	fmt.Println("")
	fmt.Println("Default driver: claude (also supports codex, opencode)")
	if !promptYesNo("Use defaults?", true) {
		// Let user customize per-agent
		for _, idx := range selectedIndices {
			agent := &stockAgents[idx]
			fmt.Printf("Driver for %s [claude/codex/opencode, default=%s]: ", agent.Name, agent.Driver)
			driverText, _ := reader.ReadString('\n')
			driverTrimmed := strings.TrimSpace(strings.ToLower(driverText))
			if driverTrimmed == "claude" || driverTrimmed == "codex" || driverTrimmed == "opencode" {
				agent.Driver = driverTrimmed
			}
		}
	}

	// Create the agents
	var created []string
	for _, idx := range selectedIndices {
		agent := stockAgents[idx]
		if err := createManagedAgent(dbPath, agent.Name, agent.Driver); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create agent %s: %v\n", agent.Name, err)
			continue
		}
		created = append(created, agent.Name)
	}

	return created
}

// createManagedAgent creates a managed agent configuration.
func createManagedAgent(dbPath string, name string, driver string) error {
	project, err := core.DiscoverProject("")
	if err != nil {
		return err
	}

	dbConn, err := db.OpenDatabase(project)
	if err != nil {
		return err
	}
	defer dbConn.Close()

	// Check if agent already exists
	existing, _ := db.GetAgent(dbConn, name)
	if existing != nil {
		return nil // Already exists, skip
	}

	// Create the managed agent
	agentGUID, err := core.GenerateGUID("usr")
	if err != nil {
		return err
	}

	config, err := db.ReadProjectConfig(dbPath)
	if err != nil {
		return err
	}

	channelID := ""
	if config != nil {
		channelID = config.ChannelID
	}

	now := time.Now().Unix()
	agent := types.Agent{
		GUID:         agentGUID,
		AgentID:      name,
		RegisteredAt: now,
		LastSeen:     now,
		Managed:      true,
		Presence:     types.PresenceOffline,
		Invoke: &types.InvokeConfig{
			Driver: driver,
		},
	}
	_ = channelID // used by AppendAgent internally

	return db.AppendAgent(dbPath, agent)
}

type joinAgentOption struct {
	AgentID          string
	DisplayName      string
	Capabilities     []string
	LastActiveAt     int64
	LastActiveOrigin string
}

type joinAgentSelection struct {
	AgentID string
	Driver  string
}

func shouldJoinExistingProject(projectRoot string) bool {
	frayDir := filepath.Join(projectRoot, ".fray")
	info, err := os.Stat(frayDir)
	if err != nil || !info.IsDir() {
		return false
	}
	if _, err := os.Stat(filepath.Join(frayDir, "shared")); err != nil {
		return false
	}
	if db.GetLocalMachineID(projectRoot) != "" {
		return false
	}
	if db.GetStorageVersion(projectRoot) >= 2 {
		return true
	}
	return len(db.GetSharedMachinesDirs(projectRoot)) > 0
}

func joinExistingProject(projectRoot string, useDefaults, jsonMode bool, out, errOut io.Writer) error {
	config, err := db.ReadProjectConfig(projectRoot)
	if err != nil {
		return writeInitError(errOut, jsonMode, err)
	}

	machineDirs := db.GetSharedMachinesDirs(projectRoot)
	machineIDs := make([]string, 0, len(machineDirs))
	for _, dir := range machineDirs {
		machineIDs = append(machineIDs, filepath.Base(dir))
	}
	sort.Strings(machineIDs)

	defaultID := defaultMachineID()
	localID, err := promptMachineID(projectRoot, defaultID)
	if err != nil {
		return writeInitError(errOut, jsonMode, err)
	}

	frayDir := filepath.Join(projectRoot, ".fray")
	localDir := filepath.Join(frayDir, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return writeInitError(errOut, jsonMode, err)
	}
	if err := writeMachineIDFile(localDir, localID); err != nil {
		return writeInitError(errOut, jsonMode, err)
	}

	sharedMachineDir := filepath.Join(frayDir, "shared", "machines", localID)
	if err := os.MkdirAll(sharedMachineDir, 0o755); err != nil {
		return writeInitError(errOut, jsonMode, err)
	}

	if err := ensureLLMRouter(projectRoot); err != nil {
		return writeInitError(errOut, jsonMode, err)
	}

	options, err := buildJoinAgentOptions(projectRoot)
	if err != nil {
		return writeInitError(errOut, jsonMode, err)
	}
	selections := promptJoinAgentSelection(options, localID, useDefaults)
	agentsCreated, err := registerJoinAgents(projectRoot, selections)
	if err != nil {
		return writeInitError(errOut, jsonMode, err)
	}

	project := core.Project{Root: projectRoot, DBPath: filepath.Join(frayDir, "fray.db")}
	dbConn, err := db.OpenDatabase(project)
	if err != nil {
		return writeInitError(errOut, jsonMode, err)
	}
	if err := db.RebuildDatabaseFromJSONL(dbConn, project.DBPath); err != nil {
		_ = dbConn.Close()
		return writeInitError(errOut, jsonMode, err)
	}
	_ = dbConn.Close()

	result := initResult{
		Initialized:    true,
		AlreadyExisted: true,
		Path:           projectRoot,
		AgentsCreated:  agentsCreated,
	}
	if config != nil {
		result.ChannelID = config.ChannelID
		result.ChannelName = config.ChannelName
	}

	if jsonMode {
		_ = json.NewEncoder(out).Encode(result)
		return nil
	}

	fmt.Fprintln(out, "Found existing fray channel (synced from other machines)")
	if config != nil && config.ChannelName != "" && config.ChannelID != "" {
		fmt.Fprintf(out, "  Channel: %s (%s)\n", config.ChannelName, config.ChannelID)
	}
	if len(machineIDs) > 0 {
		fmt.Fprintf(out, "  Machines: %s\n", strings.Join(machineIDs, ", "))
	}
	fmt.Fprintf(out, "✓ Created .fray/local/ for %s\n", localID)
	if len(agentsCreated) > 0 {
		fmt.Fprintf(out, "✓ Registered %d agents on %s: %s\n", len(agentsCreated), localID, strings.Join(agentsCreated, ", "))
	}
	fmt.Fprintln(out, "✓ Built cache from shared machines")
	return nil
}

func writeMachineIDFile(localDir, machineID string) error {
	if machineID == "" {
		return fmt.Errorf("machine id required")
	}
	path := filepath.Join(localDir, "machine-id")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	record := map[string]any{
		"id":         machineID,
		"seq":        0,
		"created_at": time.Now().Unix(),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func buildJoinAgentOptions(projectRoot string) ([]joinAgentOption, error) {
	descriptors, err := db.ReadAgentDescriptors(projectRoot)
	if err != nil {
		return nil, err
	}
	messages, err := db.ReadMessages(projectRoot)
	if err != nil {
		return nil, err
	}

	type lastSeen struct {
		ts     int64
		origin string
	}
	lastSeenByAgent := map[string]lastSeen{}
	for _, message := range messages {
		if message.FromAgent == "" {
			continue
		}
		ts := message.TS
		entry, ok := lastSeenByAgent[message.FromAgent]
		if !ok || ts > entry.ts {
			lastSeenByAgent[message.FromAgent] = lastSeen{ts: ts, origin: message.Origin}
		}
	}

	byID := map[string]joinAgentOption{}
	for _, descriptor := range descriptors {
		if descriptor.AgentID == "" {
			continue
		}
		option := joinAgentOption{
			AgentID:      descriptor.AgentID,
			DisplayName:  descriptor.AgentID,
			Capabilities: descriptor.Capabilities,
			LastActiveAt: descriptor.TS,
		}
		if descriptor.DisplayName != nil && *descriptor.DisplayName != "" {
			option.DisplayName = *descriptor.DisplayName
		}
		if seen, ok := lastSeenByAgent[descriptor.AgentID]; ok {
			option.LastActiveAt = seen.ts
			option.LastActiveOrigin = seen.origin
		}
		byID[descriptor.AgentID] = option
	}

	for agentID, seen := range lastSeenByAgent {
		if agentID == "" {
			continue
		}
		if _, ok := byID[agentID]; ok {
			continue
		}
		byID[agentID] = joinAgentOption{
			AgentID:          agentID,
			DisplayName:      agentID,
			LastActiveAt:     seen.ts,
			LastActiveOrigin: seen.origin,
		}
	}

	if len(byID) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	options := make([]joinAgentOption, 0, len(ids))
	for _, id := range ids {
		options = append(options, byID[id])
	}
	return options, nil
}

func promptJoinAgentSelection(options []joinAgentOption, machineID string, useDefaults bool) []joinAgentSelection {
	if len(options) == 0 {
		return nil
	}
	if !isTTY(os.Stdin) || useDefaults {
		return defaultJoinSelections(options)
	}

	fmt.Println("")
	fmt.Printf("Which agents do you want to run on \"%s\"?\n\n", machineID)
	for i, option := range options {
		capabilities := formatCapabilities(option.Capabilities)
		lastActive := formatLastActive(option)
		label := option.AgentID
		if option.DisplayName != "" && option.DisplayName != option.AgentID {
			label = fmt.Sprintf("%s (%s)", option.AgentID, option.DisplayName)
		}
		fmt.Printf("  %d. %s%s%s\n", i+1, label, lastActive, capabilities)
	}
	fmt.Print("Select [default=all]: ")

	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	trimmed := strings.TrimSpace(strings.ToLower(text))

	var selected []int
	switch trimmed {
	case "", "all":
		for i := range options {
			selected = append(selected, i)
		}
	case "none":
		return nil
	default:
		parts := strings.Split(trimmed, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			var idx int
			if _, err := fmt.Sscanf(part, "%d", &idx); err == nil && idx >= 1 && idx <= len(options) {
				selected = append(selected, idx-1)
			}
		}
	}

	if len(selected) == 0 {
		return nil
	}

	selections := make([]joinAgentSelection, 0, len(selected))
	for _, idx := range selected {
		selections = append(selections, joinAgentSelection{AgentID: options[idx].AgentID, Driver: "claude"})
	}

	fmt.Println("")
	fmt.Println("Default driver: claude (also supports codex, opencode)")
	if !promptYesNo("Use defaults?", true) {
		for i := range selections {
			fmt.Printf("Driver for %s [claude/codex/opencode, default=claude]: ", selections[i].AgentID)
			driverText, _ := reader.ReadString('\n')
			driverTrimmed := strings.TrimSpace(strings.ToLower(driverText))
			if driverTrimmed == "claude" || driverTrimmed == "codex" || driverTrimmed == "opencode" {
				selections[i].Driver = driverTrimmed
			}
		}
	}

	return selections
}

func defaultJoinSelections(options []joinAgentOption) []joinAgentSelection {
	selections := make([]joinAgentSelection, 0, len(options))
	for _, option := range options {
		selections = append(selections, joinAgentSelection{AgentID: option.AgentID, Driver: "claude"})
	}
	return selections
}

func formatLastActive(option joinAgentOption) string {
	if option.LastActiveAt <= 0 {
		return ""
	}
	label := formatRelative(option.LastActiveAt)
	if option.LastActiveOrigin != "" {
		label = fmt.Sprintf("%s, %s", option.LastActiveOrigin, label)
	}
	return fmt.Sprintf(" (last active: %s)", label)
}

func formatCapabilities(capabilities []string) string {
	if len(capabilities) == 0 {
		return ""
	}
	return fmt.Sprintf(" [%s]", strings.Join(capabilities, ", "))
}

func registerJoinAgents(projectRoot string, selections []joinAgentSelection) ([]string, error) {
	if len(selections) == 0 {
		return nil, nil
	}
	existingAgents, err := db.ReadAgents(projectRoot)
	if err != nil {
		return nil, err
	}
	existing := map[string]bool{}
	for _, agent := range existingAgents {
		if agent.AgentID == "" {
			continue
		}
		existing[agent.AgentID] = true
	}

	var created []string
	now := time.Now().Unix()
	for _, selection := range selections {
		if selection.AgentID == "" || existing[selection.AgentID] {
			continue
		}
		guid, err := core.GenerateGUID("usr")
		if err != nil {
			return nil, err
		}
		agent := types.Agent{
			GUID:         guid,
			AgentID:      selection.AgentID,
			RegisteredAt: now,
			LastSeen:     now,
			Managed:      true,
			Presence:     types.PresenceOffline,
			Invoke: &types.InvokeConfig{
				Driver: selection.Driver,
			},
		}
		if err := db.AppendAgent(projectRoot, agent); err != nil {
			return nil, err
		}
		created = append(created, selection.AgentID)
	}
	return created, nil
}

// ensureLLMRouter creates the .fray/llm/ directory and stock mlld templates.
// Only creates files that don't exist (preserves user customizations).
func ensureLLMRouter(projectRoot string) error {
	llmDir := filepath.Join(projectRoot, ".fray", "llm")

	// Create llm/ directory
	if err := os.MkdirAll(llmDir, 0o755); err != nil {
		return fmt.Errorf("create llm directory: %w", err)
	}

	// Create llm/run/ directory for user scripts
	runDir := filepath.Join(llmDir, "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("create llm/run directory: %w", err)
	}

	// Create llm/routers/ directory for mlld routers
	routersDir := filepath.Join(llmDir, "routers")
	if err := os.MkdirAll(routersDir, 0o755); err != nil {
		return fmt.Errorf("create llm/routers directory: %w", err)
	}

	// Create llm/slash/ directory for session lifecycle commands
	slashDir := filepath.Join(llmDir, "slash")
	if err := os.MkdirAll(slashDir, 0o755); err != nil {
		return fmt.Errorf("create llm/slash directory: %w", err)
	}

	// Write router templates
	routerTemplates := []string{
		llm.MentionsRouterTemplate,
		llm.StdoutRepairTemplate,
	}
	for _, templatePath := range routerTemplates {
		content, err := llm.ReadTemplate(templatePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", templatePath, err)
		}
		destPath := filepath.Join(routersDir, filepath.Base(templatePath))
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			if err := os.WriteFile(destPath, content, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", templatePath, err)
			}
		}
	}

	// Write status template (lives in llm/ root)
	statusContent, err := llm.ReadTemplate(llm.StatusTemplate)
	if err != nil {
		return fmt.Errorf("read status template: %w", err)
	}
	statusPath := filepath.Join(llmDir, "status.mld")
	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		if err := os.WriteFile(statusPath, statusContent, 0o644); err != nil {
			return fmt.Errorf("write status template: %w", err)
		}
	}

	// Write slash command templates (session lifecycle)
	slashTemplates := []string{
		llm.FlyTemplate,
		llm.LandTemplate,
		llm.HandTemplate,
		llm.HopTemplate,
	}
	for _, templatePath := range slashTemplates {
		content, err := llm.ReadTemplate(templatePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", templatePath, err)
		}
		destPath := filepath.Join(slashDir, filepath.Base(templatePath))
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			if err := os.WriteFile(destPath, content, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", templatePath, err)
			}
		}
	}

	// Create llm/prompts/ directory for daemon prompts
	promptsDir := filepath.Join(llmDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		return fmt.Errorf("create llm/prompts directory: %w", err)
	}

	// Write prompt templates (used by daemon for @mentions)
	promptTemplates := []string{
		llm.MentionFreshTemplate,
		llm.MentionResumeTemplate,
	}
	for _, templatePath := range promptTemplates {
		content, err := llm.ReadTemplate(templatePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", templatePath, err)
		}
		destPath := filepath.Join(promptsDir, filepath.Base(templatePath))
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			if err := os.WriteFile(destPath, content, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", templatePath, err)
			}
		}
	}

	// Create mlld-config.json (if not exists)
	// @proj resolver points to project root (absolute path)
	mlldConfigPath := filepath.Join(projectRoot, ".fray", "mlld-config.json")
	if _, err := os.Stat(mlldConfigPath); os.IsNotExist(err) {
		mlldConfig := map[string]any{
			"scriptDir": "llm/run",
			"resolvers": map[string]any{
				"prefixes": []map[string]any{
					{
						"prefix":   "@proj/",
						"resolver": "LOCAL",
						"config": map[string]any{
							"basePath": projectRoot,
						},
					},
				},
			},
		}
		configData, err := json.MarshalIndent(mlldConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal mlld config: %w", err)
		}
		if err := os.WriteFile(mlldConfigPath, configData, 0o644); err != nil {
			return fmt.Errorf("write mlld config: %w", err)
		}
	}

	return nil
}
