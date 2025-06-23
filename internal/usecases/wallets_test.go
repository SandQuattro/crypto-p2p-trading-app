package usecases

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/shared"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

// Constants for testing
const (
	seed = "your secure seed phrase here" // SEED ФРАЗА

	// Адрес депозитного кошелька пользователя
	depositUserWalletAddress = "0x986fc2a160b89e797f3e208fAB3cB97CCB67a359"
	derivationPath           = "m/44'/60'/2'/0/1"
	USDTTransferAmount       = 5.50

	// Адрес кошелька получателя для вывода с депозитных кошельков
	usdtMasterAddress = "0x0806f768B8f4673adc0aDBD70B1C3Db4767e148e"

	// Адрес кошелька получателя BNB
	bnbMasterAddress = "0x6919A6ff55ffDE58eF7b3366752beDbAba0485b4"
)

// TestCheckBalances проверяет баланс BNB и USDT на кошельке
func TestCheckBalances(t *testing.T) {
	// Пропускаем тест, если запущен в режиме короткого тестирования
	if testing.Short() {
		t.Skip("Пропускаем тест в режиме короткого тестирования")
	}

	// os.Setenv(shared.EnvBlockchainDebugMode, "true")

	// Log which blockchain network we're using
	if shared.IsBlockchainDebugMode() {
		t.Logf("Running test in DEBUG mode (using BSC Testnet)")
	} else {
		t.Logf("Running test in PRODUCTION mode (using BSC Mainnet)")
	}

	// Создаем контекст с увеличенным таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := GetBSCClient(ctx, slog.Default())
	require.NoError(t, err, "Ошибка подключения к блокчейну")
	defer client.Close()

	// Проверяем баланс USDT депозитного кошелька пользователя
	balanceUSDT, err := getTokenBalance(ctx, client, depositUserWalletAddress)
	require.NoError(t, err, "Ошибка получения баланса USDT")

	// Преобразуем в читаемый формат
	usdtBalance := WeiToEther(balanceUSDT)

	t.Logf("Баланс депозитного кошелька пользователя %s в USDT: %s (%s wei)",
		depositUserWalletAddress, usdtBalance.Text('f', 18), balanceUSDT.String())

	// Проверяем текущий баланс BNB депозитного кошелька пользователя
	address := common.HexToAddress(depositUserWalletAddress)
	balance, err := client.BalanceAt(ctx, address, nil)
	require.NoError(t, err, "Ошибка получения баланса BNB")

	// Конвертируем баланс из wei в BNB для удобства чтения
	bnbBalance := WeiToEther(balance)
	t.Logf("Баланс депозитного кошелька пользователя %s в BNB:  %s (%s wei)",
		depositUserWalletAddress, bnbBalance.Text('f', 18), balance.String())

	// Минимальный баланс BNB для оплаты газа (0.0001 BNB)
	// На основе фактических данных перевод USDT стоит около 0.00003 BNB
	// С запасом для безопасности выставляем 0.0001 BNB (примерно 3 транзакции)
	minBalance := new(big.Int).Mul(big.NewInt(100000), big.NewInt(1000000000))

	// Проверяем, достаточно ли BNB для оплаты газа
	if balance.Cmp(minBalance) < 0 {
		t.Logf("Недостаточно BNB для оплаты газа: %s < %s (%s BNB < %s BNB)",
			balance.String(), minBalance.String(),
			bnbBalance.Text('f', 9), WeiToEther(minBalance).Text('f', 9))
	} else {
		t.Logf("Достаточно BNB для оплаты газа: %s >= %s (%s BNB >= %s BNB)",
			balance.String(), minBalance.String(),
			bnbBalance.Text('f', 9), WeiToEther(minBalance).Text('f', 9))
	}
}

