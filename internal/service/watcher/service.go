// Package watcher implements blockchain address monitoring via direct RPC polling.
// It detects incoming payments by scanning for transactions to watched addresses
// and triggers the existing payment processing flow.
package watcher

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	kms "github.com/cryptolink/cryptolink/internal/kms/wallet"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/provider/bitcoin"
	"github.com/cryptolink/cryptolink/internal/provider/rpc"
	"github.com/cryptolink/cryptolink/internal/provider/trongrid"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/cryptolink/cryptolink/internal/service/wallet"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// DetectedTransfer represents a blockchain transaction detected by the watcher.
type DetectedTransfer struct {
	// PendingTx is the internal transaction record awaiting payment.
	PendingTx *transaction.Transaction

	// Wallet is the recipient wallet (nil for xpub/collector flows).
	Wallet *wallet.Wallet

	// On-chain data
	TxHash        string
	SenderAddress string
	Amount        money.Money
	Currency      money.CryptoCurrency
	NetworkID     string
}

// OnTransferDetected is a callback invoked when the watcher detects an incoming payment.
// The caller (scheduler) bridges this to processing.ProcessInboundTransaction.
type OnTransferDetected func(ctx context.Context, d DetectedTransfer) error

// Config controls watcher behavior.
type Config struct {
	// BlockScanDepth is how many recent blocks to scan for incoming transactions.
	// Default: 50 (~10 minutes on ETH, ~2 minutes on MATIC/BSC)
	BlockScanDepth int64 `yaml:"block_scan_depth" env:"WATCHER_BLOCK_SCAN_DEPTH" env-default:"500"`

	// MaxBlocksPerCycle caps how many blocks are scanned in a single poll cycle.
	// Prevents long-running scans from blocking the scheduler. The watcher catches
	// up progressively over multiple cycles.
	MaxBlocksPerCycle int64 `yaml:"max_blocks_per_cycle" env:"WATCHER_MAX_BLOCKS_PER_CYCLE" env-default:"100"`

	// MaxConcurrency limits parallel RPC calls per poll cycle.
	MaxConcurrency int `yaml:"max_concurrency" env:"WATCHER_MAX_CONCURRENCY" env-default:"4"`

	// Enabled controls whether the watcher runs.
	Enabled bool `yaml:"enabled" env:"WATCHER_ENABLED" env-default:"true"`
}

// Service watches blockchain addresses for incoming payments.
type Service struct {
	config       Config
	rpc          *rpc.Provider
	bitcoin      *bitcoin.Provider
	tron         *trongrid.Provider
	transactions *transaction.Service
	wallets      *wallet.Service
	logger       *zerolog.Logger

	// lastScannedBlock tracks the last block number scanned per chain+network
	// to avoid rescanning the same blocks.
	lastScannedBlock sync.Map // key: "chain:isTest" -> value: int64

	// lastBTCBalance tracks the last known balance (satoshis) per BTC address
	// to detect incoming payments by balance change.
	lastBTCBalance sync.Map // key: "address:isTest" -> value: int64

}

// New creates a new watcher service.
func New(
	config Config,
	rpcProvider *rpc.Provider,
	bitcoinProvider *bitcoin.Provider,
	tronProvider *trongrid.Provider,
	transactions *transaction.Service,
	wallets *wallet.Service,
	logger *zerolog.Logger,
) *Service {
	log := logger.With().Str("channel", "address_watcher").Logger()

	if config.BlockScanDepth <= 0 {
		config.BlockScanDepth = 50
	}
	if config.MaxBlocksPerCycle <= 0 {
		config.MaxBlocksPerCycle = 100
	}
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 4
	}

	return &Service{
		config:       config,
		rpc:          rpcProvider,
		bitcoin:      bitcoinProvider,
		tron:         tronProvider,
		transactions: transactions,
		wallets:      wallets,
		logger:       &log,
	}
}

// ERC-20 Transfer event topic: Transfer(address,address,uint256)
var erc20TransferTopic = common.HexToHash("0xddf252ad1be2c4e6b9b4170f7cfc9ef7a5c891b13519e1cf2e4a0329118b624e")

