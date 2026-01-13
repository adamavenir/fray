# AAP Usage Guide

Agent Addressing Protocol (AAP) provides a global identity system for AI agents. Think of it like SSH keys for agents.

## Quick Start

```bash
# Create a new agent with AAP identity
fray new alice

# View the agent's AAP identity
fray agent identity alice

# Generate a keypair for signing (optional but required for trust)
fray agent keygen alice
```

## Address Format

AAP addresses follow a hierarchical structure:

| Format | Example | Description |
|--------|---------|-------------|
| `@agent` | `@alice` | Local agent |
| `@agent.variant` | `@alice.frontend` | Agent with role variant |
| `@agent[job-n]` | `@dev[xyz1-0]` | Job worker (swarm agent) |
| `@agent@host` | `@alice@server` | Cross-machine (Phase 6) |

## Commands

### Identity Management

```bash
# Create agent with AAP identity (auto-creates)
fray new alice

# Create managed agent with daemon config + AAP
fray agent create dev --driver claude

# View AAP identity details
fray agent identity alice

# Generate keypair for existing agent
fray agent keygen alice

# Resolve address to see full identity + trust info
fray agent resolve @alice
```

### Trust Management

Trust attestations are cryptographically signed statements that grant capabilities.

```bash
# Grant capabilities to an agent (requires keypair)
fray trust grant @dev --capabilities write --scope "." --as admin

# Verify an agent has a capability
fray trust verify @dev write .

# List attestations for an agent
fray trust list --for @dev

# List attestations issued by an agent
fray trust list --from @admin

# Revoke an attestation
fray trust revoke att-abc123 --as admin
```

### Capabilities

Standard capabilities:
- `read` - Read access
- `write` - Write access
- `execute` - Run commands
- `deploy` - Deploy to production
- `delegate` - Grant capabilities to others
- `admin` - Full administrative access

### Scopes

Scopes limit where capabilities apply:
- `.` - Current project only
- `github.com/org/*` - All repos in org
- `*` - Universal (all scopes)

## Migration

Migrate existing agents to AAP:

```bash
# Migrate all agents (without keys)
fray migrate-aap

# Migrate specific agent with keypair
fray migrate-aap --agent legacy --with-key
```

## Storage Layout

```
~/.config/aap/
├── agents/
│   └── alice/
│       ├── identity.json   # Public identity
│       └── private.key     # Encrypted private key (if generated)
└── attestations/
    └── ...                 # Trust attestations
```

## Trust Levels

Agents have a trust level based on their identity:

| Level | Description | Trust Verification |
|-------|-------------|-------------------|
| `full` | Has verified keypair | Works |
| `untrusted` | Legacy agent, no keys | Fails |

Legacy agents (created before AAP) work normally but cannot participate in trust verification.

## Integration with Fray

AAP integrates with fray commands:

- `fray new` - Creates both fray record and AAP identity
- `fray agent create` - Creates managed agent with AAP identity
- `fray agent list` - Shows `[AAP]` tag for agents with identity
- Daemon logs trust level when spawning agents

## Best Practices

1. **Generate keys for important agents** - Use `fray agent keygen` for agents that need to issue or verify trust
2. **Use scopes** - Grant minimal capabilities with specific scopes
3. **Migrate legacy agents** - Run `fray migrate-aap` to backfill identities

## See Also

- `docs/AAP-SPEC.md` - Protocol specification
- `docs/AAP-IMPL-PLAN.md` - Implementation details
