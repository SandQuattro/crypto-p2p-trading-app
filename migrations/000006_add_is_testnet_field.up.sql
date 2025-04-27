-- Добавляем поле is_testnet в таблицу wallets
ALTER TABLE wallets
    ADD COLUMN IF NOT EXISTS is_testnet bool NOT NULL DEFAULT false;
