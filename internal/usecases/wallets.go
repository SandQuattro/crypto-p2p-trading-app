package usecases

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

const (
	USDTContractAddress    = "0x55d398326f99059fF775485246999027B3197955" // USDT BEP-20 контракт
	RequiredConfirmations  = 3                                            // Number of confirmations required before processing a transaction
	subscriptionRetryDelay = 10 * time.Second                             // Delay before retrying subscription
)

// Define the ERC-20 transfer method signature
var (
	transferSig = []byte{0xa9, 0x05, 0x9c, 0xbb} // keccak256("transfer(address,uint256)")[0:4]
)

type WalletService struct {
	masterKey *bip32.Key
	mu        sync.Mutex
	wallets   map[string]bool // Track generated wallets
	walletsMu sync.RWMutex    // Mutex for wallets map
	logger    *slog.Logger
}

func NewWalletService(seed string) (*WalletService, error) {
	seedBytes := bip39.NewSeed(seed, "")
	masterKey, err := bip32.NewMasterKey(seedBytes)
	if err != nil {
		return nil, err
	}
	return &WalletService{
		masterKey: masterKey,
		wallets:   make(map[string]bool),
		logger:    slog.Default(),
	}, nil
}

// SetLogger sets the logger for the wallet service
func (ws *WalletService) SetLogger(logger *slog.Logger) {
	ws.logger = logger
}

// IsOurWallet checks if the given address belongs to our system
func (ws *WalletService) IsOurWallet(address string) bool {
	ws.walletsMu.RLock()
	defer ws.walletsMu.RUnlock()
	return ws.wallets[address]
}

func (ws *WalletService) GenerateWallet() (string, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Use a counter or random number for the index
	// This is a simplified example - you might want to store the last used index
	index := uint32(time.Now().UnixNano() % 0x80000000)

	childKey, err := ws.masterKey.NewChildKey(index)
	if err != nil {
		return "", err
	}

	privKey, err := crypto.ToECDSA(childKey.Key)
	if err != nil {
		return "", err
	}

	address := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	// Track this wallet
	ws.walletsMu.Lock()
	ws.wallets[address] = true
	ws.walletsMu.Unlock()

	ws.logger.Info("Generated new wallet", "address", address)
	return address, nil
}

// SubscribeToTransactions monitors incoming transactions via Web3.
// The service will poll for new blocks and process incoming transactions.
func (ws *WalletService) SubscribeToTransactions(ctx context.Context, ts *TransactionService, rpcURL string) {
	for {
		ws.logger.Info("Starting blockchain monitoring...", "rpc_url", rpcURL)

		if err := ws.pollAndProcess(ctx, ts, rpcURL); err != nil {
			ws.logger.Info("Blockchain monitoring ended, retrying...", "delay", subscriptionRetryDelay, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(subscriptionRetryDelay):
				continue
			}
		}

		return // If we get here without error, we're done
	}
}

func (ws *WalletService) pollAndProcess(ctx context.Context, ts *TransactionService, rpcURL string) error {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}
	defer client.Close()

	// Process pending transactions every minute
	processTicker := time.NewTicker(1 * time.Minute)
	defer processTicker.Stop()

	// Poll for new blocks every 5 seconds
	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

	var lastProcessedBlock uint64

	// Get current block number to start from
	currentBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block number: %w", err)
	}

	lastProcessedBlock = currentBlock
	ws.logger.Info("Starting blockchain monitoring from block", "block", currentBlock)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-processTicker.C:
			if err = ts.ProcessPendingTransactions(ctx); err != nil {
				ws.logger.Error("Failed to process pending transactions", "error", err)
			}
		case <-pollTicker.C:
			// Get latest block number
			latestBlock, err := client.BlockNumber(ctx)
			if err != nil {
				ws.logger.Error("Failed to get latest block number", "error", err)
				continue
			}

			// Process new blocks
			if latestBlock > lastProcessedBlock {
				ws.logger.Info("New blocks detected", "from", lastProcessedBlock+1, "to", latestBlock)

				// Process each new block
				for blockNum := lastProcessedBlock + 1; blockNum <= latestBlock; blockNum++ {
					block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNum)))
					if err != nil {
						ws.logger.Error("Failed to get block", "block", blockNum, "error", err)
						continue
					}

					ws.processBlock(ctx, client, ts, block.Header())
				}

				lastProcessedBlock = latestBlock
			}
		}
	}
}

