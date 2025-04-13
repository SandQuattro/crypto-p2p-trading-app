# AML (Anti-Money Laundering) модуль

Этот модуль предоставляет функционал для проверки криптовалютных транзакций на соответствие требованиям AML (Anti-Money Laundering) и выявления подозрительной активности.

## Основные возможности

- Проверка адресов отправителей на наличие в санкционных списках
- Оценка риска транзакций по различным параметрам
- Интеграция с внешними AML-сервисами (Chainalysis, Elliptic/TRM Labs, AMLBot)
- Локальные проверки без использования внешних API
- Кэширование результатов проверок для оптимизации
- Фоновая обработка очереди AML-проверок

## Структура модуля

- `entities/` - сущности для работы с AML данными
- `repository/` - слой доступа к данным
- `services/` - сервисы для проверки транзакций через различные провайдеры
- `usecase.go` - основной сервис, координирующий все проверки

## Интеграция с внешними AML-сервисами

Модуль поддерживает интеграцию со следующими AML-сервисами:

### 1. Chainalysis

[Chainalysis](https://www.chainalysis.com/) предоставляет API для проверки криптовалютных адресов и транзакций на соответствие требованиям AML/KYC.

Основные возможности:

- Проверка адресов на санкции (OFAC и другие)
- Анализ происхождения средств
- Определение рисков, связанных с транзакциями
- Отслеживание связей между кошельками

### 2. Elliptic (TRM Labs)

[Elliptic](https://www.elliptic.co/) (или [TRM Labs](https://www.trmlabs.com/)) предоставляет схожий функционал для AML проверок криптовалютных транзакций.

Основные возможности:

- Скоринг рисков для адресов
- Мониторинг подозрительной активности
- Выявление связей с даркнет-маркетплейсами
- Проверка на соответствие нормативным требованиям

### 3. AMLBot

[AMLBot](https://web.amlbot.com/check) предоставляет бюджетное решение для AML-проверок криптовалютных адресов.

Основные возможности:

- Базовый скоринг рисков адресов
- Быстрая проверка транзакций
- Доступная ценовая политика
- Простая интеграция через API

## Локальные проверки

Модуль также включает базовый функционал локальных проверок, который может работать без внешних API:

- Проверка адресов по локальному списку рискованных/санкционных адресов
- Анализ суммы транзакции (выявление крупных переводов)
- Простой анализ паттернов адресов

## Настройка

Для настройки модуля требуется указать следующие параметры:

```bash
# Chainalysis API
CHAINALYSIS_API_KEY=your_api_key
CHAINALYSIS_API_URL=https://api.chainalysis.com/v1

# Elliptic/TRM Labs API
ELLIPTIC_API_KEY=your_api_key
ELLIPTIC_API_URL=https://api.trmlabs.com/v1

# AMLBot API
AMLBOT_API_KEY=your_api_key
AMLBOT_API_URL=https://api.amlbot.com/v1

# Параметры локальных проверок
AML_TRANSACTION_THRESHOLD=5000.0  # Пороговое значение для крупных транзакций в USDT
```

## Миграции базы данных

Для работы модуля необходимо создать следующие таблицы:

```sql
-- Таблица для результатов AML проверок
CREATE TABLE aml_checks (
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
CREATE INDEX idx_aml_checks_tx_hash ON aml_checks(transaction_hash);

-- Таблица для информации о риске адресов
CREATE TABLE address_risk_info (
    address VARCHAR(42) PRIMARY KEY,
    risk_level VARCHAR(20) NOT NULL,
    risk_score FLOAT NOT NULL,
    last_checked TIMESTAMP NOT NULL,
    category VARCHAR(50),
    source VARCHAR(50),
    tags TEXT[] NOT NULL DEFAULT '{}'
);

-- Таблица очереди транзакций для проверки
CREATE TABLE aml_transaction_checks (
    tx_hash VARCHAR(66) PRIMARY KEY,
    wallet_address VARCHAR(42) NOT NULL,
    source_address VARCHAR(42) NOT NULL,
    amount VARCHAR(78) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT FALSE
);

-- Индекс для получения необработанных транзакций
CREATE INDEX idx_aml_transaction_checks_processed ON aml_transaction_checks(processed, created_at);
```
