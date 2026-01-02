package command

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewReactCmd creates the react command.
func NewReactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "react <emoji> <message>",
		Short: "React to a message with an emoji",
		Long:  "Add a reaction to a message. Optionally chain a reply with --reply.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			agentRef, _ := cmd.Flags().GetString("as")
			replyText, _ := cmd.Flags().GetString("reply")

			if agentRef == "" {
				return writeCommandError(cmd, fmt.Errorf("--as is required"))
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

			emojiArg := args[0]
			messageRef := args[1]

			reaction, ok := core.NormalizeReactionText(emojiArg)
			if !ok {
				return writeCommandError(cmd, fmt.Errorf("invalid reaction: %q (must be emoji)", emojiArg))
			}

			msg, err := resolveMessageRef(ctx.DB, messageRef)
			if err != nil {
				return writeCommandError(cmd, err)
			}

			updated, changed, err := db.AddReaction(ctx.DB, msg.ID, agentID, reaction)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			if changed {
				update := db.MessageUpdateJSONLRecord{ID: updated.ID, Reactions: &updated.Reactions}
				if err := db.AppendMessageUpdate(ctx.Project.DBPath, update); err != nil {
					return writeCommandError(cmd, err)
				}
			}

			now := time.Now().Unix()
			updates := db.AgentUpdates{LastSeen: types.OptionalInt64{Set: true, Value: &now}}
			if err := db.UpdateAgent(ctx.DB, agentID, updates); err != nil {
				return writeCommandError(cmd, err)
			}

			var replyMsg *types.Message
			if replyText != "" {
				bases, err := db.GetAgentBases(ctx.DB)
				if err != nil {
					return writeCommandError(cmd, err)
				}
				mentions := core.ExtractMentions(replyText, bases)
				mentions = core.ExpandAllMention(mentions, bases)

				created, err := db.CreateMessage(ctx.DB, types.Message{
					TS:        now,
					FromAgent: agentID,
					Body:      replyText,
					Mentions:  mentions,
					Home:      msg.Home,
					ReplyTo:   &msg.ID,
				})
				if err != nil {
					return writeCommandError(cmd, err)
				}
				if err := db.AppendMessage(ctx.Project.DBPath, created); err != nil {
					return writeCommandError(cmd, err)
				}
				replyMsg = &created
			}

			if ctx.JSONMode {
				payload := map[string]any{
					"message_id": msg.ID,
					"from":       agentID,
					"reaction":   reaction,
					"added":      changed,
				}
				if replyMsg != nil {
					payload["reply_id"] = replyMsg.ID
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(payload)
			}

			out := cmd.OutOrStdout()
			if changed {
				fmt.Fprintf(out, "Reacted %s to #%s\n", reaction, msg.ID)
			} else {
				fmt.Fprintf(out, "Reaction %s already exists on #%s\n", reaction, msg.ID)
			}
			if replyMsg != nil {
				fmt.Fprintf(out, "  Reply: [%s] %s\n", replyMsg.ID, replyText)
			}
			return nil
		},
	}

	cmd.Flags().String("as", "", "agent ID to react as")
	cmd.Flags().String("reply", "", "optional reply message to chain after reaction")

	_ = cmd.MarkFlagRequired("as")

	return cmd
}
