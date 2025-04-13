-- Добавляем enum тип для AML статусов
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'aml_status_type') THEN
        CREATE TYPE aml_status_type AS ENUM ('none', 'flagged', 'cleared');
    END IF;
END$$;

-- Добавляем поле aml_status в таблицу transactions
ALTER TABLE transactions 
ADD COLUMN IF NOT EXISTS aml_status aml_status_type NOT NULL DEFAULT 'none';

-- Добавляем поля aml_status и aml_notes в таблицу orders
ALTER TABLE orders
ADD COLUMN IF NOT EXISTS aml_status aml_status_type NOT NULL DEFAULT 'none',
ADD COLUMN IF NOT EXISTS aml_notes TEXT;