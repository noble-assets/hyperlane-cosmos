# BLS Threshold ISM for Hyperlane-Cosmos

## Overview

Implement a new Interchain Security Module (ISM) for the `hyperlane-cosmos` repository that verifies BLS12-381 threshold signatures from the **Transcends** chain — a Commonware Simplex consensus chain with Reth (EVM) execution. The ISM will be a new standalone type (type ID 13) that verifies a single BLS threshold signature against a static group public key.

Two approaches are documented with trade-offs. The recommendation is to implement **Approach B first** (Hyperlane actor), with Approach A (block finalization + state proof) as a follow-up.

---

## Key Design Parameters

| Parameter | Value |
|-----------|-------|
| BLS Curve | BLS12-381 |
| BLS Variant | MinSig (signatures on G1 = 48 bytes, public keys on G2 = 96 bytes) |
| Certificate Model | Threshold (single group public key, non-attributable) |
| ISM Type ID | 13 (`INTERCHAIN_SECURITY_MODULE_TYPE_BLS_THRESHOLD`) |
| Go BLS Library | `supranational/blst` (CGo) |
| ISM Design | New standalone type |

### DST Discrepancy (ACTION REQUIRED)

The IBC light client plan specifies `BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_` but the Commonware codebase uses `BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_POP_`. These differ in the last segment (`_NUL_` vs `_POP_`). The DST **must match exactly** between signer and verifier. This must be confirmed with the Commonware team before implementation.

---

## Approach A: Block Finalization Certificate + State Proof

### Concept

The ISM acts as a **mini light client** for the Transcends chain. The relayer submits a block finalization certificate, a block header, and an Ethereum MPT proof. The verification chain is:

```
BLS signature → Simplex proposal → block hash → state root → MPT proof → Hyperlane state
```

### What Gets Verified

1. **BLS threshold signature** over the Simplex finalization proposal
2. **Block hash** matches the proposal's `block_digest` (keccak256 of RLP-encoded NobleHeader)
3. **State root** extracted from the inner Ethereum header
4. **MPT proof** against the state root proves a specific EVM storage slot
5. **Hyperlane checkpoint data** extracted from the proven storage

### Signed Message Reconstruction

The Simplex finalization certificate signs:
```
signed_msg = varint(19) || "_TRANSCEND_FINALIZE" || varint(epoch) || varint(view) || varint(parent_view) || block_digest[32]
```
Where varints are LEB128-encoded and `block_digest = keccak256(rlp(NobleHeader))`.

### NobleHeader Structure (RLP-encoded)

```
1. timestamp_ms: u64 (milliseconds)
2. consensus_context: FixedBytes<88>
   - [0..8)   epoch (u64 big-endian)
   - [8..16)  view (u64 big-endian)
   - [16..48) leader pubkey (32 bytes, Ed25519)
   - [48..56) parent view (u64 big-endian)
   - [56..88) parent digest (32 bytes, SHA-256)
3. inner: Standard Ethereum execution header (contains state_root)
```

### Metadata Format (Approach A)

```
[   0:  48] BLS threshold signature (compressed G1 point)
[  48:  56] Epoch (uint64 big-endian)
[  56:  64] View (uint64 big-endian)
[  64:  72] Parent view (uint64 big-endian)
[  72: 104] Block digest (32 bytes)
[ 104: 108] RLP header length N (uint32 big-endian)
[ 108: 108+N] RLP-encoded NobleHeader
[ 108+N: 108+N+4] Account proof node count
[ ...] Account proof nodes (length-prefixed RLP nodes)
[ ...] Storage proof node count
[ ...] Storage proof nodes (length-prefixed RLP nodes)
[ ...: ...+32] Merkle tree hook address (for checkpoint context)
```

### Verification Flow

```go
func (m *BlsThresholdISM) Verify(ctx, metadata, message):
  1. Parse metadata fields
  2. Reconstruct Simplex proposal message with varint encoding + namespace prefix
  3. Verify BLS signature over proposal using m.GroupPublicKey and DST
  4. RLP-decode NobleHeader, compute keccak256(rlp(header)), verify == block_digest
  5. Extract state_root from inner Ethereum header
  6. Verify MPT account proof → extract storage root
  7. Verify MPT storage proof → extract checkpoint data
  8. Compute Hyperlane checkpoint digest and verify message matches
```

