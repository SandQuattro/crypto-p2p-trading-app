package main

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	cfg "github.com/sand/crypto-p2p-trading-app/backend/config"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases/mocked"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases/repository"
	"github.com/sand/crypto-p2p-trading-app/backend/pkg/database"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/handlers"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases"
)

// Server timeout constants.
const (
	readTimeoutSeconds     = 15
	writeTimeoutSeconds    = 15
	idleTimeoutSeconds     = 60
	shutdownTimeoutSeconds = 5
	migrationsPath         = "./migrations"
)

func main() {
	ctx := context.Background()
	config, err := cfg.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	logger.Info("Starting application with configuration",
		"rpc_url", config.RPCURL,
		"server_port", config.HTTP.Port,
		"database_url", config.DB.DatabaseURL)

	// Connect to Database
	pg, err := database.New(config,
		database.MaxPoolSize(config.DB.PoolMax),
		database.ConnTimeout(config.DB.ConnectTimeout),
		database.HealthCheckPeriod(config.DB.HealthCheckPeriod),
		database.Isolation(pgx.ReadCommitted),
	)
	if err != nil {
		slog.Error("postgres connection failed", slog.String("error", err.Error()))
		return
	}
	defer pg.Close()

	// Run database migrations
	logger.Info("Running database migrations", "path", migrationsPath)
	if err = database.RunMigrations(logger, config.DatabaseURL, migrationsPath); err != nil {
		logger.Error("Failed to run database migrations", "error", err)
		log.Fatal(err)
	}
	logger.Info("Database migrations completed successfully")

	// Create repositories
	ordersRepository := repository.NewOrdersRepository(logger, pg)
	transactionsRepository := repository.NewTransactionsRepository(logger, pg, ordersRepository)

	// Create usecases and components
	dataService := mocked.NewDataService(logger)
	dataService.InitializeTradingPairs()

	orderService := usecases.NewOrderService(ordersRepository)
	transactionService := usecases.NewTransactionService(transactionsRepository)

	walletService, err := usecases.NewWalletService(logger, transactionService, config.WalletSeed)
	if err != nil {
		logger.Error("Failed to create wallet service", "error", err)
		log.Fatal(err)
	}
	walletService.SetLogger(logger)

	// Create handlers
	websocketManager := handlers.NewWebSocketManager(logger)
	httpHandler := handlers.NewHTTPHandler(logger, dataService, walletService, orderService, transactionService)
	wsHandler := handlers.NewWebSocketHandler(logger, dataService, websocketManager)

	// Create router
	router := mux.NewRouter()

	// Register WebSocket routes before HTTP routes
	wsHandler.RegisterRoutes(router)
	httpHandler.RegisterRoutes(router)

	// Configure CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	// Wrap router in CORS middleware
	handler := c.Handler(router)

	// Create HTTP server with timeouts
	server := &http.Server{
		Addr:         ":" + config.HTTP.Port,
		Handler:      handler,
		ReadTimeout:  readTimeoutSeconds * time.Second,
		WriteTimeout: writeTimeoutSeconds * time.Second,
		IdleTimeout:  idleTimeoutSeconds * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting server", "port", config.HTTP.Port)
		if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server error", "error", err)
			log.Fatal(err)
		}
	}()

	// Start blockchain subscription in a goroutine
	go func() {
		walletService.SubscribeToTransactions(ctx, transactionService, config.RPCURL)
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Give 5 seconds to complete current requests
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeoutSeconds*time.Second)
	defer cancel()

	if err = server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
		return
	}

	logger.Info("Server exited properly")
}
