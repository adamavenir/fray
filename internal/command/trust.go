package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adamavenir/fray/internal/aap"
	"github.com/adamavenir/fray/internal/core"
	"github.com/spf13/cobra"
)

// NewTrustCmd creates the parent trust command.
func NewTrustCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Manage AAP trust attestations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		NewTrustGrantCmd(),
		NewTrustVerifyCmd(),
		NewTrustListCmd(),
		NewTrustRevokeCmd(),
	)

	return cmd
}

// NewTrustGrantCmd grants trust to an agent.
func NewTrustGrantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grant <subject>",
		Short: "Grant trust to an agent",
		Long: `Create a signed trust attestation granting capabilities to an agent.

The issuer must have an AAP identity with a keypair. You will be prompted
for the passphrase to sign the attestation.

Examples:
  fray trust grant @dev --capabilities write --scope "." --as adam
  fray trust grant @dev --capabilities read,write --scope "github.com/org/*" --as admin
  fray trust grant @pm --capabilities delegate --scope "*" --as adam`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdCtx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer cmdCtx.DB.Close()

			subject := args[0]
			if !strings.HasPrefix(subject, "@") {
				subject = "@" + subject
			}

			issuer, _ := cmd.Flags().GetString("as")
			if issuer == "" {
				return writeCommandError(cmd, fmt.Errorf("--as <issuer> is required"))
			}
			issuer = core.NormalizeAgentRef(issuer)

			capabilities, _ := cmd.Flags().GetStringSlice("capabilities")
			if len(capabilities) == 0 {
				return writeCommandError(cmd, fmt.Errorf("--capabilities is required (e.g., read,write,execute,deploy,delegate,admin)"))
			}

			scope, _ := cmd.Flags().GetString("scope")
			if scope == "" {
				scope = "."
			}

			expiresIn, _ := cmd.Flags().GetDuration("expires")

			aapDir, err := core.AAPConfigDir()
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("get AAP config dir: %w", err))
			}

			registry, err := aap.NewFileRegistry(filepath.Join(aapDir, "agents"))
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("create registry: %w", err))
			}

			// Verify issuer has a key
			issuerIdentity, err := registry.Get(issuer)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("issuer @%s not found in AAP registry", issuer))
			}
			if !issuerIdentity.HasKey {
				return writeCommandError(cmd, fmt.Errorf("issuer @%s has no keypair - run 'fray agent keygen %s' first", issuer, issuer))
			}

			passphrase, err := promptPassphrase(fmt.Sprintf("Enter passphrase for @%s: ", issuer))
			if err != nil {
				return writeCommandError(cmd, err)
			}

			claim := aap.TrustClaim{
				Subject:      subject,
				Capabilities: capabilities,
				Scope:        scope,
			}
			if expiresIn > 0 {
				claim.ExpiresIn = expiresIn
			}

			attestation, err := aap.Attest(registry, issuer, passphrase, claim)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("create attestation: %w", err))
			}

			// Save attestation
			store, err := aap.NewFileAttestationStore(filepath.Join(aapDir, "attestations"))
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("create attestation store: %w", err))
			}

			if err := store.SaveAttestation(attestation); err != nil {
				return writeCommandError(cmd, fmt.Errorf("save attestation: %w", err))
			}

			if cmdCtx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"id":           attestation.Record.ID,
					"subject":      attestation.Record.Subject,
					"issuer":       attestation.Record.Issuer,
					"capabilities": attestation.Record.Capabilities,
					"scope":        attestation.Record.Scope,
					"expires_at":   attestation.Record.ExpiresAt,
				})
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Created attestation %s\n", attestation.Record.ID)
			fmt.Fprintf(out, "  Subject: %s\n", attestation.Record.Subject)
			fmt.Fprintf(out, "  Issuer: %s\n", attestation.Record.Issuer)
			fmt.Fprintf(out, "  Capabilities: %s\n", strings.Join(attestation.Record.Capabilities, ", "))
			fmt.Fprintf(out, "  Scope: %s\n", attestation.Record.Scope)
			if attestation.Record.ExpiresAt != nil {
				fmt.Fprintf(out, "  Expires: %s\n", *attestation.Record.ExpiresAt)
			}

			return nil
		},
	}

	cmd.Flags().String("as", "", "issuer agent (required)")
	cmd.Flags().StringSlice("capabilities", nil, "capabilities to grant (read,write,execute,deploy,delegate,admin)")
	cmd.Flags().String("scope", ".", "scope pattern (e.g., github.com/org/*, .)")
	cmd.Flags().Duration("expires", 0, "expiration duration (e.g., 24h, 7d)")

	return cmd
}

