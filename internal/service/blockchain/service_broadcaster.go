package blockchain

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	kms "github.com/cryptolink/cryptolink/internal/kms/wallet"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Broadcaster interface {
	BroadcastTransaction(ctx context.Context, blockchain money.Blockchain, hex string, isTest bool) (string, error)
	GetTransactionReceipt(ctx context.Context, blockchain money.Blockchain, transactionID string, isTest bool) (*TransactionReceipt, error)
}

func (s *Service) BroadcastTransaction(ctx context.Context, blockchain money.Blockchain, rawTX string, isTest bool) (string, error) {
	switch kms.Blockchain(blockchain) {
	case kms.ETH:
		rpcClient, err := s.providers.RPC.EthereumRPC(ctx, isTest)
		if err != nil {
			return "", errors.Wrap(err, "unable to get Ethereum RPC")
		}
		defer rpcClient.Close()
		return s.broadcastRawTransaction(ctx, rpcClient, rawTX)

	case kms.MATIC:
		rpcClient, err := s.providers.RPC.MaticRPC(ctx, isTest)
		if err != nil {
			return "", errors.Wrap(err, "unable to get Polygon RPC")
		}
		defer rpcClient.Close()
		return s.broadcastRawTransaction(ctx, rpcClient, rawTX)

	case kms.BSC:
		rpcClient, err := s.providers.RPC.BinanceSmartChainRPC(ctx, isTest)
		if err != nil {
			return "", errors.Wrap(err, "unable to get BSC RPC")
		}
		defer rpcClient.Close()
		return s.broadcastRawTransaction(ctx, rpcClient, rawTX)

	case kms.TRON:
		return s.providers.Trongrid.BroadcastTransaction(ctx, []byte(rawTX), isTest)

	case kms.ARBITRUM:
		rpcClient, err := s.providers.RPC.ArbitrumRPC(ctx, isTest)
		if err != nil {
			return "", errors.Wrap(err, "unable to get Arbitrum RPC")
		}
		defer rpcClient.Close()
		return s.broadcastRawTransaction(ctx, rpcClient, rawTX)

	case kms.AVAX:
		rpcClient, err := s.providers.RPC.AvalancheRPC(ctx, isTest)
		if err != nil {
			return "", errors.Wrap(err, "unable to get Avalanche RPC")
		}
		defer rpcClient.Close()
		return s.broadcastRawTransaction(ctx, rpcClient, rawTX)

	case kms.BTC:
		txID, err := s.providers.Bitcoin.BroadcastTransaction(ctx, rawTX, isTest)
		if err != nil {
			return "", errors.Wrap(err, "unable to broadcast BTC transaction")
		}
		return txID, nil

	default:
		return "", fmt.Errorf("broadcast for %q is not implemented yet", blockchain)
	}
}

type TransactionReceipt struct {
	Blockchain    money.Blockchain
	IsTest        bool
	Sender        string
	Recipient     string
	Hash          string
	Nonce         uint64
	NetworkFee    money.Money
	Success       bool
	Confirmations int64
	IsConfirmed   bool
}

func (s *Service) GetTransactionReceipt(
	ctx context.Context,
	blockchain money.Blockchain,
	transactionID string,
	isTest bool,
) (*TransactionReceipt, error) {
	receipt, err := s.getTransactionReceipt(ctx, blockchain, transactionID, isTest)
	if err != nil {
		s.logger.Error().Err(err).Msg("unable to get transaction receipt")
	}

	return receipt, err
}

func (s *Service) getTransactionReceipt(
	ctx context.Context,
	blockchain money.Blockchain,
	transactionID string,
	isTest bool,
) (*TransactionReceipt, error) {
	const (
		btcConfirmations      = 2
		ethConfirmations      = 12
		maticConfirmations    = 30
		bscConfirmations      = 15
		arbitrumConfirmations = 20
		avaxConfirmations     = 20
	)

	nativeCoin, err := s.GetNativeCoin(blockchain)
	if err != nil {
		return nil, errors.Wrapf(err, "native coin for %q is not found", blockchain)
	}

	switch kms.Blockchain(blockchain) {
	case kms.ETH:
		rpc, err := s.providers.RPC.EthereumRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}
		defer rpc.Close()

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, ethConfirmations, isTest)
	case kms.MATIC:
		rpc, err := s.providers.RPC.MaticRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}
		defer rpc.Close()

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, maticConfirmations, isTest)
	case kms.BSC:
		rpc, err := s.providers.RPC.BinanceSmartChainRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}
		defer rpc.Close()

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, bscConfirmations, isTest)
	case kms.ARBITRUM:
		rpc, err := s.providers.RPC.ArbitrumRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}
		defer rpc.Close()

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, arbitrumConfirmations, isTest)
	case kms.AVAX:
		rpc, err := s.providers.RPC.AvalancheRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}
		defer rpc.Close()

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, avaxConfirmations, isTest)
	case kms.TRON:
		receipt, err := s.providers.Trongrid.GetTransactionReceipt(ctx, transactionID, isTest)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get tron transaction receipt")
		}

		networkFee, err := nativeCoin.MakeAmount(strconv.Itoa(int(receipt.Fee)))
		if err != nil {
			return nil, errors.Wrap(err, "unable to calculate network fee")
		}

		return &TransactionReceipt{
			Blockchain:    blockchain,
			IsTest:        isTest,
			Sender:        receipt.Sender,
			Recipient:     receipt.Recipient,
			Hash:          transactionID,
			NetworkFee:    networkFee,
			Success:       receipt.Success,
			Confirmations: receipt.Confirmations,
			IsConfirmed:   receipt.IsConfirmed,
		}, nil
	case kms.BTC:
		return s.getBitcoinReceipt(ctx, nativeCoin, transactionID, btcConfirmations, isTest)
	}

	return nil, kms.ErrUnknownBlockchain
}

