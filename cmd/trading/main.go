package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	cfg "github.com/sand/crypto-p2p-trading-app/backend/config"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/core/ports"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases/mocked"
	repository "github.com/sand/crypto-p2p-trading-app/backend/internal/usecases/repository"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/workers"
	"github.com/sand/crypto-p2p-trading-app/backend/pkg/database"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	amlservices "github.com/sand/crypto-p2p-trading-app/backend/internal/aml/clients"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/handlers"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases"
)

// Server timeout constants.
const (
	readTimeoutSeconds     = 15
	writeTimeoutSeconds    = 15
	idleTimeoutSeconds     = 60
	shutdownTimeoutSeconds = 5
)

func main() {
	// Устанавливаем timezone UTC
	time.Local = time.UTC

	// Parse configuration
	config, err := cfg.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Setup logging
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if config.App.Debug {
		opts.Level = slog.LevelDebug
	}

	ctx := context.Background()

	// Определяем путь к миграциям
	migrationsPath := "./migrations"
	if workDir, err := os.Getwd(); err == nil {
		// Пробуем сначала относительный путь
		if _, err := os.Stat(filepath.Join(workDir, "migrations")); !os.IsNotExist(err) {
			migrationsPath = filepath.Join(workDir, "migrations")
		} else if _, err := os.Stat(filepath.Join(workDir, "..", "migrations")); !os.IsNotExist(err) {
			// Если не нашли, пробуем на уровень выше
			migrationsPath = filepath.Join(workDir, "..", "migrations")
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	logger.Warn("Starting application with configuration",
		"debug", config.App.Debug,
		"blockchain_debug", config.Blockchain.Debug,
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
	walletsRepository := repository.NewWalletsRepository(logger, pg)
	transactionsRepository := repository.NewTransactionsRepository(logger, pg, ordersRepository, walletsRepository)

	// Create usecases and components
	dataService := mocked.NewDataService(logger)
	dataService.InitializeTradingPairs()

	orderService := usecases.NewOrderService(ordersRepository)
	transactionService := usecases.NewTransactionService(transactionsRepository)

	walletService, err := usecases.NewWalletService(logger, config.WalletSeed, transactionService, walletsRepository, orderService)
	if err != nil {
		logger.Error("Failed to create wallet service", "error", err)
		log.Fatal(err)
	}

	// Инициализируем AML сервис
	amlService := initAMLService(logger, config, pg, transactionService)

	// Initialize and run workers
	initAndRunWorkers(ctx, logger, config, orderService, transactionService, walletService, amlService)

	// create gRPC clients
	bscClient, err := usecases.GetBSCClient(ctx, logger)
	if err != nil {
		log.Fatal(err)
	}
	defer bscClient.Close()

	// Create handlers
	websocketManager := handlers.NewWebSocketManager(logger)
	httpHandler := handlers.NewHTTPHandler(logger, bscClient, dataService, walletService, orderService, transactionService)
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
		logger.Info("Starting server", "address", server.Addr)
		if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server error", "error", err)
			log.Fatal(err)
		}
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

func initAMLService(logger *slog.Logger, config *cfg.Config, pg *database.Postgres, transactionService *usecases.TransactionServiceImpl) *usecases.AMLService {
	// Создаем AML репозиторий
	amlRepository := repository.NewAMLRepository(logger, pg)

	// Инициализируем сервисы проверки
	chainalysisService := amlservices.NewChainalysisService(
		logger,
		config.AML.ChainalysisAPIKey,
		config.AML.ChainalysisAPIURL,
	)

	ellipticService := amlservices.NewEllipticService(
		logger,
		config.AML.EllipticAPIKey,
		config.AML.EllipticAPIURL,
	)

	localAMLService := amlservices.NewLocalAMLService(
		logger,
		config.AML.TransactionThreshold,
	)

	amlbotService := amlservices.NewAMLBotService(
		logger,
		config.AML.AMLBotAPIKey,
		config.AML.AMLBotAPIURL,
	)

	// Создаем основной AML сервис
	amlService := usecases.NewAMLService(
		logger,
		amlRepository,
		chainalysisService,
		ellipticService,
		localAMLService,
		amlbotService,
		transactionService, // Используем transactionService из параметров
		pg.Transactor,      // Добавляем транзактор
	)

	logger.Info("AML service initialized",
		"chainalysis_enabled", chainalysisService.IsEnabled(),
		"elliptic_enabled", ellipticService.IsEnabled(),
		"amlbot_enabled", amlbotService.IsEnabled(),
	)

	return amlService
}

func initAndRunWorkers(
	ctx context.Context,
	logger *slog.Logger,
	config *cfg.Config,
	orderService *usecases.OrderService,
	transactionService *usecases.TransactionServiceImpl,
	walletService *usecases.WalletService,
	amlService ports.AMLService,
) {
	// Initialize blockchain processor с реальным AML сервисом
	bscBlockchainProcessor := workers.NewBinanceSmartChain(logger, config, transactionService, walletService, amlService, orderService)

	// Initialize order cleaner worker with configuration from config
	orderCleaner := workers.NewOrderCleaner(
		logger,
		orderService,
		time.Duration(config.Workers.OrderExpiration)*time.Minute,      // Use OrderExpiration from config (in minutes)
		time.Duration(config.Workers.OrderCleanupInterval)*time.Minute, // Use OrderCleanupInterval from config (in minutes)
	)

	// Start blockchain subscription in a goroutine
	go func() {
		logger.Info("Starting blockchain monitoring worker")
		bscBlockchainProcessor.SubscribeToTransactions(ctx, config.RPCURL)
	}()

	// Start order cleaner worker in a goroutine
	go func() {
		logger.Info("Starting order cleaner worker")
		orderCleaner.Start(ctx)
	}()

	logger.Info("All workers initialized and started")
}