// TestTransferFunds отправляет USDT с кошелька на другой адрес
func TestTransferFunds(t *testing.T) {
	// Пропускаем тест, если запущен в режиме короткого тестирования
	if testing.Short() {
		t.Skip("Пропускаем тест в режиме короткого тестирования")
	}

	// Log which blockchain network we're using
	if shared.IsBlockchainDebugMode() {
		t.Logf("Running test in DEBUG mode (using BSC Testnet)")
	} else {
		t.Logf("Running test in PRODUCTION mode (using BSC Mainnet)")
	}

	// Создаем контекст с увеличенным таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Подключаемся к блокчейну через хелпер-функцию
	client, err := GetBSCClient(ctx, slog.Default())
	require.NoError(t, err, "Ошибка подключения к блокчейну")
	defer client.Close()

	// Проверяем текущий баланс BNB депозитного кошелька пользователя
	address := common.HexToAddress(depositUserWalletAddress)
	balance, err := client.BalanceAt(ctx, address, nil)
	require.NoError(t, err, "Ошибка получения баланса BNB")

	// Минимальный баланс BNB для оплаты газа (0.0001 BNB)
	// На основе фактических данных перевод USDT стоит около 0.00003 BNB
	minBalance := new(big.Int).Mul(big.NewInt(100000), big.NewInt(1000000000))

	// Конвертируем в wei
	amountIntWei := EtherToWei(big.NewFloat(USDTTransferAmount))
	t.Logf("Пытаемся перевести токены USDT: %f", USDTTransferAmount)

	// Проверяем, достаточно ли BNB для оплаты газа
	if balance.Cmp(minBalance) < 0 {
		t.Logf("Недостаточно BNB для оплаты газа: %s < %s",
			balance.String(), minBalance.String())
		t.Skip("Пропускаем тест из-за недостаточного количества BNB для оплаты газа")
		return
	}

	// Конвертируем баланс из wei в BNB для удобства чтения
	bnbBalance := WeiToEther(balance)
	t.Logf("Баланс депозитного кошелька пользователяв BNB перед отправкой: %s", bnbBalance.Text('f', 18))

	// Вызываем функцию перевода
	txHash, err := TransferTokens(ctx, usdtMasterAddress, amountIntWei)
	if err != nil {
		t.Logf("Ошибка перевода средств: %s", err.Error())
		return
	}

	t.Logf("Транзакция перевода: %s", txHash)

	// Ждем подтверждения транзакции
	t.Logf("Ожидание подтверждения транзакции...")
	time.Sleep(10 * time.Second)

	// Проверяем балансы после перевода
	t.Logf("Балансы после перевода:")

	// Проверяем баланс USDT депозитного кошелька пользователяпосле перевода
	balanceUSDT, err := getTokenBalance(ctx, client, depositUserWalletAddress)
	require.NoError(t, err, "Ошибка получения баланса USDT")

	// Преобразуем в читаемый формат
	usdtBalance := WeiToEther(balanceUSDT)

	t.Logf("Баланс депозитного кошелька пользователя%s в USDT после перевода: %s (%s wei)",
		depositUserWalletAddress, usdtBalance.Text('f', 18), balanceUSDT.String())

	// Проверяем баланс USDT получателя после перевода
	receiverBalanceUSDT, err := getTokenBalance(ctx, client, usdtMasterAddress)
	require.NoError(t, err, "Ошибка получения баланса USDT получателя")

	// Преобразуем в читаемый формат
	receiverUsdtBalance := WeiToEther(receiverBalanceUSDT)

	t.Logf("Баланс получателя %s в USDT после перевода: %s (%s wei)",
		usdtMasterAddress, receiverUsdtBalance.Text('f', 18), receiverBalanceUSDT.String())
}