// PollPendingTransactions is the main entry point called by the scheduler.
// It finds all pending incoming transactions (no tx hash yet) and checks
// their recipient addresses for incoming blockchain transactions.
// The onDetected callback is called for each detected payment.
func (s *Service) PollPendingTransactions(ctx context.Context, onDetected OnTransferDetected) error {
	if !s.config.Enabled {
		return nil
	}

	// Find all pending incoming transactions that haven't been detected yet
	filter := transaction.Filter{
		Types:       []transaction.Type{transaction.TypeIncoming},
		Statuses:    []transaction.Status{transaction.StatusPending},
		HashIsEmpty: true,
	}

	txs, err := s.transactions.ListByFilter(ctx, filter, 200)
	if err != nil {
		return errors.Wrap(err, "unable to list pending transactions")
	}

	if len(txs) == 0 {
		return nil
	}

	s.logger.Info().Int("pending_count", len(txs)).Msg("polling pending transactions for incoming payments")

	// Group transactions by blockchain+isTest for efficient batch RPC calls
	type chainKey struct {
		blockchain money.Blockchain
		isTest     bool
	}

	grouped := make(map[chainKey][]*transaction.Transaction)
	for _, tx := range txs {
		key := chainKey{blockchain: tx.Currency.Blockchain, isTest: tx.IsTest}
		grouped[key] = append(grouped[key], tx)
	}

	var (
		group     errgroup.Group
		detected  int64
		failedTXs []int64
		mu        sync.Mutex
	)

	group.SetLimit(s.config.MaxConcurrency)

	// Wrap onDetected with tx hash deduplication to prevent the same
	// on-chain transaction from being matched to multiple pending payments.
	// This can happen when multiple payments share the same collector contract address.
	dedupOnDetected := func(ctx context.Context, d DetectedTransfer) error {
		existing, err := s.transactions.GetByHash(ctx, d.NetworkID, d.TxHash)
		if err == nil && existing != nil {
			s.logger.Warn().
				Str("tx_hash", d.TxHash).
				Str("network_id", d.NetworkID).
				Int64("pending_tx_id", d.PendingTx.ID).
				Int64("existing_tx_id", existing.ID).
				Msg("skipping duplicate: tx hash already assigned to another transaction")
			return nil
		}
		return onDetected(ctx, d)
	}

	for key, chainTxs := range grouped {
		key := key
		chainTxs := chainTxs

		group.Go(func() error {
			count, failed := s.pollChainTransactions(ctx, key.blockchain, key.isTest, chainTxs, dedupOnDetected)
			atomic.AddInt64(&detected, count)
			if len(failed) > 0 {
				mu.Lock()
				failedTXs = append(failedTXs, failed...)
				mu.Unlock()
			}
			return nil
		})
	}

	_ = group.Wait()

	if detected > 0 || len(failedTXs) > 0 {
		s.logger.Info().
			Int64("detected_count", detected).
			Ints64("failed_tx_ids", failedTXs).
			Msg("address watcher poll completed")
	}

	return nil
}

// pollChainTransactions checks all pending transactions for a specific blockchain.
func (s *Service) pollChainTransactions(
	ctx context.Context,
	bc money.Blockchain,
	isTest bool,
	txs []*transaction.Transaction,
	onDetected OnTransferDetected,
) (int64, []int64) {
	switch kms.Blockchain(bc) {
	case kms.ETH, kms.MATIC, kms.BSC, kms.ARBITRUM, kms.AVAX:
		return s.pollEVMTransactions(ctx, bc, isTest, txs, onDetected)
	case kms.BTC:
		return s.pollBTCTransactions(ctx, isTest, txs, onDetected)
	case kms.TRON:
		return s.pollTRONTransactions(ctx, isTest, txs, onDetected)
	default:
		s.logger.Warn().Str("blockchain", bc.String()).Msg("unsupported blockchain for address watching")
		return 0, nil
	}
}

// pendingInfo holds data for a watched address.
type pendingInfo struct {
	tx       *transaction.Transaction
	walletID *int64
}

