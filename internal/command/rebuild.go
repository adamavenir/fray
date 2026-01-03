package command

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/spf13/cobra"
)

// NewRebuildCmd creates the rebuild command.
func NewRebuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild database from JSONL files",
		Long: `Rebuild the SQLite cache from the authoritative JSONL files.

Use this command when:
- You see schema errors (e.g., "no such column")
- The database is corrupted
- After manually editing JSONL files
- After a git pull with JSONL changes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Don't use GetContext - it tries to open the DB which may fail
			// Just discover the project and delete/rebuild the DB directly
			project, err := core.DiscoverProject("")
			if err != nil {
				return writeCommandError(cmd, err)
			}

			dbPath := project.DBPath

			// Delete existing db files
			os.Remove(dbPath)
			os.Remove(dbPath + "-wal")
			os.Remove(dbPath + "-shm")

			// Open fresh - this will trigger rebuild from JSONL
			newDB, err := db.OpenDatabase(project)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("rebuild: %w", err))
			}
			defer newDB.Close()

			jsonMode, _ := cmd.Flags().GetBool("json")
			if jsonMode {
				json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"status": "rebuilt"})
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Database rebuilt from JSONL")
			}
			return nil
		},
	}

	return cmd
}