// TestTransferAllBNB отправляет все BNB с кошелька на другой адрес
func TestTransferAllBNB(t *testing.T) {
	// Пропускаем тест, если запущен в режиме короткого тестирования
	if testing.Short() {
		t.Skip("Пропускаем тест в режиме короткого тестирования")
	}

	// os.Setenv(EnvBlockchainDebugMode, "true")

	// Log which blockchain network we're using
	if shared.IsBlockchainDebugMode() {
		t.Logf("Running test in DEBUG mode (using BSC Testnet)")
	} else {
		t.Logf("Running test in PRODUCTION mode (using BSC Mainnet)")
	}

	// Создаем контекст с увеличенным таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Подключаемся к блокчейну через хелпер-функцию
	client, err := GetBSCClient(ctx, slog.Default())
	require.NoError(t, err, "Ошибка подключения к блокчейну")
	defer client.Close()

	// Проверяем текущий баланс BNB депозитного кошелька пользователя
	address := common.HexToAddress(depositUserWalletAddress)
	balance, e := client.BalanceAt(ctx, address, nil)
	require.NoError(t, e, "Ошибка получения баланса BNB")

	// Проверяем, есть ли BNB для отправки
	if balance.Cmp(big.NewInt(0)) <= 0 {
		t.Skipf("Баланс %s BNB равен %d, нечего отправлять", address, balance)
		return
	}

	// Конвертируем баланс из wei в BNB для удобства чтения
	bnbBalance := WeiToEther(balance)
	t.Logf("Текущий баланс депозитного кошелька пользователя %s в BNB: %s (%s wei)",
		depositUserWalletAddress, bnbBalance.Text('f', 18), balance.String())

	// Проверяем текущий баланс BNB получателя
	destAddress := common.HexToAddress(bnbMasterAddress)
	destBalance, e := client.BalanceAt(ctx, destAddress, nil)
	require.NoError(t, e, "Ошибка получения баланса BNB получателя")

	// Конвертируем баланс из wei в BNB для удобства чтения
	destBnbBalance := WeiToEther(destBalance)
	t.Logf("Текущий баланс получателя %s в BNB: %s (%s wei)",
		bnbMasterAddress, destBnbBalance.Text('f', 18), destBalance.String())

	// Create a real WalletService instance to call TransferAllBNB
	walletService := &WalletService{
		// You need to pass necessary values here
		logger:               slog.Default(),
		seed:                 seed,
		erc20ABI:             `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"type":"function"}]`,
		smartContractAddress: GetUSDTContractAddress(),
		pendingTxs:           make(map[string]*PendingTransaction),
		pendingTxsByAddr:     make(map[common.Address]map[uint64]string),
		walletBalances:       make(map[string]*entities.WalletBalance),
		wallets:              make(map[string]bool),
	}

	// Извлекаем userID и index из пути деривации
	userID, index, err := ParseDerivationPath(derivationPath)
	if err != nil {
		t.Logf("Ошибка ParseDerivationPath: %s", err.Error())
		return
	}

	// Вызываем функцию перевода всех BNB
	txHash, e := walletService.TransferAllBNB(ctx, bnbMasterAddress, depositUserWalletAddress, int(userID), int(index))
	if e != nil {
		t.Logf("Ошибка перевода BNB: %s", e.Error())
		return
	}

	t.Logf("Транзакция перевода BNB: %s", txHash)

	// Ждем подтверждения транзакции
	t.Logf("Ожидание подтверждения транзакции...")
	time.Sleep(10 * time.Second)

	// Проверяем балансы после перевода
	t.Logf("Балансы после перевода:")

	// Проверяем баланс BNB депозитного кошелька пользователяпосле перевода
	newBalance, e := client.BalanceAt(ctx, address, nil)
	require.NoError(t, e, "Ошибка получения баланса BNB")

	// Конвертируем баланс из wei в BNB для удобства чтения
	newBnbBalance := WeiToEther(newBalance)
	t.Logf("Баланс депозитного кошелька пользователя %s в BNB после перевода: %s (%s wei)",
		depositUserWalletAddress, newBnbBalance.Text('f', 18), newBalance.String())

	// Проверяем баланс BNB получателя после перевода
	newDestBalance, e := client.BalanceAt(ctx, destAddress, nil)
	require.NoError(t, e, "Ошибка получения баланса BNB получателя")

	// Конвертируем баланс из wei в BNB для удобства чтения
	newDestBnbBalance := WeiToEther(newDestBalance)
	t.Logf("Баланс получателя %s в BNB после перевода: %s (%s wei)",
		bnbMasterAddress, newDestBnbBalance.Text('f', 18), newDestBalance.String())

	// Рассчитываем разницу балансов
	var destDiff big.Int
	destDiff.Sub(newDestBalance, destBalance)
	destDiffBnb := WeiToEther(&destDiff)
	t.Logf("Получатель получил: %s BNB", destDiffBnb.Text('f', 18))

	// Рассчитываем комиссию
	var balanceDiff big.Int
	balanceDiff.Sub(balance, newBalance)
	var received big.Int
	received.Sub(&balanceDiff, &destDiff)
	feeBnb := WeiToEther(&received)
	t.Logf("Комиссия сети составила: %s BNB", feeBnb.Text('f', 18))
}

