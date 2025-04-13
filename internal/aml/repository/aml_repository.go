package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	tx "github.com/Thiht/transactor/pgx"
	"github.com/jackc/pgx/v5"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/aml/entities"
	"github.com/sand/crypto-p2p-trading-app/backend/pkg/database"
)

// AMLRepository управляет хранением и получением данных AML проверок
type AMLRepository struct {
	logger     *slog.Logger
	db         tx.DBGetter
	transactor *tx.Transactor
}

// NewAMLRepository создает новый репозиторий AML
func NewAMLRepository(logger *slog.Logger, pg *database.Postgres) *AMLRepository {
	return &AMLRepository{
		logger:     logger,
		db:         pg.DBGetter,
		transactor: pg.Transactor,
	}
}

// SaveCheckResult сохраняет результат AML проверки в базу данных
func (r *AMLRepository) SaveCheckResult(ctx context.Context, result *entities.AMLCheckResult) error {
	query := `INSERT INTO aml_checks 
		(transaction_hash, wallet_address, source_address, risk_level, risk_source, risk_score, approved, checked_at, notes, requires_review, external_services_used) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`

	err := r.db(ctx).QueryRow(ctx, query,
		result.TransactionHash,
		result.WalletAddress,
		result.SourceAddress,
		result.RiskLevel,
		result.RiskSource,
		result.RiskScore,
		result.Approved,
		time.Now(),
		result.Notes,
		result.RequiresReview,
		result.ExternalServicesUsed,
	).Scan(&result.ID)

	if err != nil {
		return fmt.Errorf("failed to save AML check result: %w", err)
	}

	return nil
}

// GetCheckResultByTxHash возвращает результат AML проверки по хешу транзакции
func (r *AMLRepository) GetCheckResultByTxHash(ctx context.Context, txHash string) (*entities.AMLCheckResult, error) {
	query := `SELECT id, transaction_hash, wallet_address, source_address, risk_level, risk_source, risk_score, approved, checked_at, notes, requires_review, external_services_used 
		FROM aml_checks 
		WHERE transaction_hash = $1 
		ORDER BY checked_at DESC 
		LIMIT 1`

	var result entities.AMLCheckResult
	var externalServices []string

	err := r.db(ctx).QueryRow(ctx, query, txHash).Scan(
		&result.ID,
		&result.TransactionHash,
		&result.WalletAddress,
		&result.SourceAddress,
		&result.RiskLevel,
		&result.RiskSource,
		&result.RiskScore,
		&result.Approved,
		&result.CheckedAt,
		&result.Notes,
		&result.RequiresReview,
		&externalServices,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get AML check result: %w", err)
	}

	result.ExternalServicesUsed = externalServices
	return &result, nil
}

// SaveAddressRiskInfo сохраняет информацию о риске адреса
func (r *AMLRepository) SaveAddressRiskInfo(ctx context.Context, riskInfo *entities.AddressRiskInfo) error {
	query := `INSERT INTO address_risk_info 
		(address, risk_level, risk_score, last_checked, category, source, tags) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (address) DO UPDATE 
		SET risk_level = $2, risk_score = $3, last_checked = $4, category = $5, source = $6, tags = $7`

	_, err := r.db(ctx).Exec(ctx, query,
		riskInfo.Address,
		riskInfo.RiskLevel,
		riskInfo.RiskScore,
		time.Now(),
		riskInfo.Category,
		riskInfo.Source,
		riskInfo.Tags,
	)

	if err != nil {
		return fmt.Errorf("failed to save address risk info: %w", err)
	}

	return nil
}

// GetAddressRiskInfo получает информацию о риске адреса
func (r *AMLRepository) GetAddressRiskInfo(ctx context.Context, address string) (*entities.AddressRiskInfo, error) {
	query := `SELECT address, risk_level, risk_score, last_checked, category, source, tags 
		FROM address_risk_info 
		WHERE address = $1`

	var riskInfo entities.AddressRiskInfo
	var tags []string

	err := r.db(ctx).QueryRow(ctx, query, address).Scan(
		&riskInfo.Address,
		&riskInfo.RiskLevel,
		&riskInfo.RiskScore,
		&riskInfo.LastChecked,
		&riskInfo.Category,
		&riskInfo.Source,
		&tags,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get address risk info: %w", err)
	}

	riskInfo.Tags = tags
	return &riskInfo, nil
}

// AddTransactionForChecking добавляет транзакцию в очередь на проверку
func (r *AMLRepository) AddTransactionForChecking(ctx context.Context, check *entities.TransactionCheck) error {
	query := `INSERT INTO aml_transaction_checks 
		(tx_hash, wallet_address, source_address, amount, created_at, processed) 
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tx_hash) DO NOTHING`

	_, err := r.db(ctx).Exec(ctx, query,
		check.TxHash,
		check.WalletAddress,
		check.SourceAddress,
		check.Amount,
		time.Now(),
		false,
	)

	if err != nil {
		return fmt.Errorf("failed to add transaction for checking: %w", err)
	}

	return nil
}

// GetPendingChecks получает список транзакций, ожидающих проверки
func (r *AMLRepository) GetPendingChecks(ctx context.Context, limit int) ([]entities.TransactionCheck, error) {
	query := `SELECT tx_hash, wallet_address, source_address, amount, created_at, processed 
		FROM aml_transaction_checks 
		WHERE processed = false 
		ORDER BY created_at ASC 
		LIMIT $1`

	rows, err := r.db(ctx).Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending checks: %w", err)
	}
	defer rows.Close()

	var checks []entities.TransactionCheck
	for rows.Next() {
		var check entities.TransactionCheck
		err := rows.Scan(
			&check.TxHash,
			&check.WalletAddress,
			&check.SourceAddress,
			&check.Amount,
			&check.CreatedAt,
			&check.Processed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pending check: %w", err)
		}
		checks = append(checks, check)
	}

	return checks, nil
}

// MarkCheckAsProcessed отмечает проверку как обработанную
func (r *AMLRepository) MarkCheckAsProcessed(ctx context.Context, txHash string) error {
	query := `UPDATE aml_transaction_checks 
		SET processed = true 
		WHERE tx_hash = $1`

	_, err := r.db(ctx).Exec(ctx, query, txHash)
	if err != nil {
		return fmt.Errorf("failed to mark check as processed: %w", err)
	}

	return nil
}
