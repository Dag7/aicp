// Copyright 2024 AICP Protocol Contributors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ─── BuildBlockHash Tests ────────────────────────────────────────────────────

func TestBuildBlockHash_Deterministic(t *testing.T) {
	hash1 := BuildBlockHash(1, "prev", "merkle", "did:aicp:validator")
	hash2 := BuildBlockHash(1, "prev", "merkle", "did:aicp:validator")
	assert.Equal(t, hash1, hash2)
	assert.Len(t, hash1, 64) // SHA-256 hex.
}

func TestBuildBlockHash_DifferentInputsDifferentHash(t *testing.T) {
	h1 := BuildBlockHash(1, "prevA", "merkleA", "did:aicp:v1")
	h2 := BuildBlockHash(2, "prevA", "merkleA", "did:aicp:v1")
	h3 := BuildBlockHash(1, "prevB", "merkleA", "did:aicp:v1")
	h4 := BuildBlockHash(1, "prevA", "merkleB", "did:aicp:v1")
	h5 := BuildBlockHash(1, "prevA", "merkleA", "did:aicp:v2")

	hashes := []string{h1, h2, h3, h4, h5}
	for i := 0; i < len(hashes); i++ {
		for j := i + 1; j < len(hashes); j++ {
			assert.NotEqual(t, hashes[i], hashes[j], "hash %d and %d should differ", i, j)
		}
	}
}

// ─── BuildGenesisBlock Tests ─────────────────────────────────────────────────

func TestBuildGenesisBlock(t *testing.T) {
	genesis := BuildGenesisBlock("did:aicp:validator", "sig123")

	assert.Equal(t, uint64(0), genesis.Number)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", genesis.PreviousHash)
	assert.NotEmpty(t, genesis.Hash)
	assert.NotEmpty(t, genesis.MerkleRoot)
	assert.Equal(t, 0, genesis.EntryCount)
	assert.Equal(t, "did:aicp:validator", genesis.ValidatorDID)
	assert.Equal(t, "sig123", genesis.ValidatorSig)
	assert.False(t, genesis.CreatedAt.IsZero())
}

func TestBuildGenesisBlock_Deterministic(t *testing.T) {
	g1 := BuildGenesisBlock("did:aicp:v", "sig")
	g2 := BuildGenesisBlock("did:aicp:v", "sig")
	// Hash should be deterministic (same genesis data -> same hash).
	assert.Equal(t, g1.Hash, g2.Hash)
	assert.Equal(t, g1.MerkleRoot, g2.MerkleRoot)
}

// ─── Consensus Engine Tests ──────────────────────────────────────────────────

func TestConsensusEngine_IsLeader_SingleNode(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	localDID := crypto.PrincipalDID(kp.PublicKey)
	engine := NewConsensusEngine(nil, localDID, kp.PrivateKey, nil)
	ctx := context.Background()

	// With no validators registered, single node mode: always the leader.
	assert.True(t, engine.IsLeader(ctx, 0))
	assert.True(t, engine.IsLeader(ctx, 1))
	assert.True(t, engine.IsLeader(ctx, 100))
}

func TestConsensusEngine_IsLeader_RoundRobin(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	localDID := crypto.PrincipalDID(kp.PublicKey)

	engine := NewConsensusEngine(nil, localDID, kp.PrivateKey, nil)

	// Manually set validators for testing round-robin.
	engine.validators = []aicp.Validator{
		{DID: localDID, Active: true},
		{DID: "did:aicp:v2", Active: true},
		{DID: "did:aicp:v3", Active: true},
	}

	ctx := context.Background()

	// Block 0: idx=0 -> localDID is leader.
	assert.True(t, engine.IsLeader(ctx, 0))
	// Block 1: idx=1 -> v2 is leader.
	assert.False(t, engine.IsLeader(ctx, 1))
	// Block 2: idx=2 -> v3 is leader.
	assert.False(t, engine.IsLeader(ctx, 2))
	// Block 3: idx=0 -> back to localDID.
	assert.True(t, engine.IsLeader(ctx, 3))
}