// Функция для перевода токенов
func TransferTokens(ctx context.Context, toAddress string, amount *big.Int) (string, error) {
	slog.Info("Transfer request", "amount", amount.String(), "to", toAddress)

	// Используем seed фразу для генерации кошелька
	masterKey := CreateMasterKey(seed)

	// Извлекаем userID и index из пути деривации
	userID, index, err := ParseDerivationPath(derivationPath)
	if err != nil {
		return "", err
	}

	// Получаем child key
	childKey, err := GetChildKey(masterKey, userID, index)
	if err != nil {
		return "", err
	}

	// Конвертируем в ECDSA приватный ключ
	privateKey, fromAddress, err := GetWalletPrivateKey(childKey)
	if err != nil {
		return "", err
	}

	// Проверяем, что адрес соответствует ожидаемому
	generatedAddress := fromAddress.Hex()
	expectedAddress := depositUserWalletAddress

	if !strings.EqualFold(generatedAddress, expectedAddress) {
		slog.Warn("Generated address doesn't match expected",
			"generated", generatedAddress,
			"expected", expectedAddress)
		return "", fmt.Errorf("cannot derive correct private key for wallet %s, generated %s instead",
			expectedAddress, generatedAddress)
	}

	slog.Info("Successfully derived private key for wallet", "address", generatedAddress)

	// Подключаемся к блокчейну
	client, err := GetBSCClient(ctx, slog.Default())
	if err != nil {
		return "", err
	}
	defer client.Close()

	// Логгируем адрес депозитного кошелька пользователя
	slog.Info("Sending transaction from address", "from", fromAddress.Hex())

	// Получаем nonce
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Use the appropriate USDT contract address based on mode
	tokenAddress := common.HexToAddress(GetUSDTContractAddress())

	// Создаем данные для ERC20 transfer
	data := CreateERC20TransferData(toAddress, amount)

	// Оцениваем лимит газа
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  fromAddress,
		To:    &tokenAddress,
		Value: big.NewInt(0),
		Data:  data,
	})
	if err != nil {
		return "", fmt.Errorf("failed to estimate gas: %w", err)
	}

	// Добавляем 20% буфер к лимиту газа
	gasLimit = gasLimit * 12 / 10

	// Создаем транзакцию
	tx := types.NewTransaction(
		nonce,
		tokenAddress,
		big.NewInt(0), // Не отправляем BNB, только токены
		gasLimit,
		gasPrice,
		data,
	)

	// Получаем ID цепи
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Подписываем транзакцию
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Отправляем транзакцию
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	slog.Info("Transaction sent",
		"from", fromAddress.Hex(),
		"to", toAddress,
		"amount", amount.String(),
		"tx_hash", txHash)

	return txHash, nil
}

// Вспомогательная функция для получения баланса токена
func getTokenBalance(ctx context.Context, client *ethclient.Client, addressStr string) (*big.Int, error) {
	// Use the actual WalletService already defined in wallets.go
	service := &WalletService{
		erc20ABI:             `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"type":"function"}]`,
		smartContractAddress: GetUSDTContractAddress(), // Use dynamic address based on mode
	}
	return service.GetERC20TokenBalance(ctx, client, addressStr)
}

// TestGenerateSeedPhrase tests the seed phrase generation functionality
func TestGenerateSeedPhrase(t *testing.T) {
	service := &WalletService{
		erc20ABI:             `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"type":"function"}]`,
		smartContractAddress: GetUSDTContractAddress(), // Use dynamic address based on mode
	}

	t.Run("Valid entropy values", func(t *testing.T) {
		// Test all valid entropy bit values
		entropyTests := []struct {
			bits        int
			wordCount   int
			description string
		}{
			{128, 12, "12 words seed phrase"},
			{160, 15, "15 words seed phrase"},
			{192, 18, "18 words seed phrase"},
			{224, 21, "21 words seed phrase"},
			{256, 24, "24 words seed phrase"},
		}

		for _, test := range entropyTests {
			t.Run(test.description, func(t *testing.T) {
				// Generate seed phrase
				mnemonic, err := service.GenerateSeedPhrase(test.bits)

				// Check there's no error
				require.NoError(t, err, "GenerateSeedPhrase should not return an error for valid entropy bits")

				// Count the words in the seed phrase
				words := strings.Split(mnemonic, " ")
				require.Equal(t, test.wordCount, len(words),
					"Seed phrase should have %d words when using %d entropy bits", test.wordCount, test.bits)

				// Verify the seed phrase contains only valid BIP39 words
				// (This is an implicit check since the library would error if invalid words were generated)
				require.NotEmpty(t, mnemonic, "Seed phrase should not be empty")

				t.Logf("Generated %d-word seed phrase successfully: \"%s\"", test.wordCount, mnemonic)
			})
		}
	})

	t.Run("Invalid entropy values", func(t *testing.T) {
		invalidEntropies := []int{0, 64, 100, 129, 300}

		for _, bits := range invalidEntropies {
			t.Run(fmt.Sprintf("%d bits", bits), func(t *testing.T) {
				// Try to generate seed phrase with invalid entropy
				mnemonic, err := service.GenerateSeedPhrase(bits)

				// Check that it returns an error
				require.Error(t, err, "GenerateSeedPhrase should return an error for invalid entropy bits: %d", bits)
				require.Empty(t, mnemonic, "Mnemonic should be empty when error occurs")
				require.Contains(t, err.Error(), "invalid entropy bits",
					"Error message should mention invalid entropy bits")
			})
		}
	})

	t.Run("Uniqueness of generated phrases", func(t *testing.T) {
		// Generate multiple seed phrases and ensure they're unique
		phrasesMap := make(map[string]bool)
		count := 5

		t.Log("Generated unique seed phrases:")
		for i := 0; i < count; i++ {
			mnemonic, err := service.GenerateSeedPhrase(128)
			require.NoError(t, err)

			// Check that we haven't seen this phrase before
			_, exists := phrasesMap[mnemonic]
			require.False(t, exists, "Generated seed phrases should be unique")

			phrasesMap[mnemonic] = true
			t.Logf("  %d: %s", i+1, mnemonic)
		}

		require.Equal(t, count, len(phrasesMap), "Should have generated %d unique phrases", count)
	})
}

