-- AICP Consortium Ledger: Initial Schema
-- PostgreSQL 16+

-- ─── Extensions ────────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ─── DID Documents ─────────────────────────────────────────────────────────

CREATE TABLE did_documents (
    did              TEXT        PRIMARY KEY,
    controller       TEXT        NOT NULL,
    document         JSONB       NOT NULL,
    document_hash    TEXT        NOT NULL,
    deactivated      BOOLEAN     NOT NULL DEFAULT FALSE,
    version_id       INTEGER     NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_did_documents_controller ON did_documents (controller) WHERE NOT deactivated;

CREATE TABLE did_document_versions (
    id               BIGSERIAL   PRIMARY KEY,
    did              TEXT        NOT NULL REFERENCES did_documents(did),
    version_id       INTEGER     NOT NULL,
    document         JSONB       NOT NULL,
    document_hash    TEXT        NOT NULL,
    proof_type       TEXT        NOT NULL,
    proof_value      TEXT        NOT NULL,
    proof_method     TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (did, version_id)
);

CREATE INDEX idx_did_versions_did ON did_document_versions (did);

-- ─── Verifiable Credentials ────────────────────────────────────────────────

CREATE TABLE credentials (
    credential_id    TEXT        PRIMARY KEY,
    issuer_did       TEXT        NOT NULL,
    subject_did      TEXT        NOT NULL,
    credential_type  TEXT[]      NOT NULL,
    credential       JSONB       NOT NULL,
    revoked          BOOLEAN     NOT NULL DEFAULT FALSE,
    revoked_at       TIMESTAMPTZ,
    issued_at        TIMESTAMPTZ NOT NULL,
    expires_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_credentials_issuer ON credentials (issuer_did);
CREATE INDEX idx_credentials_subject ON credentials (subject_did);
CREATE INDEX idx_credentials_type ON credentials USING GIN (credential_type);

-- ─── Revocation Lists ──────────────────────────────────────────────────────

CREATE TABLE revocation_lists (
    list_id          TEXT        PRIMARY KEY,
    issuer_did       TEXT        NOT NULL,
    encoded_list     TEXT        NOT NULL,
    total_capacity   INTEGER     NOT NULL DEFAULT 131072,
    next_free_index  INTEGER     NOT NULL DEFAULT 0,
    version_id       INTEGER     NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_revocation_lists_issuer ON revocation_lists (issuer_did);

CREATE TABLE revocation_events (
    id               BIGSERIAL   PRIMARY KEY,
    list_id          TEXT        NOT NULL REFERENCES revocation_lists(list_id),
    credential_id    TEXT        NOT NULL,
    list_index       INTEGER     NOT NULL,
    proof_type       TEXT        NOT NULL,
    proof_value      TEXT        NOT NULL,
    revoked_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_revocation_events_credential ON revocation_events (credential_id);

-- ─── Payment Receipts ──────────────────────────────────────────────────────

CREATE TABLE payment_receipts (
    receipt_id       TEXT        PRIMARY KEY,
    token_jti        TEXT        NOT NULL UNIQUE,
    payer_did        TEXT        NOT NULL,
    payee_did        TEXT        NOT NULL,
    amount_value     TEXT        NOT NULL,
    amount_currency  TEXT        NOT NULL,
    receipt_hash     TEXT        NOT NULL,
    status           TEXT        NOT NULL DEFAULT 'settled',
    adapter_name     TEXT        NOT NULL DEFAULT '',
    external_id      TEXT,
    block_number     BIGINT,
    settled_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_receipts_payer ON payment_receipts (payer_did);
CREATE INDEX idx_receipts_payee ON payment_receipts (payee_did);
CREATE INDEX idx_receipts_token_jti ON payment_receipts (token_jti);
CREATE INDEX idx_receipts_created ON payment_receipts (created_at DESC);

-- ─── Used Payment Tokens (Replay Prevention) ──────────────────────────────

CREATE TABLE used_tokens (
    jti              TEXT        PRIMARY KEY,
    used_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Spending Tracking ─────────────────────────────────────────────────────

CREATE TABLE spending_records (
    id               BIGSERIAL   PRIMARY KEY,
    did              TEXT        NOT NULL,
    amount_value     TEXT        NOT NULL,
    amount_currency  TEXT        NOT NULL,
    recorded_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_spending_did_date ON spending_records (did, recorded_at DESC);
CREATE INDEX idx_spending_did_currency ON spending_records (did, amount_currency);

-- ─── Ledger Blocks ─────────────────────────────────────────────────────────

CREATE TABLE ledger_blocks (
    number           BIGINT      PRIMARY KEY,
    hash             TEXT        NOT NULL UNIQUE,
    previous_hash    TEXT        NOT NULL,
    merkle_root      TEXT        NOT NULL,
    entry_count      INTEGER     NOT NULL DEFAULT 0,
    validator_did    TEXT        NOT NULL,
    validator_sig    TEXT        NOT NULL,
    signatures       JSONB       NOT NULL DEFAULT '[]'::JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Ledger Entries ────────────────────────────────────────────────────────

CREATE TABLE ledger_entries (
    id               BIGSERIAL   PRIMARY KEY,
    block_number     BIGINT      REFERENCES ledger_blocks(number),
    entry_type       TEXT        NOT NULL,
    entry_hash       TEXT        NOT NULL,
    entry_data_ref   TEXT        NOT NULL,
    recorded_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entries_block ON ledger_entries (block_number);
CREATE INDEX idx_entries_pending ON ledger_entries (id) WHERE block_number IS NULL;
CREATE INDEX idx_entries_type ON ledger_entries (entry_type);

-- ─── Trust Scores ──────────────────────────────────────────────────────────

CREATE TABLE trust_scores (
    did              TEXT        PRIMARY KEY,
    score            INTEGER     NOT NULL DEFAULT 0 CHECK (score >= 0 AND score <= 1000),
    identity         INTEGER     NOT NULL DEFAULT 0,
    account_age      INTEGER     NOT NULL DEFAULT 0,
    transactions     INTEGER     NOT NULL DEFAULT 0,
    endorsements     INTEGER     NOT NULL DEFAULT 0,
    credentials      INTEGER     NOT NULL DEFAULT 0,
    dispute_penalty  INTEGER     NOT NULL DEFAULT 0,
    computed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── ANS Records ───────────────────────────────────────────────────────────

CREATE TABLE ans_records (
    service_did      TEXT        PRIMARY KEY,
    aicp_version     TEXT        NOT NULL DEFAULT '1.0',
    record           JSONB       NOT NULL,
    service_name     TEXT        NOT NULL,
    categories       TEXT[]      NOT NULL DEFAULT '{}',
    discovery_tags   TEXT[]      NOT NULL DEFAULT '{}',
    search_vector    TSVECTOR,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ans_categories ON ans_records USING GIN (categories);
CREATE INDEX idx_ans_tags ON ans_records USING GIN (discovery_tags);
CREATE INDEX idx_ans_search ON ans_records USING GIN (search_vector);
CREATE INDEX idx_ans_name ON ans_records (service_name);

-- Trigger to auto-update search vector on insert/update.
CREATE OR REPLACE FUNCTION ans_search_vector_update() RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.service_name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.categories, ' '), '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.discovery_tags, ' '), '')), 'C') ||
        setweight(to_tsvector('english', COALESCE(NEW.record->>'service'->>'description', '')), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ans_search_vector
    BEFORE INSERT OR UPDATE ON ans_records
    FOR EACH ROW EXECUTE FUNCTION ans_search_vector_update();

-- ─── Validators ────────────────────────────────────────────────────────────

CREATE TABLE validators (
    did              TEXT        PRIMARY KEY,
    public_key       TEXT        NOT NULL,
    grpc_address     TEXT        NOT NULL,
    active           BOOLEAN     NOT NULL DEFAULT TRUE,
    joined_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_validators_active ON validators (did) WHERE active;

-- ─── Onboarding Sessions ──────────────────────────────────────────────────

CREATE TABLE onboarding_sessions (
    session_id       TEXT        PRIMARY KEY,
    agent_did        TEXT        NOT NULL,
    service_did      TEXT        NOT NULL,
    phase            TEXT        NOT NULL DEFAULT 'initiated',
    state            JSONB       NOT NULL DEFAULT '{}'::JSONB,
    started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_sessions_agent ON onboarding_sessions (agent_did);
CREATE INDEX idx_sessions_service ON onboarding_sessions (service_did);
CREATE INDEX idx_sessions_phase ON onboarding_sessions (phase);
CREATE INDEX idx_sessions_expires ON onboarding_sessions (expires_at);