// pollEVMTransactions scans recent blocks on an EVM chain for transactions
// to the watched addresses.
func (s *Service) pollEVMTransactions(
	ctx context.Context,
	bc money.Blockchain,
	isTest bool,
	txs []*transaction.Transaction,
	onDetected OnTransferDetected,
) (int64, []int64) {
	client, err := s.getEVMClient(ctx, bc, isTest)
	if err != nil {
		s.logger.Error().Err(err).
			Str("blockchain", bc.String()).
			Bool("is_test", isTest).
			Msg("unable to connect to RPC for address watching")

		ids := make([]int64, len(txs))
		for i, tx := range txs {
			ids[i] = tx.ID
		}
		return 0, ids
	}
	defer client.Close()

	// Get current block number
	currentBlock, err := client.BlockNumber(ctx)
	if err != nil {
		s.logger.Error().Err(err).Str("blockchain", bc.String()).Msg("unable to get current block number")
		return 0, nil
	}

	// Determine scan range
	fromBlock := int64(currentBlock) - s.config.BlockScanDepth
	if fromBlock < 0 {
		fromBlock = 0
	}

	// Check if we've scanned further — use last scanned block if available
	cacheKey := bc.String() + ":" + boolStr(isTest)
	if last, ok := s.lastScannedBlock.Load(cacheKey); ok {
		if lastBlock, ok := last.(int64); ok && lastBlock > fromBlock {
			fromBlock = lastBlock + 1
		}
	}

	if fromBlock > int64(currentBlock) {
		return 0, nil
	}

	// Cap the scan range to avoid long-running cycles
	toBlock := int64(currentBlock)
	if toBlock-fromBlock > s.config.MaxBlocksPerCycle {
		toBlock = fromBlock + s.config.MaxBlocksPerCycle
		s.logger.Info().
			Str("blockchain", bc.String()).
			Int64("from_block", fromBlock).
			Int64("to_block", toBlock).
			Int64("current_block", int64(currentBlock)).
			Int64("blocks_behind", int64(currentBlock)-toBlock).
			Msg("capping block scan range, will catch up in subsequent cycles")
	}

	// Build address lookup maps
	nativeAddresses := make(map[common.Address]pendingInfo)
	tokenAddresses := make(map[common.Address]map[common.Address]pendingInfo) // contract -> recipient -> info

	for _, tx := range txs {
		addr := s.getRecipientAddress(ctx, tx)
		if addr == "" {
			s.logger.Warn().
				Int64("tx_id", tx.ID).
				Int64("entity_id", tx.EntityID).
				Str("blockchain", bc.String()).
				Str("currency", tx.Currency.Ticker).
				Msg("skipping transaction: unable to resolve recipient address")
			continue
		}

		ethAddr := common.HexToAddress(addr)

		if tx.Currency.Type == money.Coin {
			nativeAddresses[ethAddr] = pendingInfo{tx: tx, walletID: tx.RecipientWalletID}
		} else if tx.Currency.TokenContractAddress != "" {
			contractAddr := common.HexToAddress(tx.Currency.ChooseContractAddress(tx.IsTest))
			if tokenAddresses[contractAddr] == nil {
				tokenAddresses[contractAddr] = make(map[common.Address]pendingInfo)
			}
			tokenAddresses[contractAddr][ethAddr] = pendingInfo{tx: tx, walletID: tx.RecipientWalletID}
		}
	}

	var detected int64
	var failedIDs []int64

	// 1. Scan for native coin transfers by checking block transactions
	if len(nativeAddresses) > 0 {
		d, f := s.scanNativeTransfers(ctx, client, bc, isTest, nativeAddresses, fromBlock, toBlock, onDetected)
		detected += d
		failedIDs = append(failedIDs, f...)
	}

	// 2. Scan for ERC-20 token transfers using Transfer event logs
	if len(tokenAddresses) > 0 {
		d, f := s.scanTokenTransfers(ctx, client, isTest, tokenAddresses, fromBlock, toBlock, onDetected)
		detected += d
		failedIDs = append(failedIDs, f...)
	}

	// Update last scanned block (use capped toBlock, not currentBlock, for progressive catch-up)
	s.lastScannedBlock.Store(cacheKey, toBlock)

	return detected, failedIDs
}

