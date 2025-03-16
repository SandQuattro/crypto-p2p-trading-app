package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	tx "github.com/Thiht/transactor/pgx"
	"github.com/jackc/pgx/v5"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"github.com/sand/crypto-p2p-trading-app/backend/pkg/database"
)

// WalletsRepository handles wallet tracking and management.
type WalletsRepository struct {
	logger     *slog.Logger
	db         tx.DBGetter
	transactor *tx.Transactor
}

// NewWalletsRepository creates a new wallet repository.
func NewWalletsRepository(logger *slog.Logger, pg *database.Postgres) *WalletsRepository {
	return &WalletsRepository{
		logger:     logger,
		db:         pg.DBGetter,
		transactor: pg.Transactor,
	}
}

// FindWalletByAddress retrieves a wallet by its address.
func (r *WalletsRepository) FindWalletByAddress(ctx context.Context, address string) (*entities.Wallet, error) {
	query := `SELECT id, address, derivation_path, created_at 
              FROM wallets 
              WHERE address = $1`

	var wallet entities.Wallet
	err := r.db(ctx).QueryRow(ctx, query, address).Scan(
		&wallet.ID,
		&wallet.Address,
		&wallet.DerivationPath,
		&wallet.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query wallet by wallet address: %w", err)
	}

	return &wallet, nil
}

// FindWalletByID retrieves a wallet by its id.
func (r *WalletsRepository) FindWalletByID(ctx context.Context, id int) (*entities.Wallet, error) {
	query := `SELECT id, address, derivation_path, created_at 
              FROM wallets 
              WHERE id = $1`

	var wallet entities.Wallet
	err := r.db(ctx).QueryRow(ctx, query, id).Scan(
		&wallet.ID,
		&wallet.Address,
		&wallet.DerivationPath,
		&wallet.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query wallet by wallet id: %w", err)
	}

	return &wallet, nil
}

// IsWalletTracked checks if the given address is tracked by our system.
func (r *WalletsRepository) IsWalletTracked(ctx context.Context, address string) (bool, error) {
	var exists bool
	err := r.db(ctx).QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM wallets WHERE address = $1)", address).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if wallet exists: %w", err)
	}
	return exists, nil
}

// GetAllTrackedWallets retrieves all tracked wallet addresses.
func (r *WalletsRepository) GetAllTrackedWallets(ctx context.Context) ([]entities.Wallet, error) {
	query := `SELECT id, user_id, address, derivation_path, wallet_index, created_at 
              FROM wallets 
              ORDER BY id`

	rows, err := r.db(ctx).Query(ctx, query)
	defer rows.Close()

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}

	wallets, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Wallet])
	if err != nil {
		r.logger.Error("failed to collect all tracked wallets rows", "error", err)
		return nil, err
	}

	return wallets, nil
}

// GetLastWalletIndexForUser retrieves the last used wallet index for a specific user
func (r *WalletsRepository) GetLastWalletIndexForUser(ctx context.Context, userID int64) (uint32, error) {
	var lastIndex uint32

	err := r.db(ctx).QueryRow(ctx,
		"SELECT COALESCE(MAX(wallet_index), 0) FROM wallets WHERE user_id = $1",
		userID).Scan(&lastIndex)

	if err != nil {
		return 0, fmt.Errorf("failed to get last wallet index for user %d: %w", userID, err)
	}
	return lastIndex, nil
}

// TrackWalletWithUserAndIndex adds a wallet address to the tracking system with a specific user and index
func (r *WalletsRepository) TrackWalletWithUserAndIndex(ctx context.Context, address string, derivationPath string, userID int64, index uint32) (int, error) {
	// Check if wallet already exists
	exists, err := r.IsWalletTracked(ctx, address)
	if err != nil {
		return 0, err
	}

	if exists {
		r.logger.Debug("Wallet already tracked", "address", address)
		return 0, nil
	}

	var id int
	// Insert new wallet with user ID and index
	err = r.db(ctx).QueryRow(ctx,
		"INSERT INTO wallets (address, derivation_path, user_id, wallet_index, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		address, derivationPath, userID, index, time.Now()).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert wallet: %w", err)
	}

	r.logger.Info("Wallet added to tracking", "address", address, "user", userID, "index", index)
	return id, nil
}

// GetAllTrackedWalletsForUser retrieves all tracked wallet addresses for a specific user.
func (r *WalletsRepository) GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]entities.Wallet, error) {
	query := `SELECT id, address, derivation_path, user_id, wallet_index, created_at 
              FROM wallets 
              WHERE user_id = $1
              ORDER BY wallet_index`

	rows, err := r.db(ctx).Query(ctx, query, userID)
	defer rows.Close()

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets by user id: %w", err)
	}

	wallets, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Wallet])
	if err != nil {
		r.logger.Error("failed to collect wallets rows", "error", err)
		return nil, fmt.Errorf("failed to collect all tracked user wallets rows, %w", err)
	}

	return wallets, nil
}
