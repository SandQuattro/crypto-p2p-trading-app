package workers

import (
	"context"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/handlers"
	"log/slog"
	"time"
)

// OrderCleaner worker automatically removes old pending orders
type OrderCleaner struct {
	logger       *slog.Logger
	orderService handlers.OrderService

	// Duration after which orders are considered old and should be removed
	expirationDuration time.Duration

	// How often to run the cleanup process
	cleanupInterval time.Duration
}

// NewOrderCleaner creates a new order cleaner worker
func NewOrderCleaner(
	logger *slog.Logger,
	orderService handlers.OrderService,
	expirationDuration time.Duration,
	cleanupInterval time.Duration,
) *OrderCleaner {
	return &OrderCleaner{
		logger:             logger,
		orderService:       orderService,
		expirationDuration: expirationDuration,
		cleanupInterval:    cleanupInterval,
	}
}

// Start begins the periodic cleanup of old orders
func (oc *OrderCleaner) Start(ctx context.Context) {
	oc.logger.Info("Starting order cleaner worker",
		"expiration_time", oc.expirationDuration.String(),
		"cleanup_interval", oc.cleanupInterval.String())

	// Run an initial cleanup immediately
	if err := oc.cleanupOldOrders(ctx); err != nil {
		oc.logger.Error("Initial order cleanup failed", "error", err)
	}

	// Start the ticker for periodic cleanup
	ticker := time.NewTicker(oc.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			oc.logger.Info("Order cleaner worker stopped")
			return
		case <-ticker.C:
			if err := oc.cleanupOldOrders(ctx); err != nil {
				oc.logger.Error("Order cleanup failed", "error", err)
			}
		}
	}
}

// cleanupOldOrders performs the actual cleanup of old orders
func (oc *OrderCleaner) cleanupOldOrders(ctx context.Context) error {
	oc.logger.Debug("Starting cleanup of old orders", "older_than", oc.expirationDuration.String())

	// Remove orders older than the specified duration
	count, err := oc.orderService.RemoveOldOrders(ctx, oc.expirationDuration)
	if err != nil {
		return err
	}

	if count > 0 {
		oc.logger.Info("Removed old orders", "count", count, "older_than", oc.expirationDuration.String())
	} else {
		oc.logger.Debug("No old orders to remove")
	}

	return nil
}