// NewTrustVerifyCmd verifies trust for an agent.
func NewTrustVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify <agent> <capability> [scope]",
		Short: "Verify agent has capability in scope",
		Long: `Check if an agent has a specific capability within a scope.

Returns success if a valid trust chain exists, failure otherwise.

Examples:
  fray trust verify @dev write .
  fray trust verify @dev deploy github.com/org/repo`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdCtx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer cmdCtx.DB.Close()

			addr := args[0]
			if !strings.HasPrefix(addr, "@") {
				addr = "@" + addr
			}

			capability := args[1]

			scope := "."
			if len(args) > 2 {
				scope = args[2]
			}

			aapDir, err := core.AAPConfigDir()
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("get AAP config dir: %w", err))
			}

			projectAAPDir := filepath.Join(cmdCtx.Project.Root, ".aap")
			frayDir := filepath.Dir(cmdCtx.Project.DBPath)

			resolver, err := aap.NewResolver(aap.ResolverOpts{
				GlobalRegistry:  aapDir,
				ProjectRegistry: projectAAPDir,
				FrayCompat:      true,
				FrayPath:        frayDir,
			})
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("create resolver: %w", err))
			}

			store, err := aap.NewFileAttestationStore(aapDir)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("create attestation store: %w", err))
			}

			result, err := aap.VerifyTrust(resolver, store, addr, capability, scope)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("verify trust: %w", err))
			}

			if cmdCtx.JSONMode {
				output := map[string]any{
					"address":    addr,
					"capability": capability,
					"scope":      scope,
					"verified":   result.Verified,
				}
				if result.Reason != "" {
					output["reason"] = result.Reason
				}
				if len(result.Attestations) > 0 {
					var attIDs []string
					for _, att := range result.Attestations {
						attIDs = append(attIDs, att.Record.ID)
					}
					output["attestation_chain"] = attIDs
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(output)
			}

			out := cmd.OutOrStdout()
			if result.Verified {
				fmt.Fprintf(out, "✓ %s has %s in scope %s\n", addr, capability, scope)
				if len(result.Attestations) > 0 {
					fmt.Fprintln(out, "\nAttestation chain:")
					for _, att := range result.Attestations {
						fmt.Fprintf(out, "  %s: %s -> %s (%s)\n",
							att.Record.ID, att.Record.Issuer, att.Record.Subject,
							strings.Join(att.Record.Capabilities, ","))
					}
				}
			} else {
				fmt.Fprintf(out, "✗ %s does NOT have %s in scope %s\n", addr, capability, scope)
				if result.Reason != "" {
					fmt.Fprintf(out, "  Reason: %s\n", result.Reason)
				}
			}

			return nil
		},
	}

	return cmd
}

