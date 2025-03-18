package usecases

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math/big"
	"strings"
	"sync"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/workers"

	"golang.org/x/exp/maps"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases/repository"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

type WalletsRepository interface {
	FindWalletByAddress(ctx context.Context, address string) (*entities.Wallet, error)
	FindWalletByID(ctx context.Context, id int) (*entities.Wallet, error)
	IsWalletTracked(ctx context.Context, address string) (bool, error)
	GetAllTrackedWallets(ctx context.Context) ([]entities.Wallet, error)
	GetLastWalletIndexForUser(ctx context.Context, userID int64) (uint32, error)
	TrackWalletWithUserAndIndex(ctx context.Context, address string, derivationPath string, userID int64, index uint32) (int, error)
	GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]entities.Wallet, error)
}

var _ WalletsRepository = (*repository.WalletsRepository)(nil)

type WalletService struct {
	logger *slog.Logger

	masterKey *bip32.Key
	wallets   map[string]bool // In-memory cache of tracked wallets
	walletsMu sync.RWMutex    // Mutex for wallets map

	repo WalletsRepository

	transactions *TransactionServiceImpl

	mu sync.Mutex
}

func NewWalletService(
	logger *slog.Logger,
	seed string,
	transactions *TransactionServiceImpl,
	walletsRepo *repository.WalletsRepository,
) (*WalletService, error) {
	ws := &WalletService{
		logger:       logger,
		masterKey:    CreateMasterKey(seed),
		wallets:      make(map[string]bool),
		transactions: transactions,
		repo:         walletsRepo,
	}

	// Load tracked wallets from database into memory cache
	if err := ws.loadWalletsFromDB(context.Background()); err != nil {
		logger.Error("Failed to load wallets from database", "error", err)
	}

	return ws, nil
}

// IsOurWallet checks if the given address belongs to our system
func (bsc *WalletService) IsOurWallet(ctx context.Context, address string) (bool, error) {
	// First check in-memory cache for performance
	bsc.walletsMu.RLock()
	cached, exists := bsc.wallets[address]
	bsc.walletsMu.RUnlock()

	if exists {
		return cached, nil
	}

	// If not in cache, check database
	tracked, err := bsc.repo.IsWalletTracked(ctx, address)
	if err != nil {
		return false, err
	}

	// Update cache if found in database
	if tracked {
		bsc.walletsMu.Lock()
		bsc.wallets[address] = true
		bsc.walletsMu.Unlock()
	}

	return tracked, nil
}

// GenerateWalletForUser generates a new wallet address for a specific user
func (bsc *WalletService) GenerateWalletForUser(ctx context.Context, userID int64) (int, string, error) {
	if bsc.masterKey == nil {
		return 0, "", errors.New("master key not initialized")
	}

	bsc.mu.Lock()
	defer bsc.mu.Unlock()

	// Get the last used index from the database for this user
	lastIndex, err := bsc.repo.GetLastWalletIndexForUser(ctx, userID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get last wallet index for user %d: %w", userID, err)
	}

	// Increment the index for the new wallet
	newIndex := lastIndex + 1

	// Create derivation path using the user ID and index
	// Use the user ID as part of the path to ensure uniqueness
	derivationPath := fmt.Sprintf("m/44'/60'/%d'/0/%d", userID, newIndex)

	// Get child key and private key
	childKey, err := GetChildKey(bsc.masterKey, userID, int64(newIndex))
	if err != nil {
		return 0, "", err
	}

	_, walletAddress, err := GetWalletPrivateKey(childKey)
	if err != nil {
		return 0, "", err
	}

	address := walletAddress.Hex()

	// Track this wallet in database with the user ID and index
	var walletID int
	if walletID, err = bsc.repo.TrackWalletWithUserAndIndex(ctx, address, derivationPath, userID, newIndex); err != nil {
		return 0, "", fmt.Errorf("failed to track wallet: %w", err)
	}

	// Update in-memory cache
	bsc.walletsMu.Lock()
	bsc.wallets[address] = true
	bsc.walletsMu.Unlock()

	bsc.logger.Info("Generated new wallet", "address", address, "path", derivationPath, "user", userID, "index", newIndex)
	return walletID, address, nil
}

// TrackWalletForUser adds a wallet address to the tracking system for a specific user
func (bsc *WalletService) TrackWalletForUser(ctx context.Context, address string, derivationPath string, userID int64) error {
	// Get the last used index from the database for this user
	lastIndex, err := bsc.repo.GetLastWalletIndexForUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get last wallet index for user %d: %w", userID, err)
	}

	// Increment the index for the new wallet
	newIndex := lastIndex + 1

	// Track this wallet in database with the user ID and index
	if _, err := bsc.repo.TrackWalletWithUserAndIndex(ctx, address, derivationPath, userID, newIndex); err != nil {
		return fmt.Errorf("failed to track wallet: %w", err)
	}

	// Update in-memory cache
	bsc.walletsMu.Lock()
	bsc.wallets[address] = true
	bsc.walletsMu.Unlock()

	return nil
}

