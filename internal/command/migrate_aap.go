package command

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/adamavenir/fray/internal/aap"
	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewMigrateAAPCmd creates the migrate-aap command.
func NewMigrateAAPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-aap",
		Short: "Migrate legacy agents to AAP",
		Long: `Create AAP identities for existing agents that don't have them.

This command scans all agents in the current project and creates AAP
identities for any that are missing one. By default, identities are
created without keypairs. Use --with-key to generate keypairs (will
prompt for passphrase for each agent).

Examples:
  fray migrate-aap                    # Migrate all agents (no keys)
  fray migrate-aap --agent dev        # Migrate specific agent
  fray migrate-aap --with-key         # Migrate with keypairs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdCtx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer cmdCtx.DB.Close()

			specificAgent, _ := cmd.Flags().GetString("agent")
			withKey, _ := cmd.Flags().GetBool("with-key")

			aapDir, err := core.AAPConfigDir()
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("get AAP config dir: %w", err))
			}

			registry, err := aap.NewFileRegistry(filepath.Join(aapDir, "agents"))
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("create AAP registry: %w", err))
			}

			agents, err := db.GetAllAgents(cmdCtx.DB)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("get agents: %w", err))
			}

			var migrated []string
			var skipped []string
			var failed []string

			for _, agent := range agents {
				// Filter by specific agent if provided
				if specificAgent != "" && agent.AgentID != specificAgent {
					continue
				}

				// Skip if already has AAP identity
				if agent.AAPGUID != nil {
					skipped = append(skipped, agent.AgentID)
					continue
				}

				// Check if AAP identity already exists
				existing, err := registry.Get(agent.AgentID)
				if err == nil && existing != nil {
					// AAP identity exists but fray record not linked - update it
					aapGUID := existing.Record.GUID
					updates := db.AgentUpdates{
						AAPGUID: types.OptionalString{Set: true, Value: &aapGUID},
					}
					if err := db.UpdateAgent(cmdCtx.DB, agent.AgentID, updates); err != nil {
						failed = append(failed, fmt.Sprintf("%s (update failed: %v)", agent.AgentID, err))
						continue
					}
					if err := db.AppendAgentUpdate(cmdCtx.Project.DBPath, db.AgentUpdateJSONLRecord{
						AgentID: agent.AgentID,
						AAPGUID: &aapGUID,
					}); err != nil {
						failed = append(failed, fmt.Sprintf("%s (jsonl update failed: %v)", agent.AgentID, err))
						continue
					}
					migrated = append(migrated, agent.AgentID)
					continue
				}

				// Create new AAP identity
				opts := aap.RegisterOpts{
					GenerateKey: withKey,
					Metadata: map[string]string{
						"created_by":    "fray",
						"migrated_from": agent.GUID,
					},
				}

				if withKey {
					passphrase, err := promptPassphrase(fmt.Sprintf("Enter passphrase for @%s: ", agent.AgentID))
					if err != nil {
						failed = append(failed, fmt.Sprintf("%s (passphrase failed: %v)", agent.AgentID, err))
						continue
					}
					opts.Passphrase = passphrase
				}

				identity, err := registry.Register(agent.AgentID, opts)
				if err != nil {
					failed = append(failed, fmt.Sprintf("%s (register failed: %v)", agent.AgentID, err))
					continue
				}

				// Update fray record with AAP_GUID
				aapGUID := identity.Record.GUID
				updates := db.AgentUpdates{
					AAPGUID: types.OptionalString{Set: true, Value: &aapGUID},
				}
				if err := db.UpdateAgent(cmdCtx.DB, agent.AgentID, updates); err != nil {
					failed = append(failed, fmt.Sprintf("%s (db update failed: %v)", agent.AgentID, err))
					continue
				}

				// Persist to JSONL
				if err := db.AppendAgentUpdate(cmdCtx.Project.DBPath, db.AgentUpdateJSONLRecord{
					AgentID: agent.AgentID,
					AAPGUID: &aapGUID,
				}); err != nil {
					failed = append(failed, fmt.Sprintf("%s (jsonl update failed: %v)", agent.AgentID, err))
					continue
				}

				migrated = append(migrated, agent.AgentID)
			}

			if cmdCtx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"migrated": migrated,
					"skipped":  skipped,
					"failed":   failed,
				})
			}

			out := cmd.OutOrStdout()
			if len(migrated) > 0 {
				fmt.Fprintf(out, "Migrated %d agents:\n", len(migrated))
				for _, a := range migrated {
					fmt.Fprintf(out, "  ✓ @%s\n", a)
				}
			}
			if len(skipped) > 0 {
				fmt.Fprintf(out, "\nSkipped %d agents (already have AAP identity):\n", len(skipped))
				for _, a := range skipped {
					fmt.Fprintf(out, "  - @%s\n", a)
				}
			}
			if len(failed) > 0 {
				fmt.Fprintf(out, "\nFailed %d agents:\n", len(failed))
				for _, a := range failed {
					fmt.Fprintf(out, "  ✗ %s\n", a)
				}
			}
			if len(migrated) == 0 && len(skipped) == 0 && len(failed) == 0 {
				fmt.Fprintln(out, "No agents to migrate")
			}

			return nil
		},
	}

	cmd.Flags().String("agent", "", "migrate specific agent only")
	cmd.Flags().Bool("with-key", false, "generate keypairs (prompts for passphrase)")

	return cmd
}
