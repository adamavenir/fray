# Agent Address Protocol (AAP) Specification

**Version:** 0.1.0-draft
**Status:** Draft
**Authors:** @architect, @adam

## Abstract

The Agent Address Protocol (AAP) defines a federated identity and addressing system for AI agents. It provides a universal grammar for agent addresses, identity registration, resolution, and trust attestations. AAP is designed to be consumed by messaging systems (like fray), orchestrators, IDEs, and any tool that needs to identify or invoke agents.

## Table of Contents

1. [Address Grammar](#1-address-grammar)
2. [Identity Registration](#2-identity-registration)
3. [Resolution Protocol](#3-resolution-protocol)
4. [Variant Context](#4-variant-context)
5. [Trust & Attestations](#5-trust--attestations)
6. [Cryptographic Primitives](#6-cryptographic-primitives)
7. [Storage Layout](#7-storage-layout)
8. [Federation](#8-federation)
9. [Reference Implementation](#9-reference-implementation)

---

## 1. Address Grammar

### 1.1 Canonical Format

```
@{agent}.{variant...}[job-idx]@location#session
```

### 1.2 Components

| Component | Required | Description | Examples |
|-----------|----------|-------------|----------|
| `@` | Yes | Address prefix | Always literal `@` |
| `{agent}` | Yes | Base identity name | `dev`, `opus`, `pm` |
| `.{variant}` | No | Namespace qualifiers (extensible) | `.frontend`, `.mlld.trusted` |
| `[job-idx]` | No | Ephemeral job worker | `[abc1-2]` (job suffix + index) |
| `@location` | No | Registry location: machine name OR domain | `@workstation`, `@anthropic.com`, `@github.com/org/repo` |
| `#session` | No | Specific context window | `#a7f3`, `#6cac3000` |

### 1.3 Parsing Rules

1. **Agent name**: Starts after initial `@`, ends at first `.`, `[`, second `@`, `#`, or end of string
2. **Variants**: Everything from first `.` to `[`, `@`, `#`, or end. Can be multi-level (e.g., `.project.role.trust`)
3. **Job reference**: Enclosed in `[]`, format is `{suffix}-{index}` where suffix is typically first 4 chars of job GUID
4. **Location**: Starts after second `@`, ends at `#` or end. Can be machine name, domain, or git repo path. Local resolution always tried first; remote only when explicitly addressed
5. **Session**: Starts after `#`, extends to end. Can be prefix-matched (any length)

### 1.4 Examples

```
@dev                              # Base agent, local host
@dev.frontend                     # Agent with variant
@dev.frontend.trusted             # Multiple variant levels
@dev.frontend[abc1-2]             # Job worker #2 in job abc1...
@dev.frontend@workstation         # Specific host (machine)
@devrel.mlld@anthropic.com        # Domain as host (registry)
@pm@github.com/team/shared        # Git repo as host
@dev.frontend[abc1-2]@server#a7f3 # Fully qualified
```

### 1.5 Normalization

**Everything lowercase.** All address components are normalized to lowercase before parsing, comparison, storage, and use in attestations:

- Agent names, variants, locations, session IDs: all lowercase
- No exceptions for "case-sensitive variants" - simplicity over flexibility

Implementations MUST normalize to lowercase. The ABNF grammar defines the canonical (post-normalization) format.

Note: Raw string content in metadata fields (e.g., `display_name`, `description`) preserves original case.

---

## 2. Identity Registration

### 2.1 Creating an Identity

```bash
aap register <agent-name>
```

This generates:
1. A globally unique identifier (GUID)
2. An Ed25519 keypair
3. An identity record

### 2.2 Identity Record

```json
{
  "version": "0.1.0",
  "type": "identity",
  "guid": "aap-a1b2c3d4e5f6g7h8",
  "address": "@dev",
  "agent": "dev",
  "public_key": {
    "algorithm": "ed25519",
    "key": "base64-encoded-public-key"
  },
  "created_at": "2024-01-15T12:00:00Z",
  "metadata": {
    "display_name": "Development Agent",
    "description": "General development assistant"
  }
}
```

### 2.3 GUID Format

AAP GUIDs use the format: `aap-{16-char-base36}`

Example: `aap-a1b2c3d4e5f6g7h8`

### 2.4 Agent Naming Rules

- Must start with a lowercase letter
- Can contain lowercase letters, numbers, hyphens
- Cannot start or end with hyphen
- Length: 1-64 characters
- Reserved names: `all`, `system`, `root`, `admin`

---

## 3. Resolution Protocol

### 3.1 Resolution Flow

Resolution behavior depends on whether `@host` is present in the address.

#### Without @host (e.g., `@devrel.mlld`)

Resolution is LOCAL ONLY:

```
┌──────────────────────────────────────────────────────────┐
│              Resolution Order (no @host)                  │
├──────────────────────────────────────────────────────────┤
│ 1. Local registry (~/.config/aap/agents/)                │
│    └─ Check for agents/<agent>/identity.json             │
│                                                           │
│ 2. Project registry (.aap/ or compatible like .fray/)    │
│    └─ Check for agents/<agent>/identity.json             │
│                                                           │
│ 3. STOP - no remote lookup without explicit @host        │
└──────────────────────────────────────────────────────────┘
```

#### With @host (e.g., `@devrel@anthropic.com`)

Resolution queries the specified host:

```
┌──────────────────────────────────────────────────────────┐
│              Resolution Order (with @host)                │
├──────────────────────────────────────────────────────────┤
│ 1. Check if host is local machine name                   │
│    └─ If yes, use local registry                         │
│                                                           │
│ 2. Check if host is a domain                             │
│    └─ GET https://<host>/.well-known/aap/agents/devrel   │
│                                                           │
│ 3. Check if host is a git repo                           │
│    └─ Fetch .aap/ from the repository                    │
└──────────────────────────────────────────────────────────┘
```

This design ensures local-only addresses never trigger network requests.

### 3.2 Resolution Response

```json
{
  "version": "0.1.0",
  "type": "resolution",
  "identity": {
    "guid": "aap-a1b2c3d4e5f6g7h8",
    "address": "@devrel",
    "public_key": { "algorithm": "ed25519", "key": "..." }
  },
  "invoke": {
    "driver": "claude",
    "model": "opus-4.5",
    "prompt_delivery": "args"
  },
  "variants": {
    "mlld": {
      "context_uri": "git://github.com/user/mlld/.aap/variants/devrel/",
      "context_ref": "v2.0.0",
      "context_sha": "a1b2c3d4e5f6...",
      "trust_overrides": []
    },
    "fray": {
      "context_uri": "git://github.com/adamavenir/fray/.aap/variants/devrel/",
      "context_ref": "main",
      "context_sha": "f6e5d4c3b2a1..."
    }
  },
  "registry_hosts": ["@workstation", "@server", "@github.com/team/shared"],
  "attestations": [
    { "ref": "attestations/from-adam.json" }
  ]
}
```

### 3.3 Variant Resolution

When resolving `@devrel.mlld`:

1. Resolve base identity `@devrel`
2. Look up variant `mlld` in the variants map
3. Fetch variant context from `context_uri`
4. Merge variant-specific configuration with base

### 3.4 Host Resolution

When resolving `@dev@workstation`:

1. Resolve base identity `@dev`
2. Verify `@workstation` is in the `registry_hosts` list
3. Return host-specific invoke configuration if present

**`registry_hosts` Format:**

The `registry_hosts` array contains full host addresses (with `@` prefix) normalized to lowercase:

```json
"registry_hosts": ["@workstation", "@server", "@github.com/team/shared"]
```

Implementations MUST compare the address's `@host` component against entries using normalized (lowercase) string comparison. The `@` prefix ensures unambiguous parsing when hosts contain dots or slashes.

### 3.5 Host Types

The `@host` component specifies WHERE the agent's identity is REGISTERED, not where code executes. Think of it as "which registry holds this identity."

| Protocol | Host Format | Resolution |
|----------|-------------|------------|
| Local | `@workstation` | Machine name, use local registry |
| Domain | `@anthropic.com` | HTTPS well-known endpoint |
| Git | `@github.com/org/repo` | Fetch `.aap/` from repository |

#### Git Host Resolution

For git-based hosts, implementations MUST support integrity verification:

```json
{
  "context_uri": "git://github.com/team/shared/.aap/variants/dev/",
  "context_ref": "v1.2.0",
  "context_sha": "abc123def456..."
}
```

| Field | Description |
|-------|-------------|
| `context_uri` | Resource location (see format below) |
| `context_ref` | Git ref (tag, branch, or commit) - RECOMMENDED to use tags |
| `context_sha` | Expected commit SHA - verification fails if mismatch |

**`context_uri` Format:**

| Host Type | Format | Example |
|-----------|--------|---------|
| Git repo | `git://<host>/<owner>/<repo>/<path>` | `git://github.com/team/shared/.aap/variants/dev/` |
| HTTPS | `https://<domain>/<path>` | `https://anthropic.com/.well-known/aap/variants/devrel/` |
| Local | `file://<path>` | `file:///home/user/.config/aap/variants/dev/` |

Implementations MUST support `git://` and `https://` schemes. The `file://` scheme is for local development only.

For `git://` URIs, implementations SHOULD use native git transport (clone/fetch) when available, falling back to HTTPS archive download.

When `context_sha` is present, implementations MUST verify the fetched content matches. This prevents TOCTOU attacks on mutable refs.

#### Execution vs Registry

The `@host` specifies the REGISTRY, not execution location. Execution configuration is in the `invoke` block:

```json
{
  "invoke": {
    "driver": "claude",
    "execution_host": "workstation"
  }
}
```

Policy conditions like `exclude_execution_hosts` apply to `invoke.execution_host`, NOT to the registry `@host`.

---

## 4. Variant Context

### 4.1 Purpose

Variants allow a single agent identity to have project-specific or role-specific configurations. The variant namespace can represent:

- **Roles**: `@dev.frontend`, `@pm.lead`
- **Projects**: `@devrel.mlld`, `@devrel.fray`
- **Trust levels**: `@deploy.staging`, `@deploy.prod`
- **Combinations**: `@dev.frontend.trusted`

### 4.2 Context Package Structure

```
.aap/variants/{agent}/
├── context.json         # Variant metadata
├── system-prompt.md     # Base system prompt additions
├── knowledge/           # Reference materials
│   ├── architecture.md
│   └── conventions.md
└── attestations/        # Variant-specific trust grants
    └── project-write.json
```

### 4.3 Context Manifest

```json
{
  "version": "0.1.0",
  "type": "variant_context",
  "agent": "devrel",
  "variant": "mlld",
  "description": "mlld project specialist",
  "files": [
    "system-prompt.md",
    "knowledge/architecture.md"
  ],
  "trust_grants": [
    "attestations/project-write.json"
  ]
}
```

### 4.4 Portable Variants

Variants can be referenced from any project:

```bash
# From the mlld project, define the variant
aap variant create devrel mlld --context ./.aap/variants/devrel/

# From another project, invoke the variant
aap resolve @devrel.mlld
# → Fetches context from mlld project
```

---

## 5. Trust & Attestations

### 5.1 Attestation Model

An attestation is a signed statement: "I (issuer) trust {subject} to do {capabilities} in {scope}."

### 5.2 Attestation Record

```json
{
  "version": "0.1.0",
  "type": "trust_attestation",
  "id": "att-x1y2z3w4",
  "subject": "@dev",
  "subject_guid": "aap-a1b2c3d4e5f6g7h8",
  "subject_key_id": "sha256:abc123...",
  "issuer": "@adam",
  "issuer_guid": "aap-x9y8z7w6v5u4t3s2",
  "issuer_key_id": "sha256:def456...",
  "capabilities": ["read", "write", "deploy"],
  "scope": "github.com/adamavenir/*",
  "conditions": {
    "require_variants": ["trusted"],
    "exclude_execution_hosts": ["prod-server"]
  },
  "issued_at": "2024-01-15T12:00:00Z",
  "expires_at": "2025-01-15T12:00:00Z",
  "signature": {
    "algorithm": "ed25519",
    "value": "base64-encoded-signature"
  }
}
```

### 5.2.1 Binding Fields

Attestations MUST include GUID and key fingerprint bindings to prevent name collision attacks in federated environments:

| Field | Description |
|-------|-------------|
| `subject_guid` | GUID of the subject identity |
| `subject_key_id` | SHA-256 fingerprint of subject's public key (`sha256:{hex}`) |
| `issuer_guid` | GUID of the issuer identity |
| `issuer_key_id` | SHA-256 fingerprint of issuer's public key |

Verifiers MUST:
1. Resolve both subject and issuer addresses
2. Verify `*_guid` matches resolved identity GUID
3. Verify `*_key_id` matches SHA-256 hash of resolved public key
4. Reject attestation if any binding fails

### 5.3 Standard Capabilities

| Capability | Description |
|------------|-------------|
| `read` | Read access to resources |
| `write` | Write/modify resources |
| `execute` | Run commands, scripts |
| `deploy` | Deploy to environments |
| `delegate` | Grant trust to others |
| `admin` | Administrative access |

### 5.4 Scope Patterns

Scopes use glob-like patterns:

```
github.com/adamavenir/*        # All repos under user
github.com/adamavenir/fray     # Specific repo
*.corp.com                     # All subdomains
/Users/adam/dev/*              # Local path pattern
*                              # Universal (use carefully)
```

### 5.5 Verification Flow

```
┌─────────────────────────────────────────────────────────────┐
│ VerifyTrust("@dev.frontend", "deploy", "github.com/.../fray") │
├─────────────────────────────────────────────────────────────┤
│ 1. Resolve @dev.frontend                                    │
│    └─ Get identity + attestations                           │
│                                                              │
│ 2. Filter attestations                                       │
│    └─ capability="deploy" AND scope matches target          │
│                                                              │
│ 3. Verify each attestation                                   │
│    ├─ Check signature against issuer's public key           │
│    ├─ Check not expired                                      │
│    ├─ Check GUID/key bindings match resolved identities     │
│    └─ Check conditions (see below)                           │
│                                                              │
│ 4. Check trust chains                                        │
│    └─ If issuer has "delegate", follow chain                │
│                                                              │
│ 5. Return result                                             │
│    └─ { verified: true, attestations: [...] }               │
└─────────────────────────────────────────────────────────────┘
```

#### Condition Evaluation

Conditions are evaluated against the execution context, NOT the registry address:

| Condition | Evaluated Against |
|-----------|-------------------|
| `require_variants` | The variant components in the resolved address (e.g., `.frontend` in `@dev.frontend`) |
| `exclude_execution_hosts` | The `invoke.execution_host` from the invoke config, NOT the `@host` from the address |
| `require_execution_hosts` | The `invoke.execution_host` from the invoke config |

Example: An attestation with `"exclude_execution_hosts": ["prod-server"]` blocks invocation when `invoke.execution_host == "prod-server"`, regardless of which registry (`@host`) was used to resolve the identity.

### 5.6 Trust Chains

Attestations can form chains:

```
@adam --[delegate]--> @pm --[write]--> @dev.frontend
```

When verifying `@dev.frontend` for `write`:
1. Find attestation from `@pm` granting `write`
2. Check if `@pm` has `delegate` capability (from `@adam`)
3. Verify both signatures

Chain depth limit: 5 (configurable)

### 5.7 Revocation

#### 5.7.1 Expiry-based

Attestations with `expires_at` automatically become invalid after that time.

#### 5.7.2 Explicit Revocation

```json
{
  "version": "0.1.0",
  "type": "trust_revocation",
  "id": "rev-a1b2c3d4",
  "revokes": "att-x1y2z3w4",
  "issuer": "@adam",
  "reason": "Role changed",
  "revoked_at": "2024-06-01T12:00:00Z",
  "signature": {
    "algorithm": "ed25519",
    "value": "base64-encoded-signature"
  }
}
```

Revocations are stored alongside attestations and checked during verification.

---

## 6. Cryptographic Primitives

### 6.1 Key Algorithm

AAP uses **Ed25519** for all cryptographic operations:
- 256-bit security level
- Fast signing and verification
- Compact signatures (64 bytes)
- Deterministic (same input = same signature)

### 6.2 Key Generation

```go
publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
```

### 6.3 Signing

```go
message := canonicalize(attestation)
signature := ed25519.Sign(privateKey, message)
```

### 6.4 Verification

```go
message := canonicalize(attestation)
valid := ed25519.Verify(publicKey, message, signature)
```

### 6.5 Canonicalization

AAP uses **RFC 8785 (JSON Canonicalization Scheme / JCS)** for deterministic serialization before signing.

Before signing, records are canonicalized per JCS:
1. Remove `signature` field from the record
2. Apply JCS canonicalization:
   - Object keys sorted lexicographically by UTF-16 code units
   - Numbers serialized per ECMAScript `JSON.stringify()` rules
   - No whitespace between tokens
   - Strings use minimal escaping (only required escapes)
   - Unicode characters above U+001F represented literally (UTF-8)
3. UTF-8 encode the result

**Reference**: RFC 8785 - JSON Canonicalization Scheme (JCS)
https://datatracker.ietf.org/doc/html/rfc8785

Implementations MUST use a JCS-compliant library. Ad-hoc "sort and stringify" is NOT sufficient for cross-implementation compatibility.

---

## 7. Storage Layout

### 7.1 Global Registry

```
~/.config/aap/
├── config.json              # AAP configuration
├── agents/
│   ├── dev/
│   │   ├── identity.json    # Identity record
│   │   ├── private.key      # Private key (encrypted)
│   │   └── attestations/
│   │       ├── from-adam.json
│   │       └── from-org.json
│   └── opus/
│       └── ...
├── hosts/
│   └── workstation.json     # Host registration
└── issued/                   # Attestations I've issued
    └── to-dev-from-me.json
```

### 7.2 Project Registry

```
.aap/
├── config.json              # Project AAP config
├── agents/                  # Project-local agents
│   └── legacy-fixer/
│       └── identity.json
└── variants/                # Variant contexts
    └── devrel/
        ├── context.json
        ├── system-prompt.md
        └── knowledge/
```

### 7.3 Compatibility

AAP can read from compatible formats:

- `.fray/agents.jsonl` → Parse agent records
- `.fray/fray-config.json` → Parse agent invoke config

---

## 8. Federation

### 8.1 Well-Known Endpoints

Organizations can host agent directories:

```
GET https://example.com/.well-known/aap/agents/devrel
GET https://example.com/.well-known/aap/agents/devrel.mlld
```

### 8.2 Discovery

```json
GET https://example.com/.well-known/aap/directory

{
  "version": "0.1.0",
  "type": "directory",
  "agents": ["devrel", "support", "docs"],
  "variants": {
    "devrel": ["mlld", "fray"]
  }
}
```

### 8.3 Trust Anchors

Organizations can publish trust roots:

```json
GET https://example.com/.well-known/aap/trust-root

{
  "version": "0.1.0",
  "type": "trust_root",
  "issuer": "org:example",
  "public_key": { "algorithm": "ed25519", "key": "..." },
  "agents": ["@devrel", "@support"],
  "scope": "*.example.com"
}
```

---

## 9. Reference Implementation

### 9.1 API Surface

```go
package aap

// Address parsing
type Address struct {
    Agent    string
    Variants []string
    Job      *JobRef
    Host     string
    Session  string
}

func Parse(addr string) (Address, error)
func (a Address) String() string
func (a Address) Canonical() string

// Identity management
type Identity struct {
    GUID      string
    Address   string
    PublicKey ed25519.PublicKey
    Created   time.Time
    Metadata  map[string]string
}

func Register(agent string) (*Identity, error)
func GetIdentity(agent string) (*Identity, error)

// Resolution
type Resolution struct {
    Identity      *Identity
    Invoke        *InvokeConfig
    Variants      map[string]*VariantContext
    RegistryHosts []string
    Attestations  []*Attestation
}

func Resolve(addr string) (*Resolution, error)

// Trust
type Attestation struct {
    ID            string
    Subject       string
    SubjectGUID   string    // Required: prevents name collision
    SubjectKeyID  string    // Required: sha256:{hex}
    Issuer        string
    IssuerGUID    string    // Required: prevents name collision
    IssuerKeyID   string    // Required: sha256:{hex}
    Capabilities  []string
    Scope         string
    Conditions    *Conditions
    IssuedAt      time.Time
    ExpiresAt     *time.Time
    Signature     []byte
}

func Attest(subject string, claim TrustClaim) (*Attestation, error)
func VerifyTrust(addr, capability, scope string) (*VerifyResult, error)
func Revoke(attestationID, reason string) (*Revocation, error)
```

### 9.2 CLI Commands

```bash
# Identity
aap register <name>              # Register new agent
aap identity <name>              # Show identity
aap list                         # List known agents

# Resolution
aap resolve <address>            # Resolve address
aap resolve @dev.frontend        # With variant
aap resolve @dev@workstation     # With machine

# Variants
aap variant create <agent> <name> --context <path>
aap variant list <agent>
aap variant show <agent>.<variant>

# Trust
aap trust <subject> --from <issuer> --capabilities <caps> --scope <scope>
aap verify <address> --capability <cap> --scope <scope>
aap revoke <attestation-id> --reason <reason>
aap attestations <agent>         # List attestations

# Hosts
aap host register <name>         # Register this host
aap host list                    # List known hosts
```

---

## Appendix A: ABNF Grammar

```abnf
address       = "@" agent [variants] [job-ref] [host] [session]
agent         = name
variants      = 1*("." variant)
variant       = name
job-ref       = "[" job-suffix "-" job-index "]"
job-suffix    = 4ALPHANUM
job-index     = 1*DIGIT
host          = "@" (hostname / domain / git-repo)
git-repo      = git-host "/" repo-owner "/" repo-name
git-host      = "github.com" / "gitlab.com" / "bitbucket.org" / domain
repo-owner    = repo-char *(repo-char)
repo-name     = repo-char *(repo-char)
repo-char     = LOWER / UPPER / DIGIT / "-" / "_" / "."
session       = "#" 1*ALPHANUM
name          = LOWER *(LOWER / DIGIT / "-")
hostname      = name
domain        = name *("." name)
LOWER         = %x61-7A                    ; a-z
UPPER         = %x41-5A                    ; A-Z
DIGIT         = %x30-39                    ; 0-9
ALPHANUM      = LOWER / DIGIT
```

Note: Git repository names allow uppercase, underscores, and dots per platform conventions. These are normalized to lowercase for address comparison but preserved in URIs.

---

## Appendix B: Security Considerations

### B.1 Private Key Storage

Private keys SHOULD be:
- Stored with restrictive permissions (0600)
- Optionally encrypted at rest with a passphrase
- Never transmitted over network

### B.2 Attestation Validation

Implementations MUST:
- Verify signatures before trusting attestations
- Check expiry timestamps
- Validate issuer has authority (via chain or direct)
- Enforce scope restrictions

### B.3 Rate Limiting

Remote resolution endpoints SHOULD implement rate limiting to prevent enumeration attacks.

---

## Appendix C: Relationship to Other Protocols

### C.1 What AAP Provides

- Agent addressing grammar
- Identity registration and resolution
- Trust attestations

### C.2 What AAP Does NOT Provide

- **Messaging**: Use fray, Slack, email, etc.
- **Presence tracking**: Runtime state is consumer's responsibility
- **Message storage**: Consumers choose their storage
- **Orchestration**: Spawning, daemons are separate concerns

AAP is the "DNS for agents" - it answers WHO and WHERE, not WHAT to say or HOW to run.
