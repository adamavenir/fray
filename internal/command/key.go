package command

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewKeyCmd creates the key command.
func NewKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key <message>",
		Short: "Record a key insight for a role",
		Long: `Record a key insight for a role. Keys are atomic pieces of wisdom
that help other agents in the same role.

Examples:
  fray key "Always validate inputs at boundaries" --as opus --role reviewer
  fray key "Cache at edge when latency matters" --as opus --role architect,reviewer
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			agentRef, _ := cmd.Flags().GetString("as")
			if agentRef == "" {
				agentRef = os.Getenv("FRAY_AGENT_ID")
				if agentRef == "" {
					return writeCommandError(cmd, fmt.Errorf("--as is required or set FRAY_AGENT_ID"))
				}
			}
			agentID, err := resolveAgentRef(ctx, agentRef)
			if err != nil {
				return writeCommandError(cmd, err)
			}

			agent, err := db.GetAgent(ctx.DB, agentID)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			if agent == nil {
				return writeCommandError(cmd, fmt.Errorf("agent not found: @%s. Use 'fray new' first", agentID))
			}
			if agent.LeftAt != nil {
				return writeCommandError(cmd, fmt.Errorf("agent @%s has left. Use 'fray back @%s' to resume", agentID, agentID))
			}

			rolesStr, _ := cmd.Flags().GetString("role")
			if rolesStr == "" {
				return writeCommandError(cmd, fmt.Errorf("--role is required"))
			}

			roles := parseRoles(rolesStr)
			if len(roles) == 0 {
				return writeCommandError(cmd, fmt.Errorf("at least one role is required"))
			}

			bases, err := db.GetAgentBases(ctx.DB)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			mentions := core.ExtractMentions(args[0], bases)
			mentions = core.ExpandAllMention(mentions, bases)

			now := time.Now().Unix()
			var postedThreads []string

			for _, role := range roles {
				// Ensure role hierarchy exists
				if err := ensureRoleHierarchy(ctx, role); err != nil {
					return writeCommandError(cmd, err)
				}

				// Find keys thread
				roleThreadName := fmt.Sprintf("roles/%s", role)
				roleThread, err := db.GetThreadByName(ctx.DB, roleThreadName, nil)
				if err != nil {
					return writeCommandError(cmd, err)
				}
				if roleThread == nil {
					return writeCommandError(cmd, fmt.Errorf("role thread not found: %s", roleThreadName))
				}

				keysThread, err := db.GetThreadByName(ctx.DB, "keys", &roleThread.GUID)
				if err != nil {
					return writeCommandError(cmd, err)
				}
				if keysThread == nil {
					return writeCommandError(cmd, fmt.Errorf("keys thread not found for role: %s", role))
				}

				// Create message in keys thread
				created, err := db.CreateMessage(ctx.DB, types.Message{
					TS:        now,
					FromAgent: agentID,
					Body:      args[0],
					Mentions:  mentions,
					Home:      keysThread.GUID,
				})
				if err != nil {
					return writeCommandError(cmd, err)
				}

				if err := db.AppendMessage(ctx.Project.DBPath, created); err != nil {
					return writeCommandError(cmd, err)
				}

				postedThreads = append(postedThreads, fmt.Sprintf("roles/%s/keys", role))
			}

			updates := db.AgentUpdates{LastSeen: types.OptionalInt64{Set: true, Value: &now}}
			if err := db.UpdateAgent(ctx.DB, agentID, updates); err != nil {
				return writeCommandError(cmd, err)
			}

			if ctx.JSONMode {
				payload := map[string]any{
					"agent_id": agentID,
					"roles":    roles,
					"threads":  postedThreads,
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(payload)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Recorded key to %s\n", strings.Join(postedThreads, ", "))
			return nil
		},
	}

	cmd.Flags().String("as", "", "agent ID to post as (defaults to FRAY_AGENT_ID)")
	cmd.Flags().String("role", "", "role(s) to post key for (comma-separated)")

	return cmd
}

// NewKeysCmd creates the keys view command.
func NewKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "View role keys (insights)",
		Long: `View keys (insights) for roles. Keys are atomic pieces of wisdom