### Additional Files Needed (Approach A only)

| File | Purpose |
|------|---------|
| `types/simplex_proposal.go` | Proposal reconstruction with varint + namespace |
| `types/noble_header.go` | NobleHeader RLP decoding |
| `types/mpt_proof.go` | Ethereum MPT proof verification (using go-ethereum trie package) |
| `types/bls_threshold_metadata_a.go` | Approach A metadata parsing |

### Trade-offs

**Pros:**
- Trustless — verifies actual finalized chain state, no secondary signing step
- No changes to Commonware binary
- General-purpose — can prove any EVM state
- Code reuse with IBC light client (same BLS verification, header parsing)

**Cons:**
- Significantly more complex (RLP, MPT proofs, varint encoding, header parsing)
- Larger metadata (kilobytes per verification)
- Higher computational cost on-chain
- Fragile to NobleHeader format changes and EVM storage layout changes
- Requires knowledge of Hyperlane Mailbox contract storage slot positions

---

## Approach B: Hyperlane-Specific Actor (Recommended First)

### Concept

A Hyperlane module is embedded in the Commonware Transcends chain binary. This actor watches for Hyperlane message dispatches, creates standard checkpoints, and the validator set collectively signs them using BLS threshold signatures.

The ISM only needs to verify a BLS signature over a standard Hyperlane checkpoint digest. No block headers, no MPT proofs.

### Metadata Format (Approach B)

Modeled after the existing `MessageIdMultisigMetadata`:

```
[   0:  32] Origin merkle tree hook address (bytes32)
[  32:  64] Signed checkpoint root (bytes32)
[  64:  68] Signed checkpoint index (uint32 big-endian)
[  68: 116] BLS threshold signature (48 bytes, compressed G1)
```

**Total: 116 bytes fixed** (vs 68 + N*65 for ECDSA MessageIdMultisig).

### Checkpoint Digest Computation

Reuses the existing Hyperlane checkpoint digest format:
```go
domainHash  = keccak256(origin || merkleTreeHook || "HYPERLANE")
innerHash   = keccak256(domainHash || root || index || messageId)
digest      = ethSignedMessageHash(innerHash)  // "\x19Ethereum Signed Message:\n32" prefix
```

The BLS signature is over this `digest`. This maintains compatibility with the existing checkpoint format.

### Verification Flow

```go
func (m *BlsThresholdISM) Verify(_ context.Context, rawMetadata []byte, message util.HyperlaneMessage) (bool, error) {
    metadata, err := NewBlsThresholdMetadata(rawMetadata)
    if err != nil {
        return false, err
    }
    digest := metadata.Digest(&message)      // Same as MessageIdMultisig digest
    return VerifyBLSSignature(metadata.Signature, m.GroupPublicKey, digest[:])
}
```

### Trade-offs

**Pros:**
- Much simpler ISM — only BLS verification over a standard checkpoint
- Tiny fixed metadata (116 bytes)
- Low computational cost (single pairing check)
- Robust — not dependent on EVM storage layout
- Compatible with existing relayer metadata patterns

**Cons:**
- Requires building a Hyperlane actor into the Commonware binary (Rust work)
- Additional trust surface (signing step beyond consensus finalization)
- Not general-purpose (only works for Hyperlane)
- The Commonware binary becomes coupled to Hyperlane logic

---

## Shared Implementation (Both Approaches)

### 1. New Type Constant

**File:** `x/core/01_interchain_security/types/types.go` (after line 44)

```go
INTERCHAIN_SECURITY_MODULE_TYPE_OP_L2_TO_L1          // 11 (existing)
INTERCHAIN_SECURITY_MODULE_TYPE_POLYMER               // 12 (matching Solidity enum)
INTERCHAIN_SECURITY_MODULE_TYPE_BLS_THRESHOLD          // 13 (new)
```

### 2. Protobuf: ISM Storage Type

**File:** `proto/hyperlane/core/interchain_security/v1/types.proto`

```protobuf
message BlsThresholdISM {
  string id = 1 [(gogoproto.customtype) = "...HexAddress", (gogoproto.nullable) = false];
  string owner = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  bytes group_public_key = 3;   // 96 bytes, compressed G2 point
  uint32 origin_domain = 4;     // Hyperlane domain ID of source chain
}
```

