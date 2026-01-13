# AAP Implementation Plan

## Overview

Build AAP (Agent Address Protocol) as a Go library within the fray repository, with fray being the first consumer. AAP provides the identity and trust layer; fray provides messaging and orchestration.

## Architecture Decision: Library vs Separate Binary

**Recommendation: Embedded library with optional standalone CLI**

```
github.com/adamavenir/fray/
├── internal/aap/         # AAP library (internal during development)
│   ├── address.go        # Address parsing
│   ├── identity.go       # Identity management
│   ├── resolution.go     # Resolution logic
│   ├── trust.go          # Attestations + verification
│   ├── crypto.go         # Ed25519 + JCS
│   └── storage.go        # Registry file I/O
├── pkg/aap/              # Promoted to public after API stabilization
├── cmd/aap/              # Standalone CLI (after pkg/ promotion)
├── cmd/fray/             # Fray CLI (uses internal/aap → pkg/aap)
└── internal/             # Other fray-specific code
```

**Staging:** Start in `internal/aap/` for API flexibility, promote to `pkg/aap/` after Phase 4.

**Why embedded:**
1. Single binary distribution (`fray` ships with AAP baked in)
2. Shared crypto/storage code with fray's existing JSONL infrastructure
3. Gradual adoption - fray commands like `fray agent` can evolve to use AAP
4. No extra install step for users

**Why also a standalone CLI:**
1. Other tools can integrate without pulling in fray
2. Testing/debugging AAP in isolation
3. `aap resolve @dev` useful outside fray context

## Implementation Phases

### Phase 1: Core Library (internal/aap)

**Goal:** Address parsing + local identity management

```go
// internal/aap/address.go
type Address struct {
    Agent    string
    Variants []string
    Job      *JobRef
    Host     string
    Session  string
}

func Parse(addr string) (Address, error)
func (a Address) String() string
func (a Address) Canonical() string  // Normalized lowercase
func (a Address) Base() string       // @agent only
```

**Files:**
- `internal/aap/address.go` - Parse/format addresses per ABNF grammar
- `internal/aap/address_test.go` - Table-driven tests for all address forms

**Effort:** ~2-3 focused sessions

### Phase 2: Identity + Crypto

**Goal:** Register identities, generate keys, sign/verify

```go
// internal/aap/identity.go

// IdentityRecord is the wire format for identity.json (matches AAP-SPEC.md)
type IdentityRecord struct {
    Version   string            `json:"version"`
    Type      string            `json:"type"`  // always "identity"
    GUID      string            `json:"guid"`
    Address   string            `json:"address"`
    Agent     string            `json:"agent"`
    PublicKey *PublicKeyRecord  `json:"public_key,omitempty"`  // nil for keyless mode
    CreatedAt string            `json:"created_at"`  // RFC 3339
    Metadata  map[string]string `json:"metadata,omitempty"`
}

type PublicKeyRecord struct {
    Algorithm string `json:"algorithm"`  // "ed25519"
    Key       string `json:"key"`        // base64-encoded
}

// Identity is the internal type (convenience methods, decoded key)
type Identity struct {
    Record    IdentityRecord
    PublicKey ed25519.PublicKey  // decoded from Record.PublicKey.Key
    HasKey    bool               // false for legacy/keyless identities
}

type Registry interface {
    Register(agent string, opts RegisterOpts) (*Identity, error)
    Get(agent string) (*Identity, error)
    List() ([]*Identity, error)
    // Private key access (for signing)
    Sign(agent string, data []byte) ([]byte, error)
}

// internal/aap/crypto.go

// CanonicalizeForSigning removes the signature field and applies JCS.
// Per AAP-SPEC.md section 6.5, signature must be stripped before canonicalization.
func CanonicalizeForSigning(v any) ([]byte, error)

func Sign(key ed25519.PrivateKey, data []byte) []byte
func Verify(key ed25519.PublicKey, data, sig []byte) bool
```

