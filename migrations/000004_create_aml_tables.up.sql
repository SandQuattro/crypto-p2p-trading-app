-- Таблица для результатов AML проверок
CREATE TABLE IF NOT EXISTS aml_checks (
    id SERIAL PRIMARY KEY,
    transaction_hash VARCHAR(66) NOT NULL,
    wallet_address VARCHAR(42) NOT NULL,
    source_address VARCHAR(42) NOT NULL,
    risk_level VARCHAR(20) NOT NULL,
    risk_source VARCHAR(30) NOT NULL,
    risk_score FLOAT NOT NULL,
    approved BOOLEAN NOT NULL,
    checked_at TIMESTAMP NOT NULL,
    notes TEXT,
    requires_review BOOLEAN NOT NULL DEFAULT FALSE,
    external_services_used TEXT[] NOT NULL DEFAULT '{}'
);

-- Индекс для быстрого поиска по хешу транзакции
CREATE INDEX IF NOT EXISTS idx_aml_checks_tx_hash ON aml_checks(transaction_hash);

-- Таблица для информации о риске адресов
CREATE TABLE IF NOT EXISTS address_risk_info (
    address VARCHAR(42) PRIMARY KEY,
    risk_level VARCHAR(20) NOT NULL,
    risk_score FLOAT NOT NULL,
    last_checked TIMESTAMP NOT NULL,
    category VARCHAR(50),
    source VARCHAR(50),
    tags TEXT[] NOT NULL DEFAULT '{}'
);

-- Таблица очереди транзакций для проверки
CREATE TABLE IF NOT EXISTS aml_transaction_checks (
    tx_hash VARCHAR(66) PRIMARY KEY,
    wallet_address VARCHAR(42) NOT NULL,
    source_address VARCHAR(42) NOT NULL,
    amount VARCHAR(78) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT FALSE
);

-- Индекс для получения необработанных транзакций
CREATE INDEX IF NOT EXISTS idx_aml_transaction_checks_processed ON aml_transaction_checks(processed, created_at);