// scanNativeTransfers scans blocks for native coin (ETH/MATIC/BNB/etc.) transfers.
func (s *Service) scanNativeTransfers(
	ctx context.Context,
	client *ethclient.Client,
	bc money.Blockchain,
	isTest bool,
	addresses map[common.Address]pendingInfo,
	fromBlock, toBlock int64,
	onDetected OnTransferDetected,
) (int64, []int64) {
	var detected int64
	var failedIDs []int64

	for bn := fromBlock; bn <= toBlock; bn++ {
		block, err := client.BlockByNumber(ctx, big.NewInt(bn))
		if err != nil {
			s.logger.Debug().Err(err).Int64("block", bn).Msg("unable to get block, skipping")
			continue
		}

		for _, blockTx := range block.Transactions() {
			if blockTx.To() == nil {
				continue // contract creation
			}

			recipient := *blockTx.To()
			info, ok := addresses[recipient]
			if !ok {
				continue
			}

			// Skip zero-value transactions
			if blockTx.Value().Sign() == 0 {
				continue
			}

			d, err := s.buildNativeDetection(ctx, client, bc, isTest, blockTx, info)
			if err != nil {
				s.logger.Error().Err(err).
					Int64("tx_id", info.tx.ID).
					Str("hash", blockTx.Hash().Hex()).
					Msg("failed to build native transfer detection")
				failedIDs = append(failedIDs, info.tx.ID)
				continue
			}

			if err := onDetected(ctx, d); err != nil {
				s.logger.Error().Err(err).
					Int64("tx_id", info.tx.ID).
					Str("hash", blockTx.Hash().Hex()).
					Msg("failed to process detected native transfer")
				failedIDs = append(failedIDs, info.tx.ID)
			} else {
				detected++
				delete(addresses, recipient)
			}
		}

		if len(addresses) == 0 {
			break
		}
	}

	return detected, failedIDs
}

// scanTokenTransfers uses eth_getLogs to find ERC-20 Transfer events to watched addresses.
func (s *Service) scanTokenTransfers(
	ctx context.Context,
	client *ethclient.Client,
	isTest bool,
	tokenAddresses map[common.Address]map[common.Address]pendingInfo,
	fromBlock, toBlock int64,
	onDetected OnTransferDetected,
) (int64, []int64) {
	var detected int64
	var failedIDs []int64

	for contractAddr, recipients := range tokenAddresses {
		recipientTopics := make([]common.Hash, 0, len(recipients))
		for addr := range recipients {
			recipientTopics = append(recipientTopics, common.BytesToHash(addr.Bytes()))
		}

		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(fromBlock),
			ToBlock:   big.NewInt(toBlock),
			Addresses: []common.Address{contractAddr},
			Topics: [][]common.Hash{
				{erc20TransferTopic}, // event signature
				{},                   // from: any
				recipientTopics,      // to: one of our watched addresses
			},
		}

		logs, err := client.FilterLogs(ctx, query)
		if err != nil {
			s.logger.Error().Err(err).
				Str("contract", contractAddr.Hex()).
				Msg("unable to filter ERC-20 Transfer logs")
			for _, info := range recipients {
				failedIDs = append(failedIDs, info.tx.ID)
			}
			continue
		}

		for _, logEntry := range logs {
			if len(logEntry.Topics) < 3 {
				continue
			}

			recipientAddr := common.HexToAddress(logEntry.Topics[2].Hex())
			info, ok := recipients[recipientAddr]
			if !ok {
				continue
			}

			amount := new(big.Int).SetBytes(logEntry.Data)
			senderAddr := common.HexToAddress(logEntry.Topics[1].Hex())

			cryptoAmount, err := money.NewFromBigInt(
				money.Crypto,
				info.tx.Currency.Ticker,
				amount,
				info.tx.Currency.Decimals,
			)
			if err != nil {
				s.logger.Error().Err(err).
					Str("ticker", info.tx.Currency.Ticker).
					Msg("unable to parse token amount")
				failedIDs = append(failedIDs, info.tx.ID)
				continue
			}

			var wt *wallet.Wallet
			if info.walletID != nil {
				wt, _ = s.wallets.GetByID(ctx, *info.walletID)
			}

			d := DetectedTransfer{
				PendingTx:     info.tx,
				Wallet:        wt,
				TxHash:        logEntry.TxHash.Hex(),
				SenderAddress: senderAddr.Hex(),
				Amount:        cryptoAmount,
				Currency:      info.tx.Currency,
				NetworkID:     info.tx.Currency.ChooseNetwork(isTest),
			}

			if err := onDetected(ctx, d); err != nil {
				s.logger.Error().Err(err).
					Int64("tx_id", info.tx.ID).
					Str("hash", logEntry.TxHash.Hex()).
					Msg("failed to process detected token transfer")
				failedIDs = append(failedIDs, info.tx.ID)
			} else {
				detected++
				delete(recipients, recipientAddr)
			}
		}
	}

	return detected, failedIDs
}

