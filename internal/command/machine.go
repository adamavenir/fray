package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adamavenir/fray/internal/db"
	"github.com/spf13/cobra"
)

// NewMachineCmd creates the machine parent command.
func NewMachineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "machine",
		Short: "Manage machines",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(NewMachineRenameCmd())
	return cmd
}

// NewMachineRenameCmd renames a machine and records an alias mapping.
func NewMachineRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a machine",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldID := args[0]
			newID := args[1]
			if oldID == "" || newID == "" {
				return writeCommandError(cmd, fmt.Errorf("machine ids required"))
			}
			if oldID == newID {
				return writeCommandError(cmd, fmt.Errorf("old and new machine ids match"))
			}

			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			if !db.IsMultiMachineMode(ctx.Project.DBPath) {
				return writeCommandError(cmd, fmt.Errorf("machine rename is only supported in multi-machine projects"))
			}

			if !db.MachineIDExists(ctx.Project.DBPath, oldID) {
				return writeCommandError(cmd, fmt.Errorf("machine id not found: %s", oldID))
			}
			if db.MachineIDExists(ctx.Project.DBPath, newID) {
				return writeCommandError(cmd, fmt.Errorf("machine id already exists: %s", newID))
			}

			config, err := db.ReadProjectConfig(ctx.Project.DBPath)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			if config != nil {
				if alias, ok := config.MachineAliases[oldID]; ok {
					if alias == newID {
						fmt.Fprintf(cmd.OutOrStdout(), "Machine %s already renamed to %s\n", oldID, newID)
						return nil
					}
					return writeCommandError(cmd, fmt.Errorf("machine id already aliased: %s", oldID))
				}
			}

			if _, err := db.UpdateProjectConfig(ctx.Project.DBPath, db.ProjectConfig{
				MachineAliases: map[string]string{oldID: newID},
			}); err != nil {
				return writeCommandError(cmd, err)
			}

			frayDir := filepath.Dir(ctx.Project.DBPath)
			oldDir := filepath.Join(frayDir, "shared", "machines", oldID)
			newDir := filepath.Join(frayDir, "shared", "machines", newID)
			if err := os.Rename(oldDir, newDir); err != nil {
				return writeCommandError(cmd, err)
			}

			localID := db.GetLocalMachineID(ctx.Project.DBPath)
			if localID == oldID {
				if err := updateLocalMachineID(filepath.Join(frayDir, "local", "machine-id"), newID); err != nil {
					return writeCommandError(cmd, err)
				}
			}

			if err := db.RebuildDatabaseFromJSONL(ctx.DB, ctx.Project.DBPath); err != nil {
				return writeCommandError(cmd, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Renamed machine %s → %s\n", oldID, newID)
			return nil
		},
	}
}

func updateLocalMachineID(path, newID string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	record := map[string]any{}
	if err := json.Unmarshal(data, &record); err != nil {
		return err
	}
	record["id"] = newID
	updated, err := json.Marshal(record)
	if err != nil {
		return err
	}
	updated = append(updated, '\n')
	return os.WriteFile(path, updated, 0o644)
}