### 3. Protobuf: Transaction Messages

**File:** `proto/hyperlane/core/interchain_security/v1/tx.proto`

```protobuf
rpc CreateBlsThresholdIsm(MsgCreateBlsThresholdIsm) returns (MsgCreateBlsThresholdIsmResponse);
rpc UpdateBlsThresholdIsmGroupKey(MsgUpdateBlsThresholdIsmGroupKey) returns (MsgUpdateBlsThresholdIsmGroupKeyResponse);

message MsgCreateBlsThresholdIsm {
  string creator = 1;
  bytes group_public_key = 2;
  uint32 origin_domain = 3;
}

message MsgUpdateBlsThresholdIsmGroupKey {
  string ism_id = 1 [(gogoproto.customtype) = "...HexAddress"];
  string owner = 2;
  bytes new_group_public_key = 3;
}
```

### 4. BLS Verification Wrapper

**New file:** `x/core/01_interchain_security/types/bls_verify.go`

```go
const (
    BLS_DST            = "BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_"  // CONFIRM WITH COMMONWARE
    BLS_SIG_LENGTH     = 48   // MinSig: compressed G1
    BLS_PUBKEY_LENGTH  = 96   // MinSig: compressed G2
)

func VerifyBLSSignature(signature []byte, publicKey []byte, message []byte) (bool, error) {
    // 1. Validate lengths
    // 2. Decompress signature (blst.P1Affine.Uncompress)
    // 3. Decompress public key (blst.P2Affine.Uncompress)
    // 4. Verify: sig.Verify(true, pk, true, message, []byte(BLS_DST))
}
```

### 5. ISM Implementation

**New file:** `x/core/01_interchain_security/types/bls_threshold.go`

Follows pattern from `message_id_multisig.go`:
- `ModuleType() uint8` → returns type 13
- `GetId() (HexAddress, error)` → returns `m.Id`
- `Verify(ctx, metadata, message) (bool, error)` → parse metadata, compute digest, verify BLS
- `Validate() error` → check public key length, decompressibility, subgroup membership

### 6. Metadata Implementation (Approach B)

**New file:** `x/core/01_interchain_security/types/bls_threshold_metadata.go`

```go
type BlsThresholdMetadata struct {
    MerkleTreeHook [32]byte
    MerkleRoot     [32]byte
    MerkleIndex    uint32
    Signature      []byte    // 48 bytes
}

func NewBlsThresholdMetadata(metadata []byte) (BlsThresholdMetadata, error)
func (m *BlsThresholdMetadata) Bytes() []byte
func (m *BlsThresholdMetadata) Digest(message *util.HyperlaneMessage) [32]byte
```

### 7. Registration Points

| File | Change |
|------|--------|
| `types/codec.go` | Add `&MsgCreateBlsThresholdIsm{}` to `RegisterImplementations`, add `&BlsThresholdISM{}` to `RegisterInterface` |
| `keeper/keeper.go` | Add `router.RegisterModule(types.INTERCHAIN_SECURITY_MODULE_TYPE_BLS_THRESHOLD, k)` in `SetCoreKeeper()` |
| `keeper/msg_server.go` | Add `CreateBlsThresholdIsm()` and `UpdateBlsThresholdIsmGroupKey()` handlers |
| `keeper/genesis.go` | Add case for `"/hyperlane.core.interchain_security.v1.BlsThresholdISM"` |
| `module.go` | Add amino codec registration for `MsgCreateBlsThresholdIsm` |
| `types/errors.go` | Add `ErrInvalidBlsConfiguration` (code 12), `ErrInvalidBlsSignature` (code 13) |

### 8. CLI Commands

**File:** `x/core/01_interchain_security/client/cli/tx.go`

```
create-bls-threshold-ism [group-public-key-hex] [origin-domain]
update-bls-threshold-ism-group-key [ism-id] [new-group-public-key-hex]
```

---

## Key Differences from Existing Multisig ISM

