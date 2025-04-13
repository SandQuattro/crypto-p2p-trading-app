-- Добавляем enum тип для AML статусов
CREATE TYPE aml_status_type AS ENUM ('none', 'flagged', 'cleared');

-- Добавляем поле aml_status в таблицу transactions
ALTER TABLE transactions 
ADD COLUMN aml_status aml_status_type NOT NULL DEFAULT 'none';

-- Добавляем поля aml_status и aml_notes в таблицу orders
ALTER TABLE orders 
ADD COLUMN aml_status aml_status_type NOT NULL DEFAULT 'none',
ADD COLUMN aml_notes TEXT; 