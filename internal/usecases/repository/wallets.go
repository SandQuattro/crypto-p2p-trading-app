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

// IsWalletTracked checks if the given address is tracked by our system.
func (r *WalletsRepository) IsWalletTracked(ctx context.Context, address string) (bool, error) {
	var exists bool
	err := r.db(ctx).QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM wallets WHERE address = $1)", address).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if wallet exists: %w", err)
	}
	return exists, nil
}

// TrackWallet adds a wallet address to the tracking system.
func (r *WalletsRepository) TrackWallet(ctx context.Context, address string, derivationPath string) error {
	// Check if wallet already exists
	exists, err := r.IsWalletTracked(ctx, address)
	if err != nil {
		return err
	}

	if exists {
		r.logger.Debug("Wallet already tracked", "address", address)
		return nil
	}

	// Insert new wallet
	_, err = r.db(ctx).Exec(ctx,
		"INSERT INTO wallets (address, derivation_path, created_at) VALUES ($1, $2, $3)",
		address, derivationPath, time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert wallet: %w", err)
	}

	r.logger.Info("Wallet added to tracking", "address", address)
	return nil
}

// GetAllTrackedWallets retrieves all tracked wallet addresses.
func (r *WalletsRepository) GetAllTrackedWallets(ctx context.Context) ([]entities.Wallet, error) {
	query := `SELECT id, address, derivation_path, created_at 
              FROM wallets 
              ORDER BY id`

	rows, err := r.db(ctx).Query(ctx, query)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	wallets, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Wallet])
	if err != nil {
		r.logger.Error("failed to collect wallets rows", "error", err)
		return nil, err
	}

	return wallets, nil
}

// GetWalletByAddress retrieves a wallet by its address.
func (r *WalletsRepository) GetWalletByAddress(ctx context.Context, address string) (*entities.Wallet, error) {
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
		return nil, err
	}

	return &wallet, nil
}

// DeleteWallet removes a wallet from tracking.
func (r *WalletsRepository) DeleteWallet(ctx context.Context, address string) error {
	_, err := r.db(ctx).Exec(ctx, "DELETE FROM wallets WHERE address = $1", address)
	if err != nil {
		return fmt.Errorf("failed to delete wallet: %w", err)
	}

	r.logger.Info("Wallet removed from tracking", "address", address)
	return nil
}

// GetLastWalletIndex retrieves the last used wallet index
func (r *WalletsRepository) GetLastWalletIndex(ctx context.Context) (uint32, error) {
	var lastIndex uint32
	err := r.db(ctx).QueryRow(ctx, "SELECT COALESCE(MAX(wallet_index), 0) FROM wallets").Scan(&lastIndex)
	if err != nil {
		return 0, fmt.Errorf("failed to get last wallet index: %w", err)
	}
	return lastIndex, nil
}

// TrackWalletWithIndex adds a wallet address to the tracking system with a specific index
func (r *WalletsRepository) TrackWalletWithIndex(ctx context.Context, address string, derivationPath string, index uint32) error {
	// Check if wallet already exists
	exists, err := r.IsWalletTracked(ctx, address)
	if err != nil {
		return err
	}

	if exists {
		r.logger.Debug("Wallet already tracked", "address", address)
		return nil
	}

	// Insert new wallet with index
	_, err = r.db(ctx).Exec(ctx,
		"INSERT INTO wallets (address, derivation_path, wallet_index, created_at) VALUES ($1, $2, $3, $4)",
		address, derivationPath, index, time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert wallet: %w", err)
	}

	r.logger.Info("Wallet added to tracking", "address", address, "index", index)
	return nil
}

// GetLastWalletIndexForUser retrieves the last used wallet index for a specific user
func (r *WalletsRepository) GetLastWalletIndexForUser(ctx context.Context, userID string) (uint32, error) {
	var lastIndex uint32
	err := r.db(ctx).QueryRow(ctx,
		"SELECT COALESCE(MAX(wallet_index), 0) FROM wallets WHERE user_id = $1",
		userID).Scan(&lastIndex)
	if err != nil {
		return 0, fmt.Errorf("failed to get last wallet index for user %s: %w", userID, err)
	}
	return lastIndex, nil
}

// TrackWalletWithUserAndIndex adds a wallet address to the tracking system with a specific user and index
func (r *WalletsRepository) TrackWalletWithUserAndIndex(ctx context.Context, address string, derivationPath string, userID string, index uint32) error {
	// Check if wallet already exists
	exists, err := r.IsWalletTracked(ctx, address)
	if err != nil {
		return err
	}

	if exists {
		r.logger.Debug("Wallet already tracked", "address", address)
		return nil
	}

	// Insert new wallet with user ID and index
	_, err = r.db(ctx).Exec(ctx,
		"INSERT INTO wallets (address, derivation_path, user_id, wallet_index, created_at) VALUES ($1, $2, $3, $4, $5)",
		address, derivationPath, userID, index, time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert wallet: %w", err)
	}

	r.logger.Info("Wallet added to tracking", "address", address, "user", userID, "index", index)
	return nil
}

// GetAllTrackedWalletsForUser retrieves all tracked wallet addresses for a specific user.
func (r *WalletsRepository) GetAllTrackedWalletsForUser(ctx context.Context, userID string) ([]entities.Wallet, error) {
	query := `SELECT id, address, derivation_path, user_id, wallet_index, created_at 
              FROM wallets 
              WHERE user_id = $1
              ORDER BY wallet_index`

	rows, err := r.db(ctx).Query(ctx, query, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	wallets, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Wallet])
	if err != nil {
		r.logger.Error("failed to collect wallets rows", "error", err)
		return nil, err
	}

	return wallets, nil
}