func (s *Service) getEthReceipt(
	ctx context.Context,
	rpc *ethclient.Client,
	nativeCoin money.CryptoCurrency,
	txID string,
	requiredConfirmations int64,
	isTest bool,
) (*TransactionReceipt, error) {
	hash := common.HexToHash(txID)

	var (
		tx          *types.Transaction
		receipt     *types.Receipt
		latestBlock int64
		mu          sync.Mutex
		group       errgroup.Group
	)

	group.Go(func() error {
		txByHash, _, err := rpc.TransactionByHash(ctx, hash)
		if err != nil {
			return err
		}

		mu.Lock()
		tx = txByHash
		mu.Unlock()

		return nil
	})

	group.Go(func() error {
		r, err := rpc.TransactionReceipt(ctx, hash)
		if err != nil {
			return err
		}

		mu.Lock()
		receipt = r
		mu.Unlock()

		return nil
	})

	group.Go(func() error {
		num, err := rpc.BlockNumber(ctx)
		if err != nil {
			return err
		}

		mu.Lock()
		latestBlock = int64(num)
		mu.Unlock()

		return nil
	})

	if err := group.Wait(); err != nil {
		return nil, err
	}

	gasPrice, err := nativeCoin.MakeAmountFromBigInt(receipt.EffectiveGasPrice)
	if err != nil {
		return nil, errors.Wrap(err, "unable to construct network fee")
	}

	networkFee, err := gasPrice.MultiplyInt64(int64(receipt.GasUsed))
	if err != nil {
		return nil, errors.Wrap(err, "unable to calculate network fee")
	}

	sender, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get sender")
	}

	confirmations := latestBlock - receipt.BlockNumber.Int64()

	return &TransactionReceipt{
		Blockchain:    nativeCoin.Blockchain,
		IsTest:        isTest,
		Sender:        sender.String(),
		Recipient:     tx.To().String(),
		Hash:          txID,
		Nonce:         tx.Nonce(),
		NetworkFee:    networkFee,
		Success:       receipt.Status == 1,
		Confirmations: confirmations,
		IsConfirmed:   confirmations >= requiredConfirmations,
	}, nil
}

// getBitcoinReceipt retrieves transaction receipt from the Bitcoin blockchain via Blockstream/mempool.space
func (s *Service) getBitcoinReceipt(
	ctx context.Context,
	nativeCoin money.CryptoCurrency,
	txID string,
	requiredConfirmations int64,
	isTest bool,
) (*TransactionReceipt, error) {
	txInfo, err := s.providers.Bitcoin.GetTransaction(ctx, txID, isTest)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get BTC transaction")
	}

	networkFee, err := nativeCoin.MakeAmount(strconv.FormatInt(txInfo.Fee, 10))
	if err != nil {
		return nil, errors.Wrap(err, "unable to calculate network fee")
	}

	// First input address is the sender
	sender := ""
	if len(txInfo.Inputs) > 0 {
		sender = txInfo.Inputs[0].Address
	}

	// First output address is typically the recipient
	recipient := ""
	if len(txInfo.Outputs) > 0 {
		recipient = txInfo.Outputs[0].Address
	}

	return &TransactionReceipt{
		Blockchain:    nativeCoin.Blockchain,
		IsTest:        isTest,
		Sender:        sender,
		Recipient:     recipient,
		Hash:          txID,
		NetworkFee:    networkFee,
		Success:       txInfo.Confirmed,
		Confirmations: txInfo.Confirmations,
		IsConfirmed:   txInfo.Confirmations >= requiredConfirmations,
	}, nil
}

// broadcastRawTransaction broadcasts a raw signed transaction to an EVM-compatible blockchain via RPC
func (s *Service) broadcastRawTransaction(ctx context.Context, rpc *ethclient.Client, rawTX string) (string, error) {
	// Decode the hex-encoded raw transaction
	tx := new(types.Transaction)
	txBytes := common.FromHex(rawTX)
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		return "", errors.Wrap(err, "unable to decode raw transaction")
	}

	// Broadcast the transaction
	if err := rpc.SendTransaction(ctx, tx); err != nil {
		return "", errors.Wrap(err, "unable to send transaction")
	}

	return tx.Hash().Hex(), nil
}

