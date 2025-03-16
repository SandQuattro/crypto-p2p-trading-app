package usecases

import (
	"bytes"
	"context"
	"fmt"
	"github.com/sand/crypto-p2p-trading-app/backend/config"
	"log/slog"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	USDTContractAddress    = "0x55d398326f99059fF775485246999027B3197955" // USDT BEP-20 контракт
	subscriptionRetryDelay = 10 * time.Second                             // Delay before retrying subscription
)

// Define the ERC-20 transfer method signature
var (
	transferSig = []byte{0xa9, 0x05, 0x9c, 0xbb} // keccak256("transfer(address,uint256)")[0:4]
)

type BinanceSmartChain struct {
	logger *slog.Logger
	config *config.Config

	transactions *TransactionService
	wallets      *WalletService
}

func NewBinanceSmartChain(logger *slog.Logger, config *config.Config, transactions *TransactionService, wallets *WalletService) *BinanceSmartChain {
	return &BinanceSmartChain{
		logger:       logger,
		config:       config,
		transactions: transactions,
		wallets:      wallets,
	}
}

// SubscribeToTransactions monitors incoming transactions via Web3.
// The service will poll for new blocks and process incoming transactions.
func (bsc *BinanceSmartChain) SubscribeToTransactions(ctx context.Context, rpcURL string) {
	for {
		bsc.logger.Info("Starting blockchain monitoring...", "rpc_url", rpcURL)

		if err := bsc.pollAndProcess(ctx, rpcURL); err != nil {
			bsc.logger.Info("Blockchain monitoring error, retrying...", "delay", subscriptionRetryDelay, "error", err)
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

func (bsc *BinanceSmartChain) pollAndProcess(ctx context.Context, rpcURL string) error {
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
	bsc.logger.Info("Starting blockchain monitoring from block", "block", currentBlock)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-processTicker.C:
			if err = bsc.transactions.ProcessPendingTransactions(ctx); err != nil {
				bsc.logger.Error("Failed to process pending transactions", "error", err)
			}
		case <-pollTicker.C:
			// Get latest block number
			latestBlock, err := client.BlockNumber(ctx)
			if err != nil {
				bsc.logger.Error("Failed to get latest block number", "error", err)
				continue
			}

			// Process new blocks
			if latestBlock > lastProcessedBlock {
				bsc.logger.Info("New blocks detected", "from", lastProcessedBlock+1, "to", latestBlock)

				// Process each new block
				for blockNum := lastProcessedBlock + 1; blockNum <= latestBlock; blockNum++ {
					block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNum)))
					if err != nil {
						bsc.logger.Error("Failed to get block", "block", blockNum, "error", err)
						continue
					}

					bsc.processBlock(ctx, client, block.Header())
				}

				lastProcessedBlock = latestBlock
			}
		}
	}
}

func (bsc *BinanceSmartChain) processBlock(ctx context.Context, client *ethclient.Client, header *types.Header) {
	// Get the block
	block, err := client.BlockByHash(ctx, header.Hash())
	if err != nil {
		bsc.logger.Error("Failed to get block", "error", err)
		return
	}

	blockNumber := block.NumberU64()
	bsc.logger.Info("Processing new block", "number", blockNumber, "hash", block.Hash().Hex())

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
						bsc.logger.Error("Failed to get transaction sender", "error", err)
						continue
					}

					// Check if the recipient is one of our wallets
					isOurWallet, err := bsc.wallets.IsOurWallet(ctx, recipientAddr)
					if err != nil {
						bsc.logger.Error("Failed to check if wallet is tracked", "error", err)
						continue
					}

					if isOurWallet {
						bsc.logger.Info("USDT Transfer to our wallet detected",
							"tx_hash", tx.Hash().Hex(),
							"from", sender.Hex(),
							"to", recipientAddr,
							"amount", amount.String())

						// Record the transaction
						if err = bsc.transactions.RecordTransaction(ctx, tx.Hash(), recipientAddr, amount, int64(blockNumber)); err != nil {
							bsc.logger.Error("Failed to record transaction", "error", err)
						}

						// Check confirmations after RequiredConfirmations blocks
						go bsc.checkConfirmations(ctx, client, tx.Hash(), blockNumber)
					}
				}
			}
		}
	}
}

// checkConfirmations waits for required confirmations and then confirms the transaction
func (bsc *BinanceSmartChain) checkConfirmations(ctx context.Context, client *ethclient.Client, txHash common.Hash, blockNumber uint64) {
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
				bsc.logger.Error("Failed to get current block number", "error", err)
				continue
			}

			// Check if we have enough confirmations
			if currentBlock-blockNumber >= bsc.config.Blockchain.RequiredConfirmations {
				// Confirm the transaction
				if err = bsc.transactions.ConfirmTransaction(ctx, txHash.Hex()); err != nil {
					bsc.logger.Error("Failed to confirm transaction", "error", err, "tx_hash", txHash.Hex())
				} else {
					bsc.logger.Info("Transaction confirmed", "tx_hash", txHash.Hex(), "confirmations", currentBlock-blockNumber)
				}
				return
			}

			bsc.logger.Info("Waiting for confirmations",
				"tx_hash", txHash.Hex(),
				"current", currentBlock-blockNumber,
				"required", bsc.config.Blockchain.RequiredConfirmations)
		}
	}
}