**Dependencies:**
- `crypto/ed25519` (stdlib)
- JCS library: `github.com/cyberphone/json-canonicalization`

**Storage layout:**
```
~/.config/aap/
├── agents/
│   └── dev/
│       ├── identity.json   # Public identity record (wire format)
│       └── private.key     # Ed25519 private key (encrypted, see below)
```

**Effort:** ~3-4 focused sessions

### Phase 3: Resolution Protocol

**Goal:** Resolve addresses to full identity + invoke config

```go
// internal/aap/resolution.go
type Resolution struct {
    Identity      *Identity
    Invoke        *InvokeConfig
    Variants      map[string]*VariantContext
    RegistryHosts []string
    Attestations  []*Attestation
}

type Resolver interface {
    Resolve(addr string) (*Resolution, error)
}

// Resolution chain:
// 1. Local: ~/.config/aap/agents/
// 2. Project: .aap/agents/ (or .fray/ compat)
// 3. Remote: only if @host present
```

**Fray integration point:** Resolution can fall back to `.fray/agents.jsonl` for existing agent records.

**Legacy identity handling:** Identities loaded from `.fray/agents.jsonl` (without AAP keypairs) are marked as `Untrusted`:
```go
type TrustLevel string
const (
    TrustLevelFull      TrustLevel = "full"      // Has verified keypair
    TrustLevelUntrusted TrustLevel = "untrusted" // Legacy, no keys
)

type Resolution struct {
    Identity      *Identity
    TrustLevel    TrustLevel  // Explicit trust status
    // ...
}
```

Trust verification (`VerifyTrust()`) will fail for untrusted identities. This prevents silent security downgrades - callers must explicitly check `TrustLevel` and decide how to handle legacy agents.

**Effort:** ~2-3 focused sessions

### Phase 4: Trust + Attestations

**Goal:** Issue, store, verify trust attestations

```go
// internal/aap/trust.go
type Attestation struct {
    ID           string
    Subject      string
    SubjectGUID  string
    SubjectKeyID string  // sha256:{hex}
    Issuer       string
    IssuerGUID   string
    IssuerKeyID  string
    Capabilities []string
    Scope        string
    Conditions   *Conditions
    IssuedAt     time.Time
    ExpiresAt    *time.Time
    Signature    []byte
}

func Attest(issuer, subject string, claim TrustClaim) (*Attestation, error)
func Verify(att *Attestation) error
func VerifyTrust(addr, capability, scope string) (*VerifyResult, error)
```

**Effort:** ~4-5 focused sessions (most complex part)

### Phase 5: Fray Integration

**Goal:** Wire AAP into fray's existing agent management

**Changes:**
1. `fray new` generates AAP identity (GUID, keypair)
2. `fray agent` commands use `pkg/aap` for resolution
3. Daemon resolves `@host` addresses for remote agents
4. Optional: `fray trust` commands for attestations

**Migration path:**
- Existing `.fray/agents.jsonl` records continue to work
- New registrations create both JSONL record + AAP identity
- `fray migrate-aap` to backfill AAP identities for existing agents

**Effort:** ~3-4 focused sessions

### Phase 6: Federation (Future)

**Goal:** Remote resolution for `@dev@anthropic.com`

**Components:**
- HTTP client for `.well-known/aap/` endpoints
- Git transport for `@github.com/org/repo` hosts
- Caching + TTL for remote identities

**Effort:** ~4-5 focused sessions (can defer)

## Fray Command Evolution

### Immediate (Phase 1-4)

```bash
# These work with local identities only
fray new alice              # Now also creates ~/.config/aap/agents/alice/
fray agent identity alice   # Show AAP identity + public key
fray agent resolve @alice   # Full resolution output
```

### Phase 5

```bash
# Trust management
fray trust @dev --capabilities write --scope "."
fray trust verify @dev write .
fray trust list --for @dev
fray trust revoke <attestation-id>
```

### Phase 6 (Federation)

```bash
# Remote resolution
fray agent resolve @devrel@anthropic.com
fray post "hello" --as @pm@workstation  # Cross-machine
```