| Aspect | Existing Multisig | BLS Threshold |
|--------|------------------|---------------|
| Signature scheme | ECDSA secp256k1 (65-byte recoverable) | BLS12-381 MinSig (48-byte aggregate) |
| Validator identity | List of 20-byte Ethereum addresses | Single 96-byte group public key |
| Threshold | Explicit N-of-M parameter | Implicit in group key (from DKG) |
| Attribution | Yes (can identify individual signers) | No (threshold group signature) |
| Metadata size | 68 + N*65 bytes | 116 bytes fixed (Approach B) |
| Signature count | Multiple (one per signer) | One |
| Verification cost | O(threshold) ECDSA recoveries | O(1) pairing check |
| Key rotation | Replace validator list | Replace group public key (DKG event) |
| Go dependency | `go-ethereum/crypto` | `supranational/blst` (CGo) |

---

## Implementation Phases

### Phase 1: Foundation
1. Add `supranational/blst` to `go.mod`
2. Define protobuf messages (types.proto, tx.proto, events.proto)
3. Run protobuf code generation
4. Add type constant and error codes
5. Implement `bls_verify.go` with unit tests using known test vectors

### Phase 2: ISM Core (Approach B)
6. Implement `bls_threshold.go` (ModuleType, GetId, Validate)
7. Implement `bls_threshold_metadata.go` (metadata parsing, digest, serialization)
8. Implement `Verify()` using Approach B flow
9. Write unit tests for metadata parsing, digest computation, full verify flow

### Phase 3: Integration
10. Register in codec.go, keeper.go, genesis.go, module.go
11. Implement msg server handlers (Create, Update)
12. Add CLI commands
13. Write integration tests

### Phase 4: Approach A (follow-up)
14. Implement noble_header.go (RLP decoding)
15. Implement simplex_proposal.go (varint + namespace encoding)
16. Implement mpt_proof.go (MPT verification)
17. Implement Approach A metadata and alternative Verify path
18. Comprehensive tests for each component

---

## Files Summary

### New Files

| File | Description |
|------|-------------|
| `x/core/01_interchain_security/types/bls_verify.go` | BLS12-381 MinSig verification wrapper |
| `x/core/01_interchain_security/types/bls_verify_test.go` | BLS verification unit tests |
| `x/core/01_interchain_security/types/bls_threshold.go` | BlsThresholdISM struct and methods |
| `x/core/01_interchain_security/types/bls_threshold_test.go` | ISM unit tests |
| `x/core/01_interchain_security/types/bls_threshold_metadata.go` | Approach B metadata |

### Files to Modify

| File | Change |
|------|--------|
| `proto/.../types.proto` | Add `BlsThresholdISM` message |
| `proto/.../tx.proto` | Add Create/Update RPCs and messages |
| `proto/.../events.proto` | Add BLS ISM events |
| `x/.../types/types.go` | Add type ID constant (13) |
| `x/.../types/errors.go` | Add BLS error codes (12, 13) |
| `x/.../types/codec.go` | Register BLS ISM type |
| `x/.../keeper/keeper.go` | Register with router |
| `x/.../keeper/msg_server.go` | Add Create/Update handlers |
| `x/.../keeper/genesis.go` | Add genesis type URL case |
| `x/.../module.go` | Add amino registration |
| `x/.../client/cli/tx.go` | Add CLI commands |
| `go.mod` | Add blst dependency |

---

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| **DST mismatch** (`_NUL_` vs `_POP_`)  | HIGH | Confirm exact DST with Commonware team before implementation |
| **CGo build issues** (blst) | HIGH | Test CGo builds on all target platforms early |
| **BLS variant confusion** (MinSig vs MinPk) | HIGH | Use explicit test vectors; MinSig = sigs on G1 (48 bytes) |
| **Key rotation** | MEDIUM | `MsgUpdateBlsThresholdIsmGroupKey` provides governance path |
| **Approach A: NobleHeader changes** | MEDIUM | Pin to specific version, comprehensive tests |
| **Approach A: EVM storage layout** | MEDIUM | Document exact storage slots, integration tests |
| **Gas metering** | LOW | Benchmark BLS pairing cost, adjust gas if needed |

---

## Verification & Testing

1. **Unit tests**: BLS verification with known test vectors, metadata round-trips, digest computation
2. **Integration tests**: Full ISM creation, configuration, and message verification flow
3. **Cross-language test**: Generate BLS signature in Rust (Commonware code), verify in Go (ISM) — critical for DST and encoding compatibility
4. **End-to-end**: Create ISM on testnet, submit signed checkpoint metadata, verify message processing
5. **Existing tests**: Run `go test ./...` to ensure no regressions
