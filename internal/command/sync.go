package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adamavenir/fray/internal/db"
	"github.com/spf13/cobra"
)

type syncStatusResult struct {
	Configured   bool   `json:"configured"`
	Backend      string `json:"backend,omitempty"`
	Path         string `json:"path,omitempty"`
	SharedPath   string `json:"shared_path,omitempty"`
	SharedTarget string `json:"shared_target,omitempty"`
	IsSymlink    bool   `json:"is_symlink"`
}

// NewSyncCmd creates the sync command.
func NewSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Manage sync backends",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		NewSyncStatusCmd(),
		NewSyncSetupCmd(),
	)

	return cmd
}

// NewSyncStatusCmd reports current sync configuration.
func NewSyncStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show sync configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			config := ctx.ProjectConfig
			var backend string
			var path string
			configured := false
			if config != nil && config.Sync != nil {
				configured = true
				backend = config.Sync.Backend
				path = config.Sync.Path
			}

			sharedPath := filepath.Join(ctx.Project.Root, ".fray", "shared")
			var sharedTarget string
			isSymlink := false
			if info, err := os.Lstat(sharedPath); err == nil {
				if info.Mode()&os.ModeSymlink != 0 {
					isSymlink = true
					if target, err := os.Readlink(sharedPath); err == nil {
						sharedTarget = target
					}
				}
			}

			if ctx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(syncStatusResult{
					Configured:   configured,
					Backend:      backend,
					Path:         path,
					SharedPath:   sharedPath,
					SharedTarget: sharedTarget,
					IsSymlink:    isSymlink,
				})
			}

			if !configured {
				fmt.Fprintln(cmd.OutOrStdout(), "Sync: not configured")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Backend: %s\n", backend)
				fmt.Fprintf(cmd.OutOrStdout(), "Path:    %s\n", path)
			}

			if isSymlink {
				fmt.Fprintf(cmd.OutOrStdout(), "Shared:  %s -> %s\n", sharedPath, sharedTarget)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Shared:  %s\n", sharedPath)
			}
			return nil
		},
	}

	return cmd
}

// NewSyncSetupCmd configures a sync backend and creates shared symlink.
func NewSyncSetupCmd() *cobra.Command {
	var useICloud bool
	var useDropbox bool
	var customPath string

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure sync backend",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			backend, basePath, err := resolveSyncSetupTarget(useICloud, useDropbox, customPath)
			if err != nil {
				return writeCommandError(cmd, err)
			}

			channelName := channelNameForSync(ctx.Project.Root, ctx.ProjectConfig)
			targetPath := filepath.Join(basePath, channelName, "shared")
			if err := ensureSharedSymlink(ctx.Project.Root, targetPath, ctx.Force); err != nil {
				return writeCommandError(cmd, err)
			}

			syncConfig := &db.ProjectSyncConfig{
				Backend: backend,
				Path:    targetPath,
			}
			if _, err := db.UpdateProjectConfig(ctx.Project.DBPath, db.ProjectConfig{Sync: syncConfig}); err != nil {
				return writeCommandError(cmd, err)
			}

			if ctx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(syncStatusResult{
					Configured:   true,
					Backend:      backend,
					Path:         targetPath,
					SharedPath:   filepath.Join(ctx.Project.Root, ".fray", "shared"),
					SharedTarget: targetPath,
					IsSymlink:    true,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Linked .fray/shared to %s\n", targetPath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&useICloud, "icloud", false, "use iCloud Drive for sync")
	cmd.Flags().BoolVar(&useDropbox, "dropbox", false, "use Dropbox for sync")
	cmd.Flags().StringVar(&customPath, "path", "", "custom sync base path")

	return cmd
}

func resolveSyncSetupTarget(useICloud, useDropbox bool, customPath string) (string, string, error) {
	count := 0
	if useICloud {
		count++
	}
	if useDropbox {
		count++
	}
	if customPath != "" {
		count++
	}
	if count == 0 {
		return "", "", fmt.Errorf("choose one of --icloud, --dropbox, or --path")
	}
	if count > 1 {
		return "", "", fmt.Errorf("choose only one sync backend")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	switch {
	case useICloud:
		return "icloud", filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "fray-sync"), nil
	case useDropbox:
		return "dropbox", filepath.Join(home, "Dropbox", "fray-sync"), nil
	default:
		resolved, err := expandUserPath(customPath, home)
		if err != nil {
			return "", "", err
		}
		return "path", resolved, nil
	}
}

func channelNameForSync(projectRoot string, config *db.ProjectConfig) string {
	if config != nil && config.ChannelName != "" {
		return config.ChannelName
	}
	return filepath.Base(projectRoot)
}

func expandUserPath(path, home string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	if path == "~" {
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func ensureSharedSymlink(projectRoot, targetPath string, force bool) error {
	sharedPath := filepath.Join(projectRoot, ".fray", "shared")
	if filepath.Clean(sharedPath) == filepath.Clean(targetPath) {
		return nil
	}

	info, err := os.Lstat(sharedPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			current, err := os.Readlink(sharedPath)
			if err != nil {
				return err
			}
			if filepath.Clean(current) == filepath.Clean(targetPath) {
				return nil
			}
			if !force {
				return fmt.Errorf("shared is already linked to %s (use --force to replace)", current)
			}
			if err := os.Remove(sharedPath); err != nil {
				return err
			}
		} else if info.IsDir() {
			if err := moveSharedDir(sharedPath, targetPath, force); err != nil {
				return err
			}
		} else {
			if !force {
				return fmt.Errorf("shared path exists and is not a directory")
			}
			if err := os.Remove(sharedPath); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(sharedPath), 0o755); err != nil {
		return err
	}
	return os.Symlink(targetPath, sharedPath)
}

func moveSharedDir(sharedPath, targetPath string, force bool) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	if info, err := os.Stat(targetPath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("sync target exists and is not a directory: %s", targetPath)
		}
		empty, err := dirIsEmpty(targetPath)
		if err != nil {
			return err
		}
		if !empty && !force {
			return fmt.Errorf("sync target already exists: %s", targetPath)
		}
		if err := os.RemoveAll(targetPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return os.Rename(sharedPath, targetPath)
}

func dirIsEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