func (ws *WalletService) processBlock(ctx context.Context, client *ethclient.Client, ts *TransactionService, header *types.Header) {
	// Get the block
	block, err := client.BlockByHash(ctx, header.Hash())
	if err != nil {
		ws.logger.Error("Failed to get block", "error", err)
		return
	}

	blockNumber := block.NumberU64()
	ws.logger.Info("Processing new block", "number", blockNumber, "hash", block.Hash().Hex())

	for i, tx := range block.Transactions() {
		// Check if this is a transaction to the USDT contract
		if tx.To() != nil && tx.To().Hex() == USDTContractAddress {
			// Get the input data
			data := tx.Data()

			// Check if this is a transfer call (first 4 bytes match the transfer signature)
			if len(data) >= 4 && bytes.Equal(data[:4], transferSig) {
				// Parse the transfer parameters
				if len(data) >= 4+32+32 { // 4 bytes for method ID, 32 bytes for each parameter
					// Extract recipient address (second parameter, padded to 32 bytes)
					recipientBytes := data[4:36]
					recipient := common.BytesToAddress(recipientBytes[12:]) // Remove padding
					recipientAddr := recipient.Hex()

					// Extract amount (third parameter)
					amountBytes := data[36:68]
					amount := new(big.Int).SetBytes(amountBytes)

					// Get the sender address
					sender, err := client.TransactionSender(ctx, tx, block.Hash(), uint(i))
					if err != nil {
						ws.logger.Error("Failed to get transaction sender", "error", err)
						continue
					}

					// Check if the recipient is one of our wallets
					if ws.IsOurWallet(recipientAddr) {
						ws.logger.Info("USDT Transfer to our wallet detected",
							"tx_hash", tx.Hash().Hex(),
							"from", sender.Hex(),
							"to", recipientAddr,
							"amount", amount.String())

						// Record the transaction
						if err = ts.RecordTransaction(ctx, tx.Hash(), recipientAddr, amount, int64(blockNumber)); err != nil {
							ws.logger.Error("Failed to record transaction", "error", err)
						}

						// Check confirmations after RequiredConfirmations blocks
						go ws.checkConfirmations(ctx, client, ts, tx.Hash(), blockNumber)
					}
				}
			}
		}
	}
}

// checkConfirmations waits for required confirmations and then confirms the transaction
func (ws *WalletService) checkConfirmations(ctx context.Context, client *ethclient.Client, ts *TransactionService, txHash common.Hash, blockNumber uint64) {
	// Create a ticker to check every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get current block number
			currentBlock, err := client.BlockNumber(ctx)
			if err != nil {
				ws.logger.Error("Failed to get current block number", "error", err)
				continue
			}

			// Check if we have enough confirmations
			if currentBlock-blockNumber >= RequiredConfirmations {
				// Confirm the transaction
				if err := ts.ConfirmTransaction(ctx, txHash.Hex()); err != nil {
					ws.logger.Error("Failed to confirm transaction", "error", err, "tx_hash", txHash.Hex())
				} else {
					ws.logger.Info("Transaction confirmed", "tx_hash", txHash.Hex(), "confirmations", currentBlock-blockNumber)
				}
				return
			}

			ws.logger.Info("Waiting for confirmations",
				"tx_hash", txHash.Hex(),
				"current", currentBlock-blockNumber,
				"required", RequiredConfirmations)
		}
	}
}