// buildNativeDetection constructs a DetectedTransfer for a native coin block transaction.
func (s *Service) buildNativeDetection(
	ctx context.Context,
	client *ethclient.Client,
	bc money.Blockchain,
	isTest bool,
	blockTx *types.Transaction,
	info pendingInfo,
) (DetectedTransfer, error) {
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return DetectedTransfer{}, errors.Wrap(err, "unable to get chain ID")
	}

	sender, err := types.Sender(types.LatestSignerForChainID(chainID), blockTx)
	if err != nil {
		return DetectedTransfer{}, errors.Wrap(err, "unable to recover sender")
	}

	cryptoAmount, err := money.NewFromBigInt(
		money.Crypto,
		info.tx.Currency.Ticker,
		blockTx.Value(),
		info.tx.Currency.Decimals,
	)
	if err != nil {
		return DetectedTransfer{}, errors.Wrap(err, "unable to parse transaction value")
	}

	var wt *wallet.Wallet
	if info.walletID != nil {
		wt, _ = s.wallets.GetByID(ctx, *info.walletID)
	}

	return DetectedTransfer{
		PendingTx:     info.tx,
		Wallet:        wt,
		TxHash:        blockTx.Hash().Hex(),
		SenderAddress: sender.Hex(),
		Amount:        cryptoAmount,
		Currency:      info.tx.Currency,
		NetworkID:     info.tx.Currency.ChooseNetwork(isTest),
	}, nil
}

// getRecipientAddress resolves the recipient address for a pending transaction.
func (s *Service) getRecipientAddress(ctx context.Context, tx *transaction.Transaction) string {
	if tx.RecipientAddress != "" {
		return tx.RecipientAddress
	}

	if tx.RecipientWalletID != nil {
		wt, err := s.wallets.GetByID(ctx, *tx.RecipientWalletID)
		if err != nil {
			s.logger.Error().Err(err).
				Int64("wallet_id", *tx.RecipientWalletID).
				Msg("unable to resolve wallet address for watching")
			return ""
		}
		return wt.Address
	}

	return ""
}

// getEVMClient returns the appropriate ethclient for the given blockchain.
func (s *Service) getEVMClient(ctx context.Context, bc money.Blockchain, isTest bool) (*ethclient.Client, error) {
	switch kms.Blockchain(bc) {
	case kms.ETH:
		return s.rpc.EthereumRPC(ctx, isTest)
	case kms.MATIC:
		return s.rpc.MaticRPC(ctx, isTest)
	case kms.BSC:
		return s.rpc.BinanceSmartChainRPC(ctx, isTest)
	case kms.ARBITRUM:
		return s.rpc.ArbitrumRPC(ctx, isTest)
	case kms.AVAX:
		return s.rpc.AvalancheRPC(ctx, isTest)
	default:
		return nil, errors.Errorf("unsupported EVM blockchain: %s", bc)
	}
}

