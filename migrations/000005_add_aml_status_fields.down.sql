-- Удаляем поля aml_status и aml_notes из таблицы orders
ALTER TABLE orders 
DROP COLUMN IF EXISTS aml_status,
DROP COLUMN IF EXISTS aml_notes;

-- Удаляем поле aml_status из таблицы transactions
ALTER TABLE transactions 
DROP COLUMN IF EXISTS aml_status;

-- Удаляем enum тип
DROP TYPE IF EXISTS aml_status_type; 