// NewTrustListCmd lists trust attestations.
func NewTrustListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List trust attestations",
		Long: `List trust attestations, optionally filtered by subject or issuer.

Examples:
  fray trust list                    # List all attestations
  fray trust list --for @dev         # Attestations granted to @dev
  fray trust list --from @adam       # Attestations issued by @adam`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdCtx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer cmdCtx.DB.Close()

			forAgent, _ := cmd.Flags().GetString("for")
			fromAgent, _ := cmd.Flags().GetString("from")

			aapDir, err := core.AAPConfigDir()
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("get AAP config dir: %w", err))
			}

			store, err := aap.NewFileAttestationStore(aapDir)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("open attestation store: %w", err))
			}

			// List all agents and collect their attestations
			agentsDir := filepath.Join(aapDir, "agents")
			entries, err := os.ReadDir(agentsDir)
			if err != nil && !os.IsNotExist(err) {
				return writeCommandError(cmd, fmt.Errorf("read agents dir: %w", err))
			}

			var attestations []*aap.Attestation
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				agentAtts, err := store.LoadAttestations(entry.Name())
				if err != nil {
					continue
				}
				attestations = append(attestations, agentAtts...)
			}

			// Filter
			var filtered []*aap.Attestation
			for _, att := range attestations {
				if forAgent != "" {
					subject := core.NormalizeAgentRef(forAgent)
					if !strings.HasPrefix(subject, "@") {
						subject = "@" + subject
					}
					if att.Record.Subject != subject {
						continue
					}
				}
				if fromAgent != "" {
					issuer := core.NormalizeAgentRef(fromAgent)
					if !strings.HasPrefix(issuer, "@") {
						issuer = "@" + issuer
					}
					if att.Record.Issuer != issuer {
						continue
					}
				}
				filtered = append(filtered, att)
			}

			if cmdCtx.JSONMode {
				var output []map[string]any
				for _, att := range filtered {
					entry := map[string]any{
						"id":           att.Record.ID,
						"subject":      att.Record.Subject,
						"issuer":       att.Record.Issuer,
						"capabilities": att.Record.Capabilities,
						"scope":        att.Record.Scope,
						"issued_at":    att.Record.IssuedAt,
					}
					if att.Record.ExpiresAt != nil {
						entry["expires_at"] = *att.Record.ExpiresAt
					}
					output = append(output, entry)
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(output)
			}

			out := cmd.OutOrStdout()
			if len(filtered) == 0 {
				fmt.Fprintln(out, "No attestations found")
				return nil
			}

			for _, att := range filtered {
				expires := ""
				if att.Record.ExpiresAt != nil {
					t, err := time.Parse(time.RFC3339, *att.Record.ExpiresAt)
					if err == nil && t.Before(time.Now()) {
						expires = " [EXPIRED]"
					}
				}
				fmt.Fprintf(out, "%s: %s -> %s%s\n", att.Record.ID, att.Record.Issuer, att.Record.Subject, expires)
				fmt.Fprintf(out, "  Capabilities: %s\n", strings.Join(att.Record.Capabilities, ", "))
				fmt.Fprintf(out, "  Scope: %s\n", att.Record.Scope)
			}

			return nil
		},
	}

	cmd.Flags().String("for", "", "filter by subject agent")
	cmd.Flags().String("from", "", "filter by issuer agent")

	return cmd
}

// NewTrustRevokeCmd revokes a trust attestation.
func NewTrustRevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <attestation-id>",
		Short: "Revoke a trust attestation",
		Long: `Revoke a previously issued trust attestation.

Only the original issuer can revoke an attestation. You will be prompted
for the passphrase to sign the revocation.

Examples:
  fray trust revoke att-abc12345 --as adam --reason "Role changed"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdCtx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer cmdCtx.DB.Close()

			attestationID := args[0]

			issuer, _ := cmd.Flags().GetString("as")
			if issuer == "" {
				return writeCommandError(cmd, fmt.Errorf("--as <issuer> is required"))
			}
			issuer = core.NormalizeAgentRef(issuer)

			reason, _ := cmd.Flags().GetString("reason")

			aapDir, err := core.AAPConfigDir()
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("get AAP config dir: %w", err))
			}

			registry, err := aap.NewFileRegistry(filepath.Join(aapDir, "agents"))
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("create registry: %w", err))
			}

			passphrase, err := promptPassphrase(fmt.Sprintf("Enter passphrase for @%s: ", issuer))
			if err != nil {
				return writeCommandError(cmd, err)
			}

			revocation, err := aap.Revoke(registry, issuer, passphrase, attestationID, reason)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("revoke: %w", err))
			}

			// Save revocation
			store, err := aap.NewFileAttestationStore(filepath.Join(aapDir, "attestations"))
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("create attestation store: %w", err))
			}

			if err := store.SaveRevocation(revocation); err != nil {
				return writeCommandError(cmd, fmt.Errorf("save revocation: %w", err))
			}

			if cmdCtx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"id":             revocation.Record.ID,
					"attestation_id": revocation.Record.AttestationID,
					"issuer":         revocation.Record.Issuer,
					"reason":         revocation.Record.Reason,
				})
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Revoked attestation %s\n", attestationID)
			fmt.Fprintf(out, "  Revocation ID: %s\n", revocation.Record.ID)
			if reason != "" {
				fmt.Fprintf(out, "  Reason: %s\n", reason)
			}

			return nil
		},
	}

	cmd.Flags().String("as", "", "issuer agent (required)")
	cmd.Flags().String("reason", "", "reason for revocation")

	return cmd
}