// GetAllTrackedWalletsForUser retrieves all tracked wallet addresses for a specific user
func (bsc *WalletService) GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]string, error) {
	wallets, err := bsc.repo.GetAllTrackedWalletsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracked wallets for user %d: %w", userID, err)
	}

	addresses := make([]string, len(wallets))
	for i, wallet := range wallets {
		addresses[i] = wallet.Address
	}

	return addresses, nil
}

// GetWalletDetailsForUser retrieves wallet details (ID and address) for a specific user
func (bsc *WalletService) GetWalletDetailsForUser(ctx context.Context, userID int64) ([]entities.WalletDetail, error) {
	wallets, err := bsc.repo.GetAllTrackedWalletsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var walletDetails []entities.WalletDetail
	for _, wallet := range wallets {
		walletDetails = append(walletDetails, entities.WalletDetail{
			ID:      int64(wallet.ID),
			Address: wallet.Address,
		})
	}

	return walletDetails, nil
}

// TransferFunds transfers USDT from a deposit wallet to a destination wallet
func (bsc *WalletService) TransferFunds(ctx context.Context, fromWalletID int, toAddress string, amount *big.Int) (string, error) {
	if bsc.masterKey == nil {
		return "", errors.New("master key not initialized")
	}

	// Get wallet details from database
	wallet, err := bsc.repo.FindWalletByID(ctx, fromWalletID)
	if err != nil {
		return "", fmt.Errorf("failed to find wallet with ID %d: %w", fromWalletID, err)
	}

	if wallet == nil {
		return "", fmt.Errorf("wallet with ID %d not found", fromWalletID)
	}

	// Parse derivation path to get the child key index
	derivationPath := wallet.DerivationPath
	bsc.logger.Info("Using derivation path", "path", derivationPath, "wallet", wallet.Address)

	// Extract user ID and index from derivation path
	userID, index, err := ParseDerivationPath(derivationPath)
	if err != nil {
		return "", err
	}

	// Get child key and private key
	childKey, err := GetChildKey(bsc.masterKey, userID, index)
	if err != nil {
		return "", err
	}

	privateKey, fromAddress, err := GetWalletPrivateKey(childKey)
	if err != nil {
		return "", err
	}

	// Connect to blockchain
	client, err := GetBSCClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	// Get the latest nonce for the sender address
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Create token transfer data
	// USDT contract address on BSC
	tokenAddress := common.HexToAddress(workers.USDTContractAddress)

	// Create ERC20 transfer data
	data := CreateERC20TransferData(toAddress, amount)

	// Estimate gas limit
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  fromAddress,
		To:    &tokenAddress,
		Value: big.NewInt(0),
		Data:  data,
	})
	if err != nil {
		return "", fmt.Errorf("failed to estimate gas: %w", err)
	}

	// Add 20% buffer to gas limit
	gasLimit = gasLimit * 12 / 10

	// Create the transaction
	tx := types.NewTransaction(
		nonce,
		tokenAddress,
		big.NewInt(0), // We're not sending ETH, just tokens
		gasLimit,
		gasPrice,
		data,
	)

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	bsc.logger.Info("Transaction sent",
		"from", fromAddress.Hex(),
		"to", toAddress,
		"amount", amount.String(),
		"tx_hash", txHash)

	return txHash, nil
}

// loadWalletsFromDB loads all tracked wallets from the database into memory
func (bsc *WalletService) loadWalletsFromDB(ctx context.Context) error {
	wallets, err := bsc.repo.GetAllTrackedWallets(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tracked wallets: %w", err)
	}

	bsc.walletsMu.Lock()
	defer bsc.walletsMu.Unlock()

	// Clear the existing map and repopulate it
	maps.Clear(bsc.wallets)
	for _, wallet := range wallets {
		bsc.wallets[wallet.Address] = true
	}

	bsc.logger.Info("Loaded wallets from database", "count", len(wallets))
	return nil
}

func CreateMasterKey(seed string) *bip32.Key {
	seedBytes := bip39.NewSeed(seed, "")
	masterKey, err := bip32.NewMasterKey(seedBytes)
	if err != nil {
		log.Fatal("Failed to create master key")
	}

	return masterKey
}

