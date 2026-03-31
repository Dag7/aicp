"""Tests for the provider module."""

import pytest

from aicp.provider import AICPServiceAdapter, ServiceProviderConfig


@pytest.fixture
def adapter():
    return AICPServiceAdapter(
        ServiceProviderConfig(
            service_did="did:aicp:myservice",
            service_name="Test Service",
            categories=["ai", "nlp"],
            required_credentials=["AICPAgentDelegation"],
            accepted_payments=["stripe", "paypal"],
            minimum_trust_score=200,
        )
    )


class TestBuildANSRecord:
    def test_generates_valid_record(self, adapter):
        record = adapter.build_ans_record(
            description="A test AI service",
            homepage="https://example.com",
            api_base="https://api.example.com",
        )
        assert record.aicp_version == "1.0"
        assert record.service.name == "Test Service"
        assert record.service.did == "did:aicp:myservice"
        assert record.service.category == ["ai", "nlp"]
        assert record.onboarding.required_credentials == ["AICPAgentDelegation"]
        assert record.onboarding.accepted_payments == ["stripe", "paypal"]
        assert record.onboarding.minimum_trust_score == 200
        assert record.authentication.did_methods == ["did:aicp"]
        assert record.endpoints is not None
        assert record.endpoints.api_base == "https://api.example.com"

    def test_includes_pricing(self, adapter):
        record = adapter.build_ans_record(
            pricing_model="usage",
            pricing_tiers=[{"name": "basic", "rate": "0.01", "description": "Per request"}],
        )
        assert record.pricing is not None
        assert record.pricing.model == "usage"
        assert record.pricing.tiers is not None
        assert len(record.pricing.tiers) == 1
        assert record.pricing.tiers[0].name == "basic"


class TestExtractAuth:
    def test_bearer_token(self, adapter):
        ctx = adapter.extract_auth("Bearer did:aicp:agent123")
        assert ctx.authenticated is True
        assert ctx.did == "did:aicp:agent123"
        assert ctx.token == "did:aicp:agent123"

    def test_missing_header(self, adapter):
        ctx = adapter.extract_auth(None)
        assert ctx.authenticated is False
        assert ctx.did is None

    def test_non_bearer(self, adapter):
        ctx = adapter.extract_auth("Basic dXNlcjpwYXNz")
        assert ctx.authenticated is False


class TestRequireAuth:
    def test_authenticated(self, adapter):
        ctx = adapter.require_auth("Bearer did:aicp:agent")
        assert ctx.authenticated is True

    def test_unauthenticated_raises(self, adapter):
        with pytest.raises(PermissionError, match="AICP authentication required"):
            adapter.require_auth(None)


def test_service_did(adapter):
    assert adapter.service_did == "did:aicp:myservice"