// pollBTCTransactions checks BTC addresses for incoming payments
// by comparing current balance with last known balance via Blockstream/mempool.space API.
func (s *Service) pollBTCTransactions(
	ctx context.Context,
	isTest bool,
	txs []*transaction.Transaction,
	onDetected OnTransferDetected,
) (int64, []int64) {
	var detected int64
	var failedIDs []int64

	for _, tx := range txs {
		addr := s.getRecipientAddress(ctx, tx)
		if addr == "" {
			s.logger.Warn().
				Int64("tx_id", tx.ID).
				Int64("entity_id", tx.EntityID).
				Str("currency", tx.Currency.Ticker).
				Msg("skipping BTC transaction: unable to resolve recipient address")
			continue
		}

		info, err := s.bitcoin.GetAddressInfo(ctx, addr, isTest)
		if err != nil {
			s.logger.Error().Err(err).
				Str("address", addr).
				Int64("tx_id", tx.ID).
				Msg("unable to get BTC address info")
			failedIDs = append(failedIDs, tx.ID)
			continue
		}

		currentBalance := info.Balance
		cacheKey := addr + ":" + boolStr(isTest)

		// Check if balance increased since last poll
		var lastBalance int64
		if last, ok := s.lastBTCBalance.Load(cacheKey); ok {
			lastBalance = last.(int64)
		}

		// Store current balance for next poll
		s.lastBTCBalance.Store(cacheKey, currentBalance)

		if currentBalance <= lastBalance {
			continue
		}

		// Balance increased — an incoming payment was detected
		receivedSatoshis := currentBalance - lastBalance

		// Convert satoshis to BTC amount (8 decimals)
		cryptoAmount, err := money.NewFromBigInt(
			money.Crypto,
			tx.Currency.Ticker,
			big.NewInt(receivedSatoshis),
			tx.Currency.Decimals,
		)
		if err != nil {
			s.logger.Error().Err(err).
				Int64("tx_id", tx.ID).
				Int64("satoshis", receivedSatoshis).
				Msg("unable to parse BTC amount")
			failedIDs = append(failedIDs, tx.ID)
			continue
		}

		var wt *wallet.Wallet
		if tx.RecipientWalletID != nil {
			wt, _ = s.wallets.GetByID(ctx, *tx.RecipientWalletID)
		}

		// Query recent transactions to find the actual tx hash and sender
		txHash := ""
		senderAddress := ""
		recentTxs, txErr := s.bitcoin.GetRecentTransactions(ctx, addr, isTest)
		if txErr == nil && len(recentTxs) > 0 {
			// Find the most recent transaction that sends TO our address
			for _, rtx := range recentTxs {
				for _, out := range rtx.Outputs {
					if out.Address == addr && out.Value > 0 {
						txHash = rtx.TxID
						if len(rtx.Inputs) > 0 && rtx.Inputs[0].Address != "" {
							senderAddress = rtx.Inputs[0].Address
						}
						break
					}
				}
				if txHash != "" {
					break
				}
			}
		}
		if senderAddress == "" {
			senderAddress = "unknown" // Fallback — some BTC inputs may not have parseable addresses
		}
		if txHash == "" {
			txHash = fmt.Sprintf("balance-detect-%s-%d", addr[:8], time.Now().Unix())
		}

		d := DetectedTransfer{
			PendingTx:     tx,
			Wallet:        wt,
			TxHash:        txHash,
			SenderAddress: senderAddress,
			Amount:        cryptoAmount,
			Currency:      tx.Currency,
			NetworkID:     tx.Currency.ChooseNetwork(isTest),
		}

		if err := onDetected(ctx, d); err != nil {
			s.logger.Error().Err(err).
				Int64("tx_id", tx.ID).
				Str("address", addr).
				Msg("failed to process detected BTC payment")
			failedIDs = append(failedIDs, tx.ID)
		} else {
			detected++
			s.logger.Info().
				Int64("tx_id", tx.ID).
				Str("address", addr).
				Int64("satoshis", receivedSatoshis).
				Msg("BTC incoming payment detected")
		}
	}

	return detected, failedIDs
}

