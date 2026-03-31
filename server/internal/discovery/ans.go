// Package discovery implements Layer 2 of the AICP protocol: the Agent Name
// System (ANS), providing service registration, resolution, and capability-based
// search for AI agent services.
package discovery

import (
	"context"
	"fmt"

	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ANSManager manages ANS record lifecycle: creation, update, deletion, and retrieval.
type ANSManager struct {
	backend ledger.LedgerBackend
}

// NewANSManager creates a new ANS record manager.
func NewANSManager(backend ledger.LedgerBackend) *ANSManager {
	return &ANSManager{backend: backend}
}

// Register creates or updates an ANS record for a service.
func (am *ANSManager) Register(ctx context.Context, record aicp.ANSRecord) error {
	if err := validateANSRecord(record); err != nil {
		return fmt.Errorf("invalid ANS record: %w", err)
	}

	return am.backend.StoreANSRecord(ctx, record)
}

// Get retrieves an ANS record by service DID.
func (am *ANSManager) Get(ctx context.Context, serviceDID string) (*aicp.ANSRecord, error) {
	return am.backend.GetANSRecord(ctx, serviceDID)
}

// Delete removes an ANS record.
func (am *ANSManager) Delete(ctx context.Context, serviceDID string) error {
	return am.backend.DeleteANSRecord(ctx, serviceDID)
}

// validateANSRecord performs basic validation on an ANS record.
func validateANSRecord(record aicp.ANSRecord) error {
	if record.AICPVersion == "" {
		return fmt.Errorf("aicp_version is required")
	}
	if record.Service.Name == "" {
		return fmt.Errorf("service name is required")
	}
	if record.Service.DID == "" {
		return fmt.Errorf("service DID is required")
	}
	if len(record.Service.Category) == 0 {
		return fmt.Errorf("at least one service category is required")
	}
	if len(record.Onboarding.Flows) == 0 {
		return fmt.Errorf("at least one onboarding flow is required")
	}
	if len(record.Authentication.DIDMethods) == 0 {
		return fmt.Errorf("at least one DID method must be supported")
	}
	return nil
}

// ANSRecordBuilder provides a fluent API for constructing ANS records.
type ANSRecordBuilder struct {
	record aicp.ANSRecord
}

// NewANSRecordBuilder creates a new builder.
func NewANSRecordBuilder(serviceDID, serviceName string) *ANSRecordBuilder {
	return &ANSRecordBuilder{
		record: aicp.ANSRecord{
			AICPVersion: aicp.ProtocolVersion,
			Service: aicp.ANSServiceInfo{
				DID:  serviceDID,
				Name: serviceName,
			},
			Authentication: aicp.ANSAuthInfo{
				DIDMethods:    []string{"did:aicp"},
				AuthProtocols: []string{"DID-Auth"},
			},
			Onboarding: aicp.ANSOnboardingInfo{
				Flows: []string{"standard"},
			},
		},
	}
}

// Description sets the service description.
func (b *ANSRecordBuilder) Description(desc string) *ANSRecordBuilder {
	b.record.Service.Description = desc
	return b
}

// Categories sets the service categories.
func (b *ANSRecordBuilder) Categories(categories ...string) *ANSRecordBuilder {
	b.record.Service.Category = categories
	return b
}

// DiscoveryTags adds discovery tags for search.
func (b *ANSRecordBuilder) DiscoveryTags(tags ...string) *ANSRecordBuilder {
	b.record.DiscoveryTags = tags
	return b
}

// Homepage sets the service homepage URL.
func (b *ANSRecordBuilder) Homepage(url string) *ANSRecordBuilder {
	b.record.Service.Homepage = url
	return b
}

// RequiredCredentials sets credentials required for onboarding.
func (b *ANSRecordBuilder) RequiredCredentials(creds ...string) *ANSRecordBuilder {
	b.record.Onboarding.RequiredCredentials = creds
	return b
}

// AcceptedPayments sets accepted payment methods.
func (b *ANSRecordBuilder) AcceptedPayments(methods ...string) *ANSRecordBuilder {
	b.record.Onboarding.AcceptedPayments = methods
	return b
}

// MinTrustScore sets the minimum trust score for onboarding.
func (b *ANSRecordBuilder) MinTrustScore(score int) *ANSRecordBuilder {
	b.record.Onboarding.MinimumTrustScore = score
	return b
}

// Pricing sets pricing info.
func (b *ANSRecordBuilder) Pricing(model, currency string, tiers []aicp.ANSPricingTier) *ANSRecordBuilder {
	b.record.Pricing = &aicp.ANSPricingInfo{
		Model:    model,
		Currency: currency,
		Tiers:    tiers,
	}
	return b
}

// Trust sets trust information.
func (b *ANSRecordBuilder) Trust(verifiedBy []string, establishedSince string) *ANSRecordBuilder {
	b.record.Trust = &aicp.ANSTrustInfo{
		VerifiedBy:       verifiedBy,
		EstablishedSince: establishedSince,
	}
	return b
}

// Endpoints sets custom endpoint URLs.
func (b *ANSRecordBuilder) Endpoints(onboard, apiBase, ws, health string) *ANSRecordBuilder {
	b.record.Endpoints = &aicp.ANSEndpoints{
		Onboard:   onboard,
		APIBase:   apiBase,
		WebSocket: ws,
		Health:    health,
	}
	return b
}

// Build returns the constructed ANS record.
func (b *ANSRecordBuilder) Build() aicp.ANSRecord {
	return b.record
}
