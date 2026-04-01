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

// Package repository provides domain-scoped repository interfaces that are
// narrow subsets of the LedgerBackend interface. Each interface groups related
// operations for a single domain (DID, credential, receipt, etc.), allowing
// consuming code -- especially tests -- to depend on the minimal interface
// they actually need rather than the full LedgerBackend.
package repository

import (
	"context"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// DIDRepository groups DID lifecycle operations.
type DIDRepository interface {
	AnchorDID(ctx context.Context, did string, doc aicp.DIDDocument, proof aicp.Proof) (*aicp.AnchorReceipt, error)
	ResolveDID(ctx context.Context, did string) (*aicp.DIDResolutionResult, error)
	UpdateDIDDocument(ctx context.Context, did string, doc aicp.DIDDocument, proof aicp.Proof) (*aicp.AnchorReceipt, error)
	DeactivateDID(ctx context.Context, did string, proof aicp.Proof) error
	ListAgentDIDs(ctx context.Context, principalDID string) ([]string, error)
}

// CredentialRepository groups verifiable credential storage and revocation operations.
type CredentialRepository interface {
	StoreCredential(ctx context.Context, cred aicp.VerifiableCredential) error
	GetCredential(ctx context.Context, credentialID string) (*aicp.VerifiableCredential, error)
	RevokeCredential(ctx context.Context, credentialID string, proof aicp.Proof) error
	IsCredentialRevoked(ctx context.Context, credentialID string) (bool, error)
	GetRevocationList(ctx context.Context, listID string) (*aicp.RevocationList, error)
	CreateRevocationList(ctx context.Context, list aicp.RevocationList) error
	UpdateRevocationList(ctx context.Context, list aicp.RevocationList) error
}

// ReceiptRepository groups payment receipt storage and token replay operations.
type ReceiptRepository interface {
	StoreReceipt(ctx context.Context, receipt aicp.SignedPaymentReceipt) error
	GetReceipt(ctx context.Context, receiptID string) (*aicp.SignedPaymentReceipt, error)
	ListReceiptsByAgent(ctx context.Context, agentDID string, opts aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.SignedPaymentReceipt], error)
	IsTokenUsed(ctx context.Context, jti string) (bool, error)
	MarkTokenUsed(ctx context.Context, jti string) error
}

// BlockRepository groups block and ledger entry operations.
type BlockRepository interface {
	AppendEntry(ctx context.Context, entry aicp.LedgerEntry) error
	GetPendingEntries(ctx context.Context) ([]aicp.LedgerEntry, error)
	StoreBlock(ctx context.Context, block aicp.Block) error
	GetLatestBlock(ctx context.Context) (*aicp.Block, error)
	GetBlock(ctx context.Context, number uint64) (*aicp.Block, error)
}

// TrustRepository groups trust score operations.
type TrustRepository interface {
	GetTrustScore(ctx context.Context, did string) (*aicp.TrustScore, error)
	UpdateTrustScore(ctx context.Context, score aicp.TrustScore) error
}

// ANSRepository groups Agent Name System discovery operations.
type ANSRepository interface {
	StoreANSRecord(ctx context.Context, record aicp.ANSRecord) error
	GetANSRecord(ctx context.Context, serviceDID string) (*aicp.ANSRecord, error)
	SearchANSRecords(ctx context.Context, query string, opts aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.ANSRecord], error)
	DeleteANSRecord(ctx context.Context, serviceDID string) error
}

// ValidatorRepository groups consortium validator management operations.
type ValidatorRepository interface {
	RegisterValidator(ctx context.Context, v aicp.Validator) error
	RemoveValidator(ctx context.Context, validatorDID string) error
	ListValidators(ctx context.Context) ([]aicp.Validator, error)
	GetValidator(ctx context.Context, validatorDID string) (*aicp.Validator, error)
}

// SpendingRepository groups spending limit tracking operations.
type SpendingRepository interface {
	GetDailySpending(ctx context.Context, did string, currency string) (string, error)
	GetMonthlySpending(ctx context.Context, did string, currency string) (string, error)
	RecordSpending(ctx context.Context, did string, amount aicp.MoneyAmount) error
	CheckAndRecordSpending(ctx context.Context, did string, amount aicp.MoneyAmount, dailyLimit, monthlyLimit string) (bool, error)
}