// GetBSCClient connects to one of the BSC RPC endpoints
func GetBSCClient(ctx context.Context) (*ethclient.Client, error) {
	// Список RPC эндпоинтов BSC (для резервирования)
	bscRpcEndpoints := []string{
		"https://bsc-dataseed.binance.org/",
		"https://bsc-dataseed1.binance.org/",
		"https://bsc-dataseed2.binance.org/",
		"https://bsc-dataseed3.binance.org/",
		"https://bsc-dataseed4.binance.org/",
	}

	// Пробуем подключиться к разным эндпоинтам
	var client *ethclient.Client
	var err error
	var lastErr error

	for _, endpoint := range bscRpcEndpoints {
		slog.Info("Trying to connect to BSC endpoint", "endpoint", endpoint)
		client, err = ethclient.DialContext(ctx, endpoint)
		if err == nil {
			slog.Info("Successfully connected to BSC", "endpoint", endpoint)
			return client, nil
		}
		lastErr = err
		slog.Warn("Failed to connect to BSC endpoint", "endpoint", endpoint, "error", err)
	}

	return nil, fmt.Errorf("failed to connect to any BSC endpoint: %w", lastErr)
}

// GetChildKey generates a child key from the master key based on user ID and index
func GetChildKey(masterKey *bip32.Key, userID, index int64) (*bip32.Key, error) {
	// Create a unique child key based on both user ID and index
	childKeyIndex := uint32(userID*1000 + index)
	childKey, err := masterKey.NewChildKey(childKeyIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to create child key: %w", err)
	}
	return childKey, nil
}

// GetWalletPrivateKey converts a child key to an ECDSA private key
func GetWalletPrivateKey(childKey *bip32.Key) (*ecdsa.PrivateKey, common.Address, error) {
	privateKey, err := crypto.ToECDSA(childKey.Key)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to convert to ECDSA: %w", err)
	}

	// Get the wallet address from the private key
	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	return privateKey, address, nil
}

// WeiToEther converts wei amount to ether (or any token with 18 decimals)
func WeiToEther(wei *big.Int) *big.Float {
	return new(big.Float).Quo(
		new(big.Float).SetInt(wei),
		new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
	)
}

// EtherToWei converts ether amount to wei
func EtherToWei(ether *big.Float) *big.Int {
	// Create wei representation (10^18 wei = 1 ether)
	ethWei := new(big.Float).Mul(
		ether,
		new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
	)

	// Convert to big.Int
	wei := new(big.Int)
	ethWei.Int(wei)
	return wei
}

// ParseDerivationPath extracts userID and index from derivation path
func ParseDerivationPath(derivationPath string) (int64, int64, error) {
	var userID, index int64

	// m – корневой (master) ключ.
	// 44' – следование стандарту BIP-44 (универсальный путь для multi-account HD-кошельков).
	// 60' – идентификатор монеты. В данном случае 60 – это Ethereum, согласно https://github.com/satoshilabs/slips/blob/master/slip-0044.md
	// %d' – ID аккаунта (в коде он передается в userID), используется для разделения кошельков по пользователям.
	// 0 – chain type, где:
	//	•	0 – внешние (external) адреса (обычно используются для получения средств).
	//	•	1 – внутренние (internal) адреса (обычно для сдачи в транзакциях).
	// %d – индекс адреса в этой цепочке (index), определяет конкретный адрес внутри аккаунта.
	_, err := fmt.Sscanf(derivationPath, "m/44'/60'/%d'/0/%d", &userID, &index)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse derivation path: %w", err)
	}
	return userID, index, nil
}

// GetERC20TokenBalance retrieves the balance of ERC20 token for an address
func GetERC20TokenBalance(ctx context.Context, client *ethclient.Client, contractAddress, walletAddress string, tokenABI string) (*big.Int, error) {
	tokenAddr := common.HexToAddress(contractAddress)
	parsedABI, err := abi.JSON(strings.NewReader(tokenABI))
	if err != nil {
		return nil, fmt.Errorf("error parsing ABI: %w", err)
	}

	address := common.HexToAddress(walletAddress)

	// Prepare data for balanceOf call
	data, err := parsedABI.Pack("balanceOf", address)
	if err != nil {
		return nil, fmt.Errorf("error packing data for balanceOf: %w", err)
	}

	// Call the contract
	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddr,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("error calling token contract: %w", err)
	}

	// Unpack the result
	var tokenBalance *big.Int
	err = parsedABI.UnpackIntoInterface(&tokenBalance, "balanceOf", result)
	if err != nil {
		return nil, fmt.Errorf("error unpacking balanceOf result: %w", err)
	}

	return tokenBalance, nil
}

// CreateERC20TransferData creates transaction data for ERC20 token transfer
func CreateERC20TransferData(toAddress string, amount *big.Int) []byte {
	// Create token transfer data
	transferFnSignature := []byte("transfer(address,uint256)")
	hash := crypto.Keccak256(transferFnSignature)
	methodID := hash[:4]

	// Pad the address to 32 bytes
	paddedAddress := common.LeftPadBytes(common.HexToAddress(toAddress).Bytes(), 32)

	// Pad the amount to 32 bytes
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

	// Combine the method ID, address, and amount
	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)

	return data
}
