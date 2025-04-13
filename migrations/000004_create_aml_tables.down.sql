-- Удаление индексов
DROP INDEX IF EXISTS idx_aml_transaction_checks_processed;
DROP INDEX IF EXISTS idx_aml_checks_tx_hash;

-- Удаление таблиц
DROP TABLE IF EXISTS aml_transaction_checks;
DROP TABLE IF EXISTS address_risk_info;
DROP TABLE IF EXISTS aml_checks; 