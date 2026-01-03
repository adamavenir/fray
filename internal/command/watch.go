package command

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adamavenir/fray/internal/daemon"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewWatchCmd creates the watch command.
func NewWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream messages in real-time",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			last, _ := cmd.Flags().GetInt("last")
			includeArchived, _ := cmd.Flags().GetBool("archived")

			projectName := GetProjectName(ctx.Project.Root)
			out := cmd.OutOrStdout()
			var agentBases map[string]struct{}
			if !ctx.JSONMode {
				agentBases, err = db.GetAgentBases(ctx.DB)
				if err != nil {
					return writeCommandError(cmd, err)
				}
			}

			var cursor *types.MessageCursor
			if last == 0 {
				cursor, err = db.GetLastMessageCursor(ctx.DB)
				if err != nil {
					return writeCommandError(cmd, err)
				}
				if !ctx.JSONMode {
					fmt.Fprintln(out, "--- watching (Ctrl+C to stop) ---")
				}
			} else {
				recent, err := db.GetMessages(ctx.DB, &types.MessageQueryOptions{Limit: last, IncludeArchived: includeArchived})
				if err != nil {
					return writeCommandError(cmd, err)
				}
				recent, err = db.ApplyMessageEditCounts(ctx.Project.DBPath, recent)
				if err != nil {
					return writeCommandError(cmd, err)
				}
				if len(recent) > 0 {
					if ctx.JSONMode {
						for _, msg := range recent {
							_ = json.NewEncoder(out).Encode(msg)
						}
					} else {
						for _, msg := range recent {
							fmt.Fprintln(out, FormatMessage(msg, projectName, agentBases))
						}
						fmt.Fprintln(out, "--- watching (Ctrl+C to stop) ---")
					}
					lastMsg := recent[len(recent)-1]
					cursor = &types.MessageCursor{GUID: lastMsg.ID, TS: lastMsg.TS}
				} else if !ctx.JSONMode {
					fmt.Fprintln(out, "--- watching (Ctrl+C to stop) ---")
				}
			}

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			// Heartbeat tracking for daemon-managed agents
			agentID := os.Getenv("FRAY_AGENT_ID")
			var minCheckinMs int64
			var lastActivityTime time.Time
			var lastWarningLevel int // 0=none, 1=5min, 2=2min, 3=1min

			if agentID != "" {
				agent, err := db.GetAgent(ctx.DB, agentID)
				if err == nil && agent != nil && agent.Invoke != nil {
					_, _, minCheckinMs, _ = daemon.GetTimeouts(agent.Invoke)
				}
				if minCheckinMs == 0 {
					minCheckinMs = 600000 // default 10m
				}

				// Get actual last activity time (matches daemon's done-detection logic)
				// Use max of: last post, last heartbeat, or now (for new sessions)
				lastPostTs, _ := db.GetAgentLastPostTime(ctx.DB, agentID)
				lastHeartbeatTs := int64(0)
				if agent != nil && agent.LastHeartbeat != nil {
					lastHeartbeatTs = *agent.LastHeartbeat
				}

				// Pick the most recent activity
				lastActivityMs := lastPostTs
				if lastHeartbeatTs > lastActivityMs {
					lastActivityMs = lastHeartbeatTs
				}

				if lastActivityMs > 0 {
					lastActivityTime = time.UnixMilli(lastActivityMs)
				} else {
					// No prior activity - treat as just started
					lastActivityTime = time.Now()
				}

				if !ctx.JSONMode {
					elapsed := time.Since(lastActivityTime).Round(time.Second)
					remaining := time.Duration(minCheckinMs)*time.Millisecond - elapsed
					if remaining < 0 {
						remaining = 0
					}
					fmt.Fprintf(out, "[heartbeat] @%s: last activity %s ago, recycle in %s\n",
						agentID, elapsed, remaining.Round(time.Second))
				}
			}

			// Heartbeat status ticker (every 30s)
			var heartbeatTicker *time.Ticker
			if agentID != "" && !ctx.JSONMode {
				heartbeatTicker = time.NewTicker(30 * time.Second)
				defer heartbeatTicker.Stop()
			}

			for {
				select {
				case <-stop:
					return nil
				case <-ticker.C:
					newMessages, err := db.GetMessages(ctx.DB, &types.MessageQueryOptions{Since: cursor, IncludeArchived: includeArchived})
					if err != nil {
						return writeCommandError(cmd, err)
					}
					newMessages, err = db.ApplyMessageEditCounts(ctx.Project.DBPath, newMessages)
					if err != nil {
						return writeCommandError(cmd, err)
					}
					if len(newMessages) == 0 {
						continue
					}

					// Check if any message is from our agent (resets timer)
					if agentID != "" {
						for _, msg := range newMessages {
							if msg.FromAgent == agentID {
								lastActivityTime = time.Now()
								lastWarningLevel = 0
							}
						}
					}

					if ctx.JSONMode {
						encoder := json.NewEncoder(out)
						for _, msg := range newMessages {
							_ = encoder.Encode(msg)
						}
					} else {
						for _, msg := range newMessages {
							fmt.Fprintln(out, FormatMessage(msg, projectName, agentBases))
						}
					}
					lastMsg := newMessages[len(newMessages)-1]
					cursor = &types.MessageCursor{GUID: lastMsg.ID, TS: lastMsg.TS}

				case <-func() <-chan time.Time {
					if heartbeatTicker != nil {
						return heartbeatTicker.C
					}
					return nil
				}():
					// Show heartbeat status
					elapsed := time.Since(lastActivityTime)
					remaining := time.Duration(minCheckinMs)*time.Millisecond - elapsed
					if remaining < 0 {
						remaining = 0
					}

					// Warn at thresholds: 5min, 2min, 1min
					warningLevel := 0
					if remaining <= 1*time.Minute {
						warningLevel = 3
					} else if remaining <= 2*time.Minute {
						warningLevel = 2
					} else if remaining <= 5*time.Minute {
						warningLevel = 1
					}

					if warningLevel > lastWarningLevel {
						lastWarningLevel = warningLevel
						switch warningLevel {
						case 1:
							fmt.Fprintf(out, "[heartbeat] ⚠️  %s until checkin timeout. Post something or run: fray heartbeat --as %s\n",
								remaining.Round(time.Second), agentID)
						case 2:
							fmt.Fprintf(out, "[heartbeat] ⚠️  %s remaining! Post or: fray heartbeat --as %s\n",
								remaining.Round(time.Second), agentID)
						case 3:
							fmt.Fprintf(out, "[heartbeat] 🚨 %s remaining! POST NOW or: fray heartbeat --as %s\n",
								remaining.Round(time.Second), agentID)
						}
					}
				}
			}
		},
	}

	cmd.Flags().Int("last", 10, "show last N messages before streaming")
	cmd.Flags().Bool("archived", false, "include archived messages")
	return cmd
}