recorded by agents holding roles.

Examples:
  fray keys                      # all roles, recent
  fray keys --role architect     # architect keys only
  fray keys --top 10             # top 10 by reactions
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			roleFilter, _ := cmd.Flags().GetString("role")
			topN, _ := cmd.Flags().GetInt("top")

			// Collect messages from role keys threads
			var allMessages []types.Message

			if roleFilter != "" {
				// Filter to specific role
				roleThreadName := fmt.Sprintf("roles/%s", roleFilter)
				roleThread, err := db.GetThreadByName(ctx.DB, roleThreadName, nil)
				if err != nil {
					return writeCommandError(cmd, err)
				}
				if roleThread == nil {
					if ctx.JSONMode {
						return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
							"messages": []types.Message{},
						})
					}
					fmt.Fprintf(cmd.OutOrStdout(), "No role thread found for: %s\n", roleFilter)
					return nil
				}

				keysThread, err := db.GetThreadByName(ctx.DB, "keys", &roleThread.GUID)
				if err != nil {
					return writeCommandError(cmd, err)
				}
				if keysThread != nil {
					messages, err := db.GetThreadMessages(ctx.DB, keysThread.GUID)
					if err != nil {
						return writeCommandError(cmd, err)
					}
					allMessages = messages
				}
			} else {
				// Get all threads starting with "roles/"
				allThreads, err := db.GetThreads(ctx.DB, nil)
				if err != nil {
					return writeCommandError(cmd, err)
				}

				for _, thread := range allThreads {
					if !strings.HasPrefix(thread.Name, "roles/") {
						continue
					}
					// Skip subthreads (meta, keys themselves)
					if thread.ParentThread != nil {
						continue
					}

					keysThread, err := db.GetThreadByName(ctx.DB, "keys", &thread.GUID)
					if err != nil {
						continue
					}
					if keysThread == nil {
						continue
					}

					messages, err := db.GetThreadMessages(ctx.DB, keysThread.GUID)
					if err != nil {
						continue
					}
					allMessages = append(allMessages, messages...)
				}
			}

			if len(allMessages) == 0 {
				if ctx.JSONMode {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"messages": []types.Message{},
					})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No keys found")
				return nil
			}

			// Sort by reactions if --top is specified
			if topN > 0 {
				sort.Slice(allMessages, func(i, j int) bool {
					return countReactions(allMessages[i]) > countReactions(allMessages[j])
				})
				if len(allMessages) > topN {
					allMessages = allMessages[:topN]
				}
			} else {
				// Sort by timestamp descending (recent first)
				sort.Slice(allMessages, func(i, j int) bool {
					return allMessages[i].TS > allMessages[j].TS
				})
			}

			if ctx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"messages": allMessages,
				})
			}

			out := cmd.OutOrStdout()
			bases, err := db.GetAgentBases(ctx.DB)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			projectName := GetProjectName(ctx.Project.Root)

			if topN > 0 {
				fmt.Fprintf(out, "Top %d keys:\n\n", len(allMessages))
			} else if roleFilter != "" {
				fmt.Fprintf(out, "Keys for %s:\n\n", roleFilter)
			} else {
				fmt.Fprintf(out, "All keys:\n\n")
			}

			for _, msg := range allMessages {
				fmt.Fprintln(out, FormatMessage(msg, projectName, bases))
			}
			return nil
		},
	}

	cmd.Flags().String("role", "", "filter to specific role")
	cmd.Flags().Int("top", 0, "show top N by reaction count")

	return cmd
}

func parseRoles(rolesStr string) []string {
	var roles []string
	for _, r := range strings.Split(rolesStr, ",") {
		r = strings.ToLower(strings.TrimSpace(r))
		if r != "" {
			roles = append(roles, r)
		}
	}
	return roles
}

func countReactions(msg types.Message) int {
	count := 0
	for _, entries := range msg.Reactions {
		count += len(entries)
	}
	return count
}
