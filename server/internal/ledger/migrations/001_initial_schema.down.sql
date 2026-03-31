-- AICP Consortium Ledger: Drop all tables (reverse of up migration)

DROP TRIGGER IF EXISTS trg_ans_search_vector ON ans_records;
DROP FUNCTION IF EXISTS ans_search_vector_update();

DROP TABLE IF EXISTS onboarding_sessions;
DROP TABLE IF EXISTS validators;
DROP TABLE IF EXISTS ans_records;
DROP TABLE IF EXISTS trust_scores;
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS ledger_blocks;
DROP TABLE IF EXISTS spending_records;
DROP TABLE IF EXISTS used_tokens;
DROP TABLE IF EXISTS payment_receipts;
DROP TABLE IF EXISTS revocation_events;
DROP TABLE IF EXISTS revocation_lists;
DROP TABLE IF EXISTS credentials;
DROP TABLE IF EXISTS did_document_versions;
DROP TABLE IF EXISTS did_documents;