// pollTRONTransactions checks TRON addresses for incoming payments
// using the TronGrid API to query recent transactions sent to each address.
func (s *Service) pollTRONTransactions(
	ctx context.Context,
	isTest bool,
	txs []*transaction.Transaction,
	onDetected OnTransferDetected,
) (int64, []int64) {
	if s.tron == nil {
		s.logger.Warn().Msg("TRON provider not configured, skipping TRON polling")
		return 0, nil
	}

	var detected int64
	var failedIDs []int64

	for _, tx := range txs {
		addr := s.getRecipientAddress(ctx, tx)
		if addr == "" {
			s.logger.Warn().
				Int64("tx_id", tx.ID).
				Int64("entity_id", tx.EntityID).
				Str("currency", tx.Currency.Ticker).
				Msg("skipping TRON transaction: unable to resolve recipient address")
			continue
		}

		// Only match on-chain transactions that occurred AFTER the pending
		// transaction was created, to avoid picking up old/test transfers.
		txCreatedAtMs := tx.CreatedAt.UnixMilli()

		if tx.Currency.Type == money.Coin {
			// TRX native transfer — query recent native transactions
			recentTxs, err := s.tron.GetAccountTransactions(ctx, addr, isTest, 10)
			if err != nil {
				s.logger.Error().Err(err).Str("address", addr).Int64("tx_id", tx.ID).
					Msg("unable to get TRON transactions")
				failedIDs = append(failedIDs, tx.ID)
				continue
			}

			for _, rtx := range recentTxs {
				if !rtx.Success || rtx.To != addr || rtx.Amount <= 0 {
					continue
				}
				if rtx.Type != "TransferContract" {
					continue
				}
				// Skip transactions that happened before this payment was created
				if rtx.Timestamp > 0 && rtx.Timestamp < txCreatedAtMs {
					continue
				}

				cryptoAmount, err := money.NewFromBigInt(
					money.Crypto, tx.Currency.Ticker,
					big.NewInt(rtx.Amount), tx.Currency.Decimals,
				)
				if err != nil {
					continue
				}

				var wt *wallet.Wallet
				if tx.RecipientWalletID != nil {
					wt, _ = s.wallets.GetByID(ctx, *tx.RecipientWalletID)
				}

				d := DetectedTransfer{
					PendingTx:     tx,
					Wallet:        wt,
					TxHash:        rtx.TxID,
					SenderAddress: rtx.From,
					Amount:        cryptoAmount,
					Currency:      tx.Currency,
					NetworkID:     tx.Currency.ChooseNetwork(isTest),
				}

				if err := onDetected(ctx, d); err != nil {
					s.logger.Error().Err(err).Int64("tx_id", tx.ID).Str("hash", rtx.TxID).
						Msg("failed to process detected TRON payment")
					failedIDs = append(failedIDs, tx.ID)
				} else {
					detected++
					s.logger.Info().Int64("tx_id", tx.ID).Str("hash", rtx.TxID).
						Str("address", addr).Int64("sun", rtx.Amount).
						Msg("TRON incoming payment detected")
				}
				break // One match per pending tx
			}
		} else {
			// TRC20 token transfer — query TRC20 transaction history
			recentTxs, err := s.tron.GetTRC20Transactions(ctx, addr, isTest, 10)
			if err != nil {
				s.logger.Error().Err(err).Str("address", addr).Int64("tx_id", tx.ID).
					Msg("unable to get TRC20 transactions")
				failedIDs = append(failedIDs, tx.ID)
				continue
			}

			tokenContract := tx.Currency.ChooseContractAddress(isTest)
			for _, rtx := range recentTxs {
				if rtx.To != addr {
					continue
				}
				if !strings.EqualFold(rtx.TokenAddress, tokenContract) {
					continue
				}
				// Skip transactions that happened before this payment was created
				if rtx.Timestamp > 0 && rtx.Timestamp < txCreatedAtMs {
					continue
				}

				amount := new(big.Int)
				amount.SetString(rtx.TokenAmount, 10)
				if amount.Sign() <= 0 {
					continue
				}

				cryptoAmount, err := money.NewFromBigInt(
					money.Crypto, tx.Currency.Ticker,
					amount, tx.Currency.Decimals,
				)
				if err != nil {
					continue
				}

				var wt *wallet.Wallet
				if tx.RecipientWalletID != nil {
					wt, _ = s.wallets.GetByID(ctx, *tx.RecipientWalletID)
				}

				d := DetectedTransfer{
					PendingTx:     tx,
					Wallet:        wt,
					TxHash:        rtx.TxID,
					SenderAddress: rtx.From,
					Amount:        cryptoAmount,
					Currency:      tx.Currency,
					NetworkID:     tx.Currency.ChooseNetwork(isTest),
				}

				if err := onDetected(ctx, d); err != nil {
					s.logger.Error().Err(err).Int64("tx_id", tx.ID).Str("hash", rtx.TxID).
						Msg("failed to process detected TRC20 payment")
					failedIDs = append(failedIDs, tx.ID)
				} else {
					detected++
					s.logger.Info().Int64("tx_id", tx.ID).Str("hash", rtx.TxID).
						Str("address", addr).Str("amount", rtx.TokenAmount).
						Msg("TRC20 incoming payment detected")
				}
				break
			}
		}
	}

	return detected, failedIDs
}

func boolStr(b bool) string {
	if b {
		return "test"
	}
	return "main"
}