func TestConsensusEngine_LocalValidatorDID(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	localDID := crypto.PrincipalDID(kp.PublicKey)

	engine := NewConsensusEngine(nil, localDID, kp.PrivateKey, nil)
	assert.Equal(t, localDID, engine.LocalValidatorDID())
}

func TestConsensusEngine_SignBlock(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	localDID := crypto.PrincipalDID(kp.PublicKey)

	engine := NewConsensusEngine(nil, localDID, kp.PrivateKey, nil)
	ctx := context.Background()

	sig, err := engine.SignBlock(ctx, []byte("block-hash-data"))
	require.NoError(t, err)
	assert.Equal(t, localDID, sig.ValidatorDID)
	assert.NotEmpty(t, sig.Signature)

	// Verify the signature is valid.
	valid, err := crypto.VerifyJSON(kp.PublicKey, []byte("block-hash-data"), sig.Signature)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestConsensusEngine_FinalizeBlock_SingleNode(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	localDID := crypto.PrincipalDID(kp.PublicKey)

	engine := NewConsensusEngine(nil, localDID, kp.PrivateKey, nil)
	ctx := context.Background()

	proposed := &aicp.ProposedBlock{
		Number:       1,
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
		MerkleRoot:   crypto.SHA256Hex([]byte("entry1")),
		ProposerDID:  localDID,
		Timestamp:    time.Now().UTC(),
	}

	finalized, err := engine.FinalizeBlock(ctx, proposed)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), finalized.Block.Number)
	assert.Equal(t, localDID, finalized.Block.ValidatorDID)
	assert.NotEmpty(t, finalized.Block.Hash)
	assert.GreaterOrEqual(t, len(finalized.Signatures), 1)
}

func TestConsensusEngine_CurrentLeader_NoValidators(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	engine := NewConsensusEngine(nil, crypto.PrincipalDID(kp.PublicKey), kp.PrivateKey, nil)
	ctx := context.Background()

	_, err = engine.CurrentLeader(ctx, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no validators")
}

func TestConsensusEngine_CurrentLeader_RoundRobin(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	engine := NewConsensusEngine(nil, crypto.PrincipalDID(kp.PublicKey), kp.PrivateKey, nil)
	ctx := context.Background()

	engine.validators = []aicp.Validator{
		{DID: "did:aicp:v0"},
		{DID: "did:aicp:v1"},
		{DID: "did:aicp:v2"},
	}

	leader0, err := engine.CurrentLeader(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, "did:aicp:v0", leader0.DID)

	leader1, err := engine.CurrentLeader(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "did:aicp:v1", leader1.DID)

	leader5, err := engine.CurrentLeader(ctx, 5)
	require.NoError(t, err)
	assert.Equal(t, "did:aicp:v2", leader5.DID) // 5 % 3 = 2
}

func TestConsensusEngine_RegisterSigner(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	engine := NewConsensusEngine(nil, crypto.PrincipalDID(kp.PublicKey), kp.PrivateKey, nil)

	signer := &mockSigner{}
	engine.RegisterSigner("did:aicp:remote", signer)

	engine.mu.RLock()
	_, ok := engine.signers["did:aicp:remote"]
	engine.mu.RUnlock()
	assert.True(t, ok)
}

// ─── Quorum Tests ────────────────────────────────────────────────────────────

func TestQuorum(t *testing.T) {
	tests := []struct {
		validators int
		expected   int
	}{
		{1, 1},
		{2, 1},
		{3, 1},
		{4, 3}, // f=1, 2*1+1=3
		{5, 3},
		{6, 3},
		{7, 5},  // f=2, 2*2+1=5
		{10, 7}, // f=3, 2*3+1=7
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, quorum(tt.validators), "quorum(%d)", tt.validators)
	}
}

// ─── Mock ValidatorSigner ────────────────────────────────────────────────────

type mockSigner struct{}

func (m *mockSigner) RequestSignature(_ context.Context, _ []byte, _ *aicp.ProposedBlock) (*aicp.ValidatorSig, error) {
	return &aicp.ValidatorSig{
		ValidatorDID: "did:aicp:remote",
		Signature:    "mock-signature",
	}, nil
}