## Technical Decisions

### GUID Generation

Reuse fray's existing `core.NewGUID()` with new prefix:
```go
func NewAAP_GUID() string {
    return "aap-" + base36(16)  // aap-a1b2c3d4e5f6g7h8
}
```

### Key Storage

Private keys stored encrypted at rest using XChaCha20-Poly1305:

```go
type PrivateKeyStore interface {
    Store(agent string, key ed25519.PrivateKey, passphrase []byte) error
    Load(agent string, passphrase []byte) (ed25519.PrivateKey, error)
}
```

**Encrypted key file format (`private.key`):**
```json
{
  "version": 1,
  "algorithm": "xchacha20-poly1305",
  "kdf": "argon2id",
  "kdf_params": {
    "time": 3,
    "memory": 65536,
    "threads": 4,
    "salt": "<base64-32-bytes>"
  },
  "nonce": "<base64-24-bytes>",
  "ciphertext": "<base64-encrypted-key>"
}
```

**Derivation:** `argon2id(passphrase, salt, params) → 32-byte key → XChaCha20-Poly1305(key, nonce, private_key)`

**Options (roadmap):**
1. ✓ Phase 2: Passphrase-encrypted (format above)
2. Future: System keychain (macOS Keychain, Linux secret-service)
3. Future: Hardware tokens (PKCS#11)

**Keyless mode:** When `RegisterOpts.GenerateKey = false`, no `private.key` is created and `identity.json` has `public_key: null`.

### JCS Canonicalization

Use `github.com/cyberphone/json-canonicalization` which is the reference implementation.

```go
import "github.com/cyberphone/json-canonicalization"

func Canonicalize(v any) ([]byte, error) {
    raw, err := json.Marshal(v)
    if err != nil {
        return nil, err
    }
    return jsoncanonicalizer.Transform(raw)
}
```

## Testing Strategy

1. **Unit tests** for address parsing (table-driven, all edge cases)
2. **Integration tests** for identity lifecycle (register → sign → verify)
3. **Golden tests** for attestation serialization (canonical output matches expected)
4. **Fray integration tests** using existing test infrastructure

## Rollout

**Package staging path:**

1. **Phase 1-3:** Build in `internal/aap/` (API can change freely)
2. **After Phase 4:** Once trust/attestations work, promote to `pkg/aap/`
3. **After stabilization:** `cmd/aap/` standalone CLI
4. **Phase 6:** Federation support as separate milestone

The `internal/` → `pkg/` promotion happens after the core API is proven through fray integration. This avoids premature public API commitment.

## Design Decisions

### Wire Types vs Internal Types

**Yes, separate wire and internal types.** The pattern used throughout:

```go
// Wire type: JSON-serializable, matches spec exactly
type IdentityRecord struct { ... }      // identity.json format
type AttestationRecord struct { ... }   // attestation.json format
type ResolutionRecord struct { ... }    // resolution response format

// Internal type: convenience methods, decoded fields
type Identity struct {
    Record    IdentityRecord
    PublicKey ed25519.PublicKey  // decoded
    HasKey    bool
}
```

This keeps wire format compliance explicit while allowing ergonomic internal APIs.

## Open Questions

1. **Passphrase UX**: Prompt on every sign? Cache in memory? Keychain integration?
2. **Backward compat**: How long to maintain `.fray/agents.jsonl` compat layer?
3. **Variant storage**: Where do variant contexts live? Per-project or in global registry?

## Summary

| Phase | Deliverable | Sessions |
|-------|-------------|----------|
| 1 | Address parsing | 2-3 |
| 2 | Identity + crypto | 3-4 |
| 3 | Resolution | 2-3 |
| 4 | Trust attestations | 4-5 |
| 5 | Fray integration | 3-4 |
| 6 | Federation | 4-5 (defer) |
| **Total** | | **18-24** |

Phase 1-5 gives a complete local AAP implementation integrated with fray. Phase 6 (federation) can be tackled separately once the local story is solid.