// TestWalletMonitoring tests the wallet monitoring functionality
func TestWalletMonitoring(t *testing.T) {
	// Пропускаем тест, если запущен в режиме короткого тестирования
	if testing.Short() {
		t.Skip("Пропускаем тест в режиме короткого тестирования")
	}

	// Set debug mode for this test
	os.Setenv(shared.EnvBlockchainDebugMode, "true")
	defer os.Unsetenv(shared.EnvBlockchainDebugMode) // Clean up after test

	// Log which blockchain network we're using
	if shared.IsBlockchainDebugMode() {
		t.Logf("Running test in DEBUG mode (using BSC Testnet)")
	} else {
		t.Logf("Running test in PRODUCTION mode (using BSC Mainnet)")
	}

	// Create logger
	logger := slog.Default()

	// Create a wallet service with the seed
	walletService := &WalletService{
		logger:               logger,
		seed:                 seed,
		erc20ABI:             `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"type":"function"}]`,
		smartContractAddress: GetUSDTContractAddress(),
		pendingTxs:           make(map[string]*PendingTransaction),
		pendingTxsByAddr:     make(map[common.Address]map[uint64]string),
		walletBalances:       make(map[string]*entities.WalletBalance),
		wallets:              make(map[string]bool),
		transactions:         nil, // Don't need transactions for this test
	}

	// Add the test wallet to our tracked wallets
	walletService.wallets[depositUserWalletAddress] = true

	// Log the contract address we're monitoring
	t.Logf("Monitoring contract: %s", GetUSDTContractAddress())
	t.Logf("Tracking wallet: %s", depositUserWalletAddress)

	// Create test data for an ERC20 transfer
	recipientAddress := depositUserWalletAddress
	amount := big.NewInt(1000000000000000000) // 1 token in wei

	// Create token transfer data (same as in the blockchain processor)
	transferData := CreateERC20TransferData(recipientAddress, amount)

	// Verify the data format is correct
	if len(transferData) < 4 || !bytes.Equal(transferData[:4], []byte{0xa9, 0x05, 0x9c, 0xbb}) {
		t.Fatalf("Invalid transfer data format: %x", transferData)
	}

	// Log the transfer data for debugging
	t.Logf("ERC20 transfer data: %x", transferData)
	t.Logf("First 4 bytes (method ID): %x", transferData[:4])

	// Check if IsOurWallet correctly identifies the wallet
	isOurs, err := walletService.IsOurWallet(context.Background(), depositUserWalletAddress)
	require.NoError(t, err)
	require.True(t, isOurs, "depositUserWalletAddress should be identified as our wallet")

	t.Logf("Successfully verified wallet tracking functionality")
}

// MockTransactionService is a simple mock implementation for testing
type MockTransactionService struct {
}

func (m *MockTransactionService) GetTransactionsByWallet(ctx context.Context, walletAddress string) ([]entities.Transaction, error) {
	return nil, nil
}

func (m *MockTransactionService) RecordTransaction(ctx context.Context, txHash common.Hash, walletAddress string, amount *big.Int, blockNumber int64) error {
	return nil
}

func (m *MockTransactionService) ConfirmTransaction(ctx context.Context, txHash string) error {
	return nil
}

func (m *MockTransactionService) ProcessPendingTransactions(ctx context.Context) error {
	return nil
